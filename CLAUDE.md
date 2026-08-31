# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Kubernetes operator (Kubebuilder v3, Go) that manages PostgreSQL databases and roles on
**existing** Postgres instances (it does not provision Postgres itself — see Zalando's
postgres-operator for that). Three CRDs, group `postgres.brose.bike/v1`:

- `PgInstance` — connection info (host/port/user/password/database/sslMode) for an existing
  Postgres server. Each field is a `PgProperty` that can be a literal `value`, or sourced from a
  `ConfigMapKeyRef` / `SecretKeyRef` (see `api/v1/general.go`).
- `PgDatabase` — manages a database on a referenced `PgInstance` (default privileges, public
  privileges/schema handling, optional "wait for manual deletion" strategy).
- `PgUser` — manages a login role on a referenced `PgInstance`, generates a k8s Secret with
  credentials and per-database connection strings/URIs/JDBC strings, and manages database
  ownership/privileges for that role.

## Common commands

```bash
make test               # runs manifests, generate, fmt, vet, then go test ./... via envtest
make build               # generate, fmt, vet, then go build -o bin/manager main.go
make run                 # generate, fmt, vet, then go run ./main.go (runs against ~/.kube/config)
make manifests            # regenerate CRD/RBAC/webhook YAML into config/ via controller-gen
make generate             # regenerate zz_generated.deepcopy.go via controller-gen
make fmt                 # go fmt ./...
make vet                 # go vet ./...
make docker-build         # make test, then docker build
```

Run a single test package/test with envtest binaries wired up the same way `make test` does:

```bash
KUBEBUILDER_ASSETS="$(bin/setup-envtest use 1.25.0 --bin-dir bin -p path)" go test ./controllers/... -run TestAPIs
KUBEBUILDER_ASSETS="$(bin/setup-envtest use 1.25.0 --bin-dir bin -p path)" go test ./pkg/pgapi/... -v
```

Tool binaries (`controller-gen`, `kustomize`, `setup-envtest`) are downloaded on demand into
`bin/` by the corresponding Makefile targets (`make controller-gen`, `make envtest`, ...) — CI
caches this directory keyed on the Makefile hash.

CI (`.github/workflows/ci.yaml`) runs four independent jobs: build (`make generate && make
manifests && make build && make docker-build`), linter (`make fmt && make vet` +
`.github/scripts/lint_file_headers.py`, then asserts `git status` is clean — so generated/formatted
files must be committed), tests (`make test`), and documentation (`mkdocs build`).

Every `.go` file must start with the Apache-2.0 boilerplate header from
`hack/boilerplate.go.txt` ("Copyright 2023 Brose Fahrzeugteile SE & Co. KG, Bamberg."). The linter
job checks this with `.github/scripts/lint_file_headers.py`; run it with a `format` arg to
auto-insert missing headers: `python3 .github/scripts/lint_file_headers.py format`.

## Architecture

**Layering**: `controllers/` (reconcile loops) → `pkg/services/` (builds a `pgapi` client from a
`PgInstance` CR, resolving `PgProperty` values from literals/ConfigMaps/Secrets) → `pkg/pgapi/`
(raw SQL against the target Postgres instance via `database/sql` + `lib/pq`). `api/v1/` holds the
CRD types (spec/status/conditions) plus shared helpers. `pkg/security/` generates role passwords.
`pkg/tcpostgres/` and `pkg/brose_errors/` are small supporting packages (testcontainers-based
Postgres test helper, and typed domain errors respectively).

**Dependency injection via factories, not interfaces on structs directly**: each reconciler
(`PgInstanceReconciler`, `PgDatabaseReconciler`, `PgUserReconciler`) embeds a factory function
field (`PgDatabaseAPIFactory` / `PgRoleAPIFactory`, defined in `controllers/factories.go`) that is
assigned in `SetupWithManager` to `services.NewPgInstanceAPI`. Tests override this factory field
directly to inject fakes/mocks instead of hitting a real Postgres instance — this is the seam to
use when testing reconciler logic in isolation.

**pgapi interface composition**: `pkg/pgapi` defines narrow interfaces — `PgConnector`,
`PgRoleAPI`, `PgDatabaseAPI`, `PgSchemaAPI` — that compose into `PgInstanceAPI`
(`pgapi.NewPgInstanceAPI`). Controllers depend on the narrowest interface combination they need
(e.g. `PgDatabaseReconciler` only needs `PgDatabaseAPI` + `PgSchemaAPI`; `PgUserReconciler` needs
`PgRoleAPI` + `PgDatabaseAPI` + `PgConnector`, aliased as `PgRoleAPI` in `controllers/factories.go`).

**Reconcile pattern**: every reconciler fetches its resource, builds a `pgApi` client from the
referenced `PgInstance` (recording a `postgres.brose.bike/connected` status condition on
success/failure), branches on `DeletionTimestamp` to run finalizer logic
(`apiV1.DefaultFinalizerPgUser` / equivalent), otherwise reconciles create/update state and sets
status conditions (via `setCondition`/`removeCondition` in `controllers/utils.go`) before adding
the finalizer. Errors generally return `ctrl.Result{RequeueAfter: time.Minute}` to retry rather
than failing hard.

**Connections run against the instance's configured database, then `runIn`/`runInAs` (in
`pkg/pgapi/general.go`) open a fresh `sql.DB` per-target-database to run statements, optionally
temporarily granting the connecting role membership in a target role for the duration of the
call** — needed because Postgres requires a role to `runAs` in order to alter ownership/objects it
doesn't already own.

**Testing**: `controllers/`, `api/v1/`, and `pkg/services/` use Ginkgo/Gomega with `envtest`
(`suite_test.go` in each package spins up a real API server via `envtest.Environment`, loading
CRDs from `config/crd/bases`). `pkg/pgapi` tests exercise real SQL against Postgres via
`pkg/tcpostgres` (testcontainers), so Docker must be available to run them. `controllers/utils.go`
has test-only helpers (`deleteAllCustomResources` etc.) explicitly marked
"THIS METHOD SHOULD ONLY BE USED FOR TESTING".

## Documentation

User-facing docs live in `docs/` and are built with MkDocs (`mkdocs.yml`, `requirements.txt`),
published to https://brose-ebike.github.io/postgres-operator/. `README.md` has canonical example
YAML for all three CRDs — keep it in sync with `api/v1/*_types.go` when changing the spec shape.

# OEBC-2100 — Replace deprecated `gcr.io/kubebuilder/kube-rbac-proxy`

## Context

Jira ticket **OEBC-2100** asks to remove the `gcr.io/kubebuilder/kube-rbac-proxy` sidecar
(deprecated upstream, being pulled from GCR — see [issue #82](https://github.com/brose-ebike/postgres-operator/issues/82))
in favor of controller-runtime's built-in `WithAuthenticationAndAuthorization` metrics filter.
AC: (1) replace the kube-rbac-proxy, (2) PR opened against the public repo.

The fix requires a controller-runtime bump — the pinned v0.13.0 predates the
`pkg/metrics/filters`/`pkg/metrics/server` API entirely. While scoping bundle regeneration for
this change, the only `operator-sdk` available (v1.42.3) turned out to no longer support this
repo's `go.kubebuilder.io/v3` PROJECT layout at all. After discussing tradeoffs, the decision was
made to do a full Kubebuilder go/v3 → go/v4 migration alongside the kube-rbac-proxy fix, landing
as one PR in separable commits.

Repo facts that keep this tractable: single group/version (`postgres/v1`), 3 kinds
(PgInstance/PgDatabase/PgUser), no webhooks, no multi-group. Only `main.go` imports the
`controllers` package externally.

## Target versions

- `sigs.k8s.io/controller-runtime`: v0.13.0 → **v0.24.1**
- `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, `k8s.io/apiextensions-apiserver` → **v0.36.0**
- Go 1.26.0 required (local toolchain: go1.26.5)
- `Makefile` tool versions bumped to match: Kustomize v5.x, controller-tools v0.17+/v0.18+, envtest K8s version tracked to the new client-go line

## Phase 1 — Directory reorg to go/v4 layout

- `main.go` → `cmd/main.go`
- `controllers/*.go` → `internal/controller/*.go`, `package controllers` → `package controller`,
  update the sole import site (`cmd/main.go`)
- Fix `internal/controller/suite_test.go` CRD path: `../config/crd/bases` → `../../config/crd/bases`
- `api/v1/` and `pkg/*` stay where they are
- `Dockerfile`: COPY paths updated for `cmd/`/`internal/`, build command path, builder base image bumped to Go 1.26
- `PROJECT`: `layout: [go.kubebuilder.io/v3]` → `layout: [go.kubebuilder.io/v4]` (schema `version: "3"` unchanged)

## Phase 2 — Dependency bump

`go get` controller-runtime@v0.24.1 and the four k8s.io/* modules @v0.36.0, `go mod tidy`, fix
compile breaks (`ctrl.Options.Port` removed), bump Makefile tool versions, re-run
`make manifests && make generate`.

## Phase 3 — The actual OEBC-2100 fix

- `cmd/main.go`: `Metrics: metricsserver.Options{BindAddress, SecureServing: true,
  FilterProvider: filters.WithAuthenticationAndAuthorization}`, HTTP/2 disabled via TLSOpts,
  `--metrics-bind-address` default `:8443`
- `config/manager/manager.yaml`: metrics port 8080/metrics → 8443/https
- Delete `config/default/manager_auth_proxy_patch.yaml`; remove its reference in
  `config/default/kustomization.yaml`
- `config/rbac/`: rename `auth_proxy_role.yaml` → `metrics_auth_role.yaml`,
  `auth_proxy_role_binding.yaml` → `metrics_auth_role_binding.yaml`,
  `auth_proxy_service.yaml` → `metrics_service.yaml` (resource *names* unchanged); relabel
  `app.kubernetes.io/component: kube-rbac-proxy` → `metrics` in all 4 files; update
  `config/rbac/kustomization.yaml`
- No changes needed to `config/prometheus/monitor.yaml` or docs (verified via grep)

## Phase 4 — Bundle regeneration

Run `make bundle` for real now that PROJECT declares go/v4; diff and sanity-check no
kube-rbac-proxy references remain.

## Verification

`go build ./...`, `go vet ./...`, `make manifests && make generate` (clean git status),
`make test`, `make docker-build`, `kustomize build config/default` manual inspection,
`make bundle`. Manual/live-cluster verification (401 without token, 200 with a `metrics-reader`
token) called out in the PR description as reviewer-verified.

## Commit structure

1. Directory reorg to go/v4 layout
2. Dependency bump
3. kube-rbac-proxy → built-in metrics filter replacement
4. Bundle regeneration

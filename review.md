# Review: PR #118 — Replace kube-rbac-proxy with built-in metrics auth filter

**PR:** https://github.com/brose-ebike/postgres-operator/pull/118
**Branch:** `housekeeping/OEBC-2100/replace-kube-rbac-proxy` → `main`
**Ticket:** OEBC-2100
**Stats:** 37 files changed, 742 additions
**Reviewed locally — not posted to GitHub (public repo).**

## Summary

The PR removes the deprecated `gcr.io/kubebuilder/kube-rbac-proxy` sidecar in favor of
controller-runtime's built-in `WithAuthenticationAndAuthorization` metrics filter. To get there it
also had to migrate the project from Kubebuilder go/v3 to go/v4 layout and bump
`controller-runtime` v0.13.0 → v0.24.1 (k8s.io/* → v0.36.0), which is why the diff is much larger
than the stated goal. The core RBAC-proxy-removal work looks solid and consistent (manifests, RBAC
role wiring, and bundle CSV all agree with each other). The issues below are mostly small
regressions or rough edges introduced/carried by the surrounding migration, not with the central
idea of the change.

## Findings

### 1. Silent controller failure: `PgDatabaseReconciler` setup error doesn't exit — `cmd/main.go:112-116`

```go
if err = (&controller.PgDatabaseReconciler{
    Client: mgr.GetClient(),
    Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "PgDatabase")
}
```

Unlike the `PgInstanceReconciler` block directly above and the `PgUserReconciler` block directly
below — both of which call `os.Exit(1)` on error — this block only logs. If
`PgDatabaseReconciler.SetupWithManager` ever fails (e.g. an indexer/scheme registration error), the
manager keeps starting, passes health/readiness checks, and looks healthy — but `PgDatabase` CRs
are silently never reconciled, with no alert or crash to signal it.

This bug pre-dates the PR (same asymmetry existed in the old `main.go`), but the PR touches this
exact function while renaming `controllers` → `controller`, and now carries it forward sitting
directly between two blocks that do it correctly — an easy one-line fix (`os.Exit(1)`) while
already in this code.

**Severity:** Medium (pre-existing, but PR is touching this exact code and it's a one-line fix)

### 2. Dropped `nodeAffinity` scheduling constraint — `config/default/manager_auth_proxy_patch.yaml` (deleted), `config/manager/manager.yaml`

The PR deletes `config/default/manager_auth_proxy_patch.yaml` wholesale to remove the
kube-rbac-proxy sidecar it injected. That file, however, also carried a `nodeAffinity` constraint
that isn't defined anywhere else:

```yaml
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
          - key: kubernetes.io/arch
            operator: In
            values: [amd64, arm64, ppc64le, s390x]
          - key: kubernetes.io/os
            operator: In
            values: [linux]
```

The go/v4 scaffold's replacement location for this (`config/manager/manager.yaml:39-51`) is left as
a commented-out `TODO(user)` block, i.e. genuinely absent, not just relocated. On a cluster with
mixed-arch or Windows node pools, the controller-manager pod can now be scheduled onto a node the
distroless linux image can't run on — previously prevented by this rule. Not mentioned in the PR
description or any commit message, suggesting it's an unintended side effect of deleting the patch
file rather than a deliberate removal.

**Severity:** Medium — silent regression, easy to miss since it's a deletion-by-omission

### 3. Floating Docker base image tag vs. pinned Go toolchain — `Dockerfile:2` vs `go.mod:5`

```dockerfile
FROM golang:1.26 as builder   # floating tag, tracks latest 1.26.x
```
```
toolchain go1.26.7            # go.mod, exact pin
```

If the `golang:1.26` Docker Hub image's patch version ever lags behind `go1.26.7` (e.g. right after
a `go.mod` toolchain bump, before the image catches up), `GOTOOLCHAIN=auto` (Go's default) forces
`go build`/`go mod download` inside the `RUN` step to fetch `go1.26.7` over the network mid-build.
In a network-restricted or air-gapped CI/registry build environment this fails outright; even when
it succeeds, it's a non-reproducible build step depending on Docker Hub's update cadence.

Notably, the PR author flags in the description that `make docker-build`/image build could **not**
be verified in their sandbox (no `docker` CLI) — so this path is genuinely untested by this PR.

**Severity:** Low/Medium — real risk pattern, unverified, worth pinning the exact patch tag
(`golang:1.26.7`) or confirming CI has network access for toolchain fetches.

### 4. `make release`'s per-arch cross-compilation silently no-ops — `Makefile:265-269`

```makefile
for arch in "amd64" "arm" "arm64" ; \
do \
    GOOS=linux ; \
    GOARCH=$${arch} ; \
    go build -o dist/pgcontroller_$${arch} ./cmd ; \
done
```

`GOOS=linux;` and `GOARCH=$${arch};` are plain shell variable assignments without `export`, so they
do not propagate to the `go build` child process. Verified directly:
`bash -c 'GOOS=linux; GOARCH=arm64; env | grep -E "^GOOS|^GOARCH"'` → empty output. As a result,
`dist/pgcontroller_amd64`, `dist/pgcontroller_arm`, and `dist/pgcontroller_arm64` are all actually
built for the host machine's own OS/arch — e.g. all three would be `darwin/arm64` binaries on a Mac,
or all `linux/amd64` in typical CI — silently mislabeled release artifacts.

Pre-existing bug (only the `./cmd` argument was added by this PR), but this exact line is directly
touched by the PR's `main.go` → `cmd/main.go` migration and isn't fixed or re-tested here.

**Severity:** Low (release-tooling only, pre-existing) — flagging because the PR is already editing
this line.

### 5. `ENVTEST_K8S_VERSION` now 3 minor versions behind the pinned client libraries — `Makefile:52`

```
ENVTEST_K8S_VERSION = 1.33.0
```
```go
k8s.io/api           v0.36.0
k8s.io/apimachinery  v0.36.0
k8s.io/client-go     v0.36.0
```

`client-go` is generally validated against API servers within ±1 minor version of its own version.
Here envtest downloads a 1.33 kube-apiserver/etcd while the client libraries are generated against
Kubernetes ~1.36 — a 3-minor gap. This risks the envtest-backed suites (`internal/controller`)
passing against apiserver defaulting/validation behavior that differs from what a real 1.36-era
cluster would do, silently invalidating some test coverage.

**Severity:** Low — no immediate failure expected, but a coverage/fidelity gap worth tightening
(e.g. bump `ENVTEST_K8S_VERSION` to a 1.35/1.36 release).

### 6. RBAC file renames don't match retained resource names — `config/rbac/metrics_auth_role*.yaml`

Files were renamed as part of the rbac-proxy removal:

| old | new |
|---|---|
| `auth_proxy_role.yaml` | `metrics_auth_role.yaml` |
| `auth_proxy_role_binding.yaml` | `metrics_auth_role_binding.yaml` |
| `auth_proxy_service.yaml` | `metrics_service.yaml` |

...but the underlying Kubernetes resource names/labels inside them were left unchanged:
`ClusterRole.metadata.name: proxy-role`, `ClusterRoleBinding.metadata.name: proxy-rolebinding`, and
`app.kubernetes.io/instance: proxy-role` / `proxy-rolebinding` labels. `auth_proxy_client_clusterrole.yaml`
keeps its old filename entirely (only modified, not renamed).

This isn't a functional bug — everything is internally consistent and matches the bundle CSV — but
it leaves a permanent mismatch between filename (`metrics_*`) and live resource identity
(`proxy-role`), which will confuse the next person grepping the cluster or repo for leftover
"proxy" naming from the kube-rbac-proxy removal.

**Severity:** Nit — cosmetic/naming consistency only.

## Not a bug (checked and ruled out)

- `config/rbac/role.yaml` permission consolidation — verbs preserved across `pgdatabases`/`pginstances`/`pgusers`.
- Bundle CSV kube-rbac-proxy sidecar removal — fully consistent with `manager.yaml`.
- `config/manager/manager.yaml` container port (`8443`/`https`) matches `metrics_service.yaml`'s `targetPort` and the metrics server's `SecureServing` default.
- `TLSOpts`/`SecureServing` self-signed cert defaulting — `CertDir` auto-defaults correctly (verified via `go doc`).
- Package rename `controllers` → `controller` — all references consistently updated.
- CRD schema diffs — all attributable to `controller-gen` v0.21.0 regeneration (e.g. `default: ""` on `LocalObjectReference.Name` is upstream `apimachinery` behavior, not a hand edit).

## Manual verification still needed (per PR author, not checkable in this environment either)

- `make docker-build` / actual image build (relevant to finding #3 above).
- Deploy to a real cluster and confirm: `curl -k https://<pod>:8443/metrics` returns `401` with no
  `Authorization` header and `200` with a bearer token for a principal holding the `metrics-reader`
  ClusterRole.

## Recommendation

Solid migration overall — the core kube-rbac-proxy removal is correct and well-verified by the
author. Before merging, I'd fix #1 (one-line `os.Exit(1)`) and #2 (restore the nodeAffinity, since
it's a silent scheduling regression) as they're cheap and this PR is the one that introduced/exposed
them. #3–#6 are reasonable to fix now (they're small) or track as fast-follow tickets if the team
wants to keep this PR scoped to the RBAC-proxy replacement.

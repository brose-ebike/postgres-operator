# Kustomize

`config/default` in this repository is a standard Kustomize base that installs the CRDs, RBAC and
the controller `Deployment` into a `postgres-operator` namespace (resources are named with a
`postgres-operator-` prefix). It's the same base the [static manifest](./simple.md) is built from
(`kustomize build config/default`), so using it directly makes sense if you want to customize the
deployment (e.g. resource limits, replicas, image tag) before applying it, or reference the
operator as a remote base from your own GitOps repository.

## Referencing it as a remote base

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - https://github.com/brose-ebike/postgres-operator/config/default?ref=v0.0.6
```

Pin `ref` to a released tag rather than a branch, so upgrades are explicit.

## Building it locally

```bash
git clone https://github.com/brose-ebike/postgres-operator.git
cd postgres-operator
kustomize build config/default | kubectl apply -f -
```

If you're working from a checkout of this repository (e.g. for local development), the Makefile
wraps the same commands:

```bash
make install   # applies just the CRDs
make deploy    # applies CRDs, RBAC and the controller Deployment
make undeploy  # removes what `make deploy` created
```

`make deploy`/`make undeploy` accept an `IMG=` override to point at a custom controller image,
e.g. `make deploy IMG=ghcr.io/brose-ebike/postgres-operator:v0.0.6`.

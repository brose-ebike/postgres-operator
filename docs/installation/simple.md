# Kubectl apply

The simplest way to install the operator is to apply the static manifest published with each
GitHub release. It creates the `postgres-operator` namespace, the three CRDs
(`PgInstance`, `PgDatabase`, `PgUser`), RBAC, and the controller `Deployment`.

```bash
kubectl apply -f https://github.com/brose-ebike/postgres-operator/releases/latest/download/install.yaml
```

To install a specific version instead of the latest release, replace `latest` with a release tag:

```bash
kubectl apply -f https://github.com/brose-ebike/postgres-operator/releases/download/v0.0.6/install.yaml
```

Once the controller Pod is running, continue with [Usage](../usage/index.md) to create your
first `PgInstance`.

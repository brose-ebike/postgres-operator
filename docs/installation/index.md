# Installation
Learn about the various ways you can install the postgres-operator and how to choose between them.

## Default static install

> You want to get started quickly with a single command and don't need to customize the
> deployment.

The default static configuration can be installed as follows:

```bash
kubectl apply -f https://github.com/brose-ebike/postgres-operator/releases/latest/download/install.yaml
```

More information on this install method [can be found here](./simple.md).

## Kustomize

> You want to customize the deployment (e.g. resource limits, replica count, namespace) before
> applying it, or manage the installation as part of a larger Kustomize-based GitOps setup.

The repository itself is a valid Kustomize base. See [Kustomize](./kustomize.md) for details.

## Helm

> You manage your cluster's workloads with Helm.

A Helm chart is not published for this operator yet. See [Helm](./helm.md) for the current
status and alternatives.

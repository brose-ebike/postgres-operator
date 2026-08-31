# Helm

A Helm chart is not published for this operator yet — there is no chart in this repository or in
any Helm repository. If you manage your cluster with Helm, use one of these instead:

* Apply the [static manifest](./simple.md) alongside your Helm-managed resources.
* Wrap the [Kustomize base](./kustomize.md) as a Helm post-renderer, or convert `config/default`
  into a chart yourself.

If you'd like an official Helm chart, [open an issue](https://github.com/brose-ebike/postgres-operator/issues)
or contribute one via a [Pull Request](https://github.com/brose-ebike/postgres-operator/pulls) —
see [Contribution](../contribution.md).

# Installation
Learn about the various ways you can install postgres-controller and how to choose between them.

Below you will find details on various scenarios we aim to support and that are
compatible with the documentation on this website. Furthermore, the most applicable
install methods are listed below for each of the situations.

## Default static install

> You don't require any tweaking of the cert-manager install parameters.

The default static configuration can be installed as follows:

```bash
kubectl apply -f https://github.com/brose-ebike/postgres-controller/releases/download/latest/install.yaml
```

More information on this install method [can be found here](./simple.md).

## Getting started
> You quickly want to learn how to use postgres-controller and what it can be used for.

<!-- TODO -->

## Continuous deployment

> You know how to configure your postgres-controller setup and want to automate this.

<!-- TODO: Describe HELM and kustomize installation -->
This templated cert-manager manifest can be piped into your preferred deployment tool.

In case you are using Helm for automation, cert-manager [supports installing using Helm](./helm.md).
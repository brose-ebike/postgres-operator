# Contribution

## Contributors
The list of all [Contributors](https://github.com/brose-ebike/postgres-operator/graphs/contributors) can be found on [Github](https://github.com/brose-ebike/postgres-operator/graphs/contributors).

## General

* Star the project on [Github](https://github.com/brose-ebike/postgres-operator) and help spread the word :)
* [Post an issue](https://github.com/brose-ebike/postgres-operator/issues) if you find any bugs
* Contribute improvements or fixes using a [Pull Request](https://github.com/brose-ebike/postgres-operator/pulls). 
  If you're going to contribute, thank you! Please just be sure to:
  * discuss with the authors on an issue ticket prior to doing anything big.
  * follow the style, naming and structure conventions of the rest of the project.
  * make commits atomic and easy to merge.
  * verify all tests are passing. Build the project with `make build` and `make tests` run to do this.

## How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster 

## Development Tools

* The Project was created with the [Operator SDK](https://sdk.operatorframework.io/)
  * The create resources use `operator-sdk create api --group=postgres --version=v1 --kind=Pg<ResourceName>`
* To build, test and run the project use [make](https://www.gnu.org/software/make/)
* The [go compiler](https://go.dev/) is needed to build the controller
* The following tools can be installed using make:
  * To install `kustomize` run `make kustomize`
  * To install `controller-gen` run `make controller-gen`
  * To install `envtest` run `make envtest`
  * To install `opm` run `make opm`


## Test It Out

### Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install the CRDs into the cluster:

```sh
make install
```

2. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

3. Build and push your image to the location specified by `IMG`:
	
```sh
make docker-build docker-push IMG=<some-registry>/postgres-operator:tag
```
	
4. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/postgres-operator:tag
```

### Running locally

Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)
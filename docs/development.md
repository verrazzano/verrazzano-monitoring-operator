# Development guide

This page provides information for developers who want to understand or contribute to the VMO.

## Building the operator

### Requirements

The following is required to build the operator:
* Golang 1.13 or higher
* Docker

Run the following command from $GOPATH/src/github.com/verrazzano/verrazzano-monitoring-operator:

```
make go-install
```

#### Controller Generators

The Controller makes use of the generators in k8s.io/code-generator to generate a typed client, informers, listers and deep-copy functions.

The `./hack/update-codegen.sh` script is included in the `go-install` target. It will re-generate the following files and directories:
- pkg/apis/vmcontroller/v1/zz_generated.deepcopy.go
- pkg/client

## Running the VMO

### Requirements

The following is required to run the VMO:
* Kubernetes v1.13.x and up

### Install the CRDs

Regardless of which of the following options you use to run the VMO, you'll need to first manually apply
the VMO CRDs.  This is a one-time step:

```
# Install VMO CRDs
kubectl apply -f k8s/crds/verrazzano-monitoring-operator-crds.yaml  --validate=false
```

### Running the VMO as an out-of-cluster process

While developing the VMO itself, it's usually most efficient to run it as an out-of-cluster
process, pointing to your Kubernetes cluster.  To do this:

```
export KUBECONFIG=<your_kubeconfig>

# Run the operator as a local Go process:
make go-run
```

### Running the operator as an in-cluster pod

The official way to run a Kubernetes operator is to run it as a pod within Kubernetes itself.  To build this
Docker image, assigning it a temporary tag based on the current timestamp:

```
make build
```

If your `$KUBECONFIG` points to a remote cluster, you'll have to push this image to a real Docker registry:

```
docker login --username agent <docker repo>
# Password is a secret!
make push
```

Now, replace the VMO Docker image in `k8s/manifests/verrazzano-monitoring-operator.yaml` with the temporary image built
above, then run: 

```
kubectl apply -f k8s/manifests/verrazzano-monitoring-operator.yaml
```

## Running unit and integration tests

To perform static analysis:

```
make golangci-lint
```
To run unit tests:

```
make unit-test
```

To perform both static analysis and run unit test in one go:

```
make check
```

To run integration tests against a real Kubernetes cluster, first start the VMO using either the
out-of-cluster or in-cluster techniques mentioned above, then:

Testing against a cluster on your laptop

```
make integ-test
```

Testing against a remote cluster (with worker nodes with public IPs):
```
make integ-test K8S_EXTERNAL_IP=<ip_of_a_worker_node>
```

## Other Development Notes

### Making changes to the operator-generated Kubernetes objects

As explained elsewhere, for each VMI, the VMO creates/updates a set of Deployments/Services/ConfigMaps/etc.
As we evolve the operator code, the contents of these Kubernetes objects change.  When the operator processes a VMI,
it evaluates each Kubernetes object associated with the instance, diffing its desired state against its expected
state.  If the states are different, it calls the Kubernetes API to apply the desired state.  Some special diffing logic
has been added to the VMO to account for the fact that an object's live state, retrieved from the k8s API,
is always populated with many runtime-generated default values: we simply disregard the diff comparison for any element
that is 'empty' in the desired object state.  For our purposes, an 'empty' element is defined as either:
- Missing completely from the desired state
- Set to nil or the equivalent of a Golang nil value ("", 0, etc) in the desired state
- A map/list that is set to an empty map/list in the desired state

In almost all situations, the above diffing logic will be transparent to VMO developers, and changes to the
operator that modify Deployments/Services/ConfigMaps/etc will simply take effect when the operator is deployed
to a live system.  The only limitation to the above logic is that if we ever _truly wanted to_ (for some reason, although no
good example comes to mind) set an element to an empty value, our code doesn't allow it.  In this unlikely/rare case, it would
be necessary to do that with a one-off script.

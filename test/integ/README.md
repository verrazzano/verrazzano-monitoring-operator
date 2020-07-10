# Integration Testing

The integration tests are end-to-end and designed to test real user scenarios against a live Kubernetes cluster.

## Build and run integration tests

### Requirements

The following is required to build and run the integration tests:

*	Golang 1.9 or higher
*	Kubernetes v1.9.x or higher (e.g. OKE/OCE, [terraform-kubernetes-installer](https://github.com/oracle/terraform-kubernetes-installer), Docker for Mac [with Kubernetes](https://docs.docker.com/docker-for-mac/#kubernetes))


## Run integration tests

There are a variety of ways to run and configure the integration tests including from the top-level `Makefile` and using `go test`.

### Configure integration tests

The following parameters can be used to configure the tests:

| Test parameter | Description | Makefile variable | Default value |
| -------- | -------- | -------- | -------- |
| kubeconfig   | path to kubeconfig   | KUBECONFIG   | ""   |
| externalIp   | external IP over which to access deployments   | INTEG_K8S_EXTERNAL_IP   | localhost   |
| namespace   | integration test namespace to use   | K8S_NAMESPACE   | default   |
| skipteardown   | skips tearing down VMI instances and artifacts created by the tests (useful to save test run)   | INTEG_SKIP_TEARDOWN   | false  |
| runid   | optional string that will be used to uniquely identify this test run   | INTEG_RUN_ID  | generated  |
| phase   | optional phase to test (before|after) | none   | ""  |

### Run all integration from the top-level Makefile
```
make integ-test KUBECONFIG=${KUBECONFIG} K8S_EXTERNAL_IP=${INTEG_K8S_EXTERNAL_IP}
```

### Run all integration tests using go test
```
go test -v ./test/integ/ --kubeconfig=$KUBECONFIG --externalip=$INTEG_K8S_EXTERNAL_IP --namespace=default
```

### Run a specific integration tests using go test
```
go test -v ./test/integ/ -run SimpleVMI --kubeconfig=$KUBECONFIG --externalip=$INTEG_K8S_EXTERNAL_IP --namespace=integ-test
```

## Developing new tests

### Re-compile the test framework and utils

```
go install ./test/integ/framework/ ./test/integ/client/ ./test/integ/util/
```

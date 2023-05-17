# Copyright (C) 2020, 2023, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
OPERATOR_NAME:=verrazzano-monitoring-operator
ESWAIT_NAME:=verrazzano-monitoring-instance-eswait

DOCKER_IMAGE_NAME_OPERATOR ?= ${OPERATOR_NAME}-dev
DOCKERFILE_OPERATOR = docker-images/${OPERATOR_NAME}/Dockerfile

DOCKER_IMAGE_NAME_ESWAIT ?= ${ESWAIT_NAME}-dev
DOCKERFILE_ESWAIT = docker-images/${ESWAIT_NAME}/Dockerfile

DOCKER_IMAGE_TAG ?= local-$(shell git rev-parse --short HEAD)

CREATE_LATEST_TAG=0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif


ifeq ($(MAKECMDGOALS),$(filter $(MAKECMDGOALS),push push-tag push-eswait push-tag-eswait))
    ifndef DOCKER_REPO
        $(error DOCKER_REPO must be defined as the name of the docker repository where image will be pushed)
    endif
    ifndef DOCKER_NAMESPACE
        $(error DOCKER_NAMESPACE must be defined as the name of the docker namespace where image will be pushed)
    endif
    DOCKER_IMAGE_FULLNAME_OPERATOR = ${DOCKER_REPO}/${DOCKER_NAMESPACE}/${DOCKER_IMAGE_NAME_OPERATOR}
    DOCKER_IMAGE_FULLNAME_ESWAIT = ${DOCKER_REPO}/${DOCKER_NAMESPACE}/${DOCKER_IMAGE_NAME_ESWAIT}
endif

ifdef INTEG_RUN_ID
   RUN_ID_OPT=--runid=${INTEG_RUN_ID}
endif

ifdef INTEG_INGRESS
    INGRESS_OPT="--ingress"
endif

DOCKER_NAMESPACE ?= verrazzano
DOCKER_REPO ?= ghcr.io
BIN_NAME:=${OPERATOR_NAME}
K8S_EXTERNAL_IP:=localhost
K8S_NAMESPACE:=verrazzano-system
WATCH_NAMESPACE:=
WATCH_VMI:=
EXTRA_PARAMS=
ENV_NAME=verrazzano-monitoring-operator
INTEG_SKIP_TEARDOWN:=false
INTEG_PHASE:=""
INTEG_RUN_REGEX=Test
INGRESS_CONTROLLER_SVC_NAME:=ingress-controller
GO ?= go
HELM_CHART_NAME ?= verrazzano-monitoring-operator
CONTROLLER_GEN_VERSION ?= v0.8.0
CRD_OPTIONS ?= "crd:crdVersions=v1"
CRD_PATH:=./k8s/crds
CRD_FILE:=./k8s/crds/verrazzano.io_verrazzanomonitoringinstances.yaml
.PHONY: all
all: build

BUILDVERSION=$(shell grep verrazzano-development-version .verrazzano-development-version | cut -d= -f 2)
BUILDDATE=$(shell date +"%Y-%m-%dT%H:%M:%SZ")

.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=verrazzano-monitoring-operator-cluster-role webhook paths="./..." output:crd:artifacts:config=$(CRD_PATH)
	./hack/add_header.sh $(CRD_FILE)

.PHONY: generate
generate:
	./hack/update-codegen.sh

.PHONY: controller-gen-install
controller-gen-install:
	$(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@${CONTROLLER_GEN_VERSION}

.PHONY: controller-gen
controller-gen:
ifeq (, $(shell which controller-gen))
	$(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@${CONTROLLER_GEN_VERSION}
	$(eval CONTROLLER_GEN=$(GOBIN)/controller-gen)
else
	$(eval CONTROLLER_GEN=$(shell which controller-gen))
endif
	@{ \
	set -eu; \
	ACTUAL_CONTROLLER_GEN_VERSION=$$(${CONTROLLER_GEN} --version | awk '{print $$2}') ; \
	if [ "$${ACTUAL_CONTROLLER_GEN_VERSION}" != "${CONTROLLER_GEN_VERSION}" ] ; then \
		echo  "Bad controller-gen version $${ACTUAL_CONTROLLER_GEN_VERSION}, please install ${CONTROLLER_GEN_VERSION}" ; \
		exit 1; \
	fi ; \
	}

#
# Go build related tasks
#
.PHONY: go-install
go-install:
	GO111MODULE=on $(GO) install ./cmd/...

.PHONY: go-run
go-run: go-install
	GO111MODULE=on $(GO) run cmd/verrazzano-monitoring-ctrl/main.go --kubeconfig=${KUBECONFIG} --zap-log-level=debug --namespace=${K8S_NAMESPACE} --watchNamespace=${WATCH_NAMESPACE} --watchVmi=${WATCH_VMI} ${EXTRA_PARAMS}

.PHONY: go-vendor
go-vendor:
	glide install -v

#
# Docker-related tasks and functions
#

.PHONY: build
build:
	docker build --pull --no-cache \
		--build-arg BUILDVERSION=${BUILDVERSION} \
		--build-arg BUILDDATE=${BUILDDATE} \
		--build-arg EXTLDFLAGS="-s -w" \
		-t ${DOCKER_IMAGE_NAME_OPERATOR}:${DOCKER_IMAGE_TAG} \
		-f ${DOCKERFILE_OPERATOR} \
		.

.PHONY: build-debug
build-debug:
	docker build --pull --no-cache \
		--build-arg BUILDVERSION=${BUILDVERSION} \
		--build-arg BUILDDATE=${BUILDDATE} \
		--build-arg EXTLDFLAGS="" \
		-t ${DOCKER_IMAGE_NAME_OPERATOR}:${DOCKER_IMAGE_TAG} \
		-f ${DOCKERFILE_OPERATOR} \
		.

.PHONY: buildhook
buildhook:
	rm -rf /usr/bin/verrazzano-backup-hook
	go build \
           -ldflags '-extldflags "-static"' \
           -ldflags "-X main.buildVersion=${BUILDVERSION} -X main.buildDate=${BUILDDATE}" \
           -o /usr/bin/verrazzano-backup-hook ./verrazzano-backup-hook

.PHONY: push-debug
push-debug: build-debug push-common

.PHONY: push
push: build push-common

.PHONY: push-common
push-common:
	docker tag ${DOCKER_IMAGE_NAME_OPERATOR}:${DOCKER_IMAGE_TAG} ${DOCKER_IMAGE_FULLNAME_OPERATOR}:${DOCKER_IMAGE_TAG}
	docker push ${DOCKER_IMAGE_FULLNAME_OPERATOR}:${DOCKER_IMAGE_TAG}

	if [ "${CREATE_LATEST_TAG}" == "1" ]; then \
		docker tag ${DOCKER_IMAGE_NAME_OPERATOR}:${DOCKER_IMAGE_TAG} ${DOCKER_IMAGE_FULLNAME_OPERATOR}:latest; \
		docker push ${DOCKER_IMAGE_FULLNAME_OPERATOR}:latest; \
	fi

.PHONY: push-tag
push-tag:
	docker pull ${DOCKER_IMAGE_FULLNAME_OPERATOR}:${DOCKER_IMAGE_TAG}
	docker tag ${DOCKER_IMAGE_FULLNAME_OPERATOR}:${DOCKER_IMAGE_TAG} ${DOCKER_IMAGE_FULLNAME_OPERATOR}:${TAG_NAME}
	docker push ${DOCKER_IMAGE_FULLNAME_OPERATOR}:${TAG_NAME}


#
# Tests-related tasks
#
.PHONY: unit-test
unit-test: go-install
	GO111MODULE=on $(GO) test -v ./pkg/... ./cmd/... ./verrazzano-backup-hook/

#
# Run all checks, convenient as a sanity-check before committing/pushing
#
.PHONY: check
check: golangci-lint unit-test

.PHONY: coverage
coverage:
	./build/scripts/coverage.sh html

.PHONY: integ-test
integ-test: go-install
	GO111MODULE=on $(GO) get -u github.com/oracle/oci-go-sdk
	GO111MODULE=on $(GO) test -v ./test/integ/ -timeout 30m --kubeconfig=${KUBECONFIG} --externalip=${K8S_EXTERNAL_IP} --namespace=${K8S_NAMESPACE} --skipteardown=${INTEG_SKIP_TEARDOWN} --run=${INTEG_RUN_REGEX} --phase=${INTEG_PHASE} --ingressControllerSvcName=${INGRESS_CONTROLLER_SVC_NAME} ${INGRESS_OPT} ${RUN_ID_OPT}

# find or download and execute golangci-lint
.PHONY: golangci-lint
golangci-lint:
ifeq (, $(shell command -v golangci-lint))
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.49.0
endif
	golangci-lint run

# check if the repo is clean after running generate
.PHONY: check-repo-clean
check-repo-clean: generate manifests
	./build/scripts/check_if_clean_after_generate.sh
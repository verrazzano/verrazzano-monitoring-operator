# Copyright (C) 2020, 2021, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
OPERATOR_NAME:=verrazzano-monitoring-operator
ESWAIT_NAME:=verrazzano-monitoring-instance-eswait

DOCKER_IMAGE_NAME_OPERATOR ?= ${OPERATOR_NAME}-dev
DOCKERFILE_OPERATOR = docker-images/${OPERATOR_NAME}/Dockerfile

DOCKER_IMAGE_NAME_ESWAIT ?= ${ESWAIT_NAME}-dev
DOCKERFILE_ESWAIT = docker-images/${ESWAIT_NAME}/Dockerfile

DOCKER_IMAGE_TAG ?= local-$(shell git rev-parse --short HEAD)

CREATE_LATEST_TAG=0

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
DIST_DIR:=dist
BIN_DIR:=${DIST_DIR}/bin
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

.PHONY: all
all: build

BUILDVERSION=`git describe --tags`
BUILDDATE=`date +%FT%T%z`

.PHONY: code-gen
code-gen:
	git config core.hooksPath hooks
	GO111MODULE=on $(GO) mod vendor
	chmod +x vendor/k8s.io/code-generator/*.sh
	sh hack/update-codegen.sh

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

.PHONY: docker-clean
docker-clean:
	rm -rf ${DIST_DIR}

.PHONY: k8s-dist
k8s-dist: docker-clean
	echo ${DOCKER_IMAGE_TAG} ${JENKINS_URL} ${CI_COMMIT_TAG} ${CI_COMMIT_SHA}
	echo ${DOCKER_IMAGE_NAME_OPERATOR}
	mkdir -p ${DIST_DIR}
	cp -r docker-images/verrazzano-monitoring-operator/* ${DIST_DIR}
	cp -r k8s/manifests/verrazzano-monitoring-operator.yaml $(DIST_DIR)/verrazzano-monitoring-operator.yaml

	# Fill in Docker image and tag that's being tested
	sed -i.bak "s|${DOCKER_REPO}/${DOCKER_NAMESPACE}/verrazzano-monitoring-operator:latest|${DOCKER_REPO}/${DOCKER_NAMESPACE}/${DOCKER_IMAGE_NAME_OPERATOR}:$(DOCKER_IMAGE_TAG)|g" $(DIST_DIR)/verrazzano-monitoring-operator.yaml
	sed -i.bak "s/latest/$(DOCKER_IMAGE_TAG)/g" $(DIST_DIR)/verrazzano-monitoring-operator.yaml
	sed -i.bak "s/default/${K8S_NAMESPACE}/g" $(DIST_DIR)/verrazzano-monitoring-operator.yaml

	rm -rf $(DIST_DIR)/verrazzano-monitoring-operator*.bak
	mkdir -p ${BIN_DIR}

.PHONY: build
build: k8s-dist
	docker build --pull --no-cache \
		--build-arg BUILDVERSION=${BUILDVERSION} \
		--build-arg BUILDDATE=${BUILDDATE} \
		-t ${DOCKER_IMAGE_NAME_OPERATOR}:${DOCKER_IMAGE_TAG} \
		-f ${DOCKERFILE_OPERATOR} \
		.

.PHONY: push
push: build
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

.PHONY: build-eswait
build-eswait:
	docker build --pull --no-cache \
		--build-arg BUILDVERSION=${BUILDVERSION} \
		--build-arg BUILDDATE=${BUILDDATE} \
		-t ${DOCKER_IMAGE_NAME_ESWAIT}:${DOCKER_IMAGE_TAG} \
		-f ${DOCKERFILE_ESWAIT} \
		.

.PHONY: push-eswait
push-eswait: build-eswait
	docker tag ${DOCKER_IMAGE_NAME_ESWAIT}:${DOCKER_IMAGE_TAG} ${DOCKER_IMAGE_FULLNAME_ESWAIT}:${DOCKER_IMAGE_TAG}
	docker push ${DOCKER_IMAGE_FULLNAME_ESWAIT}:${DOCKER_IMAGE_TAG}

	if [ "${CREATE_LATEST_TAG}" == "1" ]; then \
		docker tag ${DOCKER_IMAGE_NAME_ESWAIT}:${DOCKER_IMAGE_TAG} ${DOCKER_IMAGE_FULLNAME_ESWAIT}:latest; \
		docker push ${DOCKER_IMAGE_FULLNAME_ESWAIT}:latest; \
	fi

.PHONY: push-tag-eswait
push-tag-eswait:
	docker pull ${DOCKER_IMAGE_FULLNAME_ESWAIT}:${DOCKER_IMAGE_TAG}
	docker tag ${DOCKER_IMAGE_FULLNAME_ESWAIT}:${DOCKER_IMAGE_TAG} ${DOCKER_IMAGE_FULLNAME_ESWAIT}:${TAG_NAME}
	docker push ${DOCKER_IMAGE_FULLNAME_ESWAIT}:${TAG_NAME}

#
# Tests-related tasks
#
.PHONY: unit-test
unit-test: go-install
	GO111MODULE=on $(GO) test -v ./pkg/... ./cmd/...

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
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.41.1
endif
	golangci-lint run

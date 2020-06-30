# Copyright (C) 2020, Oracle Corporation and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
NAME:=verrazzano-monitoring-operator

DOCKER_IMAGE_NAME ?= ${NAME}-dev
TAG=$(shell git rev-parse HEAD)
VERSION = ${TAG}

CREATE_LATEST_TAG=0

ifeq ($(MAKECMDGOALS),$(filter $(MAKECMDGOALS),push push-tag))
	ifndef DOCKER_REPO
		$(error DOCKER_REPO must be defined as the name of the docker repository where image will be pushed)
	endif
	ifndef DOCKER_NAMESPACE
		$(error DOCKER_NAMESPACE must be defined as the name of the docker namespace where image will be pushed)
	endif
	DOCKER_IMAGE_FULLNAME = ${DOCKER_REPO}/${DOCKER_NAMESPACE}/${DOCKER_IMAGE_NAME}
endif

ifdef INTEG_RUN_ID
    RUN_ID_OPT=--runid=${INTEG_RUN_ID}
endif

ifdef INTEG_INGRESS
    INGRESS_OPT="--ingress"
endif

DOCKER_NAMESPACE ?= verrazzano
DOCKER_REPO ?= container-registry.oracle.com
DOCKER_IMAGE_TAG ?= ${VERSION}
DIST_DIR:=dist
BIN_DIR:=${DIST_DIR}/bin
BIN_NAME:=${NAME}
K8S_EXTERNAL_IP:=localhost
K8S_NAMESPACE:=default
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

#
# Go build related tasks
#
.PHONY: go-install
go-install:
	git config core.hooksPath hooks
	GO111MODULE=on $(GO) mod vendor
	chmod +x vendor/k8s.io/code-generator/*.sh
	sh hack/update-codegen.sh
	GO111MODULE=on $(GO) install ./cmd/...

.PHONY: go-run
go-run: go-install
	GO111MODULE=on $(GO) run cmd/verrazzano-monitoring-ctrl/main.go --kubeconfig=${KUBECONFIG} --v=4 --namespace=${K8S_NAMESPACE} --watchNamespace=${WATCH_NAMESPACE} --watchVmi=${WATCH_VMI} ${EXTRA_PARAMS}

.PHONY: go-fmt
go-fmt:
	gofmt -s -e -d $(shell find . -name "*.go" | grep -v /vendor/)

.PHONY: go-vet
go-vet:
	echo go vet $(shell go list ./... | grep -v /vendor/)

.PHONY: go-vendor
go-vendor:
	glide install -v

#
# Docker-related tasks
#
.PHONY: docker-clean
docker-clean:
	rm -rf ${DIST_DIR}

.PHONY: k8s-dist
k8s-dist: docker-clean
	echo ${VERSION} ${JENKINS_URL} ${CI_COMMIT_TAG} ${CI_COMMIT_SHA}
	echo ${DOCKER_IMAGE_NAME}
	mkdir -p ${DIST_DIR}
	cp -r docker-images/verrazzano-monitoring-operator/* ${DIST_DIR}
	cp -r k8s/manifests/verrazzano-monitoring-operator.yaml $(DIST_DIR)/verrazzano-monitoring-operator.yaml

	# Fill in Docker image and tag that's being tested
	sed -i.bak "s|${DOCKER_REPO}/${DOCKER_NAMESPACE}/verrazzano-monitoring-operator:latest|${DOCKER_REPO}/${DOCKER_NAMESPACE}/${DOCKER_IMAGE_NAME}:$(DOCKER_IMAGE_TAG)|g" $(DIST_DIR)/verrazzano-monitoring-operator.yaml
	sed -i.bak "s/latest/$(DOCKER_IMAGE_TAG)/g" $(DIST_DIR)/verrazzano-monitoring-operator.yaml
	sed -i.bak "s/default/${K8S_NAMESPACE}/g" $(DIST_DIR)/verrazzano-monitoring-operator.yaml

	rm -rf $(DIST_DIR)/verrazzano-monitoring-operator*.bak
	mkdir -p ${BIN_DIR}

.PHONY: build
build: k8s-dist
	docker build --pull --no-cache \
		--build-arg BUILDVERSION=${BUILDVERSION} \
		--build-arg BUILDDATE=${BUILDDATE} \
		-t ${DOCKER_IMAGE_NAME}:${DOCKER_IMAGE_TAG} \
		-f ${DIST_DIR}/Dockerfile \
		.
.PHONY: push
push: build
	docker tag ${DOCKER_IMAGE_NAME}:${DOCKER_IMAGE_TAG} ${DOCKER_IMAGE_FULLNAME}:${DOCKER_IMAGE_TAG}
	docker push ${DOCKER_IMAGE_FULLNAME}:${DOCKER_IMAGE_TAG}

	if [ "${CREATE_LATEST_TAG}" == "1" ]; then \
		docker tag ${DOCKER_IMAGE_NAME}:${DOCKER_IMAGE_TAG} ${DOCKER_IMAGE_FULLNAME}:latest; \
		docker push ${DOCKER_IMAGE_FULLNAME}:latest; \
	fi

.PHONY: push-tag
push-tag:
	docker pull ${DOCKER_IMAGE_FULLNAME}:${DOCKER_IMAGE_TAG}
	docker tag ${DOCKER_IMAGE_FULLNAME}:${DOCKER_IMAGE_TAG} ${DOCKER_IMAGE_FULLNAME}:${TAG_NAME}
	docker push ${DOCKER_IMAGE_FULLNAME}:${TAG_NAME}

#
# Tests-related tasks
#
.PHONY: unit-test
unit-test: go-install
	GO111MODULE=on $(GO) test -v ./pkg/... ./cmd/...

.PHONY: thirdparty-check
thirdparty-check:
	./build/scripts/thirdparty_check.sh

.PHONY: coverage
coverage:
	./build/scripts/coverage.sh html

.PHONY: integ-test
integ-test: go-install
#	GO111MODULE=on $(GO) get -u github.com/oracle/oci-go-sdk
#	GO111MODULE=on $(GO) test -v ./test/integ/ -timeout 30m --kubeconfig=${KUBECONFIG} --externalip=${K8S_EXTERNAL_IP} --namespace=${K8S_NAMESPACE} --skipteardown=${INTEG_SKIP_TEARDOWN} --run=${INTEG_RUN_REGEX} --phase=${INTEG_PHASE} --ingressControllerSvcName=${INGRESS_CONTROLLER_SVC_NAME} ${INGRESS_OPT} ${RUN_ID_OPT}

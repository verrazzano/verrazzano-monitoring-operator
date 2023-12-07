# Copyright (C) 2020, 2023, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
ARG FINAL_IMAGE=ghcr.io/verrazzano/ol8-static:v0.0.1-20231102152128-e7afc807

FROM ghcr.io/oracle/oraclelinux:8-slim AS build_base
RUN microdnf update -y --setopt=install_weak_deps=0 --setopt=tsflags=nodocs && \
    microdnf install -y golang-1.20.10-1.module+el8.9.0+90068+08775a1f.x86_64

ARG BUILDVERSION
ARG BUILDDATE
ARG EXTLDFLAGS

# Need to use specific WORKDIR to match verrazzano-monitoring-operator's source packages
WORKDIR /root/go/src/github.com/verrazzano/verrazzano-monitoring-operator
ENV GOPATH /root/go
ENV CGO_ENABLED 0
COPY . .
RUN go build \
    -ldflags "$EXTLDFLAGS -extldflags -static -X 'main.buildVersion=$BUILDVERSION' -X 'main.buildDate=$BUILDDATE'" \
    -o /usr/bin/verrazzano-monitoring-operator ./cmd/verrazzano-monitoring-ctrl \
    && chmod 500 /usr/bin/verrazzano-monitoring-operator

RUN groupadd -r verrazzano-monitoring-operator && \
    useradd --no-log-init -r -g verrazzano-monitoring-operator -u 1000 verrazzano-monitoring-operator

FROM $FINAL_IMAGE AS final

RUN microdnf update -y \
    && microdnf clean all \
    && rm -rf /var/cache/yum/*

# copy users/groups added in build_base stage
COPY --from=build_base /etc/passwd /etc/passwd
COPY --from=build_base /etc/group /etc/group

COPY --from=build_base --chown=verrazzano-monitoring-operator:verrazzano-monitoring-operator /usr/bin/verrazzano-monitoring-operator /usr/local/bin/verrazzano-monitoring-operator
USER 1000

ENTRYPOINT ["/usr/local/bin/verrazzano-monitoring-operator"]
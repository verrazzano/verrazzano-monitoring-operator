# Copyright (C) 2020, 2022, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
ARG BASE_IMAGE
ARG BUILDVERSION
ARG BUILDDATE

FROM ghcr.io/oracle/oraclelinux:8-slim as opensearch-hook-builder

RUN microdnf update -y \
    && microdnf install -y oraclelinux-developer-release-el8 \
    && microdnf module enable go-toolset:ol8addon \
    && microdnf install go-toolset git \
    && go version

# Need to use specific WORKDIR to match verrazzano-monitoring-operator's source packages
WORKDIR /root/go/src/github.com/verrazzano/verrazzano-monitoring-operator
ENV GOPATH /root/go
ENV CGO_ENABLED 0
COPY . .
RUN go build \
    -ldflags '-extldflags "-static"' \
    -ldflags "-X main.buildVersion=${BUILDVERSION} -X main.buildDate=${BUILDDATE}" \
    -o /usr/bin/verrazzano-backup-hook ./verrazzano-backup-hook

FROM $BASE_IMAGE

COPY --from=opensearch-hook-builder /usr/bin/verrazzano-backup-hook /usr/bin/verrazzano-backup-hook

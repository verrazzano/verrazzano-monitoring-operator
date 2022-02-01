# Copyright (C) 2020, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
FROM ghcr.io/oracle/oraclelinux:8-slim AS build_base

RUN microdnf install -y dnf

RUN dnf install -y golang \
    && go version

ARG BUILDVERSION
ARG BUILDDATE

# Need to use specific WORKDIR to match verrazzano-monitoring-operator's source packages
WORKDIR /root/go/src/github.com/verrazzano/verrazzano-monitoring-operator
ENV GOPATH /root/go
ENV CGO_ENABLED 0
COPY . .
RUN go build -mod=mod \
    -ldflags '-extldflags "-static"' \
    -ldflags "-X main.buildVersion=${BUILDVERSION} -X main.buildDate=${BUILDDATE}" \
    -o /usr/bin/verrazzano-monitoring-operator ./cmd/verrazzano-monitoring-ctrl


FROM ghcr.io/oracle/oraclelinux:8-slim AS final

RUN microdnf install -y dnf

RUN dnf install -y openssl \
    && dnf clean all

COPY --from=build_base /usr/bin/verrazzano-monitoring-operator /usr/local/bin/verrazzano-monitoring-operator

WORKDIR /usr/local/bin/
RUN groupadd -r verrazzano-monitoring-operator && useradd --no-log-init -r -g verrazzano-monitoring-operator -u 1000 verrazzano-monitoring-operator
RUN chown 1000:verrazzano-monitoring-operator /usr/local/bin/verrazzano-monitoring-operator && chmod 500 /usr/local/bin/verrazzano-monitoring-operator
USER 1000

ENTRYPOINT ["/usr/local/bin/verrazzano-monitoring-operator"]

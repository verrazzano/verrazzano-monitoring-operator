# Copyright (C) 2020, 2022, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
FROM ghcr.io/oracle/oraclelinux:7-slim AS build_base

RUN yum update -y \
    && yum-config-manager --save --setopt=ol7_ociyum_config.skip_if_unavailable=true \
    && yum install -y oracle-golang-release-el7 \
    && yum-config-manager --disable ol7_developer_golang\* \
    && yum-config-manager --enable ol7_developer_golang117 &&
    && yum -y install golang \
    && yum clean all \
    && go version

ARG BUILDVERSION
ARG BUILDDATE

# Need to use specific WORKDIR to match verrazzano-monitoring-operator's source packages
WORKDIR /root/go/src/github.com/verrazzano/verrazzano-monitoring-operator
ENV GOPATH /root/go
ENV CGO_ENABLED 0
COPY . .
RUN go build \
    -ldflags '-extldflags "-static"' \
    -ldflags "-X main.buildVersion=${BUILDVERSION} -X main.buildDate=${BUILDDATE}" \
    -o /usr/bin/verrazzano-monitoring-operator ./cmd/verrazzano-monitoring-ctrl


FROM ghcr.io/oracle/oraclelinux:7-slim AS final

RUN yum update -y \
    && yum install -y openssl \
    && yum clean all \
    && rm -rf /var/cache/yum

COPY --from=build_base /usr/bin/verrazzano-monitoring-operator /usr/local/bin/verrazzano-monitoring-operator

WORKDIR /usr/local/bin/
RUN groupadd -r verrazzano-monitoring-operator && useradd --no-log-init -r -g verrazzano-monitoring-operator -u 1000 verrazzano-monitoring-operator
RUN chown 1000:verrazzano-monitoring-operator /usr/local/bin/verrazzano-monitoring-operator && chmod 500 /usr/local/bin/verrazzano-monitoring-operator
USER 1000

ENTRYPOINT ["/usr/local/bin/verrazzano-monitoring-operator"]

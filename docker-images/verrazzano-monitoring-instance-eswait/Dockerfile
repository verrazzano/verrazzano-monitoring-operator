# Copyright (C) 2020, 2022, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
FROM ghcr.io/oracle/oraclelinux:7-slim AS build_base

RUN yum update -y \
    && yum-config-manager --save --setopt=ol7_ociyum_config.skip_if_unavailable=true \
    && yum install -y oracle-golang-release-el7 \
    && yum install -y git gcc make golang-1.17.5-1.el7.x86_64 \
    && yum clean all \
    && go version

ARG BUILDVERSION
ARG BUILDDATE

WORKDIR /root/go/src/github.com/verrazzano/verrazzano-monitoring-operator

ENV GOPATH /root/go
ENV CGO_ENABLED 0
COPY ./eswait*.go ./

RUN go test ...
RUN go build \
    -ldflags '-extldflags "-static"' \
    -ldflags "-X main.buildVersion=${BUILDVERSION} -X main.buildDate=${BUILDDATE}" \
    -o /usr/bin/eswait eswait.go

FROM ghcr.io/oracle/oraclelinux:7-slim AS final

RUN yum update -y \
    && yum install -y ca-certificates curl openssl && yum clean all && rm -rf /var/cache/yum

COPY --from=build_base /usr/bin/eswait /usr/local/bin/eswait

RUN chown nobody:nobody /usr/local/bin/eswait && chmod 500 /usr/local/bin/eswait

USER nobody

ENTRYPOINT ["/usr/local/bin/eswait"]

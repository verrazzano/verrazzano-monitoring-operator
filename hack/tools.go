// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
//go:build tools
// +build tools

/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This package imports things required by build scripts, to force `go mod` to see them as dependencies
package tools

import (
	// Code generator
	_ "k8s.io/code-generator"

	// Fix for Go 1.20
	_ "github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1"
	_ "github.com/Azure/go-autorest v14.2.0+incompatible"
	_ "github.com/Azure/go-autorest/autorest/date v0.3.0"
	_ "github.com/Azure/go-autorest/logger v0.2.1"
	_ "github.com/Azure/go-autorest/tracing v0.6.0"
	_ "github.com/NYTimes/gziphandler v1.1.1"
	_ "github.com/dustin/go-humanize v1.0.0"
	_ "github.com/felixge/httpsnoop v1.0.1"
	_ "github.com/form3tech-oss/jwt-go v3.2.3+incompatible"
	_ "github.com/getkin/kin-openapi v0.76.0"
	_ "github.com/google/btree v1.0.1"
	_ "github.com/googleapis/gnostic v0.5.5"
	_ "github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7"
	_ "github.com/grpc-ecosystem/go-grpc-middleware v1.3.0"
	_ "github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0"
	_ "github.com/inconshreveable/mousetrap v1.0.0"
	_ "github.com/jonboulle/clockwork v0.2.2"
	_ "github.com/peterbourgon/diskv v2.0.1+incompatible"
	_ "github.com/sirupsen/logrus v1.8.1"
	_ "github.com/soheilhy/cmux v0.1.5"
	_ "github.com/tmc/grpc-websocket-proxy v0.0.0-20201229170055-e5319fda7802"
	_ "github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2"
	_ "go.etcd.io/bbolt v1.3.6"
	_ "go.opentelemetry.io/contrib v0.20.0"
	_ "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.20.0"
	_ "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.20.0"
	_ "go.opentelemetry.io/otel v0.20.0"
	_ "go.opentelemetry.io/otel/exporters/otlp v0.20.0"
	_ "go.opentelemetry.io/otel/metric v0.20.0"
	_ "go.opentelemetry.io/otel/sdk v0.20.0"
	_ "go.opentelemetry.io/otel/sdk/export/metric v0.20.0"
	_ "go.opentelemetry.io/otel/sdk/metric v0.20.0"
	_ "go.opentelemetry.io/otel/trace v0.20.0"
	_ "go.opentelemetry.io/proto/otlp v0.7.0"
	_ "gopkg.in/natefinch/lumberjack.v2 v2.0.0"
)

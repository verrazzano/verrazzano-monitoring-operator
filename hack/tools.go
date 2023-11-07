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
	_ "github.com/Azure/go-ansiterm"
	_ "github.com/Azure/go-autorest"
	_ "github.com/Azure/go-autorest/autorest/date"
	_ "github.com/Azure/go-autorest/logger"
	_ "github.com/Azure/go-autorest/tracing"
	_ "github.com/NYTimes/gziphandler"
	_ "github.com/dustin/go-humanize"
	_ "github.com/felixge/httpsnoop"
	_ "github.com/form3tech-oss/jwt-go"
	_ "github.com/google/btree"
	_ "github.com/googleapis/gnostic"
	_ "github.com/gregjones/httpcache"
	_ "github.com/grpc-ecosystem/go-grpc-middleware"
	_ "github.com/grpc-ecosystem/go-grpc-prometheus"
	_ "github.com/inconshreveable/mousetrap"
	_ "github.com/jonboulle/clockwork"
	_ "github.com/peterbourgon/diskv"
	_ "github.com/sirupsen/logrus"
	_ "github.com/soheilhy/cmux"
	_ "github.com/xiang90/probing"
	_ "go.etcd.io/bbolt"
	_ "go.opentelemetry.io/contrib"
	_ "go.opentelemetry.io/otel"
	_ "go.opentelemetry.io/otel/metric"
	_ "go.opentelemetry.io/otel/sdk"
	_ "go.opentelemetry.io/otel/trace"
	_ "gopkg.in/natefinch/lumberjack.v2"
)

// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package config

import (
	"os"
	"strings"
)

var reindexConfiguration = struct {
	namespacesKey         string
	defaultNamespaces     []string
	datastreamNameKey     string
	defaultDataStreamName string
}{
	"VERRAZZANO_NAMESPACES_ARRAY",
	[]string{
		"kube-system",
		"verrazzano-system",
		"istio-system",
		"keycloak",
		"metallb-system",
		"default",
		"cert-manager",
		"local-path-storage",
		"rancher-operator-system",
		"fleet-system",
		"ingress-nginx",
		"cattle-system",
		"verrazzano-install",
		"monitoring",
	},
	"VERRAZZANO_DATA_STREAM_NAME",
	"verrazzano-system",
}

func SystemNamespaces() []string {
	reindexValues := os.Getenv(reindexConfiguration.namespacesKey)
	if reindexValues == "" {
		return reindexConfiguration.defaultNamespaces
	}
	return strings.Split(reindexValues, ",")
}

func DataStreamName() string {
	dataStreamName := os.Getenv(reindexConfiguration.datastreamNameKey)
	if dataStreamName == "" {
		return reindexConfiguration.defaultDataStreamName
	}
	return dataStreamName
}

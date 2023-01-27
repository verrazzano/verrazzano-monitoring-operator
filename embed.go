// Copyright (c) 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

// Package verrazzanomonitoringoperator is used to reference the embedded OpenSearch ISM policy files in the binary.
package verrazzanomonitoringoperator

import (
	"embed"
)

//go:embed k8s/manifests/opensearch
var openSearchISMPolicyFS embed.FS

// GetEmbeddedISMPolicy returns the embedded openSearch ISM policies file system.
func GetEmbeddedISMPolicy() embed.FS {
	return openSearchISMPolicyFS
}

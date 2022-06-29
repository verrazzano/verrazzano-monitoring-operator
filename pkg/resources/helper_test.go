// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package resources

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	vmov1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
)

func createTestVMI() *vmov1.VerrazzanoMonitoringInstance {
	return &vmov1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system",
			Namespace: "test",
		},
	}
}

func TestGetOpenSearchDashboardsHTTPEndpoint(t *testing.T) {
	osdEndpoint := GetOpenSearchDashboardsHTTPEndpoint(createTestVMI())
	assert.Equal(t, "http://vmi-system-kibana.test.svc.cluster.local:5601", osdEndpoint)
}

func TestGetOpenSearchHTTPEndpoint(t *testing.T) {
	osEndpoint := GetOpenSearchHTTPEndpoint(createTestVMI())
	assert.Equal(t, "http://vmi-system-es-master-http.test.svc.cluster.local:9200", osEndpoint)
}

func TestConvertToRegexp(t *testing.T) {
	var tests = []struct {
		pattern string
		regexp  string
	}{
		{
			"verrazzano-*",
			"^verrazzano-.*$",
		},
		{
			"verrazzano-system",
			"^verrazzano-system$",
		},
		{
			"*",
			"^.*$",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("converting pattern '%s' to regexp", tt.pattern), func(t *testing.T) {
			r := ConvertToRegexp(tt.pattern)
			assert.Equal(t, tt.regexp, r)
		})
	}
}

func TestGetCompLabel(t *testing.T) {
	var tests = []struct {
		compName     string
		expectedName string
	}{
		{
			"es-master",
			"opensearch",
		},
		{
			"es-data",
			"opensearch",
		},
		{
			"es-ingest",
			"opensearch",
		},
		{
			"foo",
			"foo",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("component name '%s' to expectedName '%s'", tt.compName, tt.expectedName), func(t *testing.T) {
			r := GetCompLabel(tt.compName)
			assert.Equal(t, tt.expectedName, r)
		})
	}
}

func TestDeepCopyMap(t *testing.T) {
	var tests = []struct {
		srcMap map[string]string
		dstMap map[string]string
	}{
		{
			map[string]string{"foo": "bar"},
			map[string]string{"foo": "bar"},
		},
	}

	for _, tt := range tests {
		t.Run("basic deepcopy test", func(t *testing.T) {
			r := DeepCopyMap(tt.srcMap)
			assert.Equal(t, tt.dstMap, r)
		})
	}
}

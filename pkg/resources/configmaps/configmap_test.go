// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package configmaps

import (
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
)

func TestVMONilConfigmap(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	configmap := NewConfig(vmo, "nilconfigmap", nil)
	assert.Equal(t, "nilconfigmap", configmap.Name, "checking configmap data as nil")
}

func TestVMOEmptyConfigmap(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	configmap := NewConfig(vmo, "emptyconfigmap", map[string]string{})
	assert.Equal(t, "emptyconfigmap", configmap.Name, "checking configmap with empty data")
}

func TestVMOConfigmap(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	configmap := NewConfig(vmo, "testconfigmap", map[string]string{"key1": "value1", "key2": "value2"})
	assert.Equal(t, 2, len(configmap.Data), "Length of configmap data")
}

func TestVMOWithCascadingDelete(t *testing.T) {
	// Without CascadingDelete
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	vmo.Spec.CascadingDelete = true
	configmap := NewConfig(vmo, "testconfigmap", map[string]string{"key1": "value1", "key2": "value2"})
	assert.Equal(t, 1, len(configmap.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")

	// Without CascadingDelete
	vmo.Spec.CascadingDelete = false
	configmap = NewConfig(vmo, "testconfigmap", map[string]string{"key1": "value1", "key2": "value2"})
	assert.Equal(t, 0, len(configmap.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
}

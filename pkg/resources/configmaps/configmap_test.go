// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package configmaps

import (
	"testing"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/stretchr/testify/assert"
)

func TestSauronNilConfigmap(t *testing.T) {
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	configmap := NewConfig(sauron, "nilconfigmap", nil)
	assert.Equal(t, "nilconfigmap", configmap.Name, "checking configmap data as nil")
}

func TestSauronEmptyConfigmap(t *testing.T) {
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	configmap := NewConfig(sauron, "emptyconfigmap", map[string]string{})
	assert.Equal(t, "emptyconfigmap", configmap.Name, "checking configmap with empty data")
}

func TestSauronConfigmap(t *testing.T) {
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	configmap := NewConfig(sauron, "testconfigmap", map[string]string{"key1": "value1", "key2": "value2"})
	assert.Equal(t, 2, len(configmap.Data), "Length of configmap data")
}

func TestSauronWithCascadingDelete(t *testing.T) {
	// Without CascadingDelete
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	sauron.Spec.CascadingDelete = true
	configmap := NewConfig(sauron, "testconfigmap", map[string]string{"key1": "value1", "key2": "value2"})
	assert.Equal(t, 1, len(configmap.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")

	// Without CascadingDelete
	sauron.Spec.CascadingDelete = false
	configmap = NewConfig(sauron, "testconfigmap", map[string]string{"key1": "value1", "key2": "value2"})
	assert.Equal(t, 0, len(configmap.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
}

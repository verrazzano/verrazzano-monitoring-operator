// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package services

import (
	"testing"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"

	"github.com/stretchr/testify/assert"
)

func TestVMOWithCascadingDelete(t *testing.T) {
	// With CascadingDelete
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled:  true,
				Replicas: 1,
			},
			AlertManager: vmcontrollerv1.AlertManager{
				Enabled: true,
			},
			Kibana: vmcontrollerv1.Kibana{
				Enabled: true,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
			},
		},
	}
	services, err := New(vmo, false)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(services) > 0, "Non-zero length generated services")
	for _, service := range services {
		assert.Equal(t, 1, len(service.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")
	}

	// Without CascadingDelete
	vmo.Spec.CascadingDelete = false
	services, err = New(vmo, false)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(services) > 0, "Non-zero length generated services")
	for _, service := range services {
		assert.Equal(t, 0, len(service.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
	}
}

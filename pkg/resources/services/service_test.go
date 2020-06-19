// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package services

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSauronWithCascadingDelete(t *testing.T) {
	// With CascadingDelete
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{
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
	services, err := New(sauron)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(services) > 0, "Non-zero length generated services")
	for _, service := range services {
		assert.Equal(t, 1, len(service.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")
	}

	// Without CascadingDelete
	sauron.Spec.CascadingDelete = false
	services, err = New(sauron)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(services) > 0, "Non-zero length generated services")
	for _, service := range services {
		assert.Equal(t, 0, len(service.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
	}
}

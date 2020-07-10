// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package statefulsets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
)

func TestVMIEmptyStatefulSetSize(t *testing.T) {
	vmi := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	statefulsets, err := New(vmi)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 0, len(statefulsets), "Length of generated statefulsets")
}

func TestVMIWithReplicas(t *testing.T) {
	vmi := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			AlertManager: vmcontrollerv1.AlertManager{
				Enabled:  true,
				Replicas: 3,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled:    true,
				MasterNode: vmcontrollerv1.ElasticsearchNode{},
			},
		},
	}
	statefulsets, err := New(vmi)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 2, len(statefulsets), "Length of generated statefulsets")
	for _, statefulset := range statefulsets {
		switch statefulset.Name {
		case resources.GetMetaName(vmi.Name, config.AlertManager.Name):
			assert.Equal(t, *resources.NewVal(3), *statefulset.Spec.Replicas, "AlertManager replicas")
		case resources.GetMetaName(vmi.Name, config.ElasticsearchMaster.Name):
			assert.Equal(t, *resources.NewVal(3), *statefulset.Spec.Replicas, "Elasticsearch Master replicas")
		default:
			t.Error("Unknown Deployment Name: " + statefulset.Name)
		}
		if statefulset.Name == resources.GetMetaName(vmi.Name, config.AlertManager.Name) {
			assert.Equal(t, *resources.NewVal(3), *statefulset.Spec.Replicas, "AlertManager replicas")
		}
	}
}

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

func TestVMOEmptyStatefulSetSize(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	statefulsets, err := New(vmo)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 0, len(statefulsets), "Length of generated statefulsets")
}

func TestVMOWithReplicas(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			AlertManager: vmcontrollerv1.AlertManager{
				Enabled:  true,
				Replicas: 3,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
				MasterNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 5,
				},
			},
		},
	}
	statefulsets, err := New(vmo)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 2, len(statefulsets), "Length of generated statefulsets")
	for _, statefulset := range statefulsets {
		switch statefulset.Name {
		case resources.GetMetaName(vmo.Name, config.AlertManager.Name):
			assert.Equal(t, *resources.NewVal(3), *statefulset.Spec.Replicas, "AlertManager replicas")
		case resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name):
			assert.Equal(t, *resources.NewVal(5), *statefulset.Spec.Replicas, "Elasticsearch Master replicas")
		default:
			t.Error("Unknown Deployment Name: " + statefulset.Name)
		}
		if statefulset.Name == resources.GetMetaName(vmo.Name, config.AlertManager.Name) {
			assert.Equal(t, *resources.NewVal(3), *statefulset.Spec.Replicas, "AlertManager replicas")
		}
	}
}

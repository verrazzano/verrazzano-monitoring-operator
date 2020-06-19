// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package pvcs

import (
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"testing"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/stretchr/testify/assert"
)

func TestSauronNoStorageVolumes(t *testing.T) {
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			Prometheus: vmcontrollerv1.Prometheus{
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
	pvcs, err := New(sauron, constants.OciFlexVolumeProvisioner)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 0, len(pvcs), "Length of generated PVCs")
}

func TestSauronWithStorageVolumes1(t *testing.T) {
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
				Storage: vmcontrollerv1.Storage{
					Size:               "50Gi",
					AvailabilityDomain: "AD1",
					PvcNames:           []string{"grafana-pvc"},
				},
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled: true,
				Storage: vmcontrollerv1.Storage{
					Size:               "50Gi",
					AvailabilityDomain: "AD1",
					PvcNames:           []string{"prometheus-pvc"},
				},
			},
			// An empty size element is interpreted as no storage
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
				Storage: vmcontrollerv1.Storage{
					Size:               "",
					AvailabilityDomain: "AD1",
				},
			},
		},
	}
	pvcs, err := New(sauron, constants.OciFlexVolumeProvisioner)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 2, len(pvcs), "Length of generated PVCs")
}

func TestSauronWithStorageVolumes2(t *testing.T) {
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
				Storage: vmcontrollerv1.Storage{
					Size:               "50Gi",
					AvailabilityDomain: "AD1",
					PvcNames:           []string{"elasticsearch-pvc"},
				},
			},
		},
	}
	pvcs, err := New(sauron, constants.OciFlexVolumeProvisioner)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 1, len(pvcs), "Length of generated PVCs")
}

func TestSauronWithCascadingDelete(t *testing.T) {
	// With CascadingDelete
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
				Storage: vmcontrollerv1.Storage{
					Size:               "50Gi",
					AvailabilityDomain: "AD1",
					PvcNames:           []string{"grafana-pvc"},
				},
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled: true,
				Storage: vmcontrollerv1.Storage{
					Size:               "50Gi",
					AvailabilityDomain: "AD1",
					PvcNames:           []string{"prometheus-pvc"},
				},
			},
			// An empty size element is interpreted as no storage
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
				Storage: vmcontrollerv1.Storage{
					Size:               "50Gi",
					AvailabilityDomain: "AD1",
				},
			},
		},
	}
	pvcs, err := New(sauron, constants.OciFlexVolumeProvisioner)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(pvcs) > 0, "Non-zero length generated PVCs")
	for _, pvs := range pvcs {
		assert.Equal(t, 1, len(pvs.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")
	}

	// Without CascadingDelete
	sauron.Spec.CascadingDelete = false
	pvcs, err = New(sauron, constants.OciFlexVolumeProvisioner)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(pvcs) > 0, "Non-zero length generated ingresses")
	for _, pvc := range pvcs {
		assert.Equal(t, 0, len(pvc.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
	}
}

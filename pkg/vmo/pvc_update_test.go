// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func makePVC(name, quantity string) *corev1.PersistentVolumeClaim {
	q, _ := resource.ParseQuantity(quantity)
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				"ReadWriteOnce",
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": q,
				},
			},
		},
	}
}

func makeDeploymentWithPVC(pvc *corev1.PersistentVolumeClaim) *appsv1.Deployment {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deploy",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{},
			},
		},
	}
	if pvc != nil {
		d.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: pvc.Name,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		}
	}
	return d
}

func TestGetUnboundPVCs(t *testing.T) {
	pvcs := []*corev1.PersistentVolumeClaim{
		makePVC("pvc1", "1Gi"),
		makePVC("pvc2", "1Gi"),
		makePVC("pvc3", "1Gi"),
		makePVC("pvc4", "1Gi"),
	}
	boundPVCNames := []string{
		"pvc2",
		"pvc4",
	}

	unboundPVCNames := getUnboundPVCs(pvcs, boundPVCNames)
	assert.Equal(t, 2, len(unboundPVCNames))
	assert.Equal(t, "pvc1", unboundPVCNames[0].Name)
	assert.Equal(t, "pvc3", unboundPVCNames[1].Name)
}

func TestGetInUsePVCNames(t *testing.T) {
	deployments := []*appsv1.Deployment{
		makeDeploymentWithPVC(makePVC("pvc1", "1Gi")),
		makeDeploymentWithPVC(makePVC("pvc2", "1Gi")),
		makeDeploymentWithPVC(nil),
	}

	isUsePVCNames := getInUsePVCNames(deployments)
	assert.Equal(t, 2, len(isUsePVCNames))
	assert.Equal(t, isUsePVCNames[0], "pvc1")
	assert.Equal(t, isUsePVCNames[1], "pvc2")
}

func TestNewPVCName(t *testing.T) {
	prefixSize := 5
	var tests = []struct {
		name        string
		resizedName bool
	}{
		{
			constants.VMOServiceNamePrefix + "pvc",
			true,
		},
		{
			"abcde-" + constants.VMOServiceNamePrefix + "pvc",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newName, err := newPVCName(tt.name, prefixSize)
			assert.NoError(t, err)
			assert.NotEqual(t, tt.name, newName)
			if tt.resizedName {
				assert.Equal(t, len(tt.name)+1+prefixSize, len(newName))
			} else {
				assert.Equal(t, len(tt.name), len(newName))
			}
		})
	}
}

func TestPVCNeedsResize(t *testing.T) {
	var tests = []struct {
		name        string
		old         *corev1.PersistentVolumeClaim
		new         *corev1.PersistentVolumeClaim
		needsResize bool
	}{
		{
			"equal PVCs don't need resize",
			makePVC("pvc", "1Gi"),
			makePVC("pvc", "1Gi"),
			false,
		},
		{
			"don't resize PVCs smaller than they already are",
			makePVC("pvc", "2Gi"),
			makePVC("pvc", "1Gi"),
			false,
		},
		{
			"needs resize when new PVC is larger than the old PVC",
			makePVC("pvc", "1Gi"),
			makePVC("pvc", "2Gi"),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.needsResize, pvcNeedsResize(tt.old, tt.new))
		})
	}
}

func TestUpdateVMOStorageForPVC(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				DataNode: vmcontrollerv1.ElasticsearchNode{
					Storage: &vmcontrollerv1.Storage{
						PvcNames: []string{
							"some other pvc",
							"oldpvc",
						},
					},
				},
			},
		},
	}

	updateVMOStorageForPVC(vmo, "oldpvc", "newpvc")
	assert.Equal(t, "newpvc", vmo.Spec.Elasticsearch.DataNode.Storage.PvcNames[1])
}

func TestIsOpenSearchPVC(t *testing.T) {
	assert.True(t, isOpenSearchPVC(makePVC("vmi-system-es-data-0", "1Gi")))
	assert.False(t, isOpenSearchPVC(makePVC("vmi-system-grafana", "1Gi")))
}

func TestSetPerNodeStorage(t *testing.T) {
	pvcNames := []string{
		"some other pvc",
		"oldpvc",
	}
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				DataNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 1,
				},
				MasterNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 1,
				},
				Storage: vmcontrollerv1.Storage{
					Size:     "1Gi",
					PvcNames: pvcNames,
				},
			},
		},
	}

	setPerNodeStorage(vmo)
	dataNode := vmo.Spec.Elasticsearch.DataNode
	assert.NotNil(t, dataNode.Storage)
	assert.Equal(t, dataNode.Storage.Size, "1Gi")
	assert.ElementsMatch(t, pvcNames, dataNode.Storage.PvcNames)
	masterNode := vmo.Spec.Elasticsearch.MasterNode
	assert.NotNil(t, masterNode.Storage)
	assert.Equal(t, masterNode.Storage.Size, "1Gi")
}

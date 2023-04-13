// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

var testvmo = vmcontrollerv1.VerrazzanoMonitoringInstance{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "system",
		Namespace: constants.VerrazzanoSystemNamespace,
	},
	Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
		Opensearch: vmcontrollerv1.Opensearch{
			DataNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 3,
			},
			IngestNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 1,
			},
			MasterNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 1,
			},
		},
	},
}

func makePVC(name, quantity string) *corev1.PersistentVolumeClaim {
	q, _ := resource.ParseQuantity(quantity)
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: constants.VerrazzanoSystemNamespace,
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
	boundPVCNames := map[string]bool{
		"pvc2": true,
		"pvc4": true,
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

	isUsePVCNames := getInUsePVCNames(deployments, &vmcontrollerv1.VerrazzanoMonitoringInstance{})
	assert.Equal(t, 2, len(isUsePVCNames))
	assert.Equal(t, isUsePVCNames["pvc1"], true)
	assert.Equal(t, isUsePVCNames["pvc2"], true)
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
			constants.VMOServiceNamePrefix + "pvc-abcde",
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
			Opensearch: vmcontrollerv1.Opensearch{
				DataNode: vmcontrollerv1.ElasticsearchNode{
					Storage: &vmcontrollerv1.Storage{
						PvcNames: []string{
							"some other pvc",
							"oldpvc",
						},
					},
				},
				Nodes: []vmcontrollerv1.ElasticsearchNode{
					{
						Roles: []vmcontrollerv1.NodeRole{
							vmcontrollerv1.DataRole,
						},
						Storage: &vmcontrollerv1.Storage{PvcNames: []string{
							"some other pvc",
							"oldpvc",
						}},
					},
				},
			},
		},
	}

	updateVMOStorageForPVC(vmo, "oldpvc", "newpvc")
	assert.Equal(t, "newpvc", vmo.Spec.Opensearch.DataNode.Storage.PvcNames[1])
	assert.Equal(t, "newpvc", vmo.Spec.Opensearch.Nodes[0].Storage.PvcNames[1])
}

func TestIsOpenSearchPVC(t *testing.T) {
	assert.True(t, isOpenSearchPVC(makePVC("vmi-system-es-data-0", "1Gi")))
	assert.True(t, isOpenSearchPVC(makePVC("vmi-system-es-data-1-03xqy", "1Gi")))
	assert.False(t, isOpenSearchPVC(makePVC("vmi-system-grafana", "1Gi")))
}

func TestSetPerNodeStorage(t *testing.T) {
	pvcNames := []string{
		"some other pvc",
		"oldpvc",
	}
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Opensearch: vmcontrollerv1.Opensearch{
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
	dataNode := vmo.Spec.Opensearch.DataNode
	assert.NotNil(t, dataNode.Storage)
	assert.Equal(t, dataNode.Storage.Size, "1Gi")
	assert.ElementsMatch(t, pvcNames, dataNode.Storage.PvcNames)
	masterNode := vmo.Spec.Opensearch.MasterNode
	assert.NotNil(t, masterNode.Storage)
	assert.Equal(t, masterNode.Storage.Size, "1Gi")
}

func TestResizePVC(t *testing.T) {
	allowVolumeExpansion := true
	disableVolumeExpansion := false
	pvcName := "pvc"
	var tests = []struct {
		name         string
		storageClass *storagev1.StorageClass
		createdPVC   bool
	}{
		{
			"should not create a new PVC when volume expansion is allowed",
			&storagev1.StorageClass{AllowVolumeExpansion: &allowVolumeExpansion},
			false,
		},
		{
			"should create a new PVC when volume expansion is not allowed",
			&storagev1.StorageClass{AllowVolumeExpansion: &disableVolumeExpansion},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existingPVC := makePVC(pvcName, "1Gi")
			expectedPVC := makePVC(pvcName, "2Gi")
			c := &Controller{
				kubeclientset: fake.NewSimpleClientset(existingPVC),
			}
			newName, err := resizePVC(c, &testvmo, existingPVC, expectedPVC, tt.storageClass)
			assert.NoError(t, err)
			if tt.createdPVC {
				assert.NotNil(t, newName)
				assert.NotEqual(t, *newName, pvcName)
			}
		})
	}
}

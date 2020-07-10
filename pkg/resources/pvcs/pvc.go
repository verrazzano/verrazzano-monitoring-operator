// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package pvcs

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New will return a new Service for VMI that needs to executed for on Complete
func New(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, storageClassName string) ([]*corev1.PersistentVolumeClaim, error) {
	var pvcList []*corev1.PersistentVolumeClaim

	if vmi.Spec.Prometheus.Enabled && vmi.Spec.Prometheus.Storage.Size != "" {
		pvcs, err := createPvcElements(vmi, &vmi.Spec.Prometheus.Storage, storageClassName)
		if err != nil {
			return pvcList, err
		}
		pvcList = append(pvcList, pvcs...)
	}
	if vmi.Spec.Elasticsearch.Enabled && vmi.Spec.Elasticsearch.Storage.Size != "" {
		pvcs, err := createPvcElements(vmi, &vmi.Spec.Elasticsearch.Storage, storageClassName)
		if err != nil {
			return pvcList, err
		}
		pvcList = append(pvcList, pvcs...)
	}
	if vmi.Spec.Grafana.Enabled && vmi.Spec.Grafana.Storage.Size != "" {
		pvcs, err := createPvcElements(vmi, &vmi.Spec.Grafana.Storage, storageClassName)
		if err != nil {
			return pvcList, err
		}
		pvcList = append(pvcList, pvcs...)
	}
	return pvcList, nil
}

// Returns slice of pvc elements
func createPvcElements(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, vmiStorage *vmcontrollerv1.Storage, storageClassName string) ([]*corev1.PersistentVolumeClaim, error) {
	storageQuantity, err := resource.ParseQuantity(vmiStorage.Size)
	if err != nil {
		return nil, err
	}
	var pvcList []*corev1.PersistentVolumeClaim
	for _, pvcName := range vmiStorage.PvcNames {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels:          resources.GetMetaLabels(vmi),
				Name:            pvcName,
				Namespace:       vmi.Namespace,
				OwnerReferences: resources.GetOwnerReferences(vmi),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.PersistentVolumeAccessMode("ReadWriteOnce"),
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": storageQuantity,
					},
				},
				StorageClassName: &storageClassName,
			},
		}
		pvcList = append(pvcList, pvc)
	}
	return pvcList, nil
}

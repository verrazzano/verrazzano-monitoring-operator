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

// New will return a new Service for Sauron that needs to executed for on Complete
func New(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, storageClassName string) ([]*corev1.PersistentVolumeClaim, error) {
	var pvcList []*corev1.PersistentVolumeClaim

	if sauron.Spec.Prometheus.Enabled && sauron.Spec.Prometheus.Storage.Size != "" {
		pvcs, err := createPvcElements(sauron, &sauron.Spec.Prometheus.Storage, storageClassName)
		if err != nil {
			return pvcList, err
		}
		pvcList = append(pvcList, pvcs...)
	}
	if sauron.Spec.Elasticsearch.Enabled && sauron.Spec.Elasticsearch.Storage.Size != "" {
		pvcs, err := createPvcElements(sauron, &sauron.Spec.Elasticsearch.Storage, storageClassName)
		if err != nil {
			return pvcList, err
		}
		pvcList = append(pvcList, pvcs...)
	}
	if sauron.Spec.Grafana.Enabled && sauron.Spec.Grafana.Storage.Size != "" {
		pvcs, err := createPvcElements(sauron, &sauron.Spec.Grafana.Storage, storageClassName)
		if err != nil {
			return pvcList, err
		}
		pvcList = append(pvcList, pvcs...)
	}
	return pvcList, nil
}

// Returns slice of pvc elements
func createPvcElements(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, sauronStorage *vmcontrollerv1.Storage, storageClassName string) ([]*corev1.PersistentVolumeClaim, error) {
	storageQuantity, err := resource.ParseQuantity(sauronStorage.Size)
	if err != nil {
		return nil, err
	}
	var pvcList []*corev1.PersistentVolumeClaim
	for _, pvcName := range sauronStorage.PvcNames {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels:          resources.GetMetaLabels(sauron),
				Name:            pvcName,
				Namespace:       sauron.Namespace,
				OwnerReferences: resources.GetOwnerReferences(sauron),
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

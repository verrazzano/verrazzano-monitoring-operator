// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package pvcs

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New will return a new Service for VMO that needs to executed for on Complete
func New(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, storageClassName string) ([]*corev1.PersistentVolumeClaim, error) {
	var pvcList []*corev1.PersistentVolumeClaim

	if vmo.Spec.Opensearch.Enabled {
		for _, dataNode := range nodes.DataNodes(vmo) {
			if dataNode.Storage != nil && dataNode.Storage.Size != "" {
				pvcs, err := createPvcElements(vmo, dataNode.Storage, config.ElasticsearchData, storageClassName)
				if err != nil {
					return pvcList, err
				}
				pvcList = append(pvcList, pvcs...)
			}
		}
	}
	if vmo.Spec.Grafana.Enabled && vmo.Spec.Grafana.Storage.Size != "" {
		pvcs, err := createPvcElements(vmo, &vmo.Spec.Grafana.Storage, config.Grafana, storageClassName)
		if err != nil {
			return pvcList, err
		}
		pvcList = append(pvcList, pvcs...)
	}
	return pvcList, nil
}

// Returns slice of pvc elements
func createPvcElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, vmoStorage *vmcontrollerv1.Storage, componentDetails config.ComponentDetails, storageClassName string) ([]*corev1.PersistentVolumeClaim, error) {
	storageQuantity, err := resource.ParseQuantity(vmoStorage.Size)
	if err != nil {
		return nil, err
	}

	var pvcList []*corev1.PersistentVolumeClaim
	for _, pvcName := range vmoStorage.PvcNames {

		resourceLabel := resources.GetMetaLabels(vmo)
		resourceLabel[constants.ComponentLabel] = resources.GetCompLabel(componentDetails.Name)

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels:          resourceLabel,
				Name:            pvcName,
				Namespace:       vmo.Namespace,
				OwnerReferences: resources.GetOwnerReferences(vmo),
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

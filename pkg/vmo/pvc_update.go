// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"strings"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

//resizePVC resizes a PVC to a new size
// if the underlying storage class does not support expansion, a new PVC will be created.
func resizePVC(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, existingPVC, expectedPVC *corev1.PersistentVolumeClaim, storageClass *storagev1.StorageClass) (*string, error) {
	if storageClass.AllowVolumeExpansion != nil && *storageClass.AllowVolumeExpansion {
		// Volume expansion means dynamic resize is possible - we can do an Update of the PVC in place
		updatedPVC := existingPVC.DeepCopy()
		updatedPVC.Spec.Resources = expectedPVC.Spec.Resources
		_, err := controller.kubeclientset.CoreV1().PersistentVolumeClaims(vmo.Namespace).Update(context.TODO(), updatedPVC, metav1.UpdateOptions{})
		return nil, err
	}

	// If we are updating an OpenSearch PVC, we need to make sure the OpenSearch cluster is ready
	// before doing the resize
	if isOpenSearchPVC(expectedPVC) {
		if err := controller.osClient.IsDataResizable(vmo); err != nil {
			return nil, err
		}
	}

	// the selectors of the existing PVC should be persisted
	expectedPVC.Spec.Selector = existingPVC.Spec.Selector
	// because the new PVC will exist concurrently with the old PVC, it needs a new name
	// the new name is based off the old name to help correlate the PVC with its deploymnt
	newName, err := newPVCName(expectedPVC.Name, 5)
	if err != nil {
		return nil, err
	}
	expectedPVC.Name = newName
	_, err = controller.kubeclientset.CoreV1().PersistentVolumeClaims(vmo.Namespace).Create(context.TODO(), expectedPVC, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// update VMO Spec Storage with the new PVC Name
	updateVMOStorageForPVC(vmo, existingPVC.Name, expectedPVC.Name)
	return &expectedPVC.Name, nil
}

//cleanupUnusedPVCs finds any unused PVCs and deletes them
func cleanupUnusedPVCs(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	selector := labels.SelectorFromSet(resources.GetMetaLabels(vmo))
	deployments, err := controller.deploymentLister.Deployments(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	inUsePVCNames := getInUsePVCNames(deployments, vmo)
	allPVCs, err := controller.pvcLister.PersistentVolumeClaims(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	unboundPVCs := getUnboundPVCs(allPVCs, inUsePVCNames)

	for _, unboundPVC := range unboundPVCs {
		err := controller.kubeclientset.CoreV1().PersistentVolumeClaims(unboundPVC.Namespace).Delete(context.TODO(), unboundPVC.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

//getInUsePVCNames gets the names of PVCs that are currently used by VMO deployments
func getInUsePVCNames(deployments []*appsv1.Deployment, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) map[string]bool {
	inUsePVCNames := map[string]bool{}
	for _, deployment := range deployments {
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				inUsePVCNames[volume.PersistentVolumeClaim.ClaimName] = true
			}
		}
	}

	for _, node := range nodes.DataNodes(vmo) {
		if node.Storage != nil {
			for _, pvc := range node.Storage.PvcNames {
				inUsePVCNames[pvc] = true
			}
		}
	}
	return inUsePVCNames
}

//getUnboundPVCs gets the VMO-managed PVCs which are not currently used by VMO deployments
func getUnboundPVCs(pvcs []*corev1.PersistentVolumeClaim, inUsePVCNames map[string]bool) []*corev1.PersistentVolumeClaim {
	var unboundPVCs []*corev1.PersistentVolumeClaim
	for _, pvc := range pvcs {
		if _, ok := inUsePVCNames[pvc.Name]; !ok {
			unboundPVCs = append(unboundPVCs, pvc)
		}
	}
	return unboundPVCs
}

//isOpenSearchPVC checks if a PVC is an OpenSearch PVC
func isOpenSearchPVC(pvc *corev1.PersistentVolumeClaim) bool {
	return strings.Contains(pvc.Name, "es-data")
}

//pvcNeedsResize a PVC only needs resize if it is greater than the existing PVC size
func pvcNeedsResize(existingPVC, expectedPVC *corev1.PersistentVolumeClaim) bool {
	existingStorage := existingPVC.Spec.Resources.Requests.Storage()
	expectedStorage := expectedPVC.Spec.Resources.Requests.Storage()
	compare := expectedStorage.Cmp(*existingStorage)
	return compare > 0
}

//newPVCName adds a prefix if not present, otherwise it rewrites the existing prefix
func newPVCName(pvcName string, size int) (string, error) {
	pvcNameSplit := strings.Split(pvcName, "-")
	suffix, err := resources.GetNewRandomID(size)
	if err != nil {
		return "", err
	}
	lastIdx := len(pvcNameSplit) - 1
	if len(pvcNameSplit[lastIdx]) != size {
		pvcNameSplit = append(pvcNameSplit, suffix)
	} else {
		pvcNameSplit[lastIdx] = suffix
	}

	return strings.Join(pvcNameSplit, "-"), nil
}

//updateVMOStorageForPVC updates the VMO storage to replace an old PVC with a new PVC
func updateVMOStorageForPVC(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, oldPVCName, newPVCName string) {
	updateStorage := func(storage *vmcontrollerv1.Storage) {
		if storage != nil {
			for idx, pvcName := range storage.PvcNames {
				if pvcName == oldPVCName {
					storage.PvcNames[idx] = newPVCName
				}
			}
		}
	}

	// Look for the PVC reference and update it
	updateStorage(&vmo.Spec.Grafana.Storage)
	updateStorage(vmo.Spec.Elasticsearch.DataNode.Storage)
}

//setPerNodeStorage updates the VMO OpenSearch storage spec to reflect the current API
func setPerNodeStorage(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	updateFunc := func(storage *vmcontrollerv1.Storage, node *vmcontrollerv1.ElasticsearchNode) {
		if node.Replicas > 0 && node.Storage == nil {
			node.Storage = &vmcontrollerv1.Storage{
				Size: storage.Size,
			}
		}
	}

	updateFunc(&vmo.Spec.Elasticsearch.Storage, &vmo.Spec.Elasticsearch.MasterNode)
	updateFunc(&vmo.Spec.Elasticsearch.Storage, &vmo.Spec.Elasticsearch.DataNode)
	if vmo.Spec.Elasticsearch.DataNode.Storage != nil && len(vmo.Spec.Elasticsearch.Storage.PvcNames) > 0 {
		vmo.Spec.Elasticsearch.DataNode.Storage.PvcNames = make([]string, len(vmo.Spec.Elasticsearch.Storage.PvcNames))
		copy(vmo.Spec.Elasticsearch.DataNode.Storage.PvcNames, vmo.Spec.Elasticsearch.Storage.PvcNames)
	}

	vmo.Spec.Elasticsearch.Storage = vmcontrollerv1.Storage{}
}

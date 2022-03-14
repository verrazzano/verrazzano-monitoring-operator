// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

//resizePVC resizes a PVC to a new size
// if the underlying storage class does not support expansion, a new PVC will be created.
func resizePVC(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, existingPVC, expectedPVC *corev1.PersistentVolumeClaim, storageClass *storagev1.StorageClass) (*string, error) {
	if storageClass.AllowVolumeExpansion != nil && *storageClass.AllowVolumeExpansion {
		// Volume expansion means dynamic resize is possible - we can do an Update of the PVC in place
		_, err := controller.kubeclientset.CoreV1().PersistentVolumeClaims(vmo.Namespace).Update(context.TODO(), expectedPVC, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}

	// If we are updating an OpenSearch PVC, we need to make sure the OpenSearch cluster is ready
	// before doing the resize
	if isOpenSearchPVC(expectedPVC) {
		if err := IsOpenSearchUpgradeable(vmo); err != nil {
			return nil, err
		}
	}

	// the selectors of the existing PVC should be persisted
	expectedPVC.Spec.Selector = existingPVC.Spec.Selector
	// because the new PVC will exist concurrently with the old PVC, it needs a new name
	// the new name is based off the old name to help correlate the PVC with its deploymnt
	expectedPVC.Name = newPVCName(expectedPVC.Name, 5)
	_, err := controller.kubeclientset.CoreV1().PersistentVolumeClaims(vmo.Namespace).Create(context.TODO(), expectedPVC, metav1.CreateOptions{})
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
	inUsePVCNames := getInUsePVCNames(deployments)
	allPVCs, err := controller.pvcLister.PersistentVolumeClaims(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	unboundPVCs := getUnbouncdPVCs(allPVCs, inUsePVCNames)

	for _, unboundPVC := range unboundPVCs {
		err := controller.kubeclientset.CoreV1().PersistentVolumeClaims(unboundPVC.Namespace).Delete(context.TODO(), unboundPVC.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

//getInUsePVCNames gets the names of PVCs that are currently used by VMO deployments
func getInUsePVCNames(deployments []*appsv1.Deployment) []string {
	var inUsePVCNames []string
	for _, deployment := range deployments {
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				inUsePVCNames = append(inUsePVCNames, volume.PersistentVolumeClaim.ClaimName)
			}
		}
	}
	return inUsePVCNames
}

//getUnbouncdPVCs gets the VMO-managed PVCs which are not currently used by VMO deployments
func getUnbouncdPVCs(pvcs []*corev1.PersistentVolumeClaim, boundPVCNames []string) []*corev1.PersistentVolumeClaim {
	isPVCBound := func(pvc *corev1.PersistentVolumeClaim, boundPVCNames []string) bool {
		for _, pvcName := range boundPVCNames {
			if pvcName == pvc.Name {
				return true
			}
		}
		return false
	}
	var unboundPVCs []*corev1.PersistentVolumeClaim
	for _, pvc := range pvcs {
		if !isPVCBound(pvc, boundPVCNames) {
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
func newPVCName(pvcName string, size int) string {
	pvcNameSplit := strings.Split(pvcName, "-")
	if pvcNameSplit[0] == "vmi" {
		pvcNameSplit = append([]string{resources.GetNewRandomPrefix(size)}, pvcNameSplit...)
	} else {
		pvcNameSplit[0] = resources.GetNewRandomPrefix(size)
	}

	return strings.Join(pvcNameSplit, "-")
}

//updateVMOStorageForPVC updates the VMO storage to replace an old PVC with a new PVC
func updateVMOStorageForPVC(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, oldPVCName, newPVCName string) {
	updateStorage := func(storage *vmcontrollerv1.Storage) bool {
		for idx, pvcName := range storage.PvcNames {
			if pvcName == oldPVCName {
				storage.PvcNames[idx] = newPVCName
				return true
			}
		}

		return false
	}

	// Look for the PVC reference and update it
	if ok := updateStorage(&vmo.Spec.Prometheus.Storage); ok {
		return
	}
	if ok := updateStorage(&vmo.Spec.Grafana.Storage); ok {
		return
	}
	updateStorage(&vmo.Spec.Elasticsearch.Storage)
}

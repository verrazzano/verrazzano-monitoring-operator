// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"
	"fmt"
	"github.com/verrazzano/pkg/diff"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/statefulsets"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// CreateStatefulSets creates/updates/deletes VMO statefulset k8s resources
func CreateStatefulSets(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	storageClass, err := getStorageClassOverride(controller, vmo.Spec.StorageClass)
	if err != nil {
		controller.log.Errorf("Failed to determine storage class for VMI %s: %v", vmo.Name, err)
		return err
	}

	initialMasterNodes, err := getInitialMasterNodes(controller, vmo)
	expectedStatefulSets, err := statefulsets.New(controller.log, vmo, storageClass, initialMasterNodes)
	if err != nil {
		controller.log.Errorf("Failed to create StatefulSet specs for VMI %s: %v", vmo.Name, err)
		return err
	}

	// Loop through the existing stateful sets and create/update as needed
	controller.log.Oncef("Creating/updating Statefulsets for VMI %s", vmo.Name)
	var statefulSetNames []string
	for _, expectedStatefulSet := range expectedStatefulSets {
		statefulSetName := expectedStatefulSet.Name
		statefulSetNames = append(statefulSetNames, statefulSetName)
		if statefulSetName == "" && expectedStatefulSet.GenerateName == "" {
			// We choose to absorb the error here as the worker would requeue the
			// resource otherwise. Instead, the next time the resource is updated
			// the resource will be queued again.
			runtime.HandleError(errors.New("statefulset name must be specified"))
			return nil
		}
		controller.log.Debugf("Applying StatefulSet '%s' in namespace '%s' for VMI '%s'\n", statefulSetName, vmo.Namespace, vmo.Name)
		existingStatefulSet, _ := controller.statefulSetLister.StatefulSets(vmo.Namespace).Get(statefulSetName)
		if existingStatefulSet != nil {
			// Existing statefulsets are updated one at a time, to prevent cluster failover
			updated, err := updateStatefulSet(controller, expectedStatefulSet, existingStatefulSet, vmo)
			// if we updated a statefulset, return to do any others in the next reconcile
			if err != nil || updated {
				return err
			}

		} else {
			_, err = controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Create(context.TODO(), expectedStatefulSet, metav1.CreateOptions{})
		}
		if err != nil {
			return controller.log.ErrorfNewErr("Failed to update StatefulSets %s%s: %v", expectedStatefulSet.Namespace, expectedStatefulSet.Name, err)
		}
	}

	// Do a second pass through the stateful sets to update PVC ownership and clean up statesful sets as needed
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingStatefulSetsList, err := controller.statefulSetLister.StatefulSets(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, statefulSet := range existingStatefulSetsList {
		latestSts, _ := controller.statefulSetLister.StatefulSets(vmo.Namespace).Get(statefulSet.Name)
		if latestSts == nil {
			break
		}
		// Update the PVC owner ref if needed
		err = updateOwnerForPVCs(controller, latestSts, vmo.Name, vmo.Namespace)
		if err != nil {
			return err
		}
		// Delete StatefulSets that shouldn't exist
		if !contains(statefulSetNames, statefulSet.Name) {
			if err := scaleDownStatefulSet(controller, statefulSetNames, statefulSet, vmo); err != nil {
				return err
			}
			// We only scale down one statefulset at a time. This gives the statefulset data
			// time to migrate and the cluster to heal itself.
			break
		}
	}

	controller.log.Oncef("Successfully applied StatefulSets for VMI %s", vmo.Name)
	return nil
}

//getInitialMasterNodes returns the initial master nodes string if the cluster is not already bootstrapped
func getInitialMasterNodes(c *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) (string, error) {
	existing, err := c.statefulSetLister.StatefulSets(vmo.Namespace).List(labels.SelectorFromSet(resources.GetMetaLabels(vmo)))
	if err != nil {
		return "", err
	}
	if len(existing) > 0 {
		return "", nil
	}
	return nodes.InitialMasterNodes(vmo.Name, nodes.StatefulSetNodes(vmo)), nil
}

func updateStatefulSet(c *Controller, current, existing *appsv1.StatefulSet, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) (bool, error) {
	statefulsets.CopyFromExisting(current, existing)
	specDiffs := diff.Diff(existing, current)
	if specDiffs != "" {
		// We should only update the cluster if it is healthy
		if err := c.osClient.IsGreen(vmo); err != nil {
			return false, err
		}
		c.log.Oncef("Statefulset %s/%s has spec differences %s", current.Namespace, current.Name, specDiffs)
		if _, err := c.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Update(context.TODO(), current, metav1.UpdateOptions{}); err != nil {
			return true, err
		}
	}
	return false, nil
}

//scaleDownStatefulSet scales down a statefulset, and deletes the statefulset if it is already at 1 or fewer replicas.
func scaleDownStatefulSet(c *Controller, statefulSetNames []string, statefulSet *appsv1.StatefulSet, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	deleteSTS := func() error {
		err := c.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{})
		if err != nil {
			c.log.Errorf("Failed to delete StatefulSet %s: %v", statefulSet.Name, err)
			return err
		}
		return nil
	}

	// If the desired list of statefulsets is zero, we should delete without any checks
	if len(statefulSetNames) < 1 {
		return deleteSTS()
	}

	// The cluster should be at its current capacity before we issue a scaledown
	if err := c.osClient.IsResizable(vmo); err != nil {
		return err
	}

	// If the statefulset has multiple replicas, scale it down. this allows existing data to be migrated
	// to another node on the cluster. If it's already on one replica, the STS can be deleted
	if *statefulSet.Spec.Replicas <= 1 {
		return deleteSTS()
	} else {
		*statefulSet.Spec.Replicas -= 1
		if _, err := c.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Update(context.TODO(), statefulSet, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// Update each PVC metadata.ownerReferences field to refer to the StatefulSet (STS).
// PVCs are created automatically by Kubernetes when the STS is created
// because the STS has a volumeClaimTemplate.  However, the PVCs are not deleted
// when the STS is deleted. By setting the PVC metadata.ownerReferences field to refer
// to the STS resource, the PVC will automatically get deleted when the STS is deleted.
// Because PVC is dynamic, when it is deleted, the bound PV will also get deleted.
// NOTE: This cannot be done automatically using the STS VolumeClaimTemplate.
func updateOwnerForPVCs(controller *Controller, statefulSet *appsv1.StatefulSet, vmoName string, vmoNamespace string) error {
	pvcNames := getPVCNames(statefulSet)
	for _, pvcName := range pvcNames {
		pvc, err := controller.pvcLister.PersistentVolumeClaims(vmoNamespace).Get(pvcName)
		if err != nil {
			return err
		}
		if len(pvc.OwnerReferences) != 0 {
			continue
		}
		pvc.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
			Name:       statefulSet.Name,
			UID:        statefulSet.UID,
		}}
		controller.log.Debugf("Setting StatefulSet owner reference for PVC %s", pvc.Name)
		_, err = controller.kubeclientset.CoreV1().PersistentVolumeClaims(vmoNamespace).Update(context.TODO(), pvc, metav1.UpdateOptions{})
		if err != nil {
			controller.log.Errorf("Failed to update the owner reference in PVC %s: %v", pvc.Name, err)
			return err
		}
	}
	return nil
}

func getPVCNames(statefulSet *appsv1.StatefulSet) []string {
	var pvcNames []string
	var i int32
	replicas := *statefulSet.Spec.Replicas
	for _, volumeClaimTemplate := range statefulSet.Spec.VolumeClaimTemplates {
		for i = 0; i < replicas; i++ {
			pvcName := fmt.Sprintf("%s-%s-%d", volumeClaimTemplate.Name, statefulSet.Name, i)
			pvcNames = append(pvcNames, pvcName)
		}
	}
	return pvcNames
}

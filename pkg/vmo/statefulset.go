// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/statefulsets"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// CreateStatefulSets creates/updates/deletes VMO statefulset k8s resources
func CreateStatefulSets(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) (bool, error) {
	storageClass, err := getStorageClassOverride(controller, vmo.Spec.StorageClass)
	if err != nil {
		controller.log.Errorf("Failed to determine storage class for VMI %s: %v", vmo.Name, err)
		return false, err
	}

	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingList, err := controller.statefulSetLister.StatefulSets(vmo.Namespace).List(selector)
	if err != nil {
		return false, err
	}
	initialMasterNodes := getInitialMasterNodes(vmo, existingList)
	expectedList, err := statefulsets.New(controller.log, vmo, storageClass, initialMasterNodes)
	if err != nil {
		controller.log.Errorf("Failed to create StatefulSet specs for VMI %s: %v", vmo.Name, err)
		return false, err
	}

	// Loop through the existing stateful sets and create/update as needed
	controller.log.Oncef("Creating/updating Statefulsets for VMI %s", vmo.Name)
	plan := statefulsets.CreatePlan(controller.log, existingList, expectedList)

	for _, sts := range plan.Create {
		if _, err := controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Create(context.TODO(), sts, metav1.CreateOptions{}); err != nil {
			return plan.ExistingCluster, logReturnError(controller.log, sts, err)
		}
	}

	// Loop through existing statefulsets again to update PVC owner references
	latestList, err := controller.statefulSetLister.StatefulSets(vmo.Namespace).List(selector)
	if err != nil {
		return plan.ExistingCluster, err
	}
	for _, sts := range latestList {
		if err := updateOwnerForPVCs(controller, sts, vmo.Name, vmo.Namespace); err != nil {
			return plan.ExistingCluster, err
		}
	}

	for _, sts := range plan.Update {
		if err := updateStatefulSet(controller, sts, vmo, plan); err != nil {
			return plan.ExistingCluster, logReturnError(controller.log, sts, err)
		}
	}

	for _, sts := range plan.Delete {
		if err := scaleDownStatefulSet(controller, expectedList, sts, vmo); err != nil {
			return plan.ExistingCluster, err
		}
		// We only scale down one statefulset at a time. This gives the statefulset data
		// time to migrate and the cluster to heal itself.
		break
	}

	if plan.Conflict == nil {
		controller.log.Oncef("Successfully applied StatefulSets for VMI %s", vmo.Name)
	} else {
		controller.log.Errorf("StatefulSet update plan conflict: %v", plan.Conflict)
	}
	return plan.ExistingCluster, plan.Conflict
}

func logReturnError(log vzlog.VerrazzanoLogger, sts *appsv1.StatefulSet, err error) error {
	return log.ErrorfNewErr("Failed to update StatefulSets %s:%s: %v", sts.Namespace, sts.Name, err)
}

//getInitialMasterNodes returns the initial master nodes string if the cluster is not already bootstrapped
func getInitialMasterNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, existing []*appsv1.StatefulSet) string {
	if len(existing) > 0 {
		return ""
	}
	return nodes.InitialMasterNodes(vmo.Name, nodes.MasterNodes(vmo))
}

func updateStatefulSet(c *Controller, sts *appsv1.StatefulSet, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, plan *statefulsets.StatefulSetPlan) error {
	// if the cluster is alive, but unhealthy we shouldn't do an update - may cause data loss/corruption
	if !plan.BounceNodes && plan.ExistingCluster {
		// We should only update an existing cluster if it is healthy
		if err := c.osClient.IsGreen(vmo); err != nil {
			return err
		}
	}

	if _, err := c.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Update(context.TODO(), sts, metav1.UpdateOptions{}); err != nil {
		return err
	}
	// if it was a single node cluster, delete the pod to ensure it picks up the updated settings.
	if plan.BounceNodes {
		return c.kubeclientset.CoreV1().Pods(vmo.Namespace).Delete(context.TODO(), sts.Name+"-0", metav1.DeleteOptions{})
	}
	return nil
}

//scaleDownStatefulSet scales down a statefulset, and deletes the statefulset if it is already at 1 or fewer replicas.
func scaleDownStatefulSet(c *Controller, expectedList []*appsv1.StatefulSet, statefulSet *appsv1.StatefulSet, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	deleteSTS := func() error {
		err := c.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{})
		if err != nil {
			c.log.Errorf("Failed to delete StatefulSet %s: %v", statefulSet.Name, err)
			return err
		}
		return nil
	}

	// don't worry about cluster health if we are deleting the cluster or the statefulset is unhealthy
	if len(expectedList) < 1 || statefulSet.Status.ReadyReplicas < 1 {
		return deleteSTS()
	}

	// The cluster should be in steady state before any nodes are removed
	if err := c.osClient.IsUpdated(vmo); err != nil {
		return err
	}

	// If the statefulset has multiple replicas, scale it down. this allows existing data to be migrated to another node on the cluster.
	// If the statefulset already has one replica, then it can be deleted.
	if *statefulSet.Spec.Replicas > 1 {
		*statefulSet.Spec.Replicas--
		if _, err := c.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Update(context.TODO(), statefulSet, metav1.UpdateOptions{}); err != nil {
			return err
		}

	} else {
		return deleteSTS()
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
	pvcNames := statefulsets.GetPVCNames(statefulSet)
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

// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang/glog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/statefulsets"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/diff"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// CreateStatefulSets creates/updates/deletes VMO statefulset k8s resources
func CreateStatefulSets(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	const finalizer = "verrazzano.io/sts"
	statefulSetList, err := statefulsets.New(vmo)
	if err != nil {
		glog.Errorf("Failed to create StatefulSet specs for vmo: %s", err)
		return err
	}

	glog.V(4).Infof("Creating/updating Statefulsets for vmo '%s' in namespace '%s'", vmo.Name, vmo.Namespace)
	var statefulSetNames []string
	for _, curStatefulSet := range statefulSetList {
		statefulSetName := curStatefulSet.Name
		statefulSetNames = append(statefulSetNames, statefulSetName)
		if statefulSetName == "" && curStatefulSet.GenerateName == "" {
			// We choose to absorb the error here as the worker would requeue the
			// resource otherwise. Instead, the next time the resource is updated
			// the resource will be queued again.
			runtime.HandleError(errors.New("statefulset name must be specified"))
			return nil
		}
		glog.V(6).Infof("Applying StatefulSet '%s' in namespace '%s' for vmo '%s'\n", statefulSetName, vmo.Namespace, vmo.Name)
		existingStatefulSet, _ := controller.statefulSetLister.StatefulSets(vmo.Namespace).Get(statefulSetName)
		if existingStatefulSet != nil {
			specDiffs := diff.CompareIgnoreTargetEmpties(existingStatefulSet, curStatefulSet)
			if specDiffs != "" {
				glog.V(6).Infof("Statefulset %s : Spec differences %s", curStatefulSet.Name, specDiffs)
				_, _ = controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Update(context.TODO(), curStatefulSet, metav1.UpdateOptions{})
			}
		} else {
			// Create StatefulSet. Also add finalizer so that we can detect when this StatefulSet is being deleted and cleanup PVCs
			// curStatefulSet.ObjectMeta.Finalizers = append(curStatefulSet.ObjectMeta.Finalizers, finalizer)
			_, err := controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Create(context.TODO(), curStatefulSet, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
		glog.V(4).Infof("Successfully applied StatefulSet '%s'\n", statefulSetName)
	}

	// Delete StatefulSets that shouldn't exist
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingStatefulSetsList, err := controller.statefulSetLister.StatefulSets(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, statefulSet := range existingStatefulSetsList {
		latestSts, _ := controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Get(context.TODO(), statefulSet.Name, metav1.GetOptions{})
		if latestSts == nil {
			break
		}
		err = updateOwnerForPVCs(controller, latestSts, vmo.Name, vmo.Namespace)
		if err != nil {
			return err
		}
		if !contains(statefulSetNames, statefulSet.Name) {
			glog.V(6).Infof("Deleting StatefulSet %s", statefulSet.Name)
			err := controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{})
			if err != nil {
				glog.Errorf("Failed to delete StatefulSet %s, for the reason (%v)", statefulSet.Name, err)
				return err
			}
		}
	}

	glog.V(4).Infof("Successfully applied StatefulSets for vmo '%s'", vmo.Name)
	return nil
}

// Update the each PVC owner reference to be the StatefulSet (STS).
// PVCs are created automatically by Kubernetes when the STS is created,
// because the STS has a volumeClaimTemplate.  However, the PVCs are not deleted
// when the STS is deleted. Set the PVC owner reference to be the STS
// so that when the STS is deleted, the PVC will automatically get deleted.
// Because PVC is dynamic, when it is deleted, the bound PV will also get deleted.
func updateOwnerForPVCs(controller *Controller, statefulSet *appsv1.StatefulSet, vmoName string, vmoNamespace string) error {

	// Get for PVCs for this STS using the specID label. Each PVC metadata.label
	// has the same specID label as the STS template.metadata.label,
	// For example: " app: hello-world-binding-es-master"
	idLabel := resources.GetSpecID(vmoName, config.ElasticsearchMaster.Name)
	selector := labels.SelectorFromSet(idLabel)
	existingPvcList, err := controller.pvcLister.PersistentVolumeClaims(vmoNamespace).List(selector)
	if err != nil {
		return err
	}
	for _, pvc := range existingPvcList {
		if len(pvc.OwnerReferences) != 0 {
			continue
		}
		pvc.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
			Name:       statefulSet.Name,
			UID:        statefulSet.UID,
		}}
		glog.V(4).Infof("Setting owner reference for PVC %s", pvc.Name)
		_, err := controller.kubeclientset.CoreV1().PersistentVolumeClaims(vmoNamespace).Update(context.TODO(), pvc, metav1.UpdateOptions{})
		if err != nil {
			glog.Errorf("Failed to update the owner reference in  PVC %s, for the reason (%v)", pvc.Name, err)
			return err
		}
	}
	replicas := int(*statefulSet.Spec.Replicas)
	numPVCs := len(existingPvcList)
	if numPVCs != replicas {
		return errors.New(fmt.Sprintf("PVC owner reference set in %v of %v PVCs", numPVCs, replicas))
	}
	return nil
}

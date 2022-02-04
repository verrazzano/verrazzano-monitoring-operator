// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"
	"github.com/verrazzano/pkg/diff"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/statefulsets"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// CreateStatefulSets creates/updates/deletes VMO statefulset k8s resources
func CreateStatefulSets(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, username, password string) error {
	statefulSetList, err := statefulsets.New(vmo, controller.kubeclientset, username, password)
	if err != nil {
		zap.S().Errorf("Failed to create StatefulSet specs for vmo: %s", err)
		return err
	}

	// Loop through the existing stateful sets and create/update as needed
	zap.S().Infof("Creating/updating Statefulsets for vmo '%s' in namespace '%s'", vmo.Name, vmo.Namespace)
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
		zap.S().Infof("Applying StatefulSet '%s' in namespace '%s' for vmo '%s'\n", statefulSetName, vmo.Namespace, vmo.Name)
		existingStatefulSet, _ := controller.statefulSetLister.StatefulSets(vmo.Namespace).Get(statefulSetName)
		if existingStatefulSet != nil {
			specDiffs := diff.Diff(existingStatefulSet, curStatefulSet)
			if specDiffs != "" {
				zap.S().Debugf("Statefulset %s : Spec differences %s", curStatefulSet.Name, specDiffs)
				_, err = controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Update(context.TODO(), curStatefulSet, metav1.UpdateOptions{})
			}
		} else {
			_, err = controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Create(context.TODO(), curStatefulSet, metav1.CreateOptions{})
		}
		if err != nil {
			return err
		}
		zap.S().Infof("Successfully applied StatefulSet '%s'\n", statefulSetName)
	}

	// Do a second pass through the stateful sets to update PVC ownership and clean up stateful sets as needed
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
		zap.S().Infof("+++ STS name = '%s' +++", latestSts.Name)
		// Update the PVC owner ref if needed
		err = updateOwnerForPVCs(controller, latestSts, vmo)
		if err != nil {
			return err
		}
		// Delete StatefulSets that shouldn't exist
		if !contains(statefulSetNames, statefulSet.Name) {
			zap.S().Infof("Deleting StatefulSet %s", statefulSet.Name)
			err := controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{})
			if err != nil {
				zap.S().Errorf("Failed to delete StatefulSet %s, for the reason (%v)", statefulSet.Name, err)
				return err
			}
		}
	}

	zap.S().Infof("Successfully applied StatefulSets for vmo '%s'", vmo.Name)
	return nil
}

// Update each PVC metadata.ownerReferences field to refer to the StatefulSet (STS).
// PVCs are created automatically by Kubernetes when the STS is created
// because the STS has a volumeClaimTemplate.  However, the PVCs are not deleted
// when the STS is deleted. By setting the PVC metadata.ownerReferences field to refer
// to the STS resource, the PVC will automatically get deleted when the STS is deleted.
// Because PVC is dynamic, when it is deleted, the bound PV will also get deleted.
// NOTE: This cannot be done automatically using the STS VolumeClaimTemplate.
func updateOwnerForPVCs(controller *Controller, statefulSet *appsv1.StatefulSet, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {

	// Get the PVCs for this STS using the specID label. Each PVC metadata.label
	// has the same specID label as the STS template.metadata.label.
	// For example: " app: hello-world-binding-es-master"
	zap.S().Info("+++ Inside updateOwnerForPVCs +++")
	var idLabel map[string]string
	var err error
	if vmo.Spec.Elasticsearch.Enabled {
		idLabel = resources.GetSpecID(vmo.Name, config.ElasticsearchMaster.Name)
		err = updateOwnerForPvcHelper(controller, statefulSet, vmo, idLabel)
		if err != nil {
			zap.S().Errorf("Failed to update the owner reference due to (%v)", err)
			return err
		}

	}
	if vmo.Spec.Grafana.Enabled {
		idLabel = resources.GetSpecID(vmo.Name, config.Grafana.Name)
		err = updateOwnerForPvcHelper(controller, statefulSet, vmo, idLabel)
		if err != nil {
			zap.S().Errorf("Failed to update the owner reference due to (%v)", err)
			return err
		}
	}
	if vmo.Spec.Prometheus.Enabled {
		idLabel = resources.GetSpecID(vmo.Name, config.Prometheus.Name)
		err = updateOwnerForPvcHelper(controller, statefulSet, vmo, idLabel)
		if err != nil {
			zap.S().Errorf("Failed to update the owner reference due to (%v)", err)
			return err
		}
	}
	if vmo.Spec.Elasticsearch.Enabled {
		idLabel = resources.GetSpecID(vmo.Name, config.ElasticsearchData.Name)
		err = updateOwnerForPvcHelper(controller, statefulSet, vmo, idLabel)
		if err != nil {
			zap.S().Errorf("Failed to update the owner reference due to (%v)", err)
			return err
		}
	}

	//expectedNumPVCs := int(*statefulSet.Spec.Replicas) * len(statefulSet.Spec.VolumeClaimTemplates)
	//actualNumPVCs := len(existingPvcList)
	//if actualNumPVCs != expectedNumPVCs {
	//	return fmt.Errorf("PVC owner reference set in %v of %v PVCs for VMO %s", actualNumPVCs, expectedNumPVCs, vmoName)
	//}
	return nil
}

func updateOwnerForPvcHelper(controller *Controller, statefulSet *appsv1.StatefulSet, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, idLabel map[string]string) error {
	selector := labels.SelectorFromSet(idLabel)
	existingPvcList, err := controller.pvcLister.PersistentVolumeClaims(vmo.Namespace).List(selector)
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
		zap.S().Infof("+++++ Setting StatefuleSet owner reference for PVC %s ++++", pvc.Name)
		_, err := controller.kubeclientset.CoreV1().PersistentVolumeClaims(vmo.Namespace).Update(context.TODO(), pvc, metav1.UpdateOptions{})
		if err != nil {
			zap.S().Errorf("Failed to update the owner reference in PVC %s, for the reason (%v)", pvc.Name, err)
			return err
		}
	}
	return nil
}

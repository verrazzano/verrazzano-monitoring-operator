// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	"context"
	"errors"

	"github.com/golang/glog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/statefulsets"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

func CreateStatefulSets(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
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
				glog.V(4).Infof("Statefulset %s : Spec differences %s", curStatefulSet.Name, specDiffs)
				_, err = controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Update(context.TODO(), curStatefulSet, metav1.UpdateOptions{})
			}
		} else {
			_, err = controller.kubeclientset.AppsV1().StatefulSets(vmo.Namespace).Create(context.TODO(), curStatefulSet, metav1.CreateOptions{})
		}
		if err != nil {
			return err
		}
		glog.V(4).Infof("Successfully applied StatefulSet '%s'\n", statefulSetName)
	}

	// Delete StatefulSets that shouldn't exist
	glog.V(4).Infof("Deleting unwanted Statefulsets for vmo '%s' in namespace '%s'", vmo.Name, vmo.Namespace)
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingStatefulSetsList, err := controller.statefulSetLister.StatefulSets(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, statefulSet := range existingStatefulSetsList {
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

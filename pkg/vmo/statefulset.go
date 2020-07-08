// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	"context"
	"errors"
	"os"

	"github.com/rs/zerolog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/statefulsets"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

func CreateStatefulSets(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	//create log for creation of stateful sets
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "VerrazzanoMonitoringInstance").Str("name", sauron.Name).Logger()

	statefulSetList, err := statefulsets.New(sauron)
	if err != nil {
		logger.Error().Msgf("Failed to create StatefulSet specs for sauron: %s", err)
		return err
	}

	logger.Info().Msgf("Creating/updating Statefulsets for sauron '%s' in namespace '%s'", sauron.Name, sauron.Namespace)
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
		logger.Debug().Msgf("Applying StatefulSet '%s' in namespace '%s' for sauron '%s'\n", statefulSetName, sauron.Namespace, sauron.Name)
		existingStatefulSet, _ := controller.statefulSetLister.StatefulSets(sauron.Namespace).Get(statefulSetName)
		if existingStatefulSet != nil {
			specDiffs := diff.CompareIgnoreTargetEmpties(existingStatefulSet, curStatefulSet)
			if specDiffs != "" {
				logger.Info().Msgf("Statefulset %s : Spec differences %s", curStatefulSet.Name, specDiffs)
				_, err = controller.kubeclientset.AppsV1().StatefulSets(sauron.Namespace).Update(context.TODO(), curStatefulSet, metav1.UpdateOptions{})
			}
		} else {
			_, err = controller.kubeclientset.AppsV1().StatefulSets(sauron.Namespace).Create(context.TODO(), curStatefulSet, metav1.CreateOptions{})
		}
		if err != nil {
			return err
		}
		logger.Info().Msgf("Successfully applied StatefulSet '%s'\n", statefulSetName)
	}

	// Delete StatefulSets that shouldn't exist
	logger.Info().Msgf("Deleting unwanted Statefulsets for sauron '%s' in namespace '%s'", sauron.Name, sauron.Namespace)
	selector := labels.SelectorFromSet(map[string]string{constants.SauronLabel: sauron.Name})
	existingStatefulSetsList, err := controller.statefulSetLister.StatefulSets(sauron.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, statefulSet := range existingStatefulSetsList {
		if !contains(statefulSetNames, statefulSet.Name) {
			logger.Debug().Msgf("Deleting StatefulSet %s", statefulSet.Name)
			err := controller.kubeclientset.AppsV1().StatefulSets(sauron.Namespace).Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{})
			if err != nil {
				logger.Error().Msgf("Failed to delete StatefulSet %s, for the reason (%v)", statefulSet.Name, err)
				return err
			}
		}
	}

	logger.Info().Msgf("Successfully applied StatefulSets for sauron '%s'", sauron.Name)
	return nil
}

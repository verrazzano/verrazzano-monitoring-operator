// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	"context"
	"errors"
	"os"

	"github.com/rs/zerolog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/ingresses"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/diff"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

func CreateIngresses(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	//create log for creation of ingresses
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "VerrazzanoMonitoringInstance").Str("name", sauron.Name).Logger()

	ingList, err := ingresses.New(sauron)
	if err != nil {
		logger.Error().Msgf("Failed to create Ingress specs for sauron: %s", err)
		return err
	}
	if sauron.Spec.IngressTargetDNSName == "" {
		logger.Debug().Msgf("No Ingress target specified, using default Ingress target: '%s'", controller.operatorConfig.DefaultIngressTargetDNSName)
		sauron.Spec.IngressTargetDNSName = controller.operatorConfig.DefaultIngressTargetDNSName
	}
	var ingressNames []string
	logger.Info().Msgf("Creating/updating Ingresses for sauron '%s' in namespace '%s'", sauron.Name, sauron.Namespace)
	for _, curIngress := range ingList {
		ingName := curIngress.Name
		ingressNames = append(ingressNames, ingName)
		if ingName == "" {
			// We choose to absorb the error here as the worker would requeue the
			// resource otherwise. Instead, the next time the resource is updated
			// the resource will be queued again.
			runtime.HandleError(errors.New("ingress name must be specified"))
			return nil
		}

		logger.Debug().Msgf("Applying Ingress '%s' in namespace '%s' for sauron '%s'\n", ingName, sauron.Namespace, sauron.Name)
		existingIngress, err := controller.ingressLister.Ingresses(sauron.Namespace).Get(ingName)
		if existingIngress != nil {
			specDiffs := diff.CompareIgnoreTargetEmpties(existingIngress, curIngress)
			if specDiffs != "" {
				logger.Info().Msgf("Ingress %s : Spec differences %s", curIngress.Name, specDiffs)
				_, err = controller.kubeclientset.ExtensionsV1beta1().Ingresses(sauron.Namespace).Update(context.TODO(), curIngress, metav1.UpdateOptions{})
			}
		} else if k8serrors.IsNotFound(err) {
			_, err = controller.kubeclientset.ExtensionsV1beta1().Ingresses(sauron.Namespace).Create(context.TODO(), curIngress, metav1.CreateOptions{})
		} else {
			logger.Error().Msgf("Problem getting existing Ingress %s in namespace %s: %v", ingName, sauron.Namespace, err)
			return err
		}

		if err != nil {
			logger.Error().Msgf("Failed to apply Ingress for sauron: %s", err)
			return err
		}
	}

	// Delete ingresses that shouldn't exist
	logger.Info().Msgf("Deleting unwanted Ingresses for sauron '%s' in namespace '%s'", sauron.Name, sauron.Namespace)
	selector := labels.SelectorFromSet(map[string]string{constants.SauronLabel: sauron.Name})
	existingIngressList, err := controller.ingressLister.Ingresses(sauron.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, ingress := range existingIngressList {
		if !contains(ingressNames, ingress.Name) {
			logger.Info().Msgf("Deleting ingress %s", ingress.Name)
			err := controller.kubeclientset.ExtensionsV1beta1().Ingresses(sauron.Namespace).Delete(context.TODO(), ingress.Name, metav1.DeleteOptions{})
			if err != nil {
				logger.Error().Msgf("Failed to delete ingress %s, for the reason (%v)", ingress.Name, err)
				return err
			}
		}
	}

	return nil
}

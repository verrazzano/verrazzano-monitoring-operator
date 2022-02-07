// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"

	"github.com/verrazzano/pkg/diff"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/ingresses"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// CreateIngresses create/update VMO ingress k8s resources
func CreateIngresses(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {

	ingList, err := ingresses.New(vmo)
	if err != nil {
		zap.S().Errorf("Failed to create Ingress specs for vmo: %s", err)
		return err
	}
	if vmo.Spec.IngressTargetDNSName == "" {
		zap.S().Debugf("No Ingress target specified, using default Ingress target: '%s'", controller.operatorConfig.DefaultIngressTargetDNSName)
		vmo.Spec.IngressTargetDNSName = controller.operatorConfig.DefaultIngressTargetDNSName
	}
	var ingressNames []string
	controller.log.Oncef("Creating/updating Ingresses for vmo '%s' in namespace '%s'", vmo.Name, vmo.Namespace)
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

		zap.S().Debugf("Applying Ingress '%s' in namespace '%s' for vmo '%s'\n", ingName, vmo.Namespace, vmo.Name)
		existingIngress, err := controller.ingressLister.Ingresses(vmo.Namespace).Get(ingName)
		if existingIngress != nil {
			specDiffs := diff.Diff(existingIngress, curIngress)
			if specDiffs != "" {
				zap.S().Debugf("Ingress %s : Spec differences %s", curIngress.Name, specDiffs)
				_, err = controller.kubeclientset.ExtensionsV1beta1().Ingresses(vmo.Namespace).Update(context.TODO(), curIngress, metav1.UpdateOptions{})
			}
		} else if k8serrors.IsNotFound(err) {
			_, err = controller.kubeclientset.ExtensionsV1beta1().Ingresses(vmo.Namespace).Create(context.TODO(), curIngress, metav1.CreateOptions{})
		} else {
			zap.S().Errorf("Problem getting existing Ingress %s in namespace %s: %v", ingName, vmo.Namespace, err)
			return err
		}

		if err != nil {
			zap.S().Errorf("Failed to apply Ingress for vmo: %s", err)
			return err
		}
	}

	// Delete ingresses that shouldn't exist
	controller.log.Oncef("Deleting unwanted Ingresses for vmo '%s' in namespace '%s'", vmo.Name, vmo.Namespace)
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingIngressList, err := controller.ingressLister.Ingresses(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, ingress := range existingIngressList {
		if !contains(ingressNames, ingress.Name) {
			controller.log.Oncef("Deleting ingress %s", ingress.Name)
			err := controller.kubeclientset.ExtensionsV1beta1().Ingresses(vmo.Namespace).Delete(context.TODO(), ingress.Name, metav1.DeleteOptions{})
			if err != nil {
				zap.S().Errorf("Failed to delete ingress %s, for the reason (%v)", ingress.Name, err)
				return err
			}
		}
	}

	return nil
}

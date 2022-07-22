// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"

	"github.com/verrazzano/pkg/diff"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/metricsexporter"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/ingresses"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// CreateIngresses create/update VMO ingress k8s resources
func CreateIngresses(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	metricsexporter.GetFunctionMetrics(metricsexporter.NamesIngress).LogStart()
	defer metricsexporter.GetFunctionMetrics(metricsexporter.NamesIngress).LogEnd(false)
	ingList, err := ingresses.New(vmo)
	if err != nil {
		controller.log.Errorf("Failed to create Ingress specs for VMI %s: %v", vmo.Name, err)
		metricsexporter.GetFunctionMetrics(metricsexporter.NamesIngress).IncError()
		return err
	}
	if vmo.Spec.IngressTargetDNSName == "" {
		controller.log.Debugf("No Ingress target specified, using default Ingress target: '%s'", controller.operatorConfig.DefaultIngressTargetDNSName)
		vmo.Spec.IngressTargetDNSName = controller.operatorConfig.DefaultIngressTargetDNSName
	}
	var ingressNames []string
	controller.log.Oncef("Creating/updating Ingresses for VMI %s", vmo.Name)
	for _, curIngress := range ingList {
		ingName := curIngress.Name
		ingressNames = append(ingressNames, ingName)
		if ingName == "" {
			// We choose to absorb the error here as the worker would requeue the
			// resource otherwise. Instead, the next time the resource is updated
			// the resource will be queued again.
			runtime.HandleError(errors.New("ingress name must be specified"))
			metricsexporter.GetFunctionMetrics(metricsexporter.NamesIngress).IncError()
			return nil
		}

		controller.log.Debugf("Applying Ingress '%s' in namespace '%s' for VMI '%s'\n", ingName, vmo.Namespace, vmo.Name)
		existingIngress, err := controller.ingressLister.Ingresses(vmo.Namespace).Get(ingName)
		if existingIngress != nil {
			specDiffs := diff.Diff(existingIngress, curIngress)
			if specDiffs != "" {
				controller.log.Debugf("Ingress %s : Spec differences %s", curIngress.Name, specDiffs)
				_, err = controller.kubeclientset.NetworkingV1().Ingresses(vmo.Namespace).Update(context.TODO(), curIngress, metav1.UpdateOptions{})
			}
		} else if k8serrors.IsNotFound(err) {
			_, err = controller.kubeclientset.NetworkingV1().Ingresses(vmo.Namespace).Create(context.TODO(), curIngress, metav1.CreateOptions{})
		} else {
			controller.log.Errorf("Failed getting existing Ingress %s/%s: %v", vmo.Namespace, ingName, err)
			metricsexporter.GetFunctionMetrics(metricsexporter.NamesIngress).IncError()
			return err
		}

		if err != nil {
			controller.log.Errorf("Failed to create/update Ingress %s/%s: %v", vmo.Namespace, ingName, err)
			metricsexporter.GetFunctionMetrics(metricsexporter.NamesIngress).IncError()
			return err
		}
	}

	// Delete ingresses that shouldn't exist
	controller.log.Oncef("Deleting unwanted Ingresses for VMI %s", vmo.Name)
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingIngressList, err := controller.ingressLister.Ingresses(vmo.Namespace).List(selector)
	if err != nil {
		metricsexporter.GetFunctionMetrics(metricsexporter.NamesIngress).IncError()
		return err
	}
	for _, ingress := range existingIngressList {
		if !contains(ingressNames, ingress.Name) {
			controller.log.Oncef("Deleting ingress %s", ingress.Name)
			err := controller.kubeclientset.NetworkingV1().Ingresses(vmo.Namespace).Delete(context.TODO(), ingress.Name, metav1.DeleteOptions{})
			if err != nil {
				controller.log.Errorf("Failed to delete Ingress %s/%s: %v", vmo.Namespace, ingress.Name, err)
				return err
			}
			metricsexporter.GetSimpleCounterMetrics(metricsexporter.NamesIngressDeleted).Inc()
		}
	}

	return nil
}

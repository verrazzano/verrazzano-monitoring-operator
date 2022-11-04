// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"
	"github.com/verrazzano/pkg/diff"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/metricsexporter"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/ingresses"
	netv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// CreateIngresses create/update VMO ingress k8s resources
func CreateIngresses(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	functionMetric, functionError := metricsexporter.GetFunctionMetrics(metricsexporter.NamesIngress)
	if functionError == nil {
		functionMetric.LogStart()
		defer functionMetric.LogEnd(false)
	} else {
		return functionError
	}

	ingList, err := ingresses.New(vmo, getRequiredExistingIngresses(controller, vmo))
	if err != nil {
		controller.log.Errorf("Failed to create Ingress specs for VMI %s: %v", vmo.Name, err)
		functionMetric.IncError()
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
			functionMetric.IncError()
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
			functionMetric.IncError()
			return err
		}

		if err != nil {
			controller.log.Errorf("Failed to create/update Ingress %s/%s: %v", vmo.Namespace, ingName, err)
			functionMetric.IncError()
			return err
		}
	}

	// Delete ingresses that shouldn't exist
	controller.log.Oncef("Deleting unwanted Ingresses for VMI %s", vmo.Name)
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingIngressList, err := controller.ingressLister.Ingresses(vmo.Namespace).List(selector)
	if err != nil {
		functionMetric.IncError()
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
			metric, metricErr := metricsexporter.GetCounterMetrics(metricsexporter.NamesIngressDeleted)
			if metricErr != nil {
				return metricErr
			}
			metric.Inc()
		}
	}

	return nil
}

// For upgrade check, if the user has deprecated Elasticsearch/Kibana ingress
// Then update the new opensearch/opensearchdashboards ingress with additional Elasticsearch/Kibana rule and hosts
// To support access to the deprecated Elasticsearch/Kibana URL. This case is only for upgrade.

// getRequiredExistingIngresses retrieves the required ingress objects
func getRequiredExistingIngresses(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) map[string]*netv1.Ingress {
	existingIngressMap := make(map[string]*netv1.Ingress)

	// Get Elasticsearch ingress object
	ingressName := resources.GetMetaName(vmo.Name, config.ElasticsearchIngest.Name)
	ingressObject, _ := controller.ingressLister.Ingresses(vmo.Namespace).Get(ingressName)
	if ingressObject != nil {
		existingIngressMap[ingressName] = ingressObject
	}
	// Get kibana ingress object
	ingressName = resources.GetMetaName(vmo.Name, config.Kibana.Name)
	ingressObject, _ = controller.ingressLister.Ingresses(vmo.Namespace).Get(ingressName)
	if ingressObject != nil {
		existingIngressMap[ingressName] = ingressObject
	}
	// Get Opensearch ingress object
	ingressName = resources.GetMetaName(vmo.Name, config.OpensearchIngest.Name)
	ingressObject, _ = controller.ingressLister.Ingresses(vmo.Namespace).Get(ingressName)
	if ingressObject != nil {
		existingIngressMap[ingressName] = ingressObject
	}
	// Get Opensearchdashboards ingress object
	ingressName = resources.GetMetaName(vmo.Name, config.OpenSearchDashboards.Name)
	ingressObject, _ = controller.ingressLister.Ingresses(vmo.Namespace).Get(ingressName)
	if ingressObject != nil {
		existingIngressMap[ingressName] = ingressObject
	}
	return existingIngressMap
}

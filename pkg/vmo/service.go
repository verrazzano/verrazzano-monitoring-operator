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
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/services"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// CreateServices creates/updates/deletes VMO service k8s resources
func CreateServices(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	counter, counterErr := metricsexporter.GetSimpleCounterMetrics(metricsexporter.NamesServices)
	if counterErr != nil {
		return counterErr
	}
	counter.Inc()

	useNodeRoleSelectors, err := clusterHasNodeRoleSelectors(controller, vmo)
	if err != nil {
		controller.log.Errorf("Failed to check node role selectors when creating services for VMI %s: %s", vmo.Name, err)
		return err
	}

	svcList, err := services.New(vmo, useNodeRoleSelectors)
	if err != nil {
		controller.log.Errorf("Failed to create Services for VMI %s: %v", vmo.Name, err)
		return err
	}
	var serviceNames []string
	controller.log.Oncef("Creating/updating Services for VMI %s", vmo.Name)
	for _, curService := range svcList {
		serviceName := curService.Name
		serviceNames = append(serviceNames, serviceName)
		if serviceName == "" {
			// We choose to absorb the error here as the worker would requeue the
			// resource otherwise. Instead, the next time the resource is updated
			// the resource will be queued again.
			runtime.HandleError(errors.New("service name must be specified"))
			return nil
		}

		controller.log.Debugf("Applying Service '%s' in namespace '%s' for VMI '%s'\n", serviceName, vmo.Namespace, vmo.Name)
		existingService, err := controller.serviceLister.Services(vmo.Namespace).Get(serviceName)
		if existingService != nil {
			specDiffs := diff.Diff(existingService, curService)
			if specDiffs != "" {
				controller.log.Debugf("Service %s : Spec differences %s", curService.Name, specDiffs)
				err = controller.kubeclientset.CoreV1().Services(vmo.Namespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
				if err != nil {
					controller.log.Errorf("Failed to delete service %s: %v", serviceName, err)
				}
				_, err = controller.kubeclientset.CoreV1().Services(vmo.Namespace).Create(context.TODO(), curService, metav1.CreateOptions{})
			}
		} else {
			_, err = controller.kubeclientset.CoreV1().Services(vmo.Namespace).Create(context.TODO(), curService, metav1.CreateOptions{})
		}

		if err != nil {
			controller.log.Errorf("Failed to apply Service for VMI %s: %v", vmo.Name, err)
			return err
		}
		controller.log.Debugf("Successfully applied Service '%s'\n", serviceName)
		metric, metricErr := metricsexporter.GetSimpleCounterMetrics(metricsexporter.NamesServicesCreated)
		if metricErr != nil {
			return metricErr
		}
		metric.Inc()
	}

	// Delete services that shouldn't exist
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingServicesList, err := controller.serviceLister.Services(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, service := range existingServicesList {
		if !contains(serviceNames, service.Name) {
			controller.log.Debugf("Deleting service %s", service.Name)
			err := controller.kubeclientset.CoreV1().Services(vmo.Namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
			if err != nil {
				controller.log.Errorf("Failed to delete service %s: %v", service.Name, err)
				return err
			}
		}
	}
	metric, metricErr := metricsexporter.GetTimestampMetrics(metricsexporter.NamesServices)
	if metricErr != nil {
		return metricErr
	}
	metric.SetLastTime()
	return nil
}

func clusterHasNodeRoleSelectors(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) (bool, error) {
	selector := services.OpenSearchPodSelector(vmo.Name)
	pods, err := controller.kubeclientset.CoreV1().Pods(vmo.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return false, err
	}
	return services.UseNodeRoleSelector(pods), nil
}

// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"

	"github.com/verrazzano/pkg/diff"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/services"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// CreateServices creates/updates/deletes VMO service k8s resources
func CreateServices(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	svcList, err := services.New(vmo)
	if err != nil {
		zap.S().Errorf("Failed to create Services for vmo: %s", err)
		return err
	}
	var serviceNames []string
	controller.log.Oncef("Creating/updating Services for vmo '%s' in namespace '%s'", vmo.Name, vmo.Namespace)
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

		zap.S().Debugf("Applying Service '%s' in namespace '%s' for vmo '%s'\n", serviceName, vmo.Namespace, vmo.Name)
		existingService, err := controller.serviceLister.Services(vmo.Namespace).Get(serviceName)
		if existingService != nil {
			specDiffs := diff.Diff(existingService, curService)
			if specDiffs != "" {
				zap.S().Debugf("Service %s : Spec differences %s", curService.Name, specDiffs)
				err = controller.kubeclientset.CoreV1().Services(vmo.Namespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
				if err != nil {
					zap.S().Errorf("Failed to delete service %s: %+v", serviceName, err)
				}
				_, err = controller.kubeclientset.CoreV1().Services(vmo.Namespace).Create(context.TODO(), curService, metav1.CreateOptions{})
			}
		} else {
			_, err = controller.kubeclientset.CoreV1().Services(vmo.Namespace).Create(context.TODO(), curService, metav1.CreateOptions{})
		}

		if err != nil {
			zap.S().Errorf("Failed to apply Service for vmo: %s", err)
			return err
		}
		zap.S().Debugf("Successfully applied Service '%s'\n", serviceName)
	}

	// Delete services that shouldn't exist
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingServicesList, err := controller.serviceLister.Services(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, service := range existingServicesList {
		if !contains(serviceNames, service.Name) {
			zap.S().Debugf("Deleting service %s", service.Name)
			err := controller.kubeclientset.CoreV1().Services(vmo.Namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
			if err != nil {
				zap.S().Errorf("Failed to delete service %s, for the reason (%v)", service.Name, err)
				return err
			}
		}
	}

	return nil
}

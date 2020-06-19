// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	"errors"
	"github.com/golang/glog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/services"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

func CreateServices(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	svcList, err := services.New(sauron)
	if err != nil {
		glog.Errorf("Failed to create Services for sauron: %s", err)
		return err
	}
	var serviceNames []string
	glog.V(4).Infof("Creating/updating Services for sauron '%s' in namespace '%s'", sauron.Name, sauron.Namespace)
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

		glog.V(6).Infof("Applying Service '%s' in namespace '%s' for sauron '%s'\n", serviceName, sauron.Namespace, sauron.Name)
		existingService, err := controller.serviceLister.Services(sauron.Namespace).Get(serviceName)
		if existingService != nil {
			specDiffs := diff.CompareIgnoreTargetEmpties(existingService, curService)
			if specDiffs != "" {
				glog.V(4).Infof("Service %s : Spec differences %s", curService.Name, specDiffs)
				err = controller.kubeclientset.CoreV1().Services(sauron.Namespace).Delete(serviceName, &metav1.DeleteOptions{})
				if err != nil {
					glog.Errorf("Failed to delete service %s: %+v", serviceName, err)
				}
				_, err = controller.kubeclientset.CoreV1().Services(sauron.Namespace).Create(curService)
			}
		} else {
			_, err = controller.kubeclientset.CoreV1().Services(sauron.Namespace).Create(curService)
		}

		if err != nil {
			glog.Errorf("Failed to apply Service for sauron: %s", err)
			return err
		}
		glog.V(6).Infof("Successfully applied Service '%s'\n", serviceName)
	}

	// Delete services that shouldn't exist
	selector := labels.SelectorFromSet(map[string]string{constants.SauronLabel: sauron.Name})
	existingServicesList, err := controller.serviceLister.Services(sauron.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, service := range existingServicesList {
		if !contains(serviceNames, service.Name) {
			glog.V(6).Infof("Deleting service %s", service.Name)
			err := controller.kubeclientset.CoreV1().Services(sauron.Namespace).Delete(service.Name, &metav1.DeleteOptions{})
			if err != nil {
				glog.Errorf("Failed to delete service %s, for the reason (%v)", service.Name, err)
				return err
			}
		}
	}

	return nil
}

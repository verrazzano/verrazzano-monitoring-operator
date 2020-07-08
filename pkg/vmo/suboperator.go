// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Returns OwnerReferences to the running deployment of the hyper operator, if it exists, or nil otherwise.
func getHyperOperatorOwnerReferences(controller *Controller) []metav1.OwnerReference {
	selector := labels.SelectorFromSet(map[string]string{constants.K8SAppLabel: "verrazzano-monitoring-operator", constants.HyperOperatorModeLabel: "true"})
	hyperOperatorDeploymentsList, _ := controller.deploymentLister.Deployments(controller.namespace).List(selector)
	if len(hyperOperatorDeploymentsList) == 0 {
		return nil
	}
	// If we find more than 1 Hyper Operator, just use the first
	return []metav1.OwnerReference{
		*metav1.NewControllerRef(hyperOperatorDeploymentsList[0], schema.GroupVersionKind{
			Group:   vmcontrollerv1.SchemeGroupVersion.Group,
			Version: vmcontrollerv1.SchemeGroupVersion.Version,
			Kind:    "Deployment",
		}),
	}
}

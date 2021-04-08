// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"
	"fmt"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/deployments"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/diff"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// CreateDeployments create/update VMO deployment k8s resources
func CreateDeployments(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string, vmoUsername string, vmoPassword string) (dirty bool, err error) {
	// Assigning the following spec members seems like a hack; is any
	// better way to make these values available where the deployments are created?
	vmo.Spec.NatGatewayIPs = controller.operatorConfig.NatGatewayIPs

	deployList, err := deployments.New(vmo, controller.operatorConfig, pvcToAdMap, vmoUsername, vmoPassword, len(controller.clusterInfo.clusterName) > 0)
	if err != nil {
		zap.S().Errorf("Failed to create Deployment specs for vmo: %s", err)
		return false, err
	}

	var prometheusDeployments []*appsv1.Deployment
	var elasticsearchDataDeployments []*appsv1.Deployment
	var deploymentNames []string
	zap.S().Infof("Creating/updating Deployments for vmo '%s' in namespace '%s'", vmo.Name, vmo.Namespace)
	for _, curDeployment := range deployList {
		deploymentName := curDeployment.Name
		deploymentNames = append(deploymentNames, deploymentName)
		if deploymentName == "" && curDeployment.GenerateName == "" {
			// We choose to absorb the error here as the worker would requeue the
			// resource otherwise. Instead, the next time the resource is updated
			// the resource will be queued again.
			runtime.HandleError(errors.New("deployment name must be specified"))
			return true, nil
		}
		zap.S().Debugf("Applying Deployment '%s' in namespace '%s' for vmo '%s'\n", deploymentName, vmo.Namespace, vmo.Name)
		existingDeployment, err := controller.deploymentLister.Deployments(vmo.Namespace).Get(deploymentName)

		if err != nil {
			if k8serrors.IsNotFound(err) {
				_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Create(context.TODO(), curDeployment, metav1.CreateOptions{})
			} else {
				return false, err
			}
		} else if existingDeployment != nil {
			if existingDeployment.Spec.Template.Labels[constants.ServiceAppLabel] == fmt.Sprintf("%s-%s", vmo.Name, config.Prometheus.Name) {
				prometheusDeployments = append(prometheusDeployments, curDeployment)
			} else if existingDeployment.Spec.Template.Labels[constants.ServiceAppLabel] == fmt.Sprintf("%s-%s", vmo.Name, config.ElasticsearchData.Name) {
				elasticsearchDataDeployments = append(elasticsearchDataDeployments, curDeployment)
			} else {
				specDiffs := diff.CompareIgnoreTargetEmpties(existingDeployment, curDeployment)
				if specDiffs != "" {
					zap.S().Debugf("Deployment %s : Spec differences %s", curDeployment.Name, specDiffs)
					_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Update(context.TODO(), curDeployment, metav1.UpdateOptions{})
				}
			}
		}
		if err != nil {
			return false, err
		}
	}

	// Rolling update through Prometheus deployments.  For each, we'll update the *next* candidate
	// deployment (only), then let subsequent runs of this function update the subsequent deployments.
	prometheusDirty, err := updateNextDeployment(controller, vmo, prometheusDeployments)
	if err != nil {
		return false, err
	}
	elasticsearchDirty, err := updateAllDeployment(controller, vmo, elasticsearchDataDeployments)
	if err != nil {
		return false, err
	}

	// Delete deployments that shouldn't exist
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingDeploymentsList, err := controller.deploymentLister.Deployments(vmo.Namespace).List(selector)
	if err != nil {
		return false, err
	}
	for _, deployment := range existingDeploymentsList {
		if !contains(deploymentNames, deployment.Name) {
			zap.S().Debugf("Deleting deployment %s", deployment.Name)
			err := controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			if err != nil {
				zap.S().Errorf("Failed to delete deployment %s, for the reason (%v)", deployment.Name, err)
				return false, err
			}
		}
	}

	return prometheusDirty || elasticsearchDirty, nil
}

// Updates the *next* candidate deployment of the given deployments list.  A deployment is a candidate only if
// its predecessors in the list have already been updated and are fully up and running.
// return false if 1) no errors occurred, and 2) no work was done
func updateNextDeployment(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, deployments []*appsv1.Deployment) (dirty bool, err error) {
	for index, curDeployment := range deployments {
		existingDeployment, err := controller.deploymentLister.Deployments(vmo.Namespace).Get(curDeployment.Name)
		if err != nil {
			return false, err
		}

		// Deployment spec differences, so call Update() and return
		specDiffs := diff.CompareIgnoreTargetEmpties(existingDeployment, curDeployment)
		if specDiffs != "" {
			zap.S().Debugf("Deployment %s : Spec differences %s", curDeployment.Name, specDiffs)
			_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Update(context.TODO(), curDeployment, metav1.UpdateOptions{})
			if err != nil {
				return false, err
			}
			//okay to return dirty=false after updating the *last* deployment
			return index < len(deployments)-1, nil
		}
		// If the (already updated) deployment is not fully up and running, then return
		if existingDeployment.Status.Replicas != 1 || existingDeployment.Status.Replicas != existingDeployment.Status.AvailableReplicas {
			return true, nil
		}
	}
	return false, nil
}

// Update all deployments in the list concurrently
func updateAllDeployment(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, deployments []*appsv1.Deployment) (dirty bool, err error) {
	for _, curDeployment := range deployments {
		_, err := controller.deploymentLister.Deployments(vmo.Namespace).Get(curDeployment.Name)
		if err != nil {
			return false, err
		}

		_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Update(context.TODO(), curDeployment, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

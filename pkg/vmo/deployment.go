// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"
	"fmt"

	"github.com/verrazzano/pkg/diff"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/metricsexporter"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/deployments"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

func updateOpenSearchDashboardsDeployment(osd *appsv1.Deployment, controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	if osd == nil {
		return nil
	}

	var err error
	existingDeployment, err := controller.deploymentLister.Deployments(vmo.Namespace).Get(osd.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Create(context.TODO(), osd, metav1.CreateOptions{})
		} else {
			return err
		}
	} else {
		err = controller.osClient.IsUpdated(vmo)
		if err != nil {
			return err
		}
		err = updateDeployment(controller, vmo, existingDeployment, osd)
	}
	if err != nil {
		metric, metricErr := metricsexporter.GetErrorMetrics(metricsexporter.NamesDeploymentUpdateError)
		if metricErr != nil {
			return metricErr
		}
		metric.Inc()
		controller.log.Errorf("Failed to update deployment %s/%s: %v", osd.Namespace, osd.Name, err)
		return err
	}

	return nil
}

// CreateDeployments create/update VMO deployment k8s resources
func CreateDeployments(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string, existingCluster bool) (dirty bool, err error) {
	// The error count is incremented by the function which calls createDeployment
	functionMetric, functionError := metricsexporter.GetFunctionMetrics(metricsexporter.NamesDeployment)
	if functionError == nil {
		functionMetric.LogStart()
		defer functionMetric.LogEnd(false)
	} else {
		return false, functionError
	}

	// Assigning the following spec members seems like a hack; is any
	// better way to make these values available where the deployments are created?
	vmo.Spec.NatGatewayIPs = controller.operatorConfig.NatGatewayIPs

	expected, err := deployments.New(vmo, controller.kubeclientset, controller.operatorConfig, pvcToAdMap)
	if err != nil {
		controller.log.Errorf("Failed to create Deployment specs for VMI %s: %v", vmo.Name, err)
		return false, err
	}
	deployList := expected.Deployments

	var openSearchDeployments []*appsv1.Deployment
	var deploymentNames []string
	controller.log.Oncef("Creating/updating ExpectedDeployments for VMI %s", vmo.Name)
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
		controller.log.Debugf("Applying Deployment '%s' in namespace '%s' for VMI '%s'\n", deploymentName, vmo.Namespace, vmo.Name)
		existingDeployment, err := controller.deploymentLister.Deployments(vmo.Namespace).Get(deploymentName)

		if err != nil {
			if k8serrors.IsNotFound(err) {
				_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Create(context.TODO(), curDeployment, metav1.CreateOptions{})
			} else {
				return false, err
			}
		} else if existingDeployment != nil {
			if existingDeployment.Spec.Template.Labels[constants.ServiceAppLabel] == fmt.Sprintf("%s-%s", vmo.Name, config.ElasticsearchData.Name) {
				openSearchDeployments = append(openSearchDeployments, curDeployment)
			} else {
				err = updateDeployment(controller, vmo, existingDeployment, curDeployment)
			}
		}
		if err != nil {
			metric, metricErr := metricsexporter.GetErrorMetrics(metricsexporter.NamesDeploymentUpdateError)
			if metricErr != nil {
				return false, metricErr
			}
			metric.Inc()
			controller.log.Errorf("Failed to update deployment %s/%s: %v", curDeployment.Namespace, curDeployment.Name, err)
			return false, err
		}
	}

	openSearchDirty, err := updateOpenSearchDeployments(controller, vmo, openSearchDeployments, existingCluster)
	if err != nil {
		return false, err
	}

	// Create the OSD deployment
	osd := deployments.NewOpenSearchDashboardsDeployment(vmo)
	if osd != nil {
		deploymentNames = append(deploymentNames, osd.Name)
		err = updateOpenSearchDashboardsDeployment(osd, controller, vmo)
		if err != nil {
			return false, err
		}
	}

	// Delete deployments that shouldn't exist
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	existingDeploymentsList, err := controller.deploymentLister.Deployments(vmo.Namespace).List(selector)
	if err != nil {
		return false, err
	}
	for _, deployment := range existingDeploymentsList {
		if !contains(deploymentNames, deployment.Name) {
			// if processing an OpenSearch data node, and the data node is expected and running
			// An OpenSearch health check should be made to prevent unexpected shard allocation
			if deployments.IsOpenSearchDataDeployment(vmo.Name, deployment) && (expected.OpenSearchDataDeployments > 0 || deployment.Status.ReadyReplicas > 0) {
				if err := controller.osClient.IsGreen(vmo); err != nil {
					controller.log.Oncef("Scale down of deployment %s not allowed: cluster health is not green", deployment.Name)
					continue
				}
				return false, deleteDeployment(controller, vmo, deployment)
			}
			if err := deleteDeployment(controller, vmo, deployment); err != nil {
				return false, err
			}
		}
	}

	return openSearchDirty, nil
}

func deleteDeployment(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, deployment *appsv1.Deployment) error {
	metric, metricErr := metricsexporter.GetCounterMetrics(metricsexporter.NamesDeploymentDeleteCounter)
	if metricErr != nil {
		return metricErr
	}
	metric.Inc()
	controller.log.Debugf("Deleting deployment %s", deployment.Name)
	err := controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
	if err != nil {
		metric, metricErr := metricsexporter.GetErrorMetrics(metricsexporter.NamesDeploymentDeleteError)
		if metricErr != nil {
			return metricErr
		}
		metric.Inc()
		controller.log.Errorf("Failed to delete deployment %s: %v", deployment.Name, err)
	}
	return nil
}

func updateDeployment(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, existingDeployment, curDeployment *appsv1.Deployment) error {
	metric, metricErr := metricsexporter.GetCounterMetrics(metricsexporter.NamesDeploymentUpdateCounter)
	if metricErr != nil {
		return metricErr
	}
	metric.Inc()
	var err error
	curDeployment.Spec.Selector = existingDeployment.Spec.Selector
	specDiffs := diff.Diff(existingDeployment, curDeployment)
	if specDiffs != "" {
		controller.log.Oncef("Deployment %s/%s has spec differences %s", curDeployment.Namespace, curDeployment.Name, specDiffs)
		controller.log.Oncef("Updating deployment %s/%s", curDeployment.Namespace, curDeployment.Name)
		_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Update(context.TODO(), curDeployment, metav1.UpdateOptions{})
	}

	return err
}

// Updates the *next* candidate deployment of the given deployments list.  A deployment is a candidate only if
// its predecessors in the list have already been updated and are fully up and running.
// return false if 1) no errors occurred, and 2) no work was done
func rollingUpdate(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, deployments []*appsv1.Deployment) (dirty bool, err error) {
	for index, current := range deployments {
		existing, err := controller.deploymentLister.Deployments(vmo.Namespace).Get(current.Name)
		if err != nil {
			return false, err
		}

		// check if the current node is ready to be updated. If it can't, skip it for the next reconcile
		if !isUpdateAllowed(controller, vmo, current) {
			continue
		}
		metric, metricErr := metricsexporter.GetCounterMetrics(metricsexporter.NamesDeploymentUpdateCounter)
		if metricErr != nil {
			return false, metricErr
		}
		metric.Inc()
		// Selector may not change, so we copy over from existing
		current.Spec.Selector = existing.Spec.Selector
		// Deployment spec differences, so call Update() and return
		specDiffs := diff.Diff(existing, current)
		if specDiffs != "" {
			controller.log.Debugf("Deployment %s : Spec differences %s", current.Name, specDiffs)
			controller.log.Oncef("Updating deployment %s in namespace %s", current.Name, current.Namespace)
			_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Update(context.TODO(), current, metav1.UpdateOptions{})
			if err != nil {
				metric, metricErr := metricsexporter.GetErrorMetrics(metricsexporter.NamesDeploymentUpdateError)
				if metricErr != nil {
					return false, metricErr
				}
				metric.Inc()
				return false, err
			}
			//okay to return dirty=false after updating the *last* deployment
			return index < len(deployments)-1, nil
		}
		// If the (already updated) deployment is not fully up and running, then return
		if existing.Status.Replicas != 1 || existing.Status.Replicas != existing.Status.AvailableReplicas {
			return true, nil
		}
	}
	return false, nil
}

func updateOpenSearchDeployments(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, deployments []*appsv1.Deployment, existingCluster bool) (dirty bool, err error) {
	// if the cluster isn't up, patch all deployments sequentially
	if !existingCluster {
		return updateAllDeployments(controller, vmo, deployments)
	}
	// if the cluster is running, do a rolling update of each deployment
	return rollingUpdate(controller, vmo, deployments)
}

// Update all deployments in the list concurrently
func updateAllDeployments(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, deployments []*appsv1.Deployment) (dirty bool, err error) {
	for _, curDeployment := range deployments {
		_, err := controller.deploymentLister.Deployments(vmo.Namespace).Get(curDeployment.Name)
		if err != nil {
			return false, err
		}
		metric, metricErr := metricsexporter.GetCounterMetrics(metricsexporter.NamesDeploymentUpdateCounter)
		if metricErr != nil {
			return false, metricErr
		}
		metric.Inc()
		_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Update(context.TODO(), curDeployment, metav1.UpdateOptions{})
		if err != nil {
			metric, metricErr := metricsexporter.GetErrorMetrics(metricsexporter.NamesDeploymentUpdateError)
			if metricErr != nil {
				return false, metricErr
			}
			metric.Inc()
			return false, err
		}
	}
	return false, nil
}

//isUpdateAllowed checks if OpenSearch nodes are allowed to update. If a data node is removed when the cluster is yellow,
// data loss may occur.
func isUpdateAllowed(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, current *appsv1.Deployment) bool {
	// if current is an OpenSearch data node
	if deployments.IsOpenSearchDataDeployment(vmo.Namespace, current) {
		// if the node is down, we should try to fix it
		if current.Status.ReadyReplicas == 0 {
			return true
		}

		// if the node is running, we shouldn't take it down unless the cluster is green (to avoid data loss)
		if err := controller.osClient.IsGreen(vmo); err != nil {
			controller.log.Oncef("OpenSearch node %s was not upgraded, since the cluster is not ready", current.Name)
			return false
		}
	}
	return true
}

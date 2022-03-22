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
		err = controller.osClient.IsOpenSearchUpdated(vmo)
		if err != nil {
			return err
		}
		addKibanaUpgradeStrategy(osd, existingDeployment)
		err = updateDeployment(controller, vmo, existingDeployment, osd)
	}
	if err != nil {
		controller.log.Errorf("Failed to update deployment %s/%s: %v", osd.Namespace, osd.Name, err)
		return err
	}

	return nil
}

// CreateDeployments create/update VMO deployment k8s resources
func CreateDeployments(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string, vmoUsername string, vmoPassword string) (dirty bool, err error) {
	// Assigning the following spec members seems like a hack; is any
	// better way to make these values available where the deployments are created?
	vmo.Spec.NatGatewayIPs = controller.operatorConfig.NatGatewayIPs

	deployList, err := deployments.New(vmo, controller.kubeclientset, controller.operatorConfig, pvcToAdMap, vmoUsername, vmoPassword)
	if err != nil {
		controller.log.Errorf("Failed to create Deployment specs for VMI %s: %v", vmo.Name, err)
		return false, err
	}

	var prometheusDeployments []*appsv1.Deployment
	var elasticsearchDataDeployments []*appsv1.Deployment
	var deploymentNames []string
	controller.log.Oncef("Creating/updating Deployments for VMI %s", vmo.Name)
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
			if existingDeployment.Spec.Template.Labels[constants.ServiceAppLabel] == fmt.Sprintf("%s-%s", vmo.Name, config.Prometheus.Name) {
				prometheusDeployments = append(prometheusDeployments, curDeployment)
			} else if existingDeployment.Spec.Template.Labels[constants.ServiceAppLabel] == fmt.Sprintf("%s-%s", vmo.Name, config.ElasticsearchData.Name) {
				elasticsearchDataDeployments = append(elasticsearchDataDeployments, curDeployment)
			} else {
				err = updateDeployment(controller, vmo, existingDeployment, curDeployment)
			}
		}
		if err != nil {
			controller.log.Errorf("Failed to update deployment %s/%s: %v", curDeployment.Namespace, curDeployment.Name, err)
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
			controller.log.Debugf("Deleting deployment %s", deployment.Name)
			err := controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			if err != nil {
				controller.log.Errorf("Failed to delete deployment %s: %v", deployment.Name, err)
				return false, err
			}
		}
	}

	return prometheusDirty || elasticsearchDirty, nil
}

func updateDeployment(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, existingDeployment, curDeployment *appsv1.Deployment) error {
	var err error
	specDiffs := diff.Diff(existingDeployment, curDeployment)
	if specDiffs != "" {
		controller.log.Oncef("Deployment %s/%s has spec differences %s", curDeployment.Namespace, curDeployment.Name, specDiffs)
		controller.log.Oncef("Updating deployment %s/%s", curDeployment.Namespace, curDeployment.Name)
		_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Update(context.TODO(), curDeployment, metav1.UpdateOptions{})
	}

	return err
}

//addKibanaUpgradeStrategy updates the Kibana deployment with an appropriate update strategy,
// whether the upgrade is a rolling upgrade or a recreate upgrade.
func addKibanaUpgradeStrategy(newDeploy, oldDeploy *appsv1.Deployment) {
	getKibanaImage := func(deploy *appsv1.Deployment) string {
		kibanaImage := ""
		for _, container := range deploy.Spec.Template.Spec.Containers {
			if container.Name == config.Kibana.Name {
				kibanaImage = container.Image
				break
			}
		}
		return kibanaImage
	}
	oldKibanaImage := getKibanaImage(oldDeploy)
	newKibanaImage := getKibanaImage(newDeploy)

	// Kibana/OSD should not have concurrent replicas with separate versions
	// this can lead to race conditions and result in data corruption
	newDeploy.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: getUpdateStrategy(newKibanaImage, oldKibanaImage),
	}
}

//getUpdateStrategy returns a deployment strategy for recreate or rolling updates
func getUpdateStrategy(newImage, oldImage string) appsv1.DeploymentStrategyType {
	if newImage == oldImage {
		return appsv1.RollingUpdateDeploymentStrategyType
	}
	return appsv1.RecreateDeploymentStrategyType
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
		specDiffs := diff.Diff(existingDeployment, curDeployment)
		if specDiffs != "" {
			controller.log.Debugf("Deployment %s : Spec differences %s", curDeployment.Name, specDiffs)
			controller.log.Oncef("Updating deployment %s in namespace %s", curDeployment.Name, curDeployment.Namespace)
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

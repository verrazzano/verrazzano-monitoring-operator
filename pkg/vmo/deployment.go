// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/verrazzano/pkg/diff"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/metricsexporter"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/deployments"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

const (
	grafanaAdminAnnotation = "grafana-admin-update"
	authProxyEnvName       = "GF_AUTH_PROXY_ENABLED"
	basicAuthEnvName       = "GF_AUTH_BASIC_ENABLED"
	vzUserEnvName          = "GF_SECURITY_ADMIN_USER"
	vzPassEnvName          = "GF_SECURITY_ADMIN_PASSWORD" //nolint:gosec
)

// GrafanaAdminState Grafana Admin Update state enum
type grafanaAdminState int

const (
	Setup grafanaAdminState = iota
	Request
	Complete
)

// grafanaUserInfo is the expected response body for a user info request from Grafana
type grafanaUserInfo struct {
	ID             int         `json:"id"`
	Email          string      `json:"email"`
	Name           string      `json:"name"`
	Login          string      `json:"login"`
	Theme          string      `json:"theme"`
	OrgID          int         `json:"orgId"`
	IsGrafanaAdmin bool        `json:"isGrafanaAdmin"`
	IsDisabled     bool        `json:"isDisabled"`
	IsExternal     bool        `json:"isExternal"`
	AuthLabels     interface{} `json:"authLabels"`
	UpdatedAt      time.Time   `json:"updatedAt"`
	CreatedAt      time.Time   `json:"createdAt"`
	AvatarURL      string      `json:"avatarUrl"`
}

// grafanaUserRegistration is used to register a new user in Grafana
type grafanaUserRegistration struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

// grafanaAdminRequest is the request body used to grant a user Grafana admin permissions
type grafanaAdminRequest struct {
	IsGrafanaAdmin bool `json:"isGrafanaAdmin"`
}

// grafanaAdminResponse is the expected response body from a Grafana admin permissions request
type grafanaMessageResponse struct {
	Message string `json:"message"`
}

func updateOpenSearchDashboardsDeployment(osd *appsv1.Deployment, controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	if osd == nil {
		return nil
	}
	var err error

	// Wait for OS to be green before deploying OS Dashboards
	if err = controller.osClient.IsGreen(vmo); err != nil {
		return err
	}

	existingDeployment, err := controller.deploymentLister.Deployments(vmo.Namespace).Get(osd.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			controller.log.Oncef("Creating deployment %s/%s", osd.Namespace, osd.Name)
			// Initialize the replica count to one, and scale up one at a time during update.
			// The OS Dashboard pods are being rolled out one at a time to avoid getting failures
			// due to indices needing to be migrated.  We considered using StatefulSets with a
			// pod management policy of "ordered ready".  However, StatefulSets do not support a
			// deployment strategy of "recreate", which is also needed to avoid the migrating indices error.
			osd.Spec.Replicas = resources.NewVal(int32(1))
			_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Create(context.TODO(), osd, metav1.CreateOptions{})
		} else {
			return err
		}
	} else {
		if err = controller.osClient.IsUpdated(vmo); err != nil {
			return err
		}
		if existingDeployment.Status.AvailableReplicas == *existingDeployment.Spec.Replicas &&
			*resources.NewVal(vmo.Spec.Kibana.Replicas) > *existingDeployment.Spec.Replicas {
			// Ok to scale up
			*osd.Spec.Replicas = *existingDeployment.Spec.Replicas + 1
			controller.log.Oncef("Incrementing replica count of deployment %s/%s to %d", osd.Namespace, osd.Name, *osd.Spec.Replicas)
		}
		if err = updateDeployment(controller, vmo, existingDeployment, osd); err == nil {
			// Return a temporary error if not finished scaling up to the desired replica count
			if *resources.NewVal(vmo.Spec.Kibana.Replicas) != *existingDeployment.Spec.Replicas {
				return fmt.Errorf("waiting to bring OS Dashboards replica up to full count")
			}
		}
	}
	if err != nil {
		if metric, metricErr := metricsexporter.GetErrorMetrics(metricsexporter.NamesDeploymentUpdateError); metricErr != nil {
			controller.log.Errorf("Failed to get error metric %s: %v", metricsexporter.NamesDeploymentUpdateError, metricErr)
		} else {
			metric.Inc()
		}
		controller.log.Errorf("Failed to update deployment %s/%s: %v", osd.Namespace, osd.Name, err)
		return err
	}

	return nil
}

// updateGrafanaAdminUser updates the Grafana deployment to make the Verrazzano user a Grafana Admin
func updateGrafanaAdminUser(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, grafanaDeployment *appsv1.Deployment, curDeployment *appsv1.Deployment) (bool, error) {
	grafanaURL := url.URL{
		Scheme: "HTTP",
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local:3000", resources.GetMetaName(vmo.Name, config.Grafana.Name), vmo.Namespace),
	}
	grafanaURL.User = url.UserPassword("admin", "admin")

	grafanaState, err := determineGrafanaState(controller, grafanaDeployment, grafanaURL)
	if err != nil {
		return false, err
	}

	switch grafanaState {
	case Setup:
		controller.log.Oncef("Setting up the Grafana deployment for the Verrazzano user admin update")
		// Update the existing authentication env vars to enable basic auth
		authBasicEnabled := false
		grafanaEnvPtr := &curDeployment.Spec.Template.Spec.Containers[0].Env
		for i, envVar := range *grafanaEnvPtr {
			if envVar.Name == authProxyEnvName {
				(*grafanaEnvPtr)[i].Value = "false"
			}
			if envVar.Name == basicAuthEnvName {
				(*grafanaEnvPtr)[i].Value = "true"
				authBasicEnabled = true
			}
		}
		// Ensure that basic auth is enabled, even if it was not present in the existing env vars
		if !authBasicEnabled {
			*grafanaEnvPtr = append(curDeployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: basicAuthEnvName, Value: "true"})
		}

		// Update the existing deployment to match the newly modified deployment
		return true, updateDeployment(controller, vmo, grafanaDeployment, curDeployment)
	case Request:
		controller.log.Oncef("Requesting the Grafana deployment for the Verrazzano user admin update")
		return true, requestGrafanaAdmin(controller, grafanaURL)
	case Complete:
		controller.log.Oncef("The Verrazzano user admin update for Grafana has completed successfully")
		return false, updateDeployment(controller, vmo, grafanaDeployment, curDeployment)
	}
	return false, nil
}

// determineGrafanaState returns a Grafana Admin State based on the status of the Grafana deployment
func determineGrafanaState(controller *Controller, deployment *appsv1.Deployment, grafanaURL url.URL) (grafanaAdminState, error) {
	// Collect necessary information from the Grafana pod env vars
	basicAuthEnabled := false
	authProxyEnabled := false
	var vzUserSecretName string
	for _, envVar := range deployment.Spec.Template.Spec.Containers[0].Env {
		if envVar.Name == authProxyEnvName && envVar.Value == "true" {
			authProxyEnabled = true
		}
		if envVar.Name == basicAuthEnvName && envVar.Value == "true" {
			basicAuthEnabled = true
		}
		if (envVar.Name == vzUserEnvName || envVar.Name == vzPassEnvName) && envVar.ValueFrom.SecretKeyRef != nil {
			vzUserSecretName = envVar.ValueFrom.SecretKeyRef.Name
		}
	}

	// Get the secret data from the Grafana admin credential secret
	vzUserSecret, err := controller.secretLister.Secrets(deployment.Namespace).Get(vzUserSecretName)
	if err != nil {
		controller.log.Errorf("Failed to get the Grafana Verrazzano credential secret %s: %v", vzUserSecretName, err)
		return 0, err
	}
	vzUser := string(vzUserSecret.Data["username"])
	vzPass := string(vzUserSecret.Data["password"])

	// Always check that the Verrazzano is not the admin user first
	// This prevents the pod from continually getting recycled and updated
	// Setup Get request as the verrazzano user, we will use the Grafana credential secret for authentication
	grafanaURL.Path = "api/users/lookup"
	grafanaURL.RawQuery = "loginOrEmail=verrazzano"
	grafanaURL.User = url.UserPassword(vzUser, vzPass)
	grafanaResponse, err := http.Get(grafanaURL.String())
	if err != nil && grafanaResponse.StatusCode != 404 {
		controller.log.Errorf("Failed to get Verrazzano user information from Grafana with request %s: %v", grafanaURL.String(), err)
		return 0, err
	}
	if grafanaResponse.StatusCode == 200 {
		var vzUserInfo grafanaUserInfo
		err = json.NewDecoder(grafanaResponse.Body).Decode(&vzUserInfo)
		if err != nil {
			controller.log.Errorf("Failed to decode the response body of the successful verrazzano request: %v", err)
			return 0, err
		}
		if vzUserInfo.IsGrafanaAdmin {
			return Complete, nil
		}
	}
	if grafanaResponse.StatusCode == 503 {
		controller.log.Progressf("Waiting for Grafana pod to be ready before request, status: %d", grafanaResponse.StatusCode)
		return Setup, nil
	}

	controller.log.Progressf("Request to get Verrazzano user info from from the Grafana pod was unsuccessful status: %d", grafanaResponse.StatusCode)

	// Check the deployment pod env vars to determine if basic auth is enabled
	// If so, we should enter the request state
	// If not, we should enter the setup state
	if basicAuthEnabled && !authProxyEnabled {
		return Request, nil
	}
	return Setup, nil
}

// requestGrafanaAdmin handles the request to Grafana to grant the Verrazzano user admin permissions
func requestGrafanaAdmin(controller *Controller, grafanaURL url.URL) error {
	// Get the Verrazzano user ID for the admin request
	grafanaURL.Path = "api/users/lookup"
	grafanaURL.RawQuery = "loginOrEmail=verrazzano"
	grafanaResponse, err := http.Get(grafanaURL.String())
	if err != nil && grafanaResponse.StatusCode != 404 {
		controller.log.Errorf("Failed to get Verrazzano user information from Grafana with request %s, status %d: %v", grafanaURL.String(), grafanaResponse.StatusCode, err)
		return err
	}
	// if the Verrazzano user is not found, we need to create one
	// this occurs if the console is not accessed before this process is run
	if grafanaResponse.StatusCode == 404 {
		controller.log.Once("Failed to find Verrazzano user in Grafana, creating now")
		vzUserReg := grafanaUserRegistration{
			Name:  "",
			Email: "verrazzano",
			Login: "verrazzano",
			// because we use an auth proxy to authenticate, we do not need to supply a valid password here
			// This field is required, so it must be populated
			Password: "verrazzano",
		}
		requestData, errReg := json.Marshal(&vzUserReg)
		if errReg != nil {
			controller.log.Errorf("Failed to encode the request to create the Verrazzano user: %v", errReg)
			return errReg
		}
		grafanaURL.Path = "api/admin/users"
		grafanaURL.RawQuery = ""
		registerResponse, errReg := http.Post(grafanaURL.String(), "application/json", bytes.NewBuffer(requestData))
		if errReg != nil || registerResponse.StatusCode != 200 {
			controller.log.Errorf("Failed to request the Verrazzano user creation, status %d: %v", registerResponse.StatusCode, errReg)
			return errReg
		}
		return err
	}

	var vzUserInfo grafanaUserInfo
	err = json.NewDecoder(grafanaResponse.Body).Decode(&vzUserInfo)
	if err != nil {
		controller.log.Errorf("Failed to decode the response body of the Verrazzano user information %s: %v", err)
		return err
	}

	// Request that the Verrazzano user be Grafana Admin
	grafanaURL.Path = fmt.Sprintf("api/admin/users/%d/permissions", vzUserInfo.ID)
	grafanaURL.RawQuery = ""
	requestData, err := json.Marshal(&grafanaAdminRequest{IsGrafanaAdmin: true})
	if err != nil {
		controller.log.Errorf("Failed to encode the Grafana admin request body data: %v", err)
		return err
	}
	grafanaRequest, err := http.NewRequest(http.MethodPut, grafanaURL.String(), bytes.NewBuffer(requestData))
	if err != nil {
		controller.log.Errorf("Failed to create request for admin permissions for the Verrazzano user: %v", err)
		return err
	}
	grafanaRequest.Header.Set("Content-type", "application/json")
	client := http.Client{}
	grafanaResponse, err = client.Do(grafanaRequest)
	if err != nil || grafanaResponse.StatusCode != 200 {
		controller.log.Errorf("Failed to request admin permissions for the Verrazzano user, status %d: %v", grafanaResponse.StatusCode, err)
		return err
	}

	// Verify the response gives a valid response message
	var adminUserResponse grafanaMessageResponse
	err = json.NewDecoder(grafanaResponse.Body).Decode(&adminUserResponse)
	if err != nil {
		controller.log.Errorf("Failed to decode the response body of the Verrazzano admin request: %v", err)
		return err
	}
	if adminUserResponse.Message != "User permissions updated" {
		controller.log.Errorf("Failed to update user permissions, Grafana response: %s", adminUserResponse.Message)
	}

	controller.log.Once("Verrazzano user successfully updated to Grafana admin")
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
	var grafanaDirty bool
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
			// if the Grafana Admin annotation is set to "true", do the Grafana Admin update process
			if val, ok := vmo.Annotations[grafanaAdminAnnotation]; ok && val == "true" && strings.Contains(curDeployment.Name, resources.GetMetaName(vmo.Name, config.Grafana.Name)) {
				// The reconciliation for the Deployment update is only passed through once without an error
				// to continue the Grafana update, we must reconcile all states at once
				contGrafanaUpdate := true
				for contGrafanaUpdate {
					contGrafanaUpdate, err = updateGrafanaAdminUser(controller, vmo, existingDeployment, curDeployment)
					if err != nil {
						return false, err
					}
				}
			} else if existingDeployment.Spec.Template.Labels[constants.ServiceAppLabel] == fmt.Sprintf("%s-%s", vmo.Name, config.ElasticsearchData.Name) {
				openSearchDeployments = append(openSearchDeployments, curDeployment)
			} else {
				err = updateDeployment(controller, vmo, existingDeployment, curDeployment)
			}
		}
		if err != nil {
			if metric, metricErr := metricsexporter.GetErrorMetrics(metricsexporter.NamesDeploymentUpdateError); metricErr != nil {
				controller.log.Errorf("Failed to get error metric %s: %v", metricsexporter.NamesDeploymentUpdateError, metricErr)
			} else {
				metric.Inc()
			}
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
	controller.log.Oncef("Deleting deployments that should not exist for VMI %s", vmo.Name)
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
			}
			if err := deleteDeployment(controller, vmo, deployment); err != nil {
				return false, err
			}
		}
	}

	return openSearchDirty || grafanaDirty, nil
}

func deleteDeployment(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, deployment *appsv1.Deployment) error {
	controller.log.Oncef("Deleting deployment %s/%s", deployment.Namespace, deployment.Name)
	metric, err := metricsexporter.GetCounterMetrics(metricsexporter.NamesDeploymentDeleteCounter)
	if err != nil {
		// log it but continue on with deleting the deployment
		controller.log.Errorf("Failed to get counter metric %s: %v", metricsexporter.NamesDeploymentDeleteCounter, err)
	} else {
		metric.Inc()
	}
	err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
	if err != nil {
		controller.log.Errorf("Failed to delete deployment %s: %v", deployment.Name, err)
		if metric, metricErr := metricsexporter.GetErrorMetrics(metricsexporter.NamesDeploymentDeleteError); metricErr != nil {
			controller.log.Errorf("Failed to get error metric %s: %v", metricsexporter.NamesDeploymentDeleteError, metricErr)
		} else {
			metric.Inc()
		}
		return err
	}
	return nil
}

func updateDeployment(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, existingDeployment, curDeployment *appsv1.Deployment) error {
	if metric, metricErr := metricsexporter.GetCounterMetrics(metricsexporter.NamesDeploymentUpdateCounter); metricErr != nil {
		controller.log.Errorf("Failed to get error metric %s: %v", metricsexporter.NamesDeploymentUpdateCounter, metricErr)
	} else {
		metric.Inc()
	}
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
				if metric, metricErr := metricsexporter.GetErrorMetrics(metricsexporter.NamesDeploymentUpdateError); err != nil {
					controller.log.Errorf("Failed to get error metric %s: %v", metricsexporter.NamesDeploymentUpdateError, metricErr)
				} else {
					metric.Inc()
				}
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
		controller.log.Oncef("Updating deployment %s in namespace %s", curDeployment.Name, curDeployment.Namespace)
		_, err = controller.kubeclientset.AppsV1().Deployments(vmo.Namespace).Update(context.TODO(), curDeployment, metav1.UpdateOptions{})
		if err != nil {
			if metric, metricErr := metricsexporter.GetErrorMetrics(metricsexporter.NamesDeploymentUpdateError); metricErr != nil {
				controller.log.Errorf("Failed to get error metric %s: %v", metricsexporter.NamesDeploymentUpdateError, metricErr)
			} else {
				metric.Inc()
			}
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

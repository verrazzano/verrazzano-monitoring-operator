// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package services

import (
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Creates OpenSearch Client service element
func createOpenSearchIngestServiceElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	var openSearchIngestService = createServiceElement(vmo, config.ElasticsearchIngest)
	if nodes.IsSingleNodeCluster(vmo) {
		openSearchIngestService.Spec.Selector = resources.GetSpecID(vmo.Name, config.ElasticsearchMaster.Name)
		// In dev mode, only a single node/pod all ingest/data goes to the 9200 port on the back end node
		openSearchIngestService.Spec.Ports = []corev1.ServicePort{resources.GetServicePort(config.ElasticsearchData)}
	}
	return openSearchIngestService
}

// Creates OpenSearch Client service element
func createOSIngestServiceElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	var openSearchIngestService = createServiceElement(vmo, config.OpensearchIngest)
	if nodes.IsSingleNodeCluster(vmo) {
		openSearchIngestService.Spec.Selector = resources.GetSpecID(vmo.Name, config.ElasticsearchMaster.Name)
		// In dev mode, only a single node/pod all ingest/data goes to the 9200 port on the back end node
		openSearchIngestService.Spec.Ports = []corev1.ServicePort{resources.GetServicePort(config.ElasticsearchData)}
	}
	return openSearchIngestService
}

// Creates OpenSearch MasterNodes service element
func createOpenSearchMasterServiceElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	openSearchMasterService := createServiceElement(vmo, config.ElasticsearchMaster)
	if !nodes.IsSingleNodeCluster(vmo) {
		// MasterNodes service is headless
		openSearchMasterService.Spec.Type = corev1.ServiceTypeClusterIP
		openSearchMasterService.Spec.ClusterIP = corev1.ClusterIPNone
	}
	return openSearchMasterService
}

// Creates OpenSearch DataNodes service element
func createOpenSearchDataServiceElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	var openSearchDataService = createServiceElement(vmo, config.ElasticsearchData)
	if nodes.IsSingleNodeCluster(vmo) {
		// In dev mode, only a single node/pod all ingest/data goes to the 9200 port on the back end node
		openSearchDataService.Spec.Selector = resources.GetSpecID(vmo.Name, config.ElasticsearchMaster.Name)
		openSearchDataService.Spec.Ports[0].TargetPort = intstr.FromInt(constants.OSHTTPPort)
	}
	return openSearchDataService
}

// Creates the master HTTP Service with Cluster IP
func createMasterServiceHTTP(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	masterHTTPService := createServiceElement(vmo, config.ElasticsearchMaster)
	masterHTTPService.Name = masterHTTPService.Name + "-http"
	masterHTTPService.Spec.Ports[0].Name = "http-" + config.ElasticsearchMaster.Name
	masterHTTPService.Spec.Ports[0].Port = constants.OSHTTPPort
	masterHTTPService.Spec.Ports[0].TargetPort = intstr.FromInt(constants.OSHTTPPort)
	return masterHTTPService
}

// Creates *all* OpenSearch service elements
func createOpenSearchServiceElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, useNodeRoleSelectors bool) []*corev1.Service {
	masterService := createOpenSearchMasterServiceElements(vmo)
	masterServiceHTTP := createMasterServiceHTTP(vmo)
	dataService := createOpenSearchDataServiceElements(vmo)
	ingestService := createOpenSearchIngestServiceElements(vmo)
	ingestServiceOS := createOSIngestServiceElements(vmo)

	// if the cluster supports node role selectors, use those instead of service app selectors
	if useNodeRoleSelectors {
		masterService.Spec.Selector = map[string]string{nodes.RoleMaster: nodes.RoleAssigned}
		masterServiceHTTP.Spec.Selector = map[string]string{nodes.RoleMaster: nodes.RoleAssigned}
		dataService.Spec.Selector = map[string]string{nodes.RoleData: nodes.RoleAssigned}
		ingestService.Spec.Selector = map[string]string{nodes.RoleIngest: nodes.RoleAssigned}
	}

	return []*corev1.Service{
		masterService,
		masterServiceHTTP,
		dataService,
		ingestService,
		ingestServiceOS,
	}
}

// OpenSearchPodSelector creates a pod selector like
// 'app in (system-es-master, system-es-data, system-es-ingest)'
// to select all pods in the vmi cluster
func OpenSearchPodSelector(vmoName string) string {
	return fmt.Sprintf("%s in (%s, %s, %s)",
		constants.ServiceAppLabel,
		fmt.Sprintf("%s-%s", vmoName, config.ElasticsearchMaster.Name),
		fmt.Sprintf("%s-%s", vmoName, config.ElasticsearchData.Name),
		fmt.Sprintf("%s-%s", vmoName, config.ElasticsearchIngest.Name),
	)
}

// UseNodeRoleSelector verifies if all OpenSearch pods are using node role selectors.
// If all pods are using node role selectors, this implies service selectors can be updated
// to use node roles instead of service app labels.
func UseNodeRoleSelector(pods *corev1.PodList) bool {
	for _, pod := range pods.Items {
		_, isData := pod.Labels[nodes.RoleData]
		_, isIngest := pod.Labels[nodes.RoleIngest]
		_, isMaster := pod.Labels[nodes.RoleMaster]

		if !isData && !isIngest && !isMaster {
			return false
		}
	}
	return true
}

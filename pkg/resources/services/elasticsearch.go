// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package services

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Creates Elasticsearch Client service element
func createElasticsearchIngestServiceElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	var elasticsearchIngestService = createServiceElement(vmo, config.ElasticsearchIngest)
	if resources.IsSingleNodeESCluster(vmo) {
		elasticsearchIngestService.Spec.Selector = resources.GetSpecID(vmo.Name, config.ElasticsearchMaster.Name)
		// In dev mode, only a single node/pod all ingest/data goes to the 9200 port on the back end node
		elasticsearchIngestService.Spec.Ports = []corev1.ServicePort{resources.GetServicePort(config.ElasticsearchData)}
	}
	return elasticsearchIngestService
}

// Creates Elasticsearch Master service element
func createElasticsearchMasterServiceElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	elasticSearchMasterService := createServiceElement(vmo, config.ElasticsearchMaster)
	if !resources.IsSingleNodeESCluster(vmo) {
		// Master service is headless
		elasticSearchMasterService.Spec.Type = corev1.ServiceTypeClusterIP
		elasticSearchMasterService.Spec.ClusterIP = corev1.ClusterIPNone
	}
	return elasticSearchMasterService
}

// Creates Elasticsearch Data service element
func createElasticsearchDataServiceElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	var elasticsearchDataService = createServiceElement(vmo, config.ElasticsearchData)
	if resources.IsSingleNodeESCluster(vmo) {
		// In dev mode, only a single node/pod all ingest/data goes to the 9200 port on the back end node
		elasticsearchDataService.Spec.Selector = resources.GetSpecID(vmo.Name, config.ElasticsearchMaster.Name)
		elasticsearchDataService.Spec.Ports[0].TargetPort = intstr.FromInt(constants.ESHttpPort)
	}
	return elasticsearchDataService
}

// Creates *all* Elasticsearch service elements
func createElasticsearchServiceElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []*corev1.Service {
	var services []*corev1.Service
	services = append(services, createElasticsearchIngestServiceElements(vmo))
	services = append(services, createElasticsearchMasterServiceElements(vmo))
	services = append(services, createElasticsearchDataServiceElements(vmo))
	return services
}

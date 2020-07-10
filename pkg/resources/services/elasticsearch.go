// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package services

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
)

// Creates Elasticsearch Client service element
func createElasticsearchIngestServiceElements(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	return createServiceElement(vmi, config.ElasticsearchIngest)
}

// Creates Elasticsearch Master service element
func createElasticsearchMasterServiceElements(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	elasticSearchMasterService := createServiceElement(vmi, config.ElasticsearchMaster)

	// Master service is headless
	elasticSearchMasterService.Spec.Type = corev1.ServiceTypeClusterIP
	elasticSearchMasterService.Spec.ClusterIP = corev1.ClusterIPNone
	return elasticSearchMasterService
}

// Creates Elasticsearch Data service element
func createElasticsearchDataServiceElements(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) *corev1.Service {
	elasticsearchDataService := createServiceElement(vmi, config.ElasticsearchData)

	// Data k8s service only needs to expose NodeExporter port
	elasticsearchDataService.Spec.Ports = []corev1.ServicePort{resources.GetServicePort(config.NodeExporter)}
	return elasticsearchDataService
}

// Creates *all* Elasticsearch service elements
func createElasticsearchServiceElements(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) []*corev1.Service {
	var services []*corev1.Service
	services = append(services, createElasticsearchIngestServiceElements(vmi))
	services = append(services, createElasticsearchMasterServiceElements(vmi))
	services = append(services, createElasticsearchDataServiceElements(vmi))
	return services
}

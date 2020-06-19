// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package services

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New creates a new Service for a Sauron resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Sauron resource that 'owns' it.
func New(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) ([]*corev1.Service, error) {
	var services []*corev1.Service

	if sauron.Spec.Grafana.Enabled {
		service := createServiceElement(sauron, config.Grafana)
		services = append(services, service)
	}
	if sauron.Spec.Prometheus.Enabled {
		service := createServiceElement(sauron, config.Prometheus)
		service.Spec.Ports = append(service.Spec.Ports, resources.GetServicePort(config.NodeExporter))
		services = append(services, service)
		services = append(services, createServiceElement(sauron, config.PrometheusGW))
	}
	if sauron.Spec.AlertManager.Enabled {
		alertManagerService := createServiceElement(sauron, config.AlertManager)
		services = append(services, alertManagerService)

		alertManagerClusterService := createServiceElement(sauron, config.AlertManagerCluster)
		alertManagerClusterService.Spec.Selector = resources.GetSpecId(sauron.Name, config.AlertManager.Name)
		alertManagerClusterService.Spec.Type = corev1.ServiceTypeClusterIP
		alertManagerClusterService.Spec.ClusterIP = corev1.ClusterIPNone
		services = append(services, alertManagerClusterService)
	}
	if sauron.Spec.Elasticsearch.Enabled {
		services = append(services, createElasticsearchServiceElements(sauron)...)
	}
	if sauron.Spec.Kibana.Enabled {
		service := createServiceElement(sauron, config.Kibana)
		services = append(services, service)
	}

	services = append(services, createServiceElement(sauron, config.Api))

	return services, nil
}
func createServiceElement(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, componentDetails config.ComponentDetails) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resources.GetMetaLabels(sauron),
			Name:            resources.GetMetaName(sauron.Name, componentDetails.Name),
			Namespace:       sauron.Namespace,
			OwnerReferences: resources.GetOwnerReferences(sauron),
		},
		Spec: corev1.ServiceSpec{
			Type:     sauron.Spec.ServiceType,
			Selector: resources.GetSpecId(sauron.Name, componentDetails.Name),
			Ports:    []corev1.ServicePort{resources.GetServicePort(componentDetails)},
		},
	}
}

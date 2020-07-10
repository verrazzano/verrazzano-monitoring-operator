// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package services

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New creates a new Service for a VMI resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the VMI resource that 'owns' it.
func New(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) ([]*corev1.Service, error) {
	var services []*corev1.Service

	if vmi.Spec.Grafana.Enabled {
		service := createServiceElement(vmi, config.Grafana)
		services = append(services, service)
	}
	if vmi.Spec.Prometheus.Enabled {
		service := createServiceElement(vmi, config.Prometheus)
		service.Spec.Ports = append(service.Spec.Ports, resources.GetServicePort(config.NodeExporter))
		services = append(services, service)
		services = append(services, createServiceElement(vmi, config.PrometheusGW))
	}
	if vmi.Spec.AlertManager.Enabled {
		alertManagerService := createServiceElement(vmi, config.AlertManager)
		services = append(services, alertManagerService)

		alertManagerClusterService := createServiceElement(vmi, config.AlertManagerCluster)
		alertManagerClusterService.Spec.Selector = resources.GetSpecId(vmi.Name, config.AlertManager.Name)
		alertManagerClusterService.Spec.Type = corev1.ServiceTypeClusterIP
		alertManagerClusterService.Spec.ClusterIP = corev1.ClusterIPNone
		services = append(services, alertManagerClusterService)
	}
	if vmi.Spec.Elasticsearch.Enabled {
		services = append(services, createElasticsearchServiceElements(vmi)...)
	}
	if vmi.Spec.Kibana.Enabled {
		service := createServiceElement(vmi, config.Kibana)
		services = append(services, service)
	}

	services = append(services, createServiceElement(vmi, config.Api))

	return services, nil
}
func createServiceElement(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, componentDetails config.ComponentDetails) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resources.GetMetaLabels(vmi),
			Name:            resources.GetMetaName(vmi.Name, componentDetails.Name),
			Namespace:       vmi.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmi),
		},
		Spec: corev1.ServiceSpec{
			Type:     vmi.Spec.ServiceType,
			Selector: resources.GetSpecId(vmi.Name, componentDetails.Name),
			Ports:    []corev1.ServicePort{resources.GetServicePort(componentDetails)},
		},
	}
}

// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InitializeVMOSpec initializes any uninitialized elements of the VMO spec.
func InitializeVMOSpec(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	// The secretName we use for basic authentication in the Nginx ingress controller
	vmo.Spec.SecretName = vmo.Name + "-basicauth"

	/*********************
	 * Create Secrets
	 **********************/
	controller.log.Oncef("Loading auth secret data")
	credsMap, err := controller.loadAllAuthSecretData(vmo.Namespace, vmo.Spec.SecretsName)
	if err != nil {
		controller.log.Errorf("Failed to extract VMO Secrets for VMI %s: %v", vmo.Name, err)
	}

	controller.log.Oncef("Reconciling auth secrets")
	err = CreateOrUpdateAuthSecrets(controller, vmo, credsMap)
	if err != nil {
		controller.log.Errorf("Failed to create VMO Secrets for VMI %s: %v", vmo.Name, err)
	}

	// Create TLS secrets or get certs
	controller.log.Oncef("Reconciling TLS secrets")
	err = CreateOrUpdateTLSSecrets(controller, vmo)
	if err != nil {
		controller.log.Errorf("Failed to create TLS Secrets for VMI %s: %v", vmo.Name, err)
	}

	// Set creation time
	if vmo.Status.CreationTime == nil {
		now := metav1.Now()
		vmo.Status.CreationTime = &now
	}

	// Set environment
	if vmo.Status.EnvName == "" {
		vmo.Status.EnvName = controller.operatorConfig.EnvName
	}

	// Service type
	if vmo.Spec.ServiceType == "" {
		vmo.Spec.ServiceType = corev1.ServiceTypeClusterIP
	}

	// Referenced ConfigMaps
	if vmo.Spec.Grafana.DashboardsConfigMap == "" {
		vmo.Spec.Grafana.DashboardsConfigMap = resources.GetMetaName(vmo.Name, constants.DashboardConfig)
	}
	if vmo.Spec.Grafana.DatasourcesConfigMap == "" {
		vmo.Spec.Grafana.DatasourcesConfigMap = resources.GetMetaName(vmo.Name, constants.DatasourceConfig)
	}

	// Kibana to OpenSearch Dashboards CR conversion
	handleOpensearchDashboardsConversion(&vmo.Spec)

	// Number of replicas for each component
	if vmo.Spec.OpensearchDashboards.Replicas == 0 {
		vmo.Spec.OpensearchDashboards.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}

	// Elasticsearch to OpenSearch CR conversion
	handleOpensearchConversion(&vmo.Spec)

	// Default roles for VMO components
	initNode(&vmo.Spec.Opensearch.MasterNode, vmcontrollerv1.MasterRole)
	initNode(&vmo.Spec.Opensearch.IngestNode, vmcontrollerv1.IngestRole)
	initNode(&vmo.Spec.Opensearch.DataNode, vmcontrollerv1.DataRole)

	// Setup default storage elements
	for _, component := range config.StorageEnableComponents {
		storageElement := resources.GetStorageElementForComponent(vmo, component)
		replicas := int(resources.GetReplicasForComponent(vmo, component))
		pvcName := resources.GetMetaName(vmo.Name, component.Name)
		initStorageElement(storageElement, replicas, pvcName)
	}

	// Setup data node storage elements
	for _, node := range nodes.DataNodes(vmo) {
		initStorageElement(node.Storage, int(node.Replicas), resources.GetMetaName(vmo.Name, node.Name))
	}

	// Overall status
	if vmo.Status.State == "" {
		vmo.Status.State = string(constants.Running)
	}

	// set label for managed-cluster-name
	vmo.Labels[constants.ClusterNameData] = controller.clusterInfo.clusterName
}

func initNode(node *vmcontrollerv1.ElasticsearchNode, role vmcontrollerv1.NodeRole) {
	if len(node.Name) < 1 {
		node.Name = "es-" + string(role)
	}
	if len(node.Roles) < 1 {
		node.Roles = []vmcontrollerv1.NodeRole{
			role,
		}
	}
}

func handleOpensearchDashboardsConversion(spec *vmcontrollerv1.VerrazzanoMonitoringInstanceSpec) {
	if spec.Kibana != nil && spec.Kibana.Enabled {
		// if both Kibana and OpensearchDashboards fields are filled out in CR
		if spec.OpensearchDashboards.Enabled {
			// remove old Kibana data
			spec.Kibana = nil
			return
		}
		// copy Kibana fields to OpensearchDashboards field
		spec.OpensearchDashboards = vmcontrollerv1.OpensearchDashboards(*spec.Kibana)
	}

	// remove old Kibana data
	spec.Kibana = nil
}

func handleOpensearchConversion(spec *vmcontrollerv1.VerrazzanoMonitoringInstanceSpec) {
	if spec.Elasticsearch != nil && spec.Elasticsearch.Enabled {
		// if both Elasticsearch and Opensearch fields are filled out in CR
		if spec.Opensearch.Enabled {
			// remove Elasticsearch data
			spec.Elasticsearch = nil
			return
		}
		// copy Elasticsearch fields to Opensearch fields
		spec.Opensearch = vmcontrollerv1.Opensearch(*spec.Elasticsearch)
	}

	// remove old Elasticsearch data
	spec.Elasticsearch = nil
}

func initStorageElement(storageElement *vmcontrollerv1.Storage, replicas int, pvcName string) {
	if storageElement == nil || storageElement.Size == "" {
		return // No storage specified, so nothing to do
	}
	// Initialize the current state of the storage element, if not already set
	if storageElement.PvcNames == nil || len(storageElement.PvcNames) == 0 {
		// Initialize slice of storageElement.PvcNames
		storageElement.PvcNames = []string{}
		storageElement.PvcNames = append(storageElement.PvcNames, pvcName)
		// Base the rest of the PVC names on the format of the first
		for i := 1; i < replicas; i++ {
			pvcName = resources.GetNextStringInSequence(pvcName)
			storageElement.PvcNames = append(storageElement.PvcNames, pvcName)
		}
	}
	if len(storageElement.PvcNames) < replicas {
		newPvcs := replicas - len(storageElement.PvcNames)
		pvcName := storageElement.PvcNames[len(storageElement.PvcNames)-1]
		for i := 0; i < newPvcs; i++ {
			pvcName = resources.GetNextStringInSequence(pvcName)
			storageElement.PvcNames = append(storageElement.PvcNames, pvcName)
		}
	}
	// If we're over the expected number of PVCs, remove the extras from the VMO spec
	for len(storageElement.PvcNames) > replicas {
		storageElement.PvcNames = storageElement.PvcNames[:len(storageElement.PvcNames)-1]
	}
}

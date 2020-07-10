// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	"github.com/golang/glog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Initializes any uninitialized elements of the VMI spec.
func InitializeVMISpec(controller *Controller, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	// The secretName we use for basic authentication in the Nginx ingress controller
	vmi.Spec.SecretName = vmi.Name + "-basicauth"

	/*********************
	 * Create Secrets
	 **********************/
	credsMap, err := controller.loadAllAuthSecretData(vmi.Namespace, vmi.Spec.SecretsName)
	if err != nil {
		glog.Errorf("Failed to extract VMI Secrets for vmi %s in namespace %s: %v", vmi.Name, vmi.Namespace, err)
	}

	err = CreateOrUpdateAuthSecrets(controller, vmi, credsMap)
	if err != nil {
		glog.Errorf("Failed to create VMI Secrets for vmi %s in namespace %s: %v", vmi.Name, vmi.Namespace, err)
	}

	// Create TLS secrets or get certs
	err = CreateOrUpdateTLSSecrets(controller, vmi)
	if err != nil {
		glog.Errorf("Failed to create TLS Secrets for vmi: %v", err)
	}

	err = EnsureTlsSecretInMonitoringNS(controller, vmi)
	if err != nil {
		glog.Errorf("Failed to copy TLS Secret to monitoring namespace: %v", err)
	}

	// Set creation time
	if vmi.Status.CreationTime == nil {
		now := metav1.Now()
		vmi.Status.CreationTime = &now
	}

	// Set environment
	if vmi.Status.EnvName == "" {
		vmi.Status.EnvName = controller.operatorConfig.EnvName
	}

	// Service type
	if vmi.Spec.ServiceType == "" {
		vmi.Spec.ServiceType = corev1.ServiceTypeClusterIP
	}

	// Referenced ConfigMaps
	if vmi.Spec.Grafana.DashboardsConfigMap == "" {
		vmi.Spec.Grafana.DashboardsConfigMap = resources.GetMetaName(vmi.Name, constants.DashboardConfig)
	}
	if vmi.Spec.Grafana.DatasourcesConfigMap == "" {
		vmi.Spec.Grafana.DatasourcesConfigMap = resources.GetMetaName(vmi.Name, constants.DatasourceConfig)
	}
	if vmi.Spec.Prometheus.ConfigMap == "" {
		vmi.Spec.Prometheus.ConfigMap = resources.GetMetaName(vmi.Name, constants.PrometheusConfig)
	}
	if vmi.Spec.Prometheus.VersionsConfigMap == "" {
		vmi.Spec.Prometheus.VersionsConfigMap = resources.GetMetaName(vmi.Name, constants.PrometheusConfigVersions)
	}
	if vmi.Spec.Prometheus.RulesConfigMap == "" {
		vmi.Spec.Prometheus.RulesConfigMap = resources.GetMetaName(vmi.Name, constants.AlertrulesConfig)
	}
	if vmi.Spec.Prometheus.RulesVersionsConfigMap == "" {
		vmi.Spec.Prometheus.RulesVersionsConfigMap = resources.GetMetaName(vmi.Name, constants.AlertrulesVersionsConfig)
	}
	if vmi.Spec.AlertManager.ConfigMap == "" {
		vmi.Spec.AlertManager.ConfigMap = resources.GetMetaName(vmi.Name, constants.AlertManagerConfig)
	}
	if vmi.Spec.AlertManager.VersionsConfigMap == "" {
		vmi.Spec.AlertManager.VersionsConfigMap = resources.GetMetaName(vmi.Name, constants.AlertManagerConfigVersions)
	}

	// Number of replicas for each component
	if vmi.Spec.Api.Replicas == 0 {
		vmi.Spec.Api.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if vmi.Spec.Kibana.Replicas == 0 {
		vmi.Spec.Kibana.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if vmi.Spec.Prometheus.Replicas == 0 {
		vmi.Spec.Prometheus.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if vmi.Spec.AlertManager.Replicas == 0 {
		vmi.Spec.AlertManager.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if vmi.Spec.Elasticsearch.IngestNode.Replicas == 0 {
		vmi.Spec.Elasticsearch.IngestNode.Replicas = int32(constants.DefaultElasticsearchIngestReplicas)
	}
	if vmi.Spec.Elasticsearch.MasterNode.Replicas == 0 {
		vmi.Spec.Elasticsearch.MasterNode.Replicas = int32(constants.DefaultElasticsearchMasterReplicas)
	}
	if vmi.Spec.Elasticsearch.DataNode.Replicas == 0 {
		vmi.Spec.Elasticsearch.DataNode.Replicas = int32(constants.DefaultElasticsearchDataReplicas)
	}
	for _, component := range config.StorageEnableComponents {
		storageElement := resources.GetStorageElementForComponent(vmi, component)
		replicas := int(resources.GetReplicasForComponent(vmi, component))
		if storageElement.Size == "" {
			continue
		}
		// Initialize the current state of the storage element, if not already set
		if storageElement.PvcNames == nil || len(storageElement.PvcNames) == 0 {
			// Initialize slice of storageElement.PvcNames
			storageElement.PvcNames = []string{}
			pvcName := resources.GetMetaName(vmi.Name, component.Name)
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
		// If we're over the expected number of PVCs, remove the extras from the VMI spec
		for len(storageElement.PvcNames) > replicas {
			storageElement.PvcNames = storageElement.PvcNames[:len(storageElement.PvcNames)-1]
		}
	}

	// Prometheus TSDB retention period
	if vmi.Spec.Prometheus.RetentionPeriod == 0 {
		vmi.Spec.Prometheus.RetentionPeriod = constants.DefaultPrometheusRetentionPeriod
	}

	// Overall status
	if vmi.Status.State == "" {
		vmi.Status.State = string(constants.Running)
	}

}

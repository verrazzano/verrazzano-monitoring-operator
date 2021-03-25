// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"go.uber.org/zap"
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
	credsMap, err := controller.loadAllAuthSecretData(vmo.Namespace, vmo.Spec.SecretsName)
	if err != nil {
		zap.S().Errorf("Failed to extract VMO Secrets for vmo %s in namespace %s: %v", vmo.Name, vmo.Namespace, err)
	}

	err = CreateOrUpdateAuthSecrets(controller, vmo, credsMap)
	if err != nil {
		zap.S().Errorf("Failed to create VMO Secrets for vmo %s in namespace %s: %v", vmo.Name, vmo.Namespace, err)
	}

	// Create TLS secrets or get certs
	err = CreateOrUpdateTLSSecrets(controller, vmo)
	if err != nil {
		zap.S().Errorf("Failed to create TLS Secrets for vmo: %v", err)
	}

	err = EnsureTLSSecretInMonitoringNS(controller, vmo)
	if err != nil {
		zap.S().Errorf("Failed to copy TLS Secret to monitoring namespace: %v", err)
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
	if vmo.Spec.Prometheus.ConfigMap == "" {
		vmo.Spec.Prometheus.ConfigMap = resources.GetMetaName(vmo.Name, constants.PrometheusConfig)
	}
	if vmo.Spec.Prometheus.VersionsConfigMap == "" {
		vmo.Spec.Prometheus.VersionsConfigMap = resources.GetMetaName(vmo.Name, constants.PrometheusConfigVersions)
	}
	if vmo.Spec.Prometheus.RulesConfigMap == "" {
		vmo.Spec.Prometheus.RulesConfigMap = resources.GetMetaName(vmo.Name, constants.AlertrulesConfig)
	}
	if vmo.Spec.Prometheus.RulesVersionsConfigMap == "" {
		vmo.Spec.Prometheus.RulesVersionsConfigMap = resources.GetMetaName(vmo.Name, constants.AlertrulesVersionsConfig)
	}
	if vmo.Spec.AlertManager.ConfigMap == "" {
		vmo.Spec.AlertManager.ConfigMap = resources.GetMetaName(vmo.Name, constants.AlertManagerConfig)
	}
	if vmo.Spec.AlertManager.VersionsConfigMap == "" {
		vmo.Spec.AlertManager.VersionsConfigMap = resources.GetMetaName(vmo.Name, constants.AlertManagerConfigVersions)
	}

	// Number of replicas for each component
	//if vmo.Spec.API.Replicas == 0 {
	//	vmo.Spec.API.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	//}
	if vmo.Spec.Kibana.Replicas == 0 {
		vmo.Spec.Kibana.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if vmo.Spec.Prometheus.Replicas == 0 {
		vmo.Spec.Prometheus.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if vmo.Spec.AlertManager.Replicas == 0 {
		vmo.Spec.AlertManager.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if vmo.Spec.Elasticsearch.MasterNode.Replicas == 0 {
		vmo.Spec.Elasticsearch.MasterNode.Replicas = int32(constants.DefaultElasticsearchMasterReplicas)
	}
	for _, component := range config.StorageEnableComponents {
		storageElement := resources.GetStorageElementForComponent(vmo, component)
		replicas := int(resources.GetReplicasForComponent(vmo, component))
		if storageElement.Size == "" {
			continue
		}
		// Initialize the current state of the storage element, if not already set
		if storageElement.PvcNames == nil || len(storageElement.PvcNames) == 0 {
			// Initialize slice of storageElement.PvcNames
			storageElement.PvcNames = []string{}
			pvcName := resources.GetMetaName(vmo.Name, component.Name)
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

	// Prometheus TSDB retention period
	if vmo.Spec.Prometheus.RetentionPeriod == 0 {
		vmo.Spec.Prometheus.RetentionPeriod = constants.DefaultPrometheusRetentionPeriod
	}

	// Overall status
	if vmo.Status.State == "" {
		vmo.Status.State = string(constants.Running)
	}

}

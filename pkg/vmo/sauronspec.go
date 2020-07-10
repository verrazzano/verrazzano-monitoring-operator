// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	"github.com/rs/zerolog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
)

// Initializes any uninitialized elements of the Sauron spec.
func InitializeSauronSpec(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	//create log for initializing Sauron
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "VerrazzanoMonitoringInstance").Str("name", sauron.Name).Logger()

	// The secretName we use for basic authentication in the Nginx ingress controller
	sauron.Spec.SecretName = sauron.Name + "-basicauth"

	/*********************
	 * Create Secrets
	 **********************/
	credsMap, err := controller.loadAllAuthSecretData(sauron.Namespace, sauron.Spec.SecretsName)
	if err != nil {
		logger.Error().Msgf("Failed to extract Sauron Secrets for sauron %s in namespace %s: %v", sauron.Name, sauron.Namespace, err)
	}

	err = CreateOrUpdateAuthSecrets(controller, sauron, credsMap)
	if err != nil {
		logger.Error().Msgf("Failed to create Sauron Secrets for sauron %s in namespace %s: %v", sauron.Name, sauron.Namespace, err)
	}

	// Create TLS secrets or get certs
	err = CreateOrUpdateTLSSecrets(controller, sauron)
	if err != nil {
		logger.Error().Msgf("Failed to create TLS Secrets for sauron: %v", err)
	}

	err = EnsureTlsSecretInMonitoringNS(controller, sauron)
	if err != nil {
		logger.Error().Msgf("Failed to copy TLS Secret to monitoring namespace: %v", err)
	}

	// Set creation time
	if sauron.Status.CreationTime == nil {
		now := metav1.Now()
		sauron.Status.CreationTime = &now
	}

	// Set environment
	if sauron.Status.EnvName == "" {
		sauron.Status.EnvName = controller.operatorConfig.EnvName
	}

	// Service type
	if sauron.Spec.ServiceType == "" {
		sauron.Spec.ServiceType = corev1.ServiceTypeClusterIP
	}

	// Referenced ConfigMaps
	if sauron.Spec.Grafana.DashboardsConfigMap == "" {
		sauron.Spec.Grafana.DashboardsConfigMap = resources.GetMetaName(sauron.Name, constants.DashboardConfig)
	}
	if sauron.Spec.Grafana.DatasourcesConfigMap == "" {
		sauron.Spec.Grafana.DatasourcesConfigMap = resources.GetMetaName(sauron.Name, constants.DatasourceConfig)
	}
	if sauron.Spec.Prometheus.ConfigMap == "" {
		sauron.Spec.Prometheus.ConfigMap = resources.GetMetaName(sauron.Name, constants.PrometheusConfig)
	}
	if sauron.Spec.Prometheus.VersionsConfigMap == "" {
		sauron.Spec.Prometheus.VersionsConfigMap = resources.GetMetaName(sauron.Name, constants.PrometheusConfigVersions)
	}
	if sauron.Spec.Prometheus.RulesConfigMap == "" {
		sauron.Spec.Prometheus.RulesConfigMap = resources.GetMetaName(sauron.Name, constants.AlertrulesConfig)
	}
	if sauron.Spec.Prometheus.RulesVersionsConfigMap == "" {
		sauron.Spec.Prometheus.RulesVersionsConfigMap = resources.GetMetaName(sauron.Name, constants.AlertrulesVersionsConfig)
	}
	if sauron.Spec.AlertManager.ConfigMap == "" {
		sauron.Spec.AlertManager.ConfigMap = resources.GetMetaName(sauron.Name, constants.AlertManagerConfig)
	}
	if sauron.Spec.AlertManager.VersionsConfigMap == "" {
		sauron.Spec.AlertManager.VersionsConfigMap = resources.GetMetaName(sauron.Name, constants.AlertManagerConfigVersions)
	}

	// Number of replicas for each component
	if sauron.Spec.Api.Replicas == 0 {
		sauron.Spec.Api.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if sauron.Spec.Kibana.Replicas == 0 {
		sauron.Spec.Kibana.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if sauron.Spec.Prometheus.Replicas == 0 {
		sauron.Spec.Prometheus.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if sauron.Spec.AlertManager.Replicas == 0 {
		sauron.Spec.AlertManager.Replicas = int32(*controller.operatorConfig.DefaultSimpleComponentReplicas)
	}
	if sauron.Spec.Elasticsearch.IngestNode.Replicas == 0 {
		sauron.Spec.Elasticsearch.IngestNode.Replicas = int32(constants.DefaultElasticsearchIngestReplicas)
	}
	if sauron.Spec.Elasticsearch.MasterNode.Replicas == 0 {
		sauron.Spec.Elasticsearch.MasterNode.Replicas = int32(constants.DefaultElasticsearchMasterReplicas)
	}
	if sauron.Spec.Elasticsearch.DataNode.Replicas == 0 {
		sauron.Spec.Elasticsearch.DataNode.Replicas = int32(constants.DefaultElasticsearchDataReplicas)
	}
	for _, component := range config.StorageEnableComponents {
		storageElement := resources.GetStorageElementForComponent(sauron, component)
		replicas := int(resources.GetReplicasForComponent(sauron, component))
		if storageElement.Size == "" {
			continue
		}
		// Initialize the current state of the storage element, if not already set
		if storageElement.PvcNames == nil || len(storageElement.PvcNames) == 0 {
			// Initialize slice of storageElement.PvcNames
			storageElement.PvcNames = []string{}
			pvcName := resources.GetMetaName(sauron.Name, component.Name)
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
		// If we're over the expected number of PVCs, remove the extras from the Sauron spec
		for len(storageElement.PvcNames) > replicas {
			storageElement.PvcNames = storageElement.PvcNames[:len(storageElement.PvcNames)-1]
		}
	}

	// Prometheus TSDB retention period
	if sauron.Spec.Prometheus.RetentionPeriod == 0 {
		sauron.Spec.Prometheus.RetentionPeriod = constants.DefaultPrometheusRetentionPeriod
	}

	// Overall status
	if sauron.Status.State == "" {
		sauron.Status.State = string(constants.Running)
	}

}

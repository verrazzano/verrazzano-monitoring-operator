// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"bytes"
	"context"
	"html/template"
	"strings"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/metricsexporter"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/configmaps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	prometheusOperatorPrometheusHost = "prometheus-operator-kube-p-prometheus.verrazzano-monitoring"
	datasourceYAMLKey                = "datasource.yaml"
)

// CreateConfigmaps to create all required configmaps for VMI
func CreateConfigmaps(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	metric, metricErr := metricsexporter.GetCounterMetrics(metricsexporter.NamesConfigMap)
	if metricErr != nil {
		return metricErr
	}
	metric.Inc()

	var configMaps []string

	// Configmap for Grafana dashboard
	dashboardTemplateMap := map[string]string{"vmo-dashboard-provider.yml": constants.DashboardProviderTmpl}
	// Only create the CM if it doesnt exist. This will allow us to override the provider file e.g. Verrazzano
	err := createConfigMapIfDoesntExist(controller, vmo, vmo.Spec.Grafana.DashboardsConfigMap, dashboardTemplateMap)
	if err != nil {
		controller.log.Debugf("Failed to create dashboard configmap %s: %v", vmo.Spec.Grafana.DashboardsConfigMap, err)
		return err

	}
	configMaps = append(configMaps, vmo.Spec.Grafana.DashboardsConfigMap)

	//configmap for grafana datasources
	replaceMap := map[string]string{constants.GrafanaTmplPrometheusURI: prometheusOperatorPrometheusHost,
		constants.GrafanaTmplAlertManagerURI: resources.GetMetaName(vmo.Name, config.AlertManager.Name)}
	dataSourceTemplate, err := asDashboardTemplate(constants.DataSourcesTmpl, replaceMap)
	if err != nil {
		controller.log.Debugf("Failed to create dashboard datasource template for VMI %s: %v", vmo.Name, err)
		return err
	}
	err = createUpdateDatasourcesConfigMap(controller, vmo, vmo.Spec.Grafana.DatasourcesConfigMap, map[string]string{datasourceYAMLKey: dataSourceTemplate})
	if err != nil {
		controller.log.Debugf("Failed to create datasource configmap %s: %v", vmo.Spec.Grafana.DatasourcesConfigMap, err)
		return err

	}
	configMaps = append(configMaps, vmo.Spec.Grafana.DatasourcesConfigMap)

	// Delete configmaps that shouldn't exist
	controller.log.Debugf("Deleting unwanted ConfigMaps for VMI %s/%s", vmo.Namespace, vmo.Name)
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	configMapList, err := controller.configMapLister.ConfigMaps(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, configMap := range configMapList {
		if !contains(configMaps, configMap.Name) {
			controller.log.Debugf("Deleting config map %s", configMap.Name)
			err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Delete(context.TODO(), configMap.Name, metav1.DeleteOptions{})
			if err != nil {
				controller.log.Debugf("Failed to delete configmap %s%s: %v", vmo.Namespace, configMap.Name, err)
				return err
			}
		}
	}
	timeMetric, timeErr := metricsexporter.GetTimestampMetrics(metricsexporter.NamesConfigMap)
	if timeErr != nil {
		return timeErr
	}
	timeMetric.SetLastTime()
	return nil
}

// createUpdateDatasourcesConfigMap creates or updates the Grafana datasource configmap. If the configmap exists and the Prometheus URL still points
// to the legacy VMO-managed Prometheus, then replace the Prometheus URL with the new Prometheus Operator-managed Prometheus URL.
func createUpdateDatasourcesConfigMap(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, configmapName string, data map[string]string) error {

	existingConfig, err := getConfigMap(controller, vmo.Namespace, configmapName)
	if err != nil {
		controller.log.Errorf("Failed to get configmap %s%s: %v", vmo.Namespace, configmapName, err)
		return err
	}
	if existingConfig == nil {
		configMap := configmaps.NewConfig(vmo, configmapName, data)
		_, err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			controller.log.Errorf("Failed to create configmap %s%s: %v", vmo.Namespace, configmapName, err)
			return err
		}
		return nil
	}

	// if the datasource still points to the legacy Prometheus instance, update it to point to the new Prometheus Operator-managed Prometheus
	if ds, found := existingConfig.Data[datasourceYAMLKey]; found {
		updatedDatasourceStr := strings.Replace(ds, resources.GetMetaName(vmo.Name, config.Prometheus.Name), prometheusOperatorPrometheusHost, 1)
		if updatedDatasourceStr != ds {
			controller.log.Infof("Replacing Prometheus URL in existing datasource configmap %s/%s", vmo.Namespace, configmapName)

			existingConfig.Data[datasourceYAMLKey] = updatedDatasourceStr
			_, err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Update(context.TODO(), existingConfig, metav1.UpdateOptions{})
			if err != nil {
				controller.log.Errorf("Failed to update configmap %s/%s: %v", vmo.Namespace, configmapName, err)
				return err
			}
		}
	}
	return nil
}

// This function is being called for configmaps which don't modify with spec changes
func createConfigMapIfDoesntExist(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, configmap string, data map[string]string) error {
	configMap := configmaps.NewConfig(vmo, configmap, data)
	_, err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		controller.log.Errorf("Failed to create configmap %s/%s: %v", vmo.Namespace, configmap, err)
		return err
	}
	return nil
}

// asDashboardTemplate replaces `namespace` placehoders in the tmplt with the namespace value
func asDashboardTemplate(tmplt string, replaceMap map[string]string) (string, error) {
	t := template.Must(template.New("dashboard").Parse(tmplt))
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, replaceMap); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getConfigMap(controller *Controller, namespace string, configmapName string) (*corev1.ConfigMap, error) {
	configMap, err := controller.configMapLister.ConfigMaps(namespace).Get(configmapName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return configMap, nil
}

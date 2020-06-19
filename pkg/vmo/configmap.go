// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"

	"github.com/golang/glog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/configmaps"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/diff"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// CreateConfigmaps to create all required configmaps for sauron
func CreateConfigmaps(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	var configMaps []string
	alertrulesMap := make(map[string]string)

	// Configmap for Grafana dashboard
	dashboardTemplateMap := map[string]string{"sauron-dashboard-provider.yml": constants.DashboardProviderTmpl}
	// Only create the CM if it doesnt exist. This will allow us to override the provider file e.g. Tiburon
	err := createConfigMapIfDoesntExist(controller, sauron, sauron.Spec.Grafana.DashboardsConfigMap, dashboardTemplateMap)
	if err != nil {
		glog.Errorf("Failed to create dashboard configmap %s, for reason %v", sauron.Spec.Grafana.DashboardsConfigMap, err)
		return err

	}
	configMaps = append(configMaps, sauron.Spec.Grafana.DashboardsConfigMap)

	//configmap for grafana datasources
	replaceMap := map[string]string{constants.GrafanaTmplPrometheusURI: resources.GetMetaName(sauron.Name, config.Prometheus.Name),
		constants.GrafanaTmplAlertManagerURI: resources.GetMetaName(sauron.Name, config.AlertManager.Name)}
	dataSourceTemplate, err := asDashboardTemplate(constants.DataSourcesTmpl, replaceMap)
	if err != nil {
		glog.Errorf("Failed to create dashboard datasource template for sauron %s dur to %v", sauron.Name, err)
		return err
	}
	err = createConfigMapIfDoesntExist(controller, sauron, sauron.Spec.Grafana.DatasourcesConfigMap, map[string]string{"datasource.yaml": dataSourceTemplate})
	if err != nil {
		glog.Errorf("Failed to create datasource configmap %s, for reason %v", sauron.Spec.Grafana.DatasourcesConfigMap, err)
		return err

	}
	configMaps = append(configMaps, sauron.Spec.Grafana.DatasourcesConfigMap)

	//configmap for alertmanager config
	err = createAMConfigMapIfDoesntExist(controller, sauron, sauron.Spec.AlertManager.ConfigMap, map[string]string{constants.AlertManagerYaml: resources.GetDefaultAlertManagerConfiguration(sauron)})
	if err != nil {
		glog.Errorf("Failed to create configmap %s for reason %v", sauron.Spec.AlertManager.ConfigMap, err)
		return err
	}
	configMaps = append(configMaps, sauron.Spec.AlertManager.ConfigMap)

	//configmap for alertmanager config versions
	//starts off with an empty configmap - Cirith will add to it later
	err = createConfigMapIfDoesntExist(controller, sauron, sauron.Spec.AlertManager.VersionsConfigMap, map[string]string{})
	if err != nil {
		glog.Errorf("Failed to create configmap %s for reason %v", sauron.Spec.AlertManager.VersionsConfigMap, err)
		return err
	}
	configMaps = append(configMaps, sauron.Spec.AlertManager.VersionsConfigMap)

	//configmap for alertrules
	err = createConfigMap(controller, sauron, sauron.Spec.Prometheus.RulesConfigMap, alertrulesMap)
	if err != nil {
		glog.Errorf("Failed to create alertrules configmap %s for reason %v", sauron.Spec.Prometheus.RulesConfigMap, err)
		return err

	}
	configMaps = append(configMaps, sauron.Spec.Prometheus.RulesConfigMap)

	//configmap for alertrules versions
	//starts off with an empty configmap - Cirith will add to it later
	err = createConfigMapIfDoesntExist(controller, sauron, sauron.Spec.Prometheus.RulesVersionsConfigMap, map[string]string{})
	if err != nil {
		glog.Errorf("Failed to create alertrules configmap %s for reason %v", sauron.Spec.Prometheus.RulesVersionsConfigMap, err)
		return err

	}
	configMaps = append(configMaps, sauron.Spec.Prometheus.RulesVersionsConfigMap)

	//configmap for prometheus config
	err = createConfigMapIfDoesntExist(controller, sauron, sauron.Spec.Prometheus.ConfigMap, map[string]string{"prometheus.yml": resources.GetDefaultPrometheusConfiguration(sauron)})
	if err != nil {
		glog.Errorf("Failed to create configmap %s for reason %v", sauron.Spec.Prometheus.ConfigMap, err)
		return err

	}
	configMaps = append(configMaps, sauron.Spec.Prometheus.ConfigMap)

	//configmap for prometheus config versions
	//starts off with an empty configmap - Cirith will add to it later.
	err = createConfigMapIfDoesntExist(controller, sauron, sauron.Spec.Prometheus.VersionsConfigMap, map[string]string{})
	if err != nil {
		glog.Errorf("Failed to create configmap %s for reason %v", sauron.Spec.Prometheus.VersionsConfigMap, err)
		return err

	}
	configMaps = append(configMaps, sauron.Spec.Prometheus.VersionsConfigMap)

	// Delete configmaps that shouldn't exist
	glog.V(4).Infof("Deleting unwanted ConfigMaps for sauron '%s' in namespace '%s'", sauron.Name, sauron.Namespace)
	selector := labels.SelectorFromSet(map[string]string{constants.SauronLabel: sauron.Name})
	configMapList, err := controller.configMapLister.ConfigMaps(sauron.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, configMap := range configMapList {
		if !contains(configMaps, configMap.Name) {
			glog.V(6).Infof("Deleting config map %s", configMap.Name)
			err := controller.kubeclientset.CoreV1().ConfigMaps(sauron.Namespace).Delete(configMap.Name, &metav1.DeleteOptions{})
			if err != nil {
				glog.Errorf("Failed to delete config map %s, for the reason (%v)", configMap.Name, err)
				return err
			}
		}
	}
	return nil
}

// This function is being called for configmaps which gets modified with spec changes
func createConfigMap(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, configmap string, data map[string]string) error {
	configMap := configmaps.NewConfig(sauron, configmap, data)
	existingConfigMap, err := getConfigMap(controller, sauron, configmap)
	if err != nil {
		glog.Errorf("Failed to get configmap %s for sauron %s", sauron.Name, configmap)
		return err
	}
	if existingConfigMap != nil {
		glog.V(6).Infof("Updating existing configmaps for %s ", existingConfigMap.Name)
		//Retain any AlertManager rules added or modified by user
		if existingConfigMap.Name == resources.GetMetaName(sauron.Name, constants.AlertrulesConfig) {
			//get custom rules if any
			customRules := getCustomRulesMap(existingConfigMap.Data)
			for k, v := range customRules {
				configMap.Data[k] = v
			}
		}
		specDiffs := diff.CompareIgnoreTargetEmpties(existingConfigMap, configMap)
		if specDiffs != "" {
			glog.V(4).Infof("ConfigMap %s : Spec differences %s", configMap.Name, specDiffs)
			_, err := controller.kubeclientset.CoreV1().ConfigMaps(sauron.Namespace).Update(configMap)
			if err != nil {
				glog.Errorf("Failed to update existing configmap %s ", configMap.Name)
			}
		}
	} else {
		_, err := controller.kubeclientset.CoreV1().ConfigMaps(sauron.Namespace).Create(configMap)
		if err != nil {
			glog.Errorf("Failed to create configmap %s for sauron %s", sauron.Name, configmap)
			return err
		}
	}
	return nil
}

// This function is being called for configmaps which don't modify with spec changes
func createConfigMapIfDoesntExist(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, configmap string, data map[string]string) error {
	existingConfig, err := getConfigMap(controller, sauron, configmap)
	if err != nil {
		glog.Errorf("Failed to get configmap %s for sauron %s", sauron.Name, configmap)
		return err
	}
	if existingConfig == nil {
		configMap := configmaps.NewConfig(sauron, configmap, data)
		_, err := controller.kubeclientset.CoreV1().ConfigMaps(sauron.Namespace).Create(configMap)
		if err != nil {
			glog.Errorf("Failed to create configmap %s for sauron %s", sauron.Name, configmap)
			return err
		}
	}
	return nil
}

// This function is being called for configmaps which don't modify with spec changes
func createAMConfigMapIfDoesntExist(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, configmap string, data map[string]string) error {
	existingConfig, err := getConfigMap(controller, sauron, configmap)
	if err != nil {
		glog.Errorf("Failed to get configmap %s for sauron %s", sauron.Name, configmap)
		return err
	}
	if existingConfig == nil {
		if sauron.Spec.AlertManager.Config != "" {
			data = map[string]string{constants.AlertManagerYaml: sauron.Spec.AlertManager.Config}
			sauron.Spec.AlertManager.Config = ""
		}
		configMap := configmaps.NewConfig(sauron, configmap, data)
		_, err := controller.kubeclientset.CoreV1().ConfigMaps(sauron.Namespace).Create(configMap)
		if err != nil {
			glog.Errorf("Failed to create configmap %s for sauron %s", sauron.Name, configmap)
			return err
		}
	}
	return nil
}

//Returns custom-rules in configmap
func getCustomRulesMap(existedData map[string]string) (customRulesMap map[string]string) {
	customRulesMap = make(map[string]string)
	for k, v := range existedData {
		//all default alert rules name starts with sauron-1
		if !strings.HasPrefix(k, constants.SauronServiceNamePrefix) {
			customRulesMap[k] = v
		}
	}
	return
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

func getConfigMap(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, configmapName string) (*corev1.ConfigMap, error) {
	configMap, err := controller.configMapLister.ConfigMaps(sauron.Namespace).Get(configmapName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return configMap, nil
}

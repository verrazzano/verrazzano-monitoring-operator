// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	proxy "github.com/verrazzano/verrazzano-monitoring-operator/pkg/proxy"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/configmaps"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/diff"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// CreateConfigmaps to create all required configmaps for vmo
func CreateConfigmaps(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	var configMaps []string
	alertrulesMap := make(map[string]string)

	// Configmap for Grafana dashboard
	dashboardTemplateMap := map[string]string{"vmo-dashboard-provider.yml": constants.DashboardProviderTmpl}
	// Only create the CM if it doesnt exist. This will allow us to override the provider file e.g. Verrazzano
	err := createConfigMapIfDoesntExist(controller, vmo, vmo.Spec.Grafana.DashboardsConfigMap, dashboardTemplateMap)
	if err != nil {
		zap.S().Errorf("Failed to create dashboard configmap %s, for reason %v", vmo.Spec.Grafana.DashboardsConfigMap, err)
		return err

	}
	configMaps = append(configMaps, vmo.Spec.Grafana.DashboardsConfigMap)

	//configmap for grafana datasources
	replaceMap := map[string]string{constants.GrafanaTmplPrometheusURI: resources.GetMetaName(vmo.Name, config.Prometheus.Name),
		constants.GrafanaTmplAlertManagerURI: resources.GetMetaName(vmo.Name, config.AlertManager.Name)}
	dataSourceTemplate, err := asDashboardTemplate(constants.DataSourcesTmpl, replaceMap)
	if err != nil {
		zap.S().Errorf("Failed to create dashboard datasource template for vmo %s dur to %v", vmo.Name, err)
		return err
	}
	err = createConfigMapIfDoesntExist(controller, vmo, vmo.Spec.Grafana.DatasourcesConfigMap, map[string]string{"datasource.yaml": dataSourceTemplate})
	if err != nil {
		zap.S().Errorf("Failed to create datasource configmap %s, for reason %v", vmo.Spec.Grafana.DatasourcesConfigMap, err)
		return err

	}
	configMaps = append(configMaps, vmo.Spec.Grafana.DatasourcesConfigMap)

	//configmap for alertmanager config
	err = createAMConfigMapIfDoesntExist(controller, vmo, vmo.Spec.AlertManager.ConfigMap, map[string]string{constants.AlertManagerYaml: resources.GetDefaultAlertManagerConfiguration(vmo)})
	if err != nil {
		zap.S().Errorf("Failed to create configmap %s for reason %v", vmo.Spec.AlertManager.ConfigMap, err)
		return err
	}
	configMaps = append(configMaps, vmo.Spec.AlertManager.ConfigMap)

	//configmap for alertmanager config versions
	//starts off with an empty configmap - Cirith will add to it later
	err = createConfigMapIfDoesntExist(controller, vmo, vmo.Spec.AlertManager.VersionsConfigMap, map[string]string{})
	if err != nil {
		zap.S().Errorf("Failed to create configmap %s for reason %v", vmo.Spec.AlertManager.VersionsConfigMap, err)
		return err
	}
	configMaps = append(configMaps, vmo.Spec.AlertManager.VersionsConfigMap)

	//configmap for alertrules
	err = createUpdateAlertRulesConfigMap(controller, vmo, vmo.Spec.Prometheus.RulesConfigMap, alertrulesMap)
	if err != nil {
		zap.S().Errorf("Failed to create alertrules configmap %s for reason %v", vmo.Spec.Prometheus.RulesConfigMap, err)
		return err

	}
	configMaps = append(configMaps, vmo.Spec.Prometheus.RulesConfigMap)

	//configmap for alertrules versions
	//starts off with an empty configmap - Cirith will add to it later
	err = createConfigMapIfDoesntExist(controller, vmo, vmo.Spec.Prometheus.RulesVersionsConfigMap, map[string]string{})
	if err != nil {
		zap.S().Errorf("Failed to create alertrules configmap %s for reason %v", vmo.Spec.Prometheus.RulesVersionsConfigMap, err)
		return err

	}
	configMaps = append(configMaps, vmo.Spec.Prometheus.RulesVersionsConfigMap)

	//configmap for prometheus config
	err = createConfigMapIfDoesntExist(controller, vmo, vmo.Spec.Prometheus.ConfigMap, map[string]string{"prometheus.yml": resources.GetDefaultPrometheusConfiguration(vmo)})
	if err != nil {
		zap.S().Errorf("Failed to create configmap %s for reason %v", vmo.Spec.Prometheus.ConfigMap, err)
		return err

	}
	configMaps = append(configMaps, vmo.Spec.Prometheus.ConfigMap)

	//configmap for prometheus config versions
	//starts off with an empty configmap - Cirith will add to it later.
	err = createConfigMapIfDoesntExist(controller, vmo, vmo.Spec.Prometheus.VersionsConfigMap, map[string]string{})
	if err != nil {
		zap.S().Errorf("Failed to create configmap %s for reason %v", vmo.Spec.Prometheus.VersionsConfigMap, err)
		return err

	}
	configMaps = append(configMaps, vmo.Spec.Prometheus.VersionsConfigMap)

	if config.ElasticsearchIngest.OidcProxy != nil {
		oidcConfig, err := addOidcProxyConfig(controller, vmo, &config.ElasticsearchIngest)
		if err != nil {
			zap.S().Errorf("Failed to create oidc-proxy configmap %s, for reason %v", oidcConfig, err)
			return err
		}
		configMaps = append(configMaps, oidcConfig)
	}
	if config.Prometheus.OidcProxy != nil {
		oidcConfig, err := addOidcProxyConfig(controller, vmo, &config.Prometheus)
		if err != nil {
			zap.S().Errorf("Failed to create oidc-proxy configmap %s, for reason %v", oidcConfig, err)
			return err
		}
		configMaps = append(configMaps, oidcConfig)
	}
	if config.Grafana.OidcProxy != nil {
		oidcConfig, err := addOidcProxyConfig(controller, vmo, &config.Grafana)
		if err != nil {
			zap.S().Errorf("Failed to create oidc-proxy configmap %s, for reason %v", oidcConfig, err)
			return err
		}
		configMaps = append(configMaps, oidcConfig)
	}
	if config.Kibana.OidcProxy != nil {
		oidcConfig, err := addOidcProxyConfig(controller, vmo, &config.Kibana)
		if err != nil {
			zap.S().Errorf("Failed to create oidc-proxy configmap %s, for reason %v", oidcConfig, err)
			return err
		}
		configMaps = append(configMaps, oidcConfig)
	}

	// Delete configmaps that shouldn't exist
	zap.S().Infof("Deleting unwanted ConfigMaps for vmo '%s' in namespace '%s'", vmo.Name, vmo.Namespace)
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmo.Name})
	configMapList, err := controller.configMapLister.ConfigMaps(vmo.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, configMap := range configMapList {
		if !contains(configMaps, configMap.Name) {
			zap.S().Debugf("Deleting config map %s", configMap.Name)
			err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Delete(context.TODO(), configMap.Name, metav1.DeleteOptions{})
			if err != nil {
				zap.S().Errorf("Failed to delete config map %s, for the reason (%v)", configMap.Name, err)
				return err
			}
		}
	}
	return nil
}

// This function is being called for configmaps which gets modified with spec changes
func createUpdateAlertRulesConfigMap(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, configmap string, data map[string]string) error {
	configMap := configmaps.NewConfig(vmo, configmap, data)
	existingConfigMap, err := getConfigMap(controller, vmo, configmap)
	if err != nil {
		zap.S().Errorf("Failed to get configmap %s for vmo %s", vmo.Name, configmap)
		return err
	}
	if existingConfigMap != nil {
		zap.S().Debugf("Updating existing configmaps for %s ", existingConfigMap.Name)
		//Retain any AlertManager rules added or modified by user
		if existingConfigMap.Name == resources.GetMetaName(vmo.Name, constants.AlertrulesConfig) {
			//get custom rules if any
			customRules := getCustomRulesMap(existingConfigMap.Data)
			for k, v := range customRules {
				configMap.Data[k] = v
			}
		}
		specDiffs := diff.CompareIgnoreTargetEmpties(existingConfigMap, configMap)
		if specDiffs != "" {
			zap.S().Infof("ConfigMap %s : Spec differences %s", configMap.Name, specDiffs)
			_, err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
			if err != nil {
				zap.S().Errorf("Failed to update existing configmap %s ", configMap.Name)
			}
		}
	} else {
		_, err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			zap.S().Errorf("Failed to create configmap %s for vmo %s", vmo.Name, configmap)
			return err
		}
	}
	return nil
}

func createUpdateConfigMap(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, configmap string, data map[string]string) error {
	configMap := configmaps.NewConfig(vmo, configmap, data)
	existingConfigMap, err := getConfigMap(controller, vmo, configmap)
	if err != nil {
		zap.S().Errorf("Failed to get configmap %s for vmo %s", vmo.Name, configmap)
		return err
	}
	if existingConfigMap != nil {
		zap.S().Debugf("Updating existing configmaps for %s ", existingConfigMap.Name)
		specDiffs := diff.CompareIgnoreTargetEmpties(existingConfigMap, configMap)
		if specDiffs != "" {
			zap.S().Debugf("ConfigMap %s : Spec differences %s", configMap.Name, len(specDiffs))
			_, err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
			if err != nil {
				zap.S().Errorf("Failed to update existing configmap %s ", configMap.Name)
			}
		}
	} else {
		_, err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			zap.S().Errorf("Failed to create configmap %s for vmo %s", vmo.Name, configmap)
			return err
		}
	}
	return nil
}

// This function is being called for configmaps which don't modify with spec changes
func createConfigMapIfDoesntExist(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, configmap string, data map[string]string) error {
	existingConfig, err := getConfigMap(controller, vmo, configmap)
	if err != nil {
		zap.S().Errorf("Failed to get configmap %s for vmo %s", vmo.Name, configmap)
		return err
	}
	if existingConfig == nil {
		configMap := configmaps.NewConfig(vmo, configmap, data)
		_, err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			zap.S().Errorf("Failed to create configmap %s for vmo %s", vmo.Name, configmap)
			return err
		}
	}
	return nil
}

// This function is being called for configmaps which don't modify with spec changes
func createAMConfigMapIfDoesntExist(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, configmap string, data map[string]string) error {
	existingConfig, err := getConfigMap(controller, vmo, configmap)
	if err != nil {
		zap.S().Errorf("Failed to get configmap %s for vmo %s", vmo.Name, configmap)
		return err
	}
	if existingConfig == nil {
		if vmo.Spec.AlertManager.Config != "" {
			data = map[string]string{constants.AlertManagerYaml: vmo.Spec.AlertManager.Config}
			vmo.Spec.AlertManager.Config = ""
		}
		configMap := configmaps.NewConfig(vmo, configmap, data)
		_, err := controller.kubeclientset.CoreV1().ConfigMaps(vmo.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			zap.S().Errorf("Failed to create configmap %s for vmo %s", vmo.Name, configmap)
			return err
		}
	}
	return nil
}

//Returns custom-rules in configmap
func getCustomRulesMap(existedData map[string]string) (customRulesMap map[string]string) {
	customRulesMap = make(map[string]string)
	for k, v := range existedData {
		//all default alert rules name starts with vmo-1
		if !strings.HasPrefix(k, constants.VMOServiceNamePrefix) {
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

func getConfigMap(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, configmapName string) (*corev1.ConfigMap, error) {
	configMap, err := controller.configMapLister.ConfigMaps(vmo.Namespace).Get(configmapName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return configMap, nil
}

// oidcAuthLuaScripts(&oidcProxyConfig)
// oidcConfLuaScripts(&oidcProxyConfig, vmo, component, controller.clusterInfo.KeycloakURL)
// oidcStartup(&oidcProxyConfig)
// oidcNginxConf(&oidcProxyConfig, component.Port, len(controller.clusterInfo.clusterName) > 0)

// getOidcProxyConfig returns an OidcProxyConfig struct
func getOidcProxyConfig(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) proxy.OidcProxyConfig {
	proxyConfig := proxy.OidcProxyConfig{}
	proxyConfig.Mode = proxy.ProxyModeOauth
	proxyConfig.OidcRealm = proxy.OidcRealmName
	proxyConfig.PKCEClientID = proxy.OidcPkceClientID
	proxyConfig.PGClientID = proxy.OidcPgClientID
	proxyConfig.OidcCallbackPath = proxy.OidcCallbackPath
	proxyConfig.OidcLogoutCallbackPath = proxy.OidcLogoutCallbackPath
	proxyConfig.RequiredRealmRole = proxy.OidcRequiredRealmRole
	proxyConfig.AuthnStateTTL = proxy.OidcAuthnStateTTL

	proxyConfig.Host = "localhost"
	proxyConfig.Port = component.Port

	// ingress and keycloak location info
	proxyConfig.Ingress = resources.OidcProxyIngressHost(vmo, component)
	verrazzanoURI := vmo.Spec.URI
	uriPrefix := fmt.Sprintf("vmi.%s.", vmo.Name)
	if strings.HasPrefix(verrazzanoURI, uriPrefix) {
		verrazzanoURI = strings.Replace(verrazzanoURI, uriPrefix, "", 1)
	}
	proxyConfig.OidcProviderHost = fmt.Sprintf("%s.%s", "keycloak", verrazzanoURI)
	proxyConfig.OidcProviderHostInCluster = "keycloak-http.keycloak.svc.cluster.local"
	// when keycloakURL is present, meanning it is a managed cluster, keycloakURL is the admin keycloak url
	if len(controller.clusterInfo.KeycloakURL) > 0 {
		u, err := url.Parse(controller.clusterInfo.KeycloakURL)
		if err == nil {
			proxyConfig.OidcProviderHost = u.Host
			proxyConfig.OidcProviderHostInCluster = ""
		} else {
			zap.S().Errorf("Failed to parse keycloak URL %s", controller.clusterInfo.KeycloakURL)
		}
	}

	if len(controller.clusterInfo.clusterName) > 0 {
		proxyConfig.SSLEnabled = true
	}

	// return
	return proxyConfig
}

func addOidcProxyConfig(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) (string, error) {
	oidcConfig := resources.OidcProxyConfigName(vmo.Name, component.Name)
	oidcProxyConfig := getOidcProxyConfig(controller, vmo, component)
	oidcConfigMap, err := proxy.GetOidcProxyConfigMapData(oidcProxyConfig)
	if err != nil {
		return oidcConfig, err
	}
	err = createUpdateConfigMap(controller, vmo, oidcConfig, oidcConfigMap)
	return oidcConfig, err
}

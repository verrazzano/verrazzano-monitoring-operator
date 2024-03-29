// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/metricsexporter"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/configmaps"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/stretchr/testify/assert"
	vmctl "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateConfigmaps(t *testing.T) {
	client := fake.NewSimpleClientset()
	controller := &Controller{
		kubeclientset:   client,
		configMapLister: &simpleConfigMapLister{kubeClient: client},
		secretLister:    &simpleSecretLister{kubeClient: client},
		log:             vzlog.DefaultLogger(),
	}
	vmo := &vmctl.VerrazzanoMonitoringInstance{}
	vmo.Name = constants.VMODefaultName
	vmo.Namespace = constants.VerrazzanoSystemNamespace
	vmo.Spec.URI = "vmi.system.v8o-env.oracledx.com"
	vmo.Spec.Grafana.DashboardsConfigMap = "myDashboardsConfigMap"
	vmo.Spec.Grafana.DatasourcesConfigMap = "myDatasourcesConfigMap"

	previousCount := testutil.ToFloat64(metricsexporter.TestDelegate.GetCounterMetric(metricsexporter.NamesConfigMap))

	err := CreateConfigmaps(controller, vmo)
	t.Logf("Error is %v", err)
	assert.Nil(t, err)
	all, _ := client.CoreV1().ConfigMaps(vmo.Namespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 2, len(all.Items))

	newCount := testutil.ToFloat64(metricsexporter.TestDelegate.GetCounterMetric(metricsexporter.NamesConfigMap))
	newTimeStamp := testutil.ToFloat64(metricsexporter.TestDelegate.GetTimestampMetric(metricsexporter.NamesConfigMap).WithLabelValues(fmt.Sprintf("%v", newCount)))
	assert.Equal(t, previousCount+1, newCount)
	assert.LessOrEqual(t, int64(newTimeStamp*10)/10, time.Now().Unix())
}

// simple ConfigMapLister implementation
type simpleConfigMapLister struct {
	kubeClient kubernetes.Interface
}

// lists all ConfigMaps
func (s *simpleConfigMapLister) List(selector labels.Selector) ([]*v1.ConfigMap, error) {
	namespaces, err := s.kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var pods []*v1.ConfigMap
	for _, namespace := range namespaces.Items {

		list, err := s.ConfigMaps(namespace.Name).List(selector)
		if err != nil {
			return nil, err
		}
		pods = append(pods, list...)
	}
	return pods, nil
}

// ConfigMaps returns an object that can list and get ConfigMaps.
func (s *simpleConfigMapLister) ConfigMaps(namespace string) corelistersv1.ConfigMapNamespaceLister {
	return simpleConfigMapNamespaceLister{
		namespace:  namespace,
		kubeClient: s.kubeClient,
	}
}

// configMapNamespaceLister implements the ConfigMapNamespaceLister
// interface.
type simpleConfigMapNamespaceLister struct {
	namespace  string
	kubeClient kubernetes.Interface
}

// List lists all ConfigMaps for a given namespace.
func (s simpleConfigMapNamespaceLister) List(selector labels.Selector) ([]*v1.ConfigMap, error) {
	var configMaps []*v1.ConfigMap

	list, err := s.kubeClient.CoreV1().ConfigMaps(s.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range list.Items {
		if selector.Matches(labels.Set(list.Items[i].Labels)) {
			configMaps = append(configMaps, &list.Items[i])
		}
	}
	return configMaps, nil
}

// Get retrieves the ConfigMap for a given namespace and name.
func (s simpleConfigMapNamespaceLister) Get(name string) (*v1.ConfigMap, error) {
	return s.kubeClient.CoreV1().ConfigMaps(s.namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// simple SecretLister implementation
type simpleSecretLister struct {
	kubeClient kubernetes.Interface
}

// lists all Secrets
func (s *simpleSecretLister) List(selector labels.Selector) ([]*v1.Secret, error) {
	namespaces, err := s.kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var secrets []*v1.Secret
	for _, namespace := range namespaces.Items {

		list, err := s.Secrets(namespace.Name).List(selector)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, list...)
	}
	return secrets, nil
}

// Secrets returns an object that can get Secrets.
func (s *simpleSecretLister) Secrets(namespace string) corelistersv1.SecretNamespaceLister {
	return simpleSecretNamespaceLister{
		namespace:  namespace,
		kubeClient: s.kubeClient,
	}
}

// simpleSecretNamespaceLister implements the SecretNamespaceLister
// interface.
type simpleSecretNamespaceLister struct {
	namespace  string
	kubeClient kubernetes.Interface
}

// List lists all Secrets for a given namespace.
func (s simpleSecretNamespaceLister) List(selector labels.Selector) ([]*v1.Secret, error) {
	var secrets []*v1.Secret

	list, err := s.kubeClient.CoreV1().Secrets(s.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range list.Items {
		if selector.Matches(labels.Set(list.Items[i].Labels)) {
			secrets = append(secrets, &list.Items[i])
		}
	}
	return secrets, nil
}

// Get retrieves the Secret for a given namespace and name.
func (s simpleSecretNamespaceLister) Get(name string) (*v1.Secret, error) {
	return s.kubeClient.CoreV1().Secrets(s.namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// TestCreateUpdateDatasourcesConfigMap tests the createUpdateDatasourcesConfigMap function
func TestCreateUpdateDatasourcesConfigMap(t *testing.T) {
	const configMapName = "myDatasourcesConfigMap"

	// GIVEN a Grafana datasources configmap exists and the Prometheus URL is the legacy URL
	//  WHEN we call the createUpdateDatasourcesConfigMap
	//  THEN the configmap is updated and the Prometheus URL points to the new Prometheus instance
	vmo := &vmctl.VerrazzanoMonitoringInstance{}
	vmo.Name = constants.VMODefaultName
	vmo.Namespace = constants.VerrazzanoSystemNamespace

	// set the Prometheus URL to the legacy URL
	replaceMap := map[string]string{constants.GrafanaTmplPrometheusURI: resources.GetMetaName(vmo.Name, config.Prometheus.Name),
		constants.GrafanaTmplAlertManagerURI: ""}
	dataSourceTemplate, err := asDashboardTemplate(constants.DataSourcesTmpl, replaceMap)
	assert.NoError(t, err)

	cm := configmaps.NewConfig(vmo, configMapName, map[string]string{datasourceYAMLKey: dataSourceTemplate})

	client := fake.NewSimpleClientset(cm)
	controller := &Controller{
		kubeclientset:   client,
		configMapLister: &simpleConfigMapLister{kubeClient: client},
		secretLister:    &simpleSecretLister{kubeClient: client},
		log:             vzlog.DefaultLogger(),
	}

	err = createUpdateDatasourcesConfigMap(controller, vmo, configMapName, map[string]string{})
	assert.NoError(t, err)

	// fetch the configmap, the Prometheus URL should now be the new Prometheus URL
	cm, err = client.CoreV1().ConfigMaps(vmo.Namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Contains(t, cm.Data[datasourceYAMLKey], "url: http://"+prometheusOperatorPrometheusHost+":9090")
}

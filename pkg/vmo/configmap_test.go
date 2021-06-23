// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"testing"

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
	}
	vmo := &vmctl.VerrazzanoMonitoringInstance{}
	vmo.Name = "system"
	vmo.Namespace = "verrazzano-system"
	vmo.Spec.URI = "vmi.system.v8o-env.oracledx.com"
	vmo.Spec.Grafana.DashboardsConfigMap = "myDashboardsConfigMap"
	vmo.Spec.Grafana.DatasourcesConfigMap = "myDatasourcesConfigMap"
	vmo.Spec.AlertManager.ConfigMap = "myAlertManagerConfigMap"
	vmo.Spec.AlertManager.VersionsConfigMap = "myAlertManagerVersionsConfigMap"
	vmo.Spec.Prometheus.RulesConfigMap = "myPrometheusRulesConfigMap"
	vmo.Spec.Prometheus.RulesVersionsConfigMap = "myPrometheusRulesVersionsConfigMap"
	vmo.Spec.Prometheus.ConfigMap = "myPrometheusConfigMap"
	vmo.Spec.Prometheus.VersionsConfigMap = "myPrometheusVersionsConfigMap"
	err := CreateConfigmaps(controller, vmo)
	t.Logf("Error is %v", err)
	assert.Nil(t, err)
	all, _ := client.CoreV1().ConfigMaps(vmo.Namespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 12, len(all.Items))
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

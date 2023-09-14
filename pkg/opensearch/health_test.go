// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	fake "k8s.io/client-go/kubernetes/fake"
	appslistersv1 "k8s.io/client-go/listers/apps/v1"
)

const (
	wrongNodeVersion = `{
	"nodes": {
		"1": {
			"version": "1.2.3",
			"roles": [
				"master"
			]
		},
		"2": {
			"version": "1.2.3",
			"roles": [
				"ingest"
			]
		},
		"3": {
			"version": "1.2.3",
			"roles": [
				"data"
			]
		},
		"4": {
			"version": "1.2.0",
			"roles": [
				"data"
			]
		},
		"5": {
			"version": "1.2.3",
			"roles": [
				"data"
			]
		}
	}
}`
	wrongCountNodes = `{
	"nodes": {
		"1": {
			"version": "1.2.3",
			"roles": [
				"master"
			]
		}
	}
}`
	healthyNodes = `{
	"nodes": {
		"1": {
			"version": "1.2.3",
			"roles": [
				"master"
			]
		},
		"2": {
			"version": "1.2.3",
			"roles": [
				"ingest"
			]
		},
		"3": {
			"version": "1.2.3",
			"roles": [
				"data"
			]
		},
		"4": {
			"version": "1.2.3",
			"roles": [
				"data"
			]
		},
		"5": {
			"version": "1.2.3",
			"roles": [
				"data"
			]
		}
	}
}`
	healthyCluster = `{
	"status": "green",
    "number_of_data_nodes": 3
}`
	unhealthyClusterStatus = `{
		"status": "yellow",
		"number_of_data_nodes": 3
}`
)

var testvmo = vmcontrollerv1.VerrazzanoMonitoringInstance{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "system",
		Namespace: constants.VerrazzanoSystemNamespace,
	},
	Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
		Opensearch: vmcontrollerv1.Opensearch{
			Enabled: true,
			DataNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 3,
			},
			IngestNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 1,
			},
			MasterNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 1,
			},
		},
	},
}

var statefulSetLister = kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Apps().V1().StatefulSets().Lister()

func mockHTTPGenerator(body1, body2 string, code1, code2 int) func(request *http.Request) (*http.Response, error) {
	return func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "_cluster/health") {
			return &http.Response{
				StatusCode: code1,
				Body:       io.NopCloser(strings.NewReader(body1)),
			}, nil
		}
		return &http.Response{
			StatusCode: code2,
			Body:       io.NopCloser(strings.NewReader(body2)),
		}, nil
	}
}

func TestIsOpenSearchHealthy(t *testing.T) {
	config.ESWaitTargetVersion = "1.2.3"
	var tests = []struct {
		name     string
		httpFunc func(request *http.Request) (*http.Response, error)
		isError  bool
	}{
		{
			"healthy when cluster health is green and nodes are ready",
			mockHTTPGenerator(healthyCluster, healthyNodes, 200, 200),
			false,
		},
		{
			"unhealthy when cluster health is yellow",
			mockHTTPGenerator(unhealthyClusterStatus, healthyNodes, 200, 200),
			true,
		},
		{
			"unhealthy when expected node version is not all updated",
			mockHTTPGenerator(healthyNodes, wrongNodeVersion, 200, 200),
			true,
		},
		{
			"unhealthy when expected node version is not all updated",
			mockHTTPGenerator(healthyNodes, wrongCountNodes, 200, 200),
			true,
		},
		{
			"unhealthy when cluster status code is not OK",
			mockHTTPGenerator(healthyCluster, healthyNodes, 403, 200),
			true,
		},
		{
			"unhealthy when cluster is unreachable",
			func(request *http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewOSClient(statefulSetLister)
			o.DoHTTP = tt.httpFunc
			err := o.IsUpdated(&testvmo)
			if tt.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsOpenSearchResizable(t *testing.T) {
	var notEnoughNodesVMO = vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system",
			Namespace: constants.VerrazzanoSystemNamespace,
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Opensearch: vmcontrollerv1.Opensearch{
				DataNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 1,
				},
				IngestNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 1,
				},
				MasterNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 1,
				},
			},
		},
	}
	o := NewOSClient(statefulSetLister)
	assert.Error(t, o.IsDataResizable(&notEnoughNodesVMO))
}

// simple StatefulsetLister implementation
type simpleStatefulSetLister struct {
	kubeClient kubernetes.Interface
}

// lists all StatefulSets
func (s *simpleStatefulSetLister) List(selector labels.Selector) ([]*appsv1.StatefulSet, error) {
	namespaces, err := s.kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var statefulSets []*appsv1.StatefulSet
	for _, namespace := range namespaces.Items {

		list, err := s.StatefulSets(namespace.Name).List(selector)
		if err != nil {
			return nil, err
		}
		statefulSets = append(statefulSets, list...)
	}
	return statefulSets, nil
}

// StatefulSets returns an object that can list and get StatefulSets.
func (s *simpleStatefulSetLister) StatefulSets(namespace string) appslistersv1.StatefulSetNamespaceLister {
	return simpleStatefulSetNamespaceLister{
		namespace:  namespace,
		kubeClient: s.kubeClient,
	}
}

// GetPodStatefulSets is a fake implementation for StatefulSetLister.GetPodStatefulSets
func (s *simpleStatefulSetLister) GetPodStatefulSets(pod *v1.Pod) ([]*appsv1.StatefulSet, error) {
	return nil, nil
}

// simpleStatefulSetNamespaceLister implements the StatefulSetNamespaceLister
// interface.
type simpleStatefulSetNamespaceLister struct {
	namespace  string
	kubeClient kubernetes.Interface
}

// List lists all StatefulSets for a given namespace.
func (s simpleStatefulSetNamespaceLister) List(selector labels.Selector) ([]*appsv1.StatefulSet, error) {
	var statefulSets []*appsv1.StatefulSet

	list, err := s.kubeClient.AppsV1().StatefulSets(s.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range list.Items {
		if selector.Matches(labels.Set(list.Items[i].Labels)) {
			statefulSets = append(statefulSets, &list.Items[i])
		}
	}
	return statefulSets, nil
}

// GetPodStatefulSets is a fake implementation for StatefulSetNamespaceLister.GetPodStatefulSets
func (s *simpleStatefulSetNamespaceLister) GetPodStatefulSets(pod *v1.Pod) ([]*appsv1.StatefulSet, error) {
	return nil, nil
}

// Get is a fake implementation for StatefulSetNamespaceLister.Get
func (s simpleStatefulSetNamespaceLister) Get(name string) (*appsv1.StatefulSet, error) {
	return nil, nil
}

// TestIsOpenSearchReady Tests the value returned by  IsOpenSearchReady in few scenarios
// GIVEN a default VMI instance
// WHEN I call IsOpenSearchReady
// THEN the IsOpenSearchReady returns true only when the Opensearch StatefulSet pods are ready, false otherwise
func TestIsOpenSearchReady(t *testing.T) {
	var tests = []struct {
		name      string
		clientSet *fake.Clientset
		isReady   bool
	}{
		{
			"not ready when no stateful set is found",
			fake.NewSimpleClientset(),
			false,
		},
		{
			"not ready when opensearch statefulset is not found",
			fake.NewSimpleClientset(&appsv1.StatefulSet{}),
			false,
		},
		{
			"not ready when opensearch statefulset set is not ready",
			fake.NewSimpleClientset(&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						constants.VMOLabel: constants.VMODefaultName, constants.ComponentLabel: constants.ComponentOpenSearchValue,
					},
					Namespace: testvmo.GetNamespace(),
				},
				Status: appsv1.StatefulSetStatus{
					Replicas: 1,
				},
			}),
			false,
		},
		{
			"ready when opensearch statefulset set is ready",
			fake.NewSimpleClientset(&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						constants.VMOLabel: constants.VMODefaultName, constants.ComponentLabel: constants.ComponentOpenSearchValue,
					},
					Namespace: testvmo.GetNamespace(),
				},
				Status: appsv1.StatefulSetStatus{
					Replicas:      1,
					ReadyReplicas: 1,
				},
			}),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewOSClient(&simpleStatefulSetLister{kubeClient: tt.clientSet})
			ready := o.IsOpenSearchReady(&testvmo)
			assert.Equal(t, ready, tt.isReady)
		})
	}
}

// TestSetAutoExpandIndicesNoErrorWhenOSNotReady Tests the SetAutoExpandIndices does not return error when OpenSearch is not ready
// GIVEN a default VMI instance
// WHEN I call SetAutoExpandIndices
// THEN the SetAutoExpandIndices does not return error when the Opensearch StatefulSet pods are not ready
func TestSetAutoExpandIndicesNoErrorWhenOSNotReady(t *testing.T) {
	o := NewOSClient(&simpleStatefulSetLister{kubeClient: fake.NewSimpleClientset(&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				constants.VMOLabel: constants.VMODefaultName, constants.ComponentLabel: constants.ComponentOpenSearchValue,
			},
			Namespace: testvmo.GetNamespace(),
		},
		Status: appsv1.StatefulSetStatus{
			Replicas: 1,
		},
	})})
	assert.NoError(t, <-o.SetAutoExpandIndices(&vmcontrollerv1.VerrazzanoMonitoringInstance{Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
		Opensearch: vmcontrollerv1.Opensearch{
			Enabled: true,
			MasterNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 1,
				Roles:    []vmcontrollerv1.NodeRole{vmcontrollerv1.MasterRole},
			},
		},
	}}))
}

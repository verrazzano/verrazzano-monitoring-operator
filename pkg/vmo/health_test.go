// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"errors"
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
	"testing"
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
		Elasticsearch: vmcontrollerv1.Elasticsearch{
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

func mockHTTPGenerator(body1, body2 string, code1, code2 int) func(client *http.Client, request *http.Request) (*http.Response, error) {
	return func(client *http.Client, request *http.Request) (*http.Response, error) {
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
		httpFunc func(client *http.Client, request *http.Request) (*http.Response, error)
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
			func(client *http.Client, request *http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doHTTP = tt.httpFunc
			err := IsOpenSearchUpdated(&testvmo)
			if tt.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			resetDoHTTP()
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
			Elasticsearch: vmcontrollerv1.Elasticsearch{
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
	assert.Error(t, IsOpenSearchResizable(&notEnoughNodesVMO))
}

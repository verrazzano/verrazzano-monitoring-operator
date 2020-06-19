// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package services

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestElasticsearchDefaultServices1(t *testing.T) {
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "mySauron",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				IngestNode: vmcontrollerv1.ElasticsearchNode{Replicas: 5},
				MasterNode: vmcontrollerv1.ElasticsearchNode{Replicas: 4},
				DataNode:   vmcontrollerv1.ElasticsearchNode{Replicas: 3},
				Enabled:    true,
			},
		},
	}
	services := createElasticsearchServiceElements(sauron)
	assert.Equal(t, 4, len(services), "Length of generated services")
}

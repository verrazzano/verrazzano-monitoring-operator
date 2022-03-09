// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package services

import (
	"testing"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestElasticsearchDefaultServices1(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "myVMO",
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
	services := createElasticsearchServiceElements(vmo)
	assert.Equal(t, 4, len(services), "Length of generated services")
}

func TestElasticsearchDevProfileDefaultServices(t *testing.T) {
	vmo := createDevProfileES()

	services := createElasticsearchServiceElements(vmo)
	assert.Equal(t, 4, len(services), "Length of generated services")

	ingestService := services[0]
	masterService := services[1]
	dataService := services[2]
	masterHTTPService := services[3]

	expectedSelector := resources.GetSpecID(vmo.Name, config.ElasticsearchMaster.Name)

	assert.Equal(t, ingestService.Spec.Selector, expectedSelector)
	assert.EqualValues(t, ingestService.Spec.Ports[0].Port, constants.ESHttpPort)

	assert.EqualValues(t, masterService.Spec.Ports[0].Port, constants.ESTransportPort)

	assert.EqualValues(t, dataService.Spec.Ports[0].Port, constants.ESHttpPort)
	assert.Equal(t, dataService.Spec.Ports[0].TargetPort, intstr.FromInt(constants.ESHttpPort))
	assert.Equal(t, dataService.Spec.Selector, expectedSelector)

	assert.EqualValues(t, constants.ESHttpPort, masterHTTPService.Spec.Ports[0].Port)
	assert.EqualValues(t, intstr.FromInt(constants.ESHttpPort), masterHTTPService.Spec.Ports[0].TargetPort)
}

func createDevProfileES() *vmcontrollerv1.VerrazzanoMonitoringInstance {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "myDevVMO",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
				Storage: vmcontrollerv1.Storage{Size: ""},
				MasterNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 1,
				},
			},
		},
	}
	return vmo
}

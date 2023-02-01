// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsOpenSearchDeployment(t *testing.T) {
	createDeploy := func() *appsv1.Deployment {
		return &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: v1.ObjectMeta{},
				},
			},
		}
	}

	deployWrongLabels := createDeploy()
	deployWrongLabels.Spec.Template.Labels = map[string]string{
		"foo": "bar",
		"app": "system-es-master",
	}
	deployRightLabels := createDeploy()
	deployRightLabels.Spec.Template.Labels = map[string]string{
		"a":   "b",
		"app": "system-es-data",
	}

	var tests = []struct {
		name     string
		deploy   *appsv1.Deployment
		expected bool
	}{
		{
			"doesn't match deploy with no labels",
			&appsv1.Deployment{},
			false,
		},
		{
			"doesn't match deploy with wrong labels",
			deployWrongLabels,
			false,
		},
		{
			"matches deploy with data deployment labels",
			deployRightLabels,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsOpenSearchDataDeployment("system", tt.deploy))
		})
	}
}

func TestElasticsearchDefaultDeployments1(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "myVMO",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Opensearch: vmcontrollerv1.Opensearch{
				IngestNode: vmcontrollerv1.ElasticsearchNode{Replicas: 1},
				MasterNode: vmcontrollerv1.ElasticsearchNode{Replicas: 1},
				DataNode:   vmcontrollerv1.ElasticsearchNode{Replicas: 1},
				Enabled:    true,
				Storage: vmcontrollerv1.Storage{
					Size:     "50GI",
					PvcNames: []string{"pvc1"},
				},
			},
		},
	}
	var es Elasticsearch = ElasticsearchBasic{}
	deployments := es.createElasticsearchDeploymentElements(vmo, map[string]string{})
	assert.Equal(t, 2, len(deployments), "Length of generated deployments")
}

func TestElasticsearchDefaultDeployments2(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "myVMO",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Opensearch: vmcontrollerv1.Opensearch{
				IngestNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 5,
					Roles:    []vmcontrollerv1.NodeRole{vmcontrollerv1.IngestRole},
					Name:     config.ElasticsearchIngest.Name,
				},
				MasterNode: vmcontrollerv1.ElasticsearchNode{Replicas: 4},
				DataNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 3,
					Storage: &vmcontrollerv1.Storage{
						Size:     "50GI",
						PvcNames: []string{"pvc1", "pvc2", "pvc3"},
					},
					Roles: []vmcontrollerv1.NodeRole{vmcontrollerv1.DataRole},
					Name:  config.ElasticsearchData.Name,
				},
				Enabled: true,
			},
		},
	}
	var es Elasticsearch = ElasticsearchBasic{}
	deployments := es.createElasticsearchDeploymentElements(vmo, map[string]string{})
	assert.Equal(t, 4, len(deployments), "Length of generated deployments")

	ingestDeployment, _ := getDeploymentByName(resources.GetMetaName(vmo.Name, config.ElasticsearchIngest.Name), deployments)
	assert.NotNil(t, ingestDeployment, "Client deployment")
	assert.Equal(t, int32(5), *ingestDeployment.Spec.Replicas, "Client replicas")
	ingestEnv := ingestDeployment.Spec.Template.Spec.Containers[0].Env
	assert.Equal(t, "ingest", getEnvVarValue("node.roles", ingestEnv))

	for i := 0; i < 3; i++ {
		dataDeployment, _ := getDeploymentByName(resources.GetMetaName(vmo.Name, fmt.Sprintf("%s-%d", config.ElasticsearchData.Name, i)), deployments)
		assert.Equal(t, "pvc"+strconv.Itoa(i+1), dataDeployment.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName, fmt.Sprintf("PVC for index %d", i))
		assert.NotNil(t, dataDeployment, fmt.Sprintf("DataNodes deployment for index %d", i))
		assert.Equal(t, int32(1), *dataDeployment.Spec.Replicas, fmt.Sprintf("DataNodes replicas for index %d", i))
		assert.Equal(t, "data", getEnvVarValue("node.roles", dataDeployment.Spec.Template.Spec.Containers[0].Env))
	}
}

func getEnvVarValue(envVarName string, envVarList []corev1.EnvVar) string {
	for _, envVar := range envVarList {
		if envVar.Name == envVarName {
			return envVar.Value
		}
	}
	return ""
}

// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestElasticsearchDefaultDeployments1(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "myVMO",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Elasticsearch: vmcontrollerv1.Elasticsearch{
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
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				IngestNode: vmcontrollerv1.ElasticsearchNode{Replicas: 5},
				MasterNode: vmcontrollerv1.ElasticsearchNode{Replicas: 4},
				DataNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 3,
					Storage: &vmcontrollerv1.Storage{
						Size:     "50GI",
						PvcNames: []string{"pvc1", "pvc2", "pvc3"},
					},
				},
				Enabled: true,
			},
		},
	}
	var es Elasticsearch = ElasticsearchBasic{}
	deployments := es.createElasticsearchDeploymentElements(vmo, map[string]string{})
	assert.Equal(t, 4, len(deployments), "Length of generated deployments")

	clientDeployment, _ := getDeploymentByName(resources.GetMetaName(vmo.Name, config.ElasticsearchIngest.Name), deployments)
	assert.NotNil(t, clientDeployment, "Client deployment")
	assert.Equal(t, int32(5), *clientDeployment.Spec.Replicas, "Client replicas")
	assert.Equal(t, "false", getEnvVarValue("node.master", clientDeployment.Spec.Template.Spec.Containers[0].Env), "MasterNodes setting on client")
	assert.Equal(t, "false", getEnvVarValue("node.data", clientDeployment.Spec.Template.Spec.Containers[0].Env), "DataNodes setting on client")
	assert.Equal(t, "true", getEnvVarValue("node.ingest", clientDeployment.Spec.Template.Spec.Containers[0].Env), "IngestNodes setting on client")

	//	masterDeployment, _ := getDeploymentByName(resources.GetMetaName(vmo.Name, constants.ElasticsearchMaster.Name), deployments)
	//	assert.NotNil(t, masterDeployment, "MasterNodes deployment")
	//	assert.Equal(t, int32(4), *masterDeployment.Spec.Replicas, "MasterNodes replicas")
	//	assert.Equal(t, "true", getEnvVarValue("NODE_MASTER", masterDeployment.Spec.Template.Spec.Containers[0].Env), "MasterNodes setting on master")
	//	assert.Equal(t, "false", getEnvVarValue("NODE_DATA", masterDeployment.Spec.Template.Spec.Containers[0].Env), "DataNodes setting on master")
	//	assert.Equal(t, "false", getEnvVarValue("NODE_INGEST", masterDeployment.Spec.Template.Spec.Containers[0].Env), "IngestNodes setting on master")
	//	assert.NotNil(t, masterDeployment.Spec.Template.Spec.Containers[0].LivenessProbe, "MasterNodes deployment liveness probe")
	//	assert.NotNil(t, masterDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe, "MasterNodes deployment readiness probe")

	for i := 0; i < 3; i++ {
		dataDeployment, _ := getDeploymentByName(resources.GetMetaName(vmo.Name, fmt.Sprintf("%s-%d", config.ElasticsearchData.Name, i)), deployments)
		assert.Equal(t, "pvc"+strconv.Itoa(i+1), dataDeployment.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName, fmt.Sprintf("PVC for index %d", i))
		assert.NotNil(t, dataDeployment, fmt.Sprintf("DataNodes deployment for index %d", i))
		assert.Equal(t, int32(1), *dataDeployment.Spec.Replicas, fmt.Sprintf("DataNodes replicas for index %d", i))
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

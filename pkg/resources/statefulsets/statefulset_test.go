// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"fmt"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
)

func TestVMOEmptyStatefulSetSize(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	statefulsets, err := New(vmo)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 0, len(statefulsets), "Length of generated statefulsets")
}

func TestVMOWithReplicas(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			AlertManager: vmcontrollerv1.AlertManager{
				Enabled:  true,
				Replicas: 3,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
				MasterNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 5,
				},
				Storage: vmcontrollerv1.Storage{
					Size: "50Gi",
				},
			},
		},
	}
	statefulsets, err := New(vmo)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 2, len(statefulsets), "Length of generated statefulsets")
	for _, statefulset := range statefulsets {
		switch statefulset.Name {
		case resources.GetMetaName(vmo.Name, config.AlertManager.Name):
			assert.Equal(t, *resources.NewVal(3), *statefulset.Spec.Replicas, "AlertManager replicas")
		case resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name):
			verifyElasticSearch(t, vmo, statefulset)

		default:
			t.Error("Unknown Deployment Name: " + statefulset.Name)
		}
		if statefulset.Name == resources.GetMetaName(vmo.Name, config.AlertManager.Name) {
			assert.Equal(t, *resources.NewVal(3), *statefulset.Spec.Replicas, "AlertManager replicas")
		}
	}
}

// Verify the Statefulset used by Elasticsearch master
func verifyElasticSearch(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, sts *appsv1.StatefulSet) {
	assert := assert.New(t)
	const esMasterVolName = "elasticsearch-master"
	const esMasterData = "/usr/share/elasticsearch/data"

	assert.Equal(*resources.NewVal(5), *sts.Spec.Replicas, "Incorrect Elasticsearch Master replicas count")
	affin := resources.CreateZoneAntiAffinityElement(vmo.Name, config.ElasticsearchMaster.Name)
	assert.Equal(affin, sts.Spec.Template.Spec.Affinity, "Incorrect Elasticsearch affinity")
	var elasticsearchUID int64 = 1000
	assert.Equal(elasticsearchUID, *sts.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser,
		"Incorrect Elasticsearch.SecurityContext.RunAsUser")

	assert.Len(sts.Spec.Template.Spec.Containers, 1, "Incorrect number of Containers")
	assert.Len(sts.Spec.Template.Spec.Containers[0].Ports, 2, "Incorrect number of Ports")
	assert.Equal("transport", sts.Spec.Template.Spec.Containers[0].Ports[0].Name, "Incorrect Container Port")
	assert.Zero(sts.Spec.Template.Spec.Containers[0].Ports[0].HostPort, "Incorrect Container HostPort")
	assert.Equal(int32(constants.ESTransportPort), sts.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort, "Incorrect Container HostPort")
	assert.Equal("http", sts.Spec.Template.Spec.Containers[0].Ports[1].Name, "Incorrect Container Port")
	assert.Zero(sts.Spec.Template.Spec.Containers[0].Ports[1].HostPort, "Incorrect Container HostPort")
	assert.Equal(int32(constants.ESHttpPort), sts.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort, "Incorrect Container HostPort")

	var i int32
	initialMasterNodes := make([]string, 0)
	for i = 0; i < *sts.Spec.Replicas; i++ {
		initialMasterNodes = append(initialMasterNodes, resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)+"-"+fmt.Sprintf("%d", i))
	}
	assert.Len(sts.Spec.Template.Spec.Containers[0].Env, 8, "Incorrect number of Env Vars")
	assert.Equal("discovery.seed_hosts", sts.Spec.Template.Spec.Containers[0].Env[0].Name, "Incorrect Env[0].Name")
	assert.Equal(resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name), sts.Spec.Template.Spec.Containers[0].Env[0].Value, "Incorrect Env[0].Value")
	assert.Equal("node.name", sts.Spec.Template.Spec.Containers[0].Env[1].Name, "Incorrect Env[1].Name")
	assert.Equal("metadata.name", sts.Spec.Template.Spec.Containers[0].Env[1].ValueFrom.FieldRef.FieldPath,
		"Incorrect Env[1].ValueFrom")
	assert.Equal("cluster.name", sts.Spec.Template.Spec.Containers[0].Env[2].Name, "Incorrect Env[2].Name")
	assert.Equal(vmo.Name, sts.Spec.Template.Spec.Containers[0].Env[2].Value, "Incorrect Env[2].Value")
	assert.Equal("node.master", sts.Spec.Template.Spec.Containers[0].Env[3].Name, "Incorrect Env[3].Name")
	assert.Equal("true", sts.Spec.Template.Spec.Containers[0].Env[3].Value, "Incorrect Env[3].Value")
	assert.Equal("node.ingest", sts.Spec.Template.Spec.Containers[0].Env[4].Name, "Incorrect Env[4].Name")
	assert.Equal("false", sts.Spec.Template.Spec.Containers[0].Env[4].Value, "Incorrect Env[4].Value")
	assert.Equal("node.data", sts.Spec.Template.Spec.Containers[0].Env[5].Name, "Incorrect Env[5].Name")
	assert.Equal("false", sts.Spec.Template.Spec.Containers[0].Env[5].Value, "Incorrect Env[5].Value")
	assert.Equal("HTTP_ENABLE", sts.Spec.Template.Spec.Containers[0].Env[6].Name, "Incorrect Env[6].Name")
	assert.Equal("true", sts.Spec.Template.Spec.Containers[0].Env[6].Value, "Incorrect Env[6].Value")
	assert.Equal("cluster.initial_master_nodes", sts.Spec.Template.Spec.Containers[0].Env[7].Name, "Incorrect Env[7].Name")
	assert.Equal(strings.Join(initialMasterNodes, ","), sts.Spec.Template.Spec.Containers[0].Env[7].Value, "Incorrect Env[7].Value")

	assert.Equal(int32(90), sts.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds,
		"Incorrect Readiness Probe InitialDelaySeconds")
	assert.Equal(int32(3), sts.Spec.Template.Spec.Containers[0].ReadinessProbe.SuccessThreshold,
		"Incorrect Readiness Probe SuccessThreshold")
	assert.Equal(int32(5), sts.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds,
		"Incorrect Readiness Probe PeriodSeconds")
	assert.Equal(int32(5), sts.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds,
		"Incorrect Readiness Probe TimeoutSeconds")

	assert.Equal(int32(int32(config.ElasticsearchMaster.Port)), sts.Spec.Template.Spec.Containers[0].LivenessProbe.Handler.TCPSocket.Port.IntVal,
		"Incorrect LivenessProbe Probe Port")
	assert.Equal(int32(10), sts.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds,
		"Incorrect LivenessProbe Probe InitialDelaySeconds")
	assert.Equal(int32(10), sts.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds,
		"Incorrect LivenessProbe Probe PeriodSeconds")
	assert.Equal(int32(5), sts.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds,
		"Incorrect LivenessProbe Probe TimeoutSeconds")
	assert.Equal(int32(5), sts.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold,
		"Incorrect LivenessProbe Probe FailureThreshold")

	assert.Len(sts.Spec.Template.Spec.Containers[0].VolumeMounts, 1, "Incorrect number of VolumeMounts")
	assert.Equal(esMasterVolName, sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name, "Incorrect VolumeMount name")
	assert.Equal(esMasterData, sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath, "Incorrect VolumeMount mount path")

	assert.Len(sts.Spec.Template.Spec.InitContainers, 1, "Incorrect number of InitContainers")
	assert.Len(sts.Spec.Template.Spec.InitContainers[0].VolumeMounts, 1, "Incorrect number of VolumeMounts")
	assert.Equal(esMasterVolName, sts.Spec.Template.Spec.InitContainers[0].VolumeMounts[0].Name, "Incorrect VolumeMount name")
	assert.Equal(esMasterData, sts.Spec.Template.Spec.InitContainers[0].VolumeMounts[0].MountPath, "Incorrect VolumeMount mount path")

	assert.Len(sts.Spec.VolumeClaimTemplates, 1, "Incorrect number of VolumeClaimTemplates")
	assert.Equal(sts.Spec.VolumeClaimTemplates[0].ObjectMeta.Name, esMasterVolName, "Incorrect VolumeClaimTemplate name")
	assert.Equal(sts.Spec.VolumeClaimTemplates[0].ObjectMeta.Namespace, vmo.Namespace, "Incorrect VolumeClaimTemplate name")
	assert.Len(sts.Spec.VolumeClaimTemplates[0].Spec.AccessModes, 1, "Incorrect number of VolumeClaimTemplate accesss modes")
	assert.Equal(sts.Spec.VolumeClaimTemplates[0].Spec.AccessModes[0], corev1.ReadWriteOnce, "Incorrect VolumeClaimTemplate accesss modes")
	assert.Equal(sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage], resource.MustParse(vmo.Spec.Elasticsearch.Storage.Size),
		"Incorrect VolumeClaimTemplate resource request size")

}

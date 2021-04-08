// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"fmt"
	"strings"
	"testing"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
)

// TestVMOEmptyStatefulSetSize tests the creation of a VMI without StatefulSets
// GIVEN a VMI spec with empty AlertManager and ElasticSearch specs
//  WHEN I call New
//  THEN there should be no StatefulSets created
func TestVMOEmptyStatefulSetSize(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	statefulsets, err := New(vmo, false)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 0, len(statefulsets), "Incorrect number of statefulsets")
}

// TestVMOEmptyStatefulSetSize tests the creation of a VMI without StatefulSets
// GIVEN a VMI spec with both AlertManager and ElasticSearch specs having 'enabled' set to false
//  WHEN I call New
//  THEN there should be no StatefulSets created
func TestVMODisabledSpecs(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			AlertManager: vmcontrollerv1.AlertManager{
				Enabled: false,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: false,
			},
		},
	}
	statefulsets, err := New(vmo, false)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 0, len(statefulsets), "Incorrect number of statefulsets")
}

// TestVMOProdProfile tests the creation of a VMI StatefulSets for a Production profile
// GIVEN a VMI spec with an AlertManager spec and an ElasticSearch spec
//  WHEN I call New
//  THEN there should a StatefulSet for AlertManager and one for ElasticSearch
//   AND those objects should have the expected values
func TestVMOProdProfile(t *testing.T) {
	runTestVMO(t, false)
}

// TestVMODevProfile tests the creation of a VMI StatefulSets for a Development/small-memory profile
// GIVEN a VMI spec with an AlertManager spec and an ElasticSearch spec
//  WHEN I call New
//  THEN there should a StatefulSet for AlertManager and one for ElasticSearch
//   AND those objects should have the expected values
//   AND ElasticSearch should be configured for a single-node cluster type
func TestVMODevProfile(t *testing.T) {
	runTestVMO(t, true)
}

func runTestVMO(t *testing.T, isDevProfileTest bool) {
	// Initialize
	var alertManagerReplicas int32 = 3
	var masterNodeReplicas int32 = 3
	var dataNodeReplicas int32 = 2
	var ingestNodeReplicas int32 = 1
	storageSize := "50Gi"

	if isDevProfileTest {
		//alertManagerReplicas := 3
		masterNodeReplicas = 1
		dataNodeReplicas = 0
		ingestNodeReplicas = 0
		storageSize = ""
	}

	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			AlertManager: vmcontrollerv1.AlertManager{
				Enabled:  true,
				Replicas: alertManagerReplicas,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled:    true,
				MasterNode: vmcontrollerv1.ElasticsearchNode{Replicas: masterNodeReplicas},
				DataNode:   vmcontrollerv1.ElasticsearchNode{Replicas: dataNodeReplicas},
				IngestNode: vmcontrollerv1.ElasticsearchNode{Replicas: ingestNodeReplicas},
				Storage: vmcontrollerv1.Storage{
					Size: storageSize,
				},
			},
		},
	}

	// Create the stateful sets
	statefulsets, err := New(vmo, false)
	if err != nil {
		t.Error(err)
	}

	if isDevProfileTest {
		assert.True(t, resources.IsSingleNodeESCluster(vmo), "Single node ES setup, expected IsSingleNodeESCluster to be true")
		assert.False(t, resources.IsValidMultiNodeESCluster(vmo), "Single node ES setup, expected IsValidMultiNodeESCluster to be false")
		verifyDevProfileVMOComponents(t, statefulsets, vmo, alertManagerReplicas, masterNodeReplicas, storageSize)
	} else {
		assert.False(t, resources.IsSingleNodeESCluster(vmo), "Single node ES setup, expected IsSingleNodeESCluster to be false")
		assert.True(t, resources.IsValidMultiNodeESCluster(vmo), "Single node ES setup, expected IsValidMultiNodeESCluster to be true")
		verifyProdProfileVMOComponents(t, statefulsets, vmo, alertManagerReplicas, masterNodeReplicas, storageSize)
	}
}

func verifyProdProfileVMOComponents(t *testing.T, statefulsets []*appsv1.StatefulSet, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance,
	alertManagerReplicas int32, elasticSearchReplicas int32, storageSize string) {
	// Do assertions
	assert.Equal(t, 2, len(statefulsets), "Incorrect number of statefulsets")
	for _, statefulset := range statefulsets {
		switch statefulset.Name {
		case resources.GetMetaName(vmo.Name, config.AlertManager.Name):
			verifyAlertManager(t, vmo, statefulset, alertManagerReplicas)
		case resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name):
			verifyElasticSearch(t, vmo, statefulset, elasticSearchReplicas, storageSize)
		default:
			t.Error("Unknown Deployment Name: " + statefulset.Name)
		}
	}
}

func verifyDevProfileVMOComponents(t *testing.T, statefulsets []*appsv1.StatefulSet, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance,
	alertManagerReplicas int32, elasticSearchReplicas int32, storageSize string) {
	// Do assertions
	assert.Equal(t, 2, len(statefulsets), "Incorrect number of statefulsets")
	for _, statefulset := range statefulsets {
		switch statefulset.Name {
		case resources.GetMetaName(vmo.Name, config.AlertManager.Name):
			verifyAlertManager(t, vmo, statefulset, alertManagerReplicas)
		case resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name):
			verifyElasticSearchDevProfile(t, vmo, statefulset, elasticSearchReplicas, storageSize)
		default:
			t.Error("Unknown Deployment Name: " + statefulset.Name)
		}
	}
}

// Verify the Statefulset used by Alert Manager
func verifyAlertManager(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance,
	sts *appsv1.StatefulSet, replicas int32) {

	assert := assert.New(t)

	assert.Equal(*resources.NewVal(int32(replicas)), *sts.Spec.Replicas, "Incorrect AlertManager replicas count")
	affin := resources.CreateZoneAntiAffinityElement(vmo.Name, config.AlertManager.Name)
	assert.Equal(affin, sts.Spec.Template.Spec.Affinity, "Incorrect  affinity")

	assert.Len(sts.Spec.Template.Spec.Containers, 2, "Incorrect number of Containers")
	assert.Equal(config.AlertManager.ImagePullPolicy, sts.Spec.Template.Spec.Containers[0].ImagePullPolicy, "Incorrect Image Pull Policy")
	assert.Len(sts.Spec.Template.Spec.Containers[0].Command, 1, "Incorrect number of Commands")
	assert.Equal("/bin/alertmanager", sts.Spec.Template.Spec.Containers[0].Command[0], "Incorrect Command")

	assert.Len(sts.Spec.Template.Spec.Containers[0].Args, 5, "Incorrect number of Args")
	assert.Equal(fmt.Sprintf("--config.file=%s", constants.AlertManagerConfigContainerLocation),
		sts.Spec.Template.Spec.Containers[0].Args[0], "Incorrect Arg[0]")
	assert.Equal(fmt.Sprintf("--cluster.listen-address=0.0.0.0:%d", config.AlertManagerCluster.Port),
		sts.Spec.Template.Spec.Containers[0].Args[1], "Incorrect Arg[1]")
	assert.Equal(fmt.Sprintf("--cluster.advertise-address=$(POD_IP):%d", config.AlertManagerCluster.Port),
		sts.Spec.Template.Spec.Containers[0].Args[2], "Incorrect Arg[2]")
	assert.Equal("--cluster.pushpull-interval=10s",
		sts.Spec.Template.Spec.Containers[0].Args[3], "Incorrect Arg[3]")
	alertManagerClusterService := resources.GetMetaName(vmo.Name, config.AlertManagerCluster.Name)
	firstReplicaName := fmt.Sprintf("%s-%d.%s", sts.Name, 0, alertManagerClusterService)
	assert.Equal(fmt.Sprintf("--cluster.peer=%s:%d", firstReplicaName, config.AlertManagerCluster.Port),
		sts.Spec.Template.Spec.Containers[0].Args[4], "Incorrect Arg[4]")

	assert.Len(sts.Spec.Template.Spec.Containers[0].Env, 1, "Incorrect number of Env Vars")
	assert.Equal("POD_IP", sts.Spec.Template.Spec.Containers[0].Env[0].Name, "Incorrect Env[0].Name")
	assert.Equal("v1", sts.Spec.Template.Spec.Containers[0].Env[0].ValueFrom.FieldRef.APIVersion,
		"Incorrect Env[0].ValueFrom.APIVersion")
	assert.Equal("status.podIP", sts.Spec.Template.Spec.Containers[0].Env[0].ValueFrom.FieldRef.FieldPath,
		"Incorrect Env[0].ValueFrom.FieldPath")

	assert.Equal(int32(5), sts.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds,
		"Incorrect LivenessProbe Probe InitialDelaySeconds")
	assert.Equal(int32(1), sts.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds,
		"Incorrect LivenessProbe Probe TimeoutSeconds")
	assert.Equal(int32(10), sts.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds,
		"Incorrect LivenessProbe Probe PeriodSeconds")

	assert.Equal(int32(5), sts.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds,
		"Incorrect LivenessProbe Probe InitialDelaySeconds")
	assert.Equal(int32(1), sts.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds,
		"Incorrect LivenessProbe Probe TimeoutSeconds")
	assert.Equal(int32(10), sts.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds,
		"Incorrect LivenessProbe Probe PeriodSeconds")

	const volName = "alert-config-volume"
	assert.Len(sts.Spec.Template.Spec.Volumes, 1, "Incorrect number of VolumeMounts")
	assert.Equal(volName, sts.Spec.Template.Spec.Volumes[0].Name, "Incorrect VolumeMount name")
	assert.Equal(corev1.LocalObjectReference{Name: vmo.Spec.AlertManager.ConfigMap}, sts.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference,
		"Incorrect VolumeMount VolumeSource.ConfigMap.LocalObjectReference")

	assert.Len(sts.Spec.Template.Spec.Containers[0].VolumeMounts, 1, "Incorrect number of VolumeMounts")
	assert.Equal(volName, sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name, "Incorrect VolumeMount name")
	assert.Equal(constants.AlertManagerConfigMountPath, sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath, "Incorrect VolumeMount mount path")

	assert.Len(sts.Spec.Template.Spec.Containers[1].Args, 2, "Incorrect number of Container[1] Args")
	assert.Equal("-volume-dir="+constants.AlertManagerConfigMountPath, sts.Spec.Template.Spec.Containers[1].Args[0],
		"Incorrect number of Container[1] Arg[0]")
}

// Verify the Statefulset used by Elasticsearch master
func verifyElasticSearch(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance,
	sts *appsv1.StatefulSet, replicas int32, storageSize string) {

	assert := assert.New(t)
	const esMasterVolName = "elasticsearch-master"
	const esMasterData = "/usr/share/elasticsearch/data"

	assert.Equal(*resources.NewVal(int32(replicas)), *sts.Spec.Replicas, "Incorrect Elasticsearch Master replicas count")
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
	assert.Equal("discovery.seed_hosts", sts.Spec.Template.Spec.Containers[0].Env[3].Name, "Incorrect Env[3].Name")
	assert.Equal(resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name), sts.Spec.Template.Spec.Containers[0].Env[3].Value, "Incorrect Env[3].Value")
	assert.Equal("node.name", sts.Spec.Template.Spec.Containers[0].Env[0].Name, "Incorrect Env[0].Name")
	assert.Equal("metadata.name", sts.Spec.Template.Spec.Containers[0].Env[0].ValueFrom.FieldRef.FieldPath,
		"Incorrect Env[0].ValueFrom")
	assert.Equal("cluster.name", sts.Spec.Template.Spec.Containers[0].Env[1].Name, "Incorrect Env[1].Name")
	assert.Equal(vmo.Name, sts.Spec.Template.Spec.Containers[0].Env[1].Value, "Incorrect Env[1].Value")
	assert.Equal("node.master", sts.Spec.Template.Spec.Containers[0].Env[4].Name, "Incorrect Env[4].Name")
	assert.Equal("true", sts.Spec.Template.Spec.Containers[0].Env[4].Value, "Incorrect Env[4].Value")
	assert.Equal("node.ingest", sts.Spec.Template.Spec.Containers[0].Env[5].Name, "Incorrect Env[5].Name")
	assert.Equal("false", sts.Spec.Template.Spec.Containers[0].Env[5].Value, "Incorrect Env[5].Value")
	assert.Equal("node.data", sts.Spec.Template.Spec.Containers[0].Env[6].Name, "Incorrect Env[6].Name")
	assert.Equal("false", sts.Spec.Template.Spec.Containers[0].Env[6].Value, "Incorrect Env[6].Value")
	assert.Equal("HTTP_ENABLE", sts.Spec.Template.Spec.Containers[0].Env[2].Name, "Incorrect Env[2].Name")
	assert.Equal("true", sts.Spec.Template.Spec.Containers[0].Env[2].Value, "Incorrect Env[2].Value")
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
	assert.Equal(sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage], resource.MustParse(storageSize),
		"Incorrect VolumeClaimTemplate resource request size")
}

// Verify the Statefulset used by Elasticsearch master
func verifyElasticSearchDevProfile(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance,
	sts *appsv1.StatefulSet, replicas int32, storageSize string) {

	assert := assert.New(t)
	const esMasterVolName = "elasticsearch-master"
	const esMasterData = "/usr/share/elasticsearch/data"

	assert.Equal(*resources.NewVal(int32(replicas)), *sts.Spec.Replicas, "Incorrect Elasticsearch Master replicas count")
	affin := resources.CreateZoneAntiAffinityElement(vmo.Name, config.ElasticsearchMaster.Name)
	assert.Equal(affin, sts.Spec.Template.Spec.Affinity, "Incorrect Elasticsearch affinity")
	var elasticsearchUID int64 = 1000
	assert.Equal(elasticsearchUID, *sts.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser,
		"Incorrect Elasticsearch.SecurityContext.RunAsUser")

	assert.Len(sts.Spec.Template.Spec.Containers, 2, "Incorrect number of Containers")
	assert.Len(sts.Spec.Template.Spec.Containers[0].Ports, 2, "Incorrect number of Ports")
	assert.Equal("transport", sts.Spec.Template.Spec.Containers[0].Ports[0].Name, "Incorrect Container Port")
	assert.Zero(sts.Spec.Template.Spec.Containers[0].Ports[0].HostPort, "Incorrect Container HostPort")
	assert.Equal(int32(constants.ESTransportPort), sts.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort, "Incorrect Container HostPort")
	assert.Equal("http", sts.Spec.Template.Spec.Containers[0].Ports[1].Name, "Incorrect Container Port")
	assert.Zero(sts.Spec.Template.Spec.Containers[0].Ports[1].HostPort, "Incorrect Container HostPort")
	assert.Equal(int32(constants.ESHttpPort), sts.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort, "Incorrect Container HostPort")

	assert.Len(sts.Spec.Template.Spec.Containers[0].Env, 8, "Incorrect number of Env Vars")
	assert.Equal("node.name", sts.Spec.Template.Spec.Containers[0].Env[0].Name, "Incorrect Env[0].Name")
	assert.Equal("metadata.name", sts.Spec.Template.Spec.Containers[0].Env[0].ValueFrom.FieldRef.FieldPath,
		"Incorrect Env[0].ValueFrom")
	assert.Equal("cluster.name", sts.Spec.Template.Spec.Containers[0].Env[1].Name, "Incorrect Env[1].Name")
	assert.Equal(vmo.Name, sts.Spec.Template.Spec.Containers[0].Env[1].Value, "Incorrect Env[1].Value")
	assert.Equal("HTTP_ENABLE", sts.Spec.Template.Spec.Containers[0].Env[2].Name, "Incorrect Env[2].Name")
	assert.Equal("true", sts.Spec.Template.Spec.Containers[0].Env[2].Value, "Incorrect Env[2].Value")
	assert.Equal("discovery.type", sts.Spec.Template.Spec.Containers[0].Env[3].Name, "Incorrect Env[3].Name")
	assert.Equal("single-node", sts.Spec.Template.Spec.Containers[0].Env[3].Value, "Incorrect Env[3].Value")
	assert.Equal("node.master", sts.Spec.Template.Spec.Containers[0].Env[4].Name, "Incorrect Env[4].Name")
	assert.Equal("true", sts.Spec.Template.Spec.Containers[0].Env[4].Value, "Incorrect Env[4].Value")
	assert.Equal("node.ingest", sts.Spec.Template.Spec.Containers[0].Env[5].Name, "Incorrect Env[5].Name")
	assert.Equal("true", sts.Spec.Template.Spec.Containers[0].Env[5].Value, "Incorrect Env[5].Value")
	assert.Equal("node.data", sts.Spec.Template.Spec.Containers[0].Env[6].Name, "Incorrect Env[6].Name")
	assert.Equal("true", sts.Spec.Template.Spec.Containers[0].Env[6].Value, "Incorrect Env[6].Value")
	assert.Equal("ES_JAVA_OPTS", sts.Spec.Template.Spec.Containers[0].Env[7].Name, "Incorrect Env[7].Name")
	assert.Equal("-Xms512m -Xmx512m", sts.Spec.Template.Spec.Containers[0].Env[7].Value, "Incorrect Env[7].Value")

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

	assert.Len(sts.Spec.VolumeClaimTemplates, 0, "Incorrect number of VolumeClaimTemplates")

	volumes := sts.Spec.Template.Spec.Volumes
	assert.Len(volumes, 2)
	assert.Equal(esMasterVolName, volumes[0].Name, "Incorrect name for master volume")
	volumeSource := volumes[0].VolumeSource
	assert.NotNil(volumeSource.EmptyDir, "volumeSource should be EmptyDir")
}

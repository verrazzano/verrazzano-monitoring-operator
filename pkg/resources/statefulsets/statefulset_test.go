// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const defaultStorageClass = "default"

var storageClass = storagev1.StorageClass{
	ObjectMeta: metav1.ObjectMeta{
		Name: defaultStorageClass,
	},
}

// TestVMOEmptyStatefulSetSize tests the creation of a VMI without StatefulSets
// GIVEN a VMI spec with empty AlertManager and ElasticSearch specs
//  WHEN I call New
//  THEN there should be no StatefulSets created
func TestVMOEmptyStatefulSetSize(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	statefulsets, err := New(vzlog.DefaultLogger(), vmo, &storageClass, "vmi-system-es-master-0")
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
	statefulsets, err := New(vzlog.DefaultLogger(), vmo, &storageClass, "vmi-system-es-master-0")
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
	var masterNodeReplicas int32 = 3
	var dataNodeReplicas int32 = 2
	var ingestNodeReplicas int32 = 1
	storageSize := "50Gi"

	if isDevProfileTest {
		masterNodeReplicas = 1
		dataNodeReplicas = 0
		ingestNodeReplicas = 0
		storageSize = ""
	}

	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "system",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
				MasterNode: vmcontrollerv1.ElasticsearchNode{
					Name:     "es-master",
					Replicas: masterNodeReplicas,
					Storage: &vmcontrollerv1.Storage{
						Size: storageSize,
					},
					Roles: []vmcontrollerv1.NodeRole{
						vmcontrollerv1.MasterRole,
						vmcontrollerv1.DataRole,
						vmcontrollerv1.IngestRole,
					},
				},
				DataNode: vmcontrollerv1.ElasticsearchNode{
					Name:     "es-data",
					Replicas: dataNodeReplicas,
					Storage: &vmcontrollerv1.Storage{
						Size: storageSize,
					},
					Roles: []vmcontrollerv1.NodeRole{
						vmcontrollerv1.DataRole,
					},
				},
				IngestNode: vmcontrollerv1.ElasticsearchNode{
					Name:     "es-ingest",
					Replicas: ingestNodeReplicas,
					Roles: []vmcontrollerv1.NodeRole{
						vmcontrollerv1.IngestRole,
					},
				},
			},
		},
	}

	initialMasterNodes := nodes.InitialMasterNodes(vmo.Name, nodes.StatefulSetNodes(vmo))
	// Create the stateful sets
	statefulsets, err := New(vzlog.DefaultLogger(), vmo, &storageClass, initialMasterNodes)
	if err != nil {
		t.Error(err)
	}

	if isDevProfileTest {
		assert.True(t, nodes.IsSingleNodeESCluster(vmo), "Single node ES setup, expected IsSingleNodeESCluster to be true")
		verifyDevProfileVMOComponents(t, statefulsets, vmo, masterNodeReplicas, storageSize)
	} else {
		assert.False(t, nodes.IsSingleNodeESCluster(vmo), "Single node ES setup, expected IsSingleNodeESCluster to be false")
		verifyProdProfileVMOComponents(t, statefulsets, vmo, masterNodeReplicas, storageSize)
	}
}

func verifyProdProfileVMOComponents(t *testing.T, statefulsets []*appsv1.StatefulSet, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, elasticSearchReplicas int32, storageSize string) {
	// Do assertions
	assert.Equal(t, 1, len(statefulsets), "Incorrect number of statefulsets")
	for _, statefulset := range statefulsets {
		switch statefulset.Name {
		case resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name):
			verifyElasticSearch(t, vmo, statefulset, elasticSearchReplicas, storageSize)
		default:
			t.Error("Unknown Deployment Name: " + statefulset.Name)
		}
	}
}

func verifyDevProfileVMOComponents(t *testing.T, statefulsets []*appsv1.StatefulSet, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance,
	elasticSearchReplicas int32, storageSize string) {
	// Do assertions
	assert.Equal(t, 1, len(statefulsets), "Incorrect number of statefulsets")
	for _, statefulset := range statefulsets {
		switch statefulset.Name {
		case resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name):
			verifyElasticSearchDevProfile(t, vmo, statefulset, elasticSearchReplicas, storageSize)
		default:
			t.Error("Unknown Deployment Name: " + statefulset.Name)
		}
	}
}

// Verify the Statefulset used by Elasticsearch master
func verifyElasticSearch(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance,
	sts *appsv1.StatefulSet, replicas int32, storageSize string) {

	assert := assert.New(t)
	const esMasterVolName = "elasticsearch-master"
	const esMasterData = "/usr/share/opensearch/data"

	assert.Equal(*resources.NewVal(replicas), *sts.Spec.Replicas, "Incorrect Elasticsearch MasterNodes replicas count")
	affin := resources.CreateZoneAntiAffinityElement(vmo.Name, config.ElasticsearchMaster.Name)
	assert.Equal(affin, sts.Spec.Template.Spec.Affinity, "Incorrect Elasticsearch affinity")
	var elasticsearchUID int64 = 1000
	assert.Equal(elasticsearchUID, *sts.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser,
		"Incorrect Elasticsearch.SecurityContext.RunAsUser")

	assert.Len(sts.Spec.Template.Spec.Containers, 1, "Incorrect number of Containers")
	assert.Len(sts.Spec.Template.Spec.Containers[0].Ports, 2, "Incorrect number of Ports")
	assert.Equal("transport", sts.Spec.Template.Spec.Containers[0].Ports[0].Name, "Incorrect Container Port")
	assert.Zero(sts.Spec.Template.Spec.Containers[0].Ports[0].HostPort, "Incorrect Container HostPort")
	assert.Equal(int32(constants.OSTransportPort), sts.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort, "Incorrect Container HostPort")
	assert.Equal("http", sts.Spec.Template.Spec.Containers[0].Ports[1].Name, "Incorrect Container Port")
	assert.Zero(sts.Spec.Template.Spec.Containers[0].Ports[1].HostPort, "Incorrect Container HostPort")
	assert.Equal(int32(constants.OSHTTPPort), sts.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort, "Incorrect Container HostPort")

	env := sts.Spec.Template.Spec.Containers[0].Env
	assert.Len(env, 9, "Incorrect number of Env Vars")
	assert.Equal("node.name", env[0].Name, "Incorrect Env[0].Name")
	assert.Equal("metadata.name", env[0].ValueFrom.FieldRef.FieldPath,
		"Incorrect Env[0].ValueFrom")
	assert.Equal("cluster.name", env[1].Name, "Incorrect Env[1].Name")
	assert.Equal(vmo.Name, env[1].Value, "Incorrect Env[1].Value")
	assert.Equal("HTTP_ENABLE", env[2].Name, "Incorrect Env[2].Name")
	assert.Equal("true", env[2].Value, "Incorrect Env[2].Value")
	assert.Equal("logger.org.opensearch", env[3].Name, "Incorrect Env[3].Name")
	assert.Equal("info", env[3].Value, "Incorrect Env[3].Value")
	assert.Equal(constants.ObjectStoreAccessKeyVarName, env[4].Name, "Incorrect Env[4].Name")
	assert.Equal(constants.ObjectStoreAccessKey, env[4].ValueFrom.SecretKeyRef.Key, "Incorrect Env[4] Secret Key name")
	assert.Equal(constants.VerrazzanoBackupScrtName, env[4].ValueFrom.SecretKeyRef.Name, "Incorrect Env[4] Secret name")
	assert.Equal(constants.ObjectStoreCustomerKeyVarName, env[5].Name, "Incorrect Env[5].Name")
	assert.Equal(constants.ObjectStoreCustomerKey, env[5].ValueFrom.SecretKeyRef.Key, "Incorrect Env[5] Secret Key name")
	assert.Equal(constants.VerrazzanoBackupScrtName, env[5].ValueFrom.SecretKeyRef.Name, "Incorrect Env[5] Secret name")
	assert.Equal("node.roles", env[6].Name, "Incorrect Env[6].Name")
	assert.Equal("master,data,ingest", env[6].Value, "Incorrect Env[6].Value")
	assert.Equal("discovery.seed_hosts", env[7].Name, "Incorrect Env[7].Name")
	assert.Equal(resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name), env[7].Value, "Incorrect Env[7].Value")
	assert.Equal("cluster.initial_master_nodes", env[8].Name, "Incorrect Env[8].Name")
	assert.Equal("vmi-system-es-master-0,vmi-system-es-master-1,vmi-system-es-master-2", env[8].Value, "Incorrect Env[8].Value")

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
	assert.Equal(int32(30), sts.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds,
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
	const esMasterData = "/usr/share/opensearch/data"

	assert.Equal(*resources.NewVal(int32(replicas)), *sts.Spec.Replicas, "Incorrect Elasticsearch MasterNodes replicas count")
	affin := resources.CreateZoneAntiAffinityElement(vmo.Name, config.ElasticsearchMaster.Name)
	assert.Equal(affin, sts.Spec.Template.Spec.Affinity, "Incorrect Elasticsearch affinity")
	var elasticsearchUID int64 = 1000
	assert.Equal(elasticsearchUID, *sts.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser,
		"Incorrect Elasticsearch.SecurityContext.RunAsUser")

	assert.Len(sts.Spec.Template.Spec.Containers, 1, "Incorrect number of Containers")
	assert.Len(sts.Spec.Template.Spec.Containers[0].Ports, 2, "Incorrect number of Ports")
	assert.Equal("transport", sts.Spec.Template.Spec.Containers[0].Ports[0].Name, "Incorrect Container Port")
	assert.Zero(sts.Spec.Template.Spec.Containers[0].Ports[0].HostPort, "Incorrect Container HostPort")
	assert.Equal(int32(constants.OSTransportPort), sts.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort, "Incorrect Container HostPort")
	assert.Equal("http", sts.Spec.Template.Spec.Containers[0].Ports[1].Name, "Incorrect Container Port")
	assert.Zero(sts.Spec.Template.Spec.Containers[0].Ports[1].HostPort, "Incorrect Container HostPort")
	assert.Equal(int32(constants.OSHTTPPort), sts.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort, "Incorrect Container HostPort")

	env := sts.Spec.Template.Spec.Containers[0].Env
	assert.Len(env, 9, "Incorrect number of Env Vars")
	assert.Equal("node.name", env[0].Name, "Incorrect Env[0].Name")
	assert.Equal("metadata.name", env[0].ValueFrom.FieldRef.FieldPath,
		"Incorrect Env[0].ValueFrom")
	assert.Equal("cluster.name", env[1].Name, "Incorrect Env[2].Name")
	assert.Equal(vmo.Name, env[1].Value, "Incorrect Env[2].Value")
	assert.Equal("HTTP_ENABLE", env[2].Name, "Incorrect Env[3].Name")
	assert.Equal("true", env[2].Value, "Incorrect Env[3].Value")
	assert.Equal("logger.org.opensearch", env[3].Name, "Incorrect Env[4].Name")
	assert.Equal("info", env[3].Value, "Incorrect Env[4].Value")
	assert.Equal(constants.ObjectStoreAccessKeyVarName, env[4].Name, "Incorrect Env[5].Name")
	assert.Equal(constants.ObjectStoreAccessKey, env[4].ValueFrom.SecretKeyRef.Key, "Incorrect Env[5] Secret Key name")
	assert.Equal(constants.VerrazzanoBackupScrtName, env[4].ValueFrom.SecretKeyRef.Name, "Incorrect Env[5] Secret name")
	assert.Equal(constants.ObjectStoreCustomerKeyVarName, env[5].Name, "Incorrect Env[6].Name")
	assert.Equal(constants.ObjectStoreCustomerKey, env[5].ValueFrom.SecretKeyRef.Key, "Incorrect Env[6] Secret Key name")
	assert.Equal(constants.VerrazzanoBackupScrtName, env[5].ValueFrom.SecretKeyRef.Name, "Incorrect Env[6] Secret name")
	assert.Equal("node.roles", env[6].Name, "Incorrect Env[6].Name")
	assert.Equal("master,data,ingest", env[6].Value, "Incorrect Env[6].Value")
	assert.Equal("discovery.type", env[7].Name, "Incorrect Env[7].Name")
	assert.Equal("single-node", env[7].Value, "Incorrect Env[7].Value")
	assert.Equal("ES_JAVA_OPTS", env[8].Name, "Incorrect Env[8].Name")
	assert.Equal("-Xms700m -Xmx700m", env[8].Value, "Incorrect Env[8].Value")

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
	assert.Equal(int32(30), sts.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds,
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
	assert.Len(volumes, 1)
	assert.Equal(esMasterVolName, volumes[0].Name, "Incorrect name for master volume")
	volumeSource := volumes[0].VolumeSource
	assert.NotNil(volumeSource.EmptyDir, "volumeSource should be EmptyDir")
}

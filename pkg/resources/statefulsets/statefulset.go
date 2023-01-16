// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"fmt"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/memory"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// New creates StatefulSet objects for a VMO resource
func New(log vzlog.VerrazzanoLogger, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, storageClass *storagev1.StorageClass, initialMasterNodes string) ([]*appsv1.StatefulSet, error) {
	var statefulSets []*appsv1.StatefulSet

	// OpenSearch MasterNodes
	if vmo.Spec.Elasticsearch.Enabled {
		statefulSets = append(statefulSets, createOpenSearchStatefulSets(log, vmo, storageClass, initialMasterNodes)...)
	}
	return statefulSets, nil
}

func createOpenSearchStatefulSets(log vzlog.VerrazzanoLogger, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, storageClass *storagev1.StorageClass, initialMasterNodes string) []*appsv1.StatefulSet {
	var statefulSets []*appsv1.StatefulSet
	for _, node := range nodes.MasterNodes(vmo) {
		if node.Replicas > 0 {
			statefulSet := createOpenSearchStatefulSet(log, vmo, storageClass, node, initialMasterNodes)
			statefulSets = append(statefulSets, statefulSet)
		}
	}

	return statefulSets
}

// Creates StatefulSet for OpenSearch
func createOpenSearchStatefulSet(log vzlog.VerrazzanoLogger, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, storageClass *storagev1.StorageClass, node vmcontrollerv1.ElasticsearchNode, initialMasterNodes string) *appsv1.StatefulSet {
	// Headless service for OpenSearch
	headlessService := resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)
	statefulSetName := resources.GetMetaName(vmo.Name, node.Name)
	// Create base statefulset object
	statefulSet := createStatefulSetElement(vmo, &node.Resources, config.ElasticsearchMaster, headlessService, statefulSetName)
	// Add node labels
	statefulSet.Spec.Selector.MatchLabels[constants.NodeGroupLabel] = node.Name
	statefulSet.Spec.Template.Labels[constants.NodeGroupLabel] = node.Name

	statefulSet.Spec.Replicas = resources.NewVal(node.Replicas)
	statefulSet.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.ElasticsearchMaster.Name)

	podSecurityContext := &corev1.PodSecurityContext{
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
	statefulSet.Spec.Template.Spec.SecurityContext = podSecurityContext

	var elasticsearchUID int64 = 1000
	esMasterContainer := &statefulSet.Spec.Template.Spec.Containers[0]
	esMasterContainer.SecurityContext.RunAsUser = &elasticsearchUID
	esMasterContainer.SecurityContext.AllowPrivilegeEscalation = resources.NewBool(false)
	esMasterContainer.SecurityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
	esMasterContainer.Ports[0].Name = "transport"
	esMasterContainer.Ports = append(esMasterContainer.Ports, corev1.ContainerPort{Name: "http", ContainerPort: int32(constants.OSHTTPPort), Protocol: "TCP"})

	javaOpts, err := memory.PodMemToJvmHeapArgs(node.Resources.RequestMemory, constants.DefaultDevProfileESMemArgs) // Default JVM heap settings if none provided
	if err != nil {
		javaOpts = constants.DefaultDevProfileESMemArgs
		log.Errorf("Failed to derive heap sizes from MasterNodes pod, using default %s: %v", javaOpts, err)
	}

	if node.JavaOpts != "" {
		javaOpts = node.JavaOpts
	}
	// Adding command for add keystore values at pod bootup
	esMasterContainer.Command = []string{
		"sh",
		"-c",
		resources.CreateOpenSearchContainerCMD(javaOpts, resources.GetOpenSearchPluginList(vmo)),
	}
	var envVars = []corev1.EnvVar{
		{
			Name: "node.name",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{Name: "cluster.name", Value: vmo.Name},
		// HTTP is enabled on the master here solely for our readiness check below (on _cluster/health)
		{Name: "HTTP_ENABLE", Value: "true"},
		{Name: "logger.org.opensearch", Value: "info"},
		{Name: constants.ObjectStoreAccessKeyVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: constants.VerrazzanoBackupScrtName,
					},
					Key: constants.ObjectStoreAccessKey,
					Optional: func(opt bool) *bool {
						return &opt
					}(true),
				},
			},
		},
		{Name: constants.ObjectStoreCustomerKeyVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: constants.VerrazzanoBackupScrtName,
					},
					Key: constants.ObjectStoreCustomerKey,
					Optional: func(opt bool) *bool {
						return &opt
					}(true),
				},
			},
		},
	}
	var readinessProbeCondition string
	envVars = append(envVars,
		corev1.EnvVar{Name: "OPENSEARCH_JAVA_OPTS", Value: javaOpts},
	)
	if nodes.IsSingleNodeCluster(vmo) {
		node.Roles = []vmcontrollerv1.NodeRole{
			vmcontrollerv1.MasterRole,
			vmcontrollerv1.DataRole,
			vmcontrollerv1.IngestRole,
		}
		log.Oncef("ES topology for %s indicates a single-node cluster (single master node only)", vmo.Name)
		envVars = append(envVars,
			corev1.EnvVar{Name: "node.roles", Value: nodes.GetRolesString(&node)},
			corev1.EnvVar{Name: "discovery.type", Value: "single-node"},
		)
	} else {
		envVars = append(envVars,
			corev1.EnvVar{Name: "node.roles", Value: nodes.GetRolesString(&node)},
			corev1.EnvVar{
				Name:  "discovery.seed_hosts",
				Value: headlessService,
			},
		)
		if initialMasterNodes != "" {
			envVars = append(envVars, corev1.EnvVar{Name: constants.ClusterInitialMasterNodes, Value: initialMasterNodes})
		}
	}
	esMasterContainer.Env = envVars

	basicAuthParams := ""
	readinessProbeCondition = `kpo

        echo 'Cluster is not yet ready'
        exit 1
`
	// Customized Readiness and Liveness probes
	esMasterContainer.ReadinessProbe =
		&corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"sh",
						"-c",
						`#!/usr/bin/env bash -e
# If the node is starting up wait for the cluster to be ready' )
# Once it has started only check that the node itself is responding
START_FILE=/tmp/.es_start_file
http () {
    local path="${1}"
    curl -v -XGET -s -k ` + basicAuthParams + ` --fail http://127.0.0.1:9200${path}
}
if [ -f "${START_FILE}" ]; then
    echo 'Elasticsearch is already running, lets check the node is healthy'
    http "` + config.ElasticsearchMaster.ReadinessHTTPPath + `"
else
    echo 'Waiting for elasticsearch cluster to become cluster to be ready'
    if http "` + config.ElasticsearchMaster.ReadinessHTTPPath + `" ; then
        touch ${START_FILE}
    else` + readinessProbeCondition + `
    fi
    exit 0
fi`,
					},
				},
			},
			InitialDelaySeconds: 90,
			SuccessThreshold:    3,
			PeriodSeconds:       5,
			TimeoutSeconds:      5,
		}

	esMasterContainer.LivenessProbe =
		&corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.IntOrString{
						IntVal: int32(config.ElasticsearchMaster.Port),
					},
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			FailureThreshold:    5,
		}

	const esMasterVolName = "elasticsearch-master"
	const esMasterData = "/usr/share/opensearch/data"

	// Add the pv volume mount to the main container
	esMasterContainer.VolumeMounts =
		append(esMasterContainer.VolumeMounts, corev1.VolumeMount{
			Name:      esMasterVolName,
			MountPath: esMasterData,
		})

	// Add init container
	statefulSet.Spec.Template.Spec.InitContainers = append(statefulSet.Spec.Template.Spec.InitContainers,
		*resources.GetElasticsearchMasterInitContainer())

	// Add the pv volume mount to the init container
	statefulSet.Spec.Template.Spec.InitContainers[0].VolumeMounts =
		[]corev1.VolumeMount{{
			Name:      esMasterVolName,
			MountPath: esMasterData,
		}}

	// Add the pvc templates, this will result in a PV + PVC being created automatically for each
	// pod in the stateful set.
	if node.Storage != nil && len(node.Storage.Size) > 0 {
		statefulSet.Spec.VolumeClaimTemplates =
			[]corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      esMasterVolName,
					Namespace: vmo.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(node.Storage.Size)},
					},
				},
			}}
		// Only set the storage class name if one was explicitly specified by the user.
		// This is to facilitate upgrades where storage class name is empty,
		// since you cannot update this field of a statefulset
		if storageClass != nil {
			statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName = &storageClass.Name
		}
	} else {
		statefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: esMasterVolName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
	}

	// add istio annotations required for inter component communication
	if statefulSet.Spec.Template.Annotations == nil {
		statefulSet.Spec.Template.Annotations = make(map[string]string)
	}
	statefulSet.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeInboundPorts"] = fmt.Sprintf("%d", constants.OSTransportPort)
	statefulSet.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeOutboundPorts"] = fmt.Sprintf("%d", constants.OSTransportPort)

	// set Node Role labels for role based selectors
	nodes.SetNodeRoleLabels(&node, statefulSet.Spec.Template.Labels)
	return statefulSet
}

// Creates a statefulset element for the given VMO and component
func createStatefulSetElement(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, vmoResources *vmcontrollerv1.Resources,
	componentDetails config.ComponentDetails, serviceName, statefulSetName string) *appsv1.StatefulSet {
	labels := resources.GetSpecID(vmo.Name, componentDetails.Name)
	resourceLabel := resources.GetMetaLabels(vmo)
	resourceLabel[constants.ComponentLabel] = resources.GetCompLabel(componentDetails.Name)
	podLabels := resources.DeepCopyMap(labels)
	podLabels[constants.ComponentLabel] = resources.GetCompLabel(componentDetails.Name)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resourceLabel,
			Name:            statefulSetName,
			Namespace:       vmo.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmo),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: resources.NewVal(1),
			// The default PodManagementPolicy (OrderedReady) has known issues where a statefulset with
			// a crashing pod is never updated on further statefulset changes, so use Parallel here
			PodManagementPolicy: appsv1.ParallelPodManagement,
			ServiceName:         serviceName,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						resources.CreateContainerElement(nil, vmoResources, componentDetails),
					},
					ServiceAccountName:            constants.ServiceAccountName,
					TerminationGracePeriodSeconds: resources.New64Val(1),
				},
			},
		},
	}
}

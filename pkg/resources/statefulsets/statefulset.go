// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"context"
	"fmt"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/memory"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"strconv"
	"strings"

	"go.uber.org/zap"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// New creates StatefulSet objects for a VMO resource
func New(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, kubeclientset kubernetes.Interface, username, password string) ([]*appsv1.StatefulSet, error) {
	var statefulSets []*appsv1.StatefulSet

	// Alert Manager
	if vmo.Spec.AlertManager.Enabled {
		statefulSets = append(statefulSets, createAlertManagerStatefulSet(vmo))
	}
	// Elasticsearch Master
	if vmo.Spec.Elasticsearch.Enabled {
		statefulSets = append(statefulSets, createElasticsearchMasterStatefulSet(vmo))
		statefulSets = append(statefulSets, createElasticSearchDataStatefulSet(vmo))
	}

	// Grafana
	if vmo.Spec.Grafana.Enabled {
		statefulSets = append(statefulSets, createGrafanaStatefulSet(vmo, username, password))
	}
	// Prometheus
	if vmo.Spec.Prometheus.Enabled {
		statefulSets = append(statefulSets, createPrometheusStatefulSet(vmo, kubeclientset))
	}

	return statefulSets, nil
}

// Creates StatefulSet for Elasticsearch Master
func createElasticsearchMasterStatefulSet(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *appsv1.StatefulSet {
	var readinessProbeCondition string

	statefulSet := createStatefulSetElement(vmo, &vmo.Spec.Elasticsearch.MasterNode.Resources, config.ElasticsearchMaster, "")

	statefulSet.Spec.Replicas = resources.NewVal(vmo.Spec.Elasticsearch.MasterNode.Replicas)
	statefulSet.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.ElasticsearchMaster.Name)

	var elasticsearchUID int64 = 1000
	esMasterContainer := &statefulSet.Spec.Template.Spec.Containers[0]
	esMasterContainer.SecurityContext.RunAsUser = &elasticsearchUID
	esMasterContainer.Ports[0].Name = "transport"
	esMasterContainer.Ports = append(esMasterContainer.Ports, corev1.ContainerPort{Name: "http", ContainerPort: int32(constants.ESHttpPort), Protocol: "TCP"})

	// Set the default logging to INFO; this can be overridden later at runtime
	esMasterContainer.Args = []string{
		"elasticsearch",
		"-E",
		"logger.org.elasticsearch=INFO",
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
	}
	if resources.IsSingleNodeESCluster(vmo) {
		zap.S().Infof("ES topology for %s indicates a single-node cluster (single master node only)")
		javaOpts, err := memory.PodMemToJvmHeapArgs(vmo.Spec.Elasticsearch.MasterNode.Resources.RequestMemory) // Default JVM heap settings if none provided
		if err != nil {
			javaOpts = constants.DefaultDevProfileESMemArgs
			zap.S().Errorf("Unable to derive heap sizes from Master pod, using default %s.  Error: %v", javaOpts, err)
		}
		if vmo.Spec.Elasticsearch.MasterNode.JavaOpts != "" {
			javaOpts = vmo.Spec.Elasticsearch.IngestNode.JavaOpts
		}
		envVars = append(envVars,
			corev1.EnvVar{Name: "discovery.type", Value: "single-node"},
			corev1.EnvVar{Name: "node.master", Value: "true"},
			corev1.EnvVar{Name: "node.ingest", Value: "true"},
			corev1.EnvVar{Name: "node.data", Value: "true"},
			corev1.EnvVar{Name: "ES_JAVA_OPTS", Value: javaOpts},
		)
	} else {
		var i int32
		initialMasterNodes := make([]string, 0)
		for i = 0; i < *statefulSet.Spec.Replicas; i++ {
			initialMasterNodes = append(initialMasterNodes, resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)+"-"+fmt.Sprintf("%d", i))
		}

		envVars = append(envVars,
			corev1.EnvVar{
				Name:  "discovery.seed_hosts",
				Value: resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name),
			},
			corev1.EnvVar{Name: "node.master", Value: "true"},
			corev1.EnvVar{Name: "node.ingest", Value: "false"},
			corev1.EnvVar{Name: "node.data", Value: "false"},
			corev1.EnvVar{Name: "cluster.initial_master_nodes", Value: strings.Join(initialMasterNodes, ",")},
		)
	}
	esMasterContainer.Env = envVars

	basicAuthParams := ""
	readinessProbeCondition = `
        echo 'Cluster is not yet ready'
        exit 1
`
	// Customized Readiness and Liveness probes
	esMasterContainer.ReadinessProbe =
		&corev1.Probe{
			Handler: corev1.Handler{
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
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.IntOrString{
						IntVal: int32(config.ElasticsearchMaster.Port),
					},
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			FailureThreshold:    5,
		}

	const esMasterVolName = "elasticsearch-master"
	const esMasterData = "/usr/share/elasticsearch/data"

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
	storageSize := vmo.Spec.Elasticsearch.Storage.Size
	if len(storageSize) > 0 {
		statefulSet.Spec.VolumeClaimTemplates =
			[]corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      esMasterVolName,
					Namespace: vmo.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(storageSize)},
					},
				},
			}}
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
	statefulSet.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeInboundPorts"] = fmt.Sprintf("%d", constants.ESTransportPort)
	statefulSet.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeOutboundPorts"] = fmt.Sprintf("%d", constants.ESTransportPort)

	return statefulSet
}

func createElasticSearchDataStatefulSet(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *appsv1.StatefulSet {
	zap.S().Infof("+++ Invoked createElasticSearchDataStatefulSet +++")
	const esDataVolume = "vmi-system-es-data"

	statefulSet := createStatefulSetElement(vmo, &vmo.Spec.Grafana.Resources, config.Grafana, "")
	esContainer := &statefulSet.Spec.Template.Spec.Containers[0]
	esContainer.Env = append(esContainer.Env,
		corev1.EnvVar{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		corev1.EnvVar{
			Name: "node.name",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		corev1.EnvVar{Name: "cluster.name", Value: vmo.Name})
	// Set the default logging to INFO; this can be overridden later at runtime
	esContainer.Args = []string{
		"elasticsearch",
		"-E",
		"logger.org.elasticsearch=INFO",
	}

	esContainer.Ports = []corev1.ContainerPort{
		{Name: "http", ContainerPort: int32(constants.ESHttpPort)},
		{Name: "transport", ContainerPort: int32(constants.ESTransportPort)},
	}

	// Common Elasticsearch readiness and liveness settings
	if esContainer.LivenessProbe != nil {
		esContainer.LivenessProbe.InitialDelaySeconds = 60
		esContainer.LivenessProbe.TimeoutSeconds = 3
		esContainer.LivenessProbe.PeriodSeconds = 20
		esContainer.LivenessProbe.FailureThreshold = 5
	}
	if esContainer.ReadinessProbe != nil {
		esContainer.ReadinessProbe.InitialDelaySeconds = 60
		esContainer.ReadinessProbe.TimeoutSeconds = 3
		esContainer.ReadinessProbe.PeriodSeconds = 10
		esContainer.ReadinessProbe.FailureThreshold = 10
	}
	// Add init containers
	statefulSet.Spec.Template.Spec.InitContainers = append(statefulSet.Spec.Template.Spec.InitContainers, *resources.GetElasticsearchInitContainer())

	// Istio does not work with ElasticSearch.  Uncomment the following line when istio is present
	// deploymentElement.Spec.Template.Annotations = map[string]string{"sidecar.istio.io/inject": "false"}

	var elasticsearchUID int64 = 1000
	esContainer.SecurityContext.RunAsUser = &elasticsearchUID

	// Default JVM heap settings if none provided
	javaOpts, err := memory.PodMemToJvmHeapArgs(vmo.Spec.Elasticsearch.DataNode.Resources.RequestMemory)
	if err != nil {
		javaOpts = constants.DefaultESDataMemArgs
		zap.S().Errorf("Unable to derive heap sizes from Data pod, using default %s.  Error: %v", javaOpts, err)
	}
	if vmo.Spec.Elasticsearch.DataNode.JavaOpts != "" {
		javaOpts = vmo.Spec.Elasticsearch.DataNode.JavaOpts
	}

	initialMasterNodes := make([]string, 0)
	masterReplicas := resources.NewVal(vmo.Spec.Elasticsearch.MasterNode.Replicas)
	var i int32
	for i = 0; i < *masterReplicas; i++ {
		initialMasterNodes = append(initialMasterNodes, resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)+"-"+fmt.Sprintf("%d", i))
	}
	statefulSet.Spec.Replicas = resources.NewVal(vmo.Spec.Elasticsearch.DataNode.Replicas)

	//availabilityDomain := getAvailabilityDomainForPvcIndex(&vmo.Spec.Elasticsearch.Storage, pvcToAdMap, i)
	//if availabilityDomain == "" {
	//	// With shard allocation awareness, we must provide something for the AD, even in the case of the simple
	//	// VMO with no persistence volumes
	//	availabilityDomain = "None"
	//}

	// Anti-affinity on other data pod *nodes* (try out best to spread across many nodes)
	statefulSet.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: resources.GetSpecID(vmo.Name, config.ElasticsearchData.Name),
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
	}

	// When the deployment does not have a pod security context with an FSGroup attribute, any mounted volumes are
	// initially owned by root/root.  Previous versions of the ES image were run as "root", and chown'd the mounted
	// directory to "elasticsearch", but we don't want to run as "root".  The current ES image creates a group
	// "elasticsearch" (GID 1000), and a user "elasticsearch" (UID 1000) in that group.  When we provide FSGroup =
	// 1000 below, the volume is owned by root/elasticsearch, with permissions "rwxrwsr-x".  This allows the ES
	// image to run as UID 1000, and have sufficient permissions to write to the mounted volume.
	elasticsearchGid := int64(1000)
	statefulSet.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		FSGroup: &elasticsearchGid,
	}

	statefulSet.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	//statefulSet.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
	//statefulSet.Spec.UpdateStrategy.RollingUpdate = nil
	statefulSet.Spec.Template.Spec.Containers[0].Env = append(statefulSet.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "discovery.seed_hosts", Value: resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)},
		corev1.EnvVar{Name: "cluster.initial_master_nodes", Value: strings.Join(initialMasterNodes, ",")},
		//corev1.EnvVar{Name: "node.attr.availability_domain", Value: availabilityDomain},
		corev1.EnvVar{Name: "node.master", Value: "false"},
		corev1.EnvVar{Name: "node.ingest", Value: "false"},
		corev1.EnvVar{Name: "node.data", Value: "true"},
		corev1.EnvVar{Name: "ES_JAVA_OPTS", Value: javaOpts},
	)

	// add the required istio annotations to allow inter-es component communication
	if statefulSet.Spec.Template.Annotations == nil {
		statefulSet.Spec.Template.Annotations = make(map[string]string)
	}
	statefulSet.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeInboundPorts"] = fmt.Sprintf("%d", constants.ESTransportPort)
	statefulSet.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeOutboundPorts"] = fmt.Sprintf("%d", constants.ESTransportPort)

	// es data vaolume mounts
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      esDataVolume,
			MountPath: "/usr/share/elasticsearch/data",
		},
	}

	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMounts...)
	//statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes, volumes...)

	// Add the pvc templates, this will result in a PV + PVC being created automatically for each
	// pod in the stateful set.
	storageSize := vmo.Spec.Elasticsearch.Storage.Size
	if len(storageSize) > 0 {
		statefulSet.Spec.VolumeClaimTemplates =
			[]corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      esDataVolume,
					Namespace: vmo.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(storageSize)},
					},
				},
			}}
	} else {
		statefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: esDataVolume,
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
	return statefulSet
}

func createPrometheusStatefulSet(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, kubeclientset kubernetes.Interface) *appsv1.StatefulSet {
	const prometheusVolume = "vmi-system-prometheus"
	statefulSet := createStatefulSetElement(vmo, &vmo.Spec.Prometheus.Resources, config.Prometheus, "")
	statefulSet.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	//statefulSet.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType

	statefulSet.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.Prometheus.ImagePullPolicy
	statefulSet.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = &config.Prometheus.RunAsUser

	statefulSet.Spec.Template.Spec.Containers[0].Command = []string{"/bin/prometheus"}
	statefulSet.Spec.Template.Spec.Containers[0].Args = []string{
		"--config.file=" + constants.PrometheusConfigContainerLocation,
		"--storage.tsdb.path=" + config.Prometheus.DataDir,
		fmt.Sprintf("--storage.tsdb.retention.time=%dd", vmo.Spec.Prometheus.RetentionPeriod),
		"--web.enable-lifecycle",
		"--web.enable-admin-api",
		"--storage.tsdb.no-lockfile"}

	env := statefulSet.Spec.Template.Spec.Containers[0].Env
	http2 := "disabled"
	if vmo.Spec.Prometheus.HTTP2Enabled {
		http2 = ""
	}
	env = append(env, corev1.EnvVar{Name: "PROMETHEUS_COMMON_DISABLE_HTTP2", Value: http2})
	statefulSet.Spec.Template.Spec.Containers[0].Env = env

	if statefulSet.Spec.Template.Annotations == nil {
		statefulSet.Spec.Template.Annotations = make(map[string]string)
	}
	// These annotations are required uniquely for prometheus to support both the request routing to keycloak via the envoy and the writing
	// of the Istio certs to a volume that can be accessed for scraping
	statefulSet.Spec.Template.Annotations["proxy.istio.io/config"] = `{"proxyMetadata":{ "OUTPUT_CERTS": "/etc/istio-output-certs"}}`
	statefulSet.Spec.Template.Annotations["sidecar.istio.io/userVolumeMount"] = `[{"name": "istio-certs-dir", "mountPath": "/etc/istio-output-certs"}]`

	// If Keycloak isn't deployed configure Prometheus to avoid the Istio sidecar for metrics scraping.
	// This is done by adding the traffic.sidecar.istio.io/excludeOutboundIPRanges: 0.0.0.0/0 annotation.
	_, err := kubeclientset.AppsV1().StatefulSets("keycloak").Get(context.TODO(), "keycloak", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			statefulSet.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeOutboundIPRanges"] = "0.0.0.0/0"
			return nil
		}
		zap.S().Errorf("unable to get keycloak pod: %v", err)
		return nil
	}

	// Set the Istio annotation on Prometheus to exclude Keycloak HTTP Service IP address.
	// The includeOutboundIPRanges implies all others are excluded.
	// This is done by adding the traffic.sidecar.istio.io/includeOutboundIPRanges=<Keycloak IP>/32 annotation.
	svc, err := kubeclientset.CoreV1().Services("keycloak").Get(context.TODO(), "keycloak-http", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		zap.S().Errorf("unable to get keycloak-http service: %v", err)
		return nil
	}

	statefulSet.Spec.Template.Annotations["traffic.sidecar.istio.io/includeOutboundIPRanges"] = fmt.Sprintf("%s/32", svc.Spec.ClusterIP)

	// Volumes for Prometheus config and alert rules.  The istio-certs-dir volume supports the output of the istio
	// certs for use by prometheus scrape configurations
	configVolumes := []corev1.Volume{
		//{
		//	Name: prometheusVolume,
		//	VolumeSource: corev1.VolumeSource{
		//		ConfigMap: &corev1.ConfigMapVolumeSource{
		//			LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.Prometheus.RulesConfigMap},
		//		},
		//	},
		//},
		{
			Name: "rules-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.Prometheus.RulesConfigMap},
				},
			},
		},
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.Prometheus.ConfigMap},
				},
			},
		},
		{
			Name: "istio-certs-dir",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		},
	}
	statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes, configVolumes...)

	configVolumeMounts := []corev1.VolumeMount{
		{
			Name:      prometheusVolume,
			MountPath: "/prometheus",
		},
		{
			Name:      "rules-volume",
			MountPath: constants.PrometheusRulesMountPath,
		},
		{
			Name:      "config-volume",
			MountPath: constants.PrometheusConfigMountPath,
		},
	}
	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts, configVolumeMounts...)
	istioVolumeMount := corev1.VolumeMount{
		Name:      "istio-certs-dir",
		MountPath: constants.IstioCertsMountPath,
	}
	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts, istioVolumeMount)

	// Readiness/liveness settings
	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = 30
	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds = 3
	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds = 10
	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold = 10
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds = 5
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds = 3
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds = 10
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold = 5

	statefulSet.Spec.Replicas = resources.NewVal(vmo.Spec.Prometheus.Replicas)
	statefulSet.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.Prometheus.Name)

	// Config-reloader container
	statefulSet.Spec.Template.Spec.Containers = append(statefulSet.Spec.Template.Spec.Containers, corev1.Container{
		Name:            config.ConfigReloader.Name,
		Image:           config.ConfigReloader.Image,
		ImagePullPolicy: config.ConfigReloader.ImagePullPolicy,
	})
	statefulSet.Spec.Template.Spec.Containers[1].Args = []string{"-volume-dir=" + constants.PrometheusConfigMountPath, "-volume-dir=" + constants.PrometheusRulesMountPath, "-webhook-url=http://localhost:9090/-/reload"}
	statefulSet.Spec.Template.Spec.Containers[1].VolumeMounts = configVolumeMounts

	// Prometheus init container
	statefulSet.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:            config.PrometheusInit.Name,
			Image:           config.PrometheusInit.Image,
			ImagePullPolicy: config.PrometheusInit.ImagePullPolicy,
			Command:         []string{"sh", "-c", fmt.Sprintf("chown -R %d:%d /prometheus", constants.NobodyUID, constants.NobodyUID)},
			VolumeMounts:    []corev1.VolumeMount{{Name: prometheusVolume, MountPath: config.PrometheusInit.DataDir}},
		},
	}
	if vmo.Spec.Prometheus.Storage.Size == "" {
		statefulSet.Spec.Template.Spec.Volumes = append(
			statefulSet.Spec.Template.Spec.Volumes,
			corev1.Volume{Name: prometheusVolume, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
		statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{Name: prometheusVolume, MountPath: config.Prometheus.DataDir})
	}

	// Add the pvc templates, this will result in a PV + PVC being created automatically for each
	// pod in the stateful set.
	storageSize := vmo.Spec.Prometheus.Storage.Size
	if len(storageSize) > 0 {
		statefulSet.Spec.VolumeClaimTemplates =
			[]corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prometheusVolume,
					Namespace: vmo.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(storageSize)},
					},
				},
			}}
	} else {
		statefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: prometheusVolume,
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
	return statefulSet
}

func createGrafanaStatefulSet(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, username, password string) *appsv1.StatefulSet {
	const grafanaVolume = "vmi-system-grafana"

	statefulSet := createStatefulSetElement(vmo, &vmo.Spec.Grafana.Resources, config.Grafana, "")
	statefulSet.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	//statefulSet.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
	statefulSet.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.Grafana.ImagePullPolicy

	statefulSet.Spec.Replicas = resources.NewVal(vmo.Spec.Grafana.Replicas)
	statefulSet.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.Grafana.Name)

	statefulSet.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{Name: "GF_PATHS_PROVISIONING", Value: "/etc/grafana/provisioning"},
		{Name: "GF_SERVER_ENABLE_GZIP", Value: "true"},
		{Name: "PROMETHEUS_TARGETS", Value: "http://" + constants.VMOServiceNamePrefix + vmo.Name + "-" + config.Prometheus.Name + ":" + strconv.Itoa(config.Prometheus.Port)},
	}
	if config.Grafana.OidcProxy == nil {
		statefulSet.Spec.Template.Spec.Containers[0].Env = append(statefulSet.Spec.Template.Spec.Containers[0].Env, []corev1.EnvVar{
			{Name: "GF_SECURITY_ADMIN_USER", Value: username},
			{Name: "GF_SECURITY_ADMIN_PASSWORD", Value: password},
			{Name: "GF_AUTH_ANONYMOUS_ENABLED", Value: "false"},
			{Name: "GF_AUTH_BASIC_ENABLED", Value: "true"},
			{Name: "GF_USERS_ALLOW_SIGN_UP", Value: "true"},
			{Name: "GF_USERS_AUTO_ASSIGN_ORG", Value: "true"},
			{Name: "GF_USERS_AUTO_ASSIGN_ORG_ROLE", Value: "Admin"},
			{Name: "GF_AUTH_DISABLE_LOGIN_FORM", Value: "false"},
			{Name: "GF_AUTH_DISABLE_SIGNOUT_MENU", Value: "false"},
		}...)
	} else {
		statefulSet.Spec.Template.Spec.Containers[0].Env = append(statefulSet.Spec.Template.Spec.Containers[0].Env, []corev1.EnvVar{
			{Name: "GF_AUTH_ANONYMOUS_ENABLED", Value: "false"},
			{Name: "GF_AUTH_BASIC_ENABLED", Value: "false"},
			{Name: "GF_USERS_ALLOW_SIGN_UP", Value: "false"},
			{Name: "GF_USERS_AUTO_ASSIGN_ORG", Value: "true"},
			{Name: "GF_USERS_AUTO_ASSIGN_ORG_ROLE", Value: "Editor"},
			{Name: "GF_AUTH_DISABLE_LOGIN_FORM", Value: "true"},
			{Name: "GF_AUTH_DISABLE_SIGNOUT_MENU", Value: "true"},
			{Name: "GF_AUTH_PROXY_ENABLED", Value: "true"},
			{Name: "GF_AUTH_PROXY_HEADER_NAME", Value: "X-WEBAUTH-USER"},
			{Name: "GF_AUTH_PROXY_HEADER_PROPERTY", Value: "username"},
			{Name: "GF_AUTH_PROXY_AUTO_SIGN_UP", Value: "true"},
		}...)
	}
	if vmo.Spec.URI != "" {
		externalDomainName := config.Grafana.Name + "." + vmo.Spec.URI
		statefulSet.Spec.Template.Spec.Containers[0].Env = append(statefulSet.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "GF_SERVER_DOMAIN", Value: externalDomainName})
		statefulSet.Spec.Template.Spec.Containers[0].Env = append(statefulSet.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "GF_SERVER_ROOT_URL", Value: "https://" + externalDomainName})
	}
	// container will be restarted (per restart policy) if it fails the following liveness check:
	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = 15
	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds = 3
	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds = 20

	// container will be removed from services if fails the following readiness check.
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds = 5
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds = 3
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds = 20

	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe = statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe

	// dashboard volume
	volumes := []corev1.Volume{
		//{
		//	Name: grafanaVolume,
		//	VolumeSource: corev1.VolumeSource{
		//		ConfigMap: &corev1.ConfigMapVolumeSource{
		//			LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.Grafana.DashboardsConfigMap},
		//		},
		//	},
		//},
		{
			Name: "dashboards-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.Grafana.DashboardsConfigMap},
				},
			},
		},
		{
			Name: "datasources-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.Grafana.DatasourcesConfigMap},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      grafanaVolume,
			MountPath: "/var/lib/grafana",
		},
		{
			Name:      "dashboards-volume",
			MountPath: "/etc/grafana/provisioning/dashboards",
		},

		{
			Name:      "datasources-volume",
			MountPath: "/etc/grafana/provisioning/datasources",
		},
	}

	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMounts...)
	statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes, volumes...)

	// When the sts does not have a pod security context with an FSGroup attribute, any mounted volumes are
	// initially owned by root/root.  Previous versions of the Grafana image were run as "root", and chown'd the mounted
	// directory to "grafana", but we don't want to run as "root".  The current Grafana image creates a group
	// "grafana" (GID 472), and a user "grafana" (UID 472) in that group.  When we provide FSGroup =
	// 472 below, the volume is owned by root/grafana, with permissions "rwxrwsr-x".  This allows the Grafana
	// image to run as UID 472, and have sufficient permissions to write to the mounted volume.
	grafanaGid := int64(472)
	statefulSet.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		FSGroup: &grafanaGid,
	}

	// Add the pvc templates, this will result in a PV + PVC being created automatically for each
	// pod in the stateful set.
	storageSize := vmo.Spec.Grafana.Storage.Size
	if len(storageSize) > 0 {
		statefulSet.Spec.VolumeClaimTemplates =
			[]corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      grafanaVolume,
					Namespace: vmo.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(storageSize)},
					},
				},
			}}
	} else {
		statefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: grafanaVolume,
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
	return statefulSet
}

// Creates StatefulSet for AlertManager
func createAlertManagerStatefulSet(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *appsv1.StatefulSet {
	alertManagerClusterService := resources.GetMetaName(vmo.Name, config.AlertManagerCluster.Name)
	statefulSet := createStatefulSetElement(vmo, &vmo.Spec.AlertManager.Resources, config.AlertManager, alertManagerClusterService)
	statefulSet.Spec.Replicas = resources.NewVal(vmo.Spec.AlertManager.Replicas)
	statefulSet.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.AlertManager.Name)
	statefulSet.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.AlertManager.ImagePullPolicy

	// Construct command line args, with a cluster peer entry for each replica
	statefulSet.Spec.Template.Spec.Containers[0].Command = []string{"/bin/alertmanager"}
	statefulSet.Spec.Template.Spec.Containers[0].Args = []string{
		fmt.Sprintf("--config.file=%s", constants.AlertManagerConfigContainerLocation),
		fmt.Sprintf("--cluster.listen-address=0.0.0.0:%d", config.AlertManagerCluster.Port),
		fmt.Sprintf("--cluster.advertise-address=$(POD_IP):%d", config.AlertManagerCluster.Port),
		"--cluster.pushpull-interval=10s",
	}

	if vmo.Spec.URI != "" {
		alertManagerExternalURL := "https://" + config.AlertManager.Name + "." + vmo.Spec.URI
		statefulSet.Spec.Template.Spec.Containers[0].Args = append(statefulSet.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--web.external-url=%s", alertManagerExternalURL))
		statefulSet.Spec.Template.Spec.Containers[0].Args = append(statefulSet.Spec.Template.Spec.Containers[0].Args, "--web.route-prefix=/")
	}

	// We'll be using the first replica of the statefulset as the discovery "hub" for all cluster members.  This will be the
	// *only* entry in the cluster peer list used by each cluster member.  The main reason to limit this peer list to the
	// first member is so that when scaling up from 1 to N, our first replica is not restarted, and therefore does
	// not lose its state before replicating over to subsequent replicas.

	// First replica is addressed in the form <statefulset_name>-0.<service_name>
	firstReplicaName := fmt.Sprintf("%s-%d.%s", statefulSet.Name, 0, alertManagerClusterService)
	arg := fmt.Sprintf("--cluster.peer=%s:%d", firstReplicaName, config.AlertManagerCluster.Port)
	statefulSet.Spec.Template.Spec.Containers[0].Args = append(statefulSet.Spec.Template.Spec.Containers[0].Args, arg)
	statefulSet.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "status.podIP",
				},
			},
		},
	}

	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = 5
	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds = 1
	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds = 10

	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds = 5
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds = 1
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds = 10

	// Alertmanager config volume
	volumes := []corev1.Volume{
		{
			Name: "alert-config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.AlertManager.ConfigMap},
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "alert-config-volume",
			MountPath: constants.AlertManagerConfigMountPath,
		},
	}
	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMounts...)
	statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes, volumes...)

	// Config-reloader container
	statefulSet.Spec.Template.Spec.Containers = append(statefulSet.Spec.Template.Spec.Containers, corev1.Container{
		Name:            config.ConfigReloader.Name,
		Image:           config.ConfigReloader.Image,
		ImagePullPolicy: constants.DefaultImagePullPolicy,
	})
	statefulSet.Spec.Template.Spec.Containers[1].Args = []string{"-volume-dir=" + constants.AlertManagerConfigMountPath, "-webhook-url=" + constants.AlertManagerWebhookURL}
	statefulSet.Spec.Template.Spec.Containers[1].VolumeMounts = volumeMounts

	return statefulSet
}

// Creates a statefulset element for the given VMO and component
func createStatefulSetElement(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, vmoResources *vmcontrollerv1.Resources,
	componentDetails config.ComponentDetails, serviceName string) *appsv1.StatefulSet {
	labels := resources.GetSpecID(vmo.Name, componentDetails.Name)
	statefulSetName := resources.GetMetaName(vmo.Name, componentDetails.Name)
	if serviceName == "" {
		serviceName = statefulSetName
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resources.GetMetaLabels(vmo),
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
					Labels: labels,
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

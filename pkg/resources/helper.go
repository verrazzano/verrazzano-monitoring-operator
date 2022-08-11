// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package resources

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	runes                  = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	masterHTTPEndpoint     = "VMO_MASTER_HTTP_ENDPOINT"
	dashboardsHTTPEndpoint = "VMO_DASHBOARDS_HTTP_ENDPOINT"
)

const serviceClusterLocal = ".svc.cluster.local"

//CopyImmutableEnvVars copies the initial master node environment variable from an existing container to an expected container
// cluster.initial_master_nodes shouldn't be changed after it's set.
func CopyImmutableEnvVars(expected, existing []corev1.Container, containerName string) {
	getContainer := func(containers []corev1.Container) (int, *corev1.Container) {
		for idx, c := range containers {
			if c.Name == containerName {
				return idx, &c
			}
		}
		return -1, nil
	}

	// Initial master nodes should not change
	idx, currentContainer := getContainer(expected)
	_, existingContainer := getContainer(existing)
	if currentContainer == nil || existingContainer == nil {
		return
	}

	getAndSetVar := func(varName string) {
		envVar := GetEnvVar(existingContainer, varName)
		if envVar != nil {
			SetEnvVar(currentContainer, envVar)
		}
	}

	getAndSetVar(constants.ClusterInitialMasterNodes)
	getAndSetVar("node.roles")
	expected[idx] = *currentContainer
}

//GetEnvVar retrieves a container EnvVar if it is present
func GetEnvVar(container *corev1.Container, name string) *corev1.EnvVar {
	for _, envVar := range container.Env {
		if envVar.Name == name {
			return &envVar
		}
	}
	return nil
}

//SetEnvVar sets a container EnvVar, overriding if it was laready present
func SetEnvVar(container *corev1.Container, envVar *corev1.EnvVar) {
	for idx, env := range container.Env {
		if env.Name == envVar.Name {
			container.Env[idx] = *envVar
			return
		}
	}
	container.Env = append(container.Env, *envVar)
}

func GetOpenSearchHTTPEndpoint(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) string {
	// The master HTTP port may be overridden if necessary.
	// This can be useful in situations where the VMO does not have direct access to the cluster service,
	// such as when you are using port-forwarding.
	masterServiceEndpoint := os.Getenv(masterHTTPEndpoint)
	if len(masterServiceEndpoint) > 0 {
		return masterServiceEndpoint
	}
	return fmt.Sprintf("http://%s-http.%s%s:%d",
		GetMetaName(vmo.Name, config.ElasticsearchMaster.Name),
		vmo.Namespace,
		serviceClusterLocal,
		constants.OSHTTPPort)
}

func GetOpenSearchDashboardsHTTPEndpoint(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) string {
	dashboardsServiceEndpoint := os.Getenv(dashboardsHTTPEndpoint)
	if len(dashboardsServiceEndpoint) > 0 {
		return dashboardsServiceEndpoint
	}
	return fmt.Sprintf("http://%s.%s%s:%d", GetMetaName(vmo.Name, config.Kibana.Name),
		vmo.Namespace,
		serviceClusterLocal,
		constants.OSDashboardsHTTPPort)
}

func GetOwnerLabels(owner string) map[string]string {
	return map[string]string{
		"owner": owner,
	}
}

//GetNewRandomID generates a random alphanumeric string of the format [a-z0-9]{size}
func GetNewRandomID(size int) (string, error) {
	builder := strings.Builder{}
	for i := 0; i < size; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(runes))))
		if err != nil {
			return "", err
		}
		builder.WriteRune(runes[idx.Int64()])
	}
	return builder.String(), nil
}

// GetMetaName returns name
func GetMetaName(vmoName string, componentName string) string {
	return constants.VMOServiceNamePrefix + vmoName + "-" + componentName
}

// GetMetaLabels returns k8s-app and vmo lables
func GetMetaLabels(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) map[string]string {
	return map[string]string{constants.K8SAppLabel: constants.VMOGroup, constants.VMOLabel: vmo.Name}
}

// GetCompLabel returns a component value for opensearch
func GetCompLabel(componentName string) string {
	var componentLabelValue string
	switch componentName {
	case config.ElasticsearchMaster.Name, config.ElasticsearchData.Name, config.ElasticsearchIngest.Name:
		componentLabelValue = constants.ComponentOpenSearchValue
	default:
		componentLabelValue = componentName
	}
	return componentLabelValue
}

// DeepCopyMap performs a deepcopy of a map
func DeepCopyMap(srcMap map[string]string) map[string]string {
	result := make(map[string]string, len(srcMap))
	for k, v := range srcMap {
		result[k] = v
	}
	return result
}

// GetSpecID returns app label
func GetSpecID(vmoName string, componentName string) map[string]string {
	return map[string]string{constants.ServiceAppLabel: vmoName + "-" + componentName}
}

// GetServicePort returns service port
func GetServicePort(componentDetails config.ComponentDetails) corev1.ServicePort {
	return corev1.ServicePort{Name: "http-" + componentDetails.Name, Port: int32(componentDetails.Port)}
}

// GetOwnerReferences returns owner references
func GetOwnerReferences(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []metav1.OwnerReference {
	var ownerReferences []metav1.OwnerReference
	if vmo.Spec.CascadingDelete {
		ownerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(vmo, schema.GroupVersionKind{
				Group:   vmcontrollerv1.SchemeGroupVersion.Group,
				Version: vmcontrollerv1.SchemeGroupVersion.Version,
				Kind:    constants.VMOKind,
			}),
		}
	}
	return ownerReferences
}

// SliceContains returns whether or not the given slice contains the given string
func SliceContains(slice []string, value string) bool {
	for _, a := range slice {
		if a == value {
			return true
		}
	}
	return false
}

// GetStorageElementForComponent returns storage for a given component
func GetStorageElementForComponent(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) (storage *vmcontrollerv1.Storage) {
	switch component.Name {
	case config.Grafana.Name:
		return &vmo.Spec.Grafana.Storage
	case config.ElasticsearchData.Name:
		return vmo.Spec.Elasticsearch.DataNode.Storage
	}
	return nil
}

// GetReplicasForComponent returns number of replicas for a given component
func GetReplicasForComponent(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) (replicas int32) {
	switch component.Name {
	case config.Grafana.Name:
		return int32(1)
	case config.ElasticsearchData.Name:
		return vmo.Spec.Elasticsearch.DataNode.Replicas
	}
	return 0
}

// GetNextStringInSequence returns the next string in the incremental sequence given a string
func GetNextStringInSequence(name string) string {
	tokens := strings.Split(name, "-")
	if len(tokens) < 2 {
		return name + "-1" // Starting a new sequence
	}
	number, err := strconv.Atoi(tokens[len(tokens)-1])
	if err != nil {
		return name + "-1" // Starting a new sequence
	}
	tokens[len(tokens)-1] = strconv.Itoa(number + 1)
	return strings.Join(tokens, "-")
}

// CreateContainerElement creates a generic container element for the given component of the given VMO object.
func CreateContainerElement(vmoStorage *vmcontrollerv1.Storage,
	vmoResources *vmcontrollerv1.Resources, componentDetails config.ComponentDetails) corev1.Container {

	var volumeMounts []corev1.VolumeMount
	if vmoStorage != nil && vmoStorage.PvcNames != nil && vmoStorage.Size != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{MountPath: componentDetails.DataDir, Name: constants.StorageVolumeName})
	}

	limitResourceList := corev1.ResourceList{}
	requestResourceList := corev1.ResourceList{}
	if vmoResources != nil {
		if vmoResources.LimitCPU != "" {
			limitResourceList[corev1.ResourceCPU] = resource.MustParse(vmoResources.LimitCPU)
		}
		if vmoResources.LimitMemory != "" {
			limitResourceList[corev1.ResourceMemory] = resource.MustParse(vmoResources.LimitMemory)
		}
		if vmoResources.RequestCPU != "" {
			requestResourceList[corev1.ResourceCPU] = resource.MustParse(vmoResources.RequestCPU)
		}
		if vmoResources.RequestMemory != "" {
			requestResourceList[corev1.ResourceMemory] = resource.MustParse(vmoResources.RequestMemory)
		}
	}

	var livenessProbe *corev1.Probe
	if componentDetails.LivenessHTTPPath != "" {
		livenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   componentDetails.LivenessHTTPPath,
					Port:   intstr.IntOrString{IntVal: int32(componentDetails.Port)},
					Scheme: "HTTP",
				},
			},
		}
	}

	var readinessProbe *corev1.Probe
	if componentDetails.ReadinessHTTPPath != "" {
		readinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   componentDetails.ReadinessHTTPPath,
					Port:   intstr.IntOrString{IntVal: int32(componentDetails.Port)},
					Scheme: "HTTP",
				},
			},
		}
	}
	return corev1.Container{
		Name:            componentDetails.Name,
		Image:           componentDetails.Image,
		ImagePullPolicy: constants.DefaultImagePullPolicy,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &componentDetails.Privileged,
		},
		Ports: []corev1.ContainerPort{{Name: componentDetails.Name, ContainerPort: int32(componentDetails.Port)}},
		Resources: corev1.ResourceRequirements{
			Requests: requestResourceList,
			Limits:   limitResourceList,
		},
		VolumeMounts:   volumeMounts,
		LivenessProbe:  livenessProbe,
		ReadinessProbe: readinessProbe,
	}
}

// CreateZoneAntiAffinityElement return an Affinity resource for a given VMO instance and component
func CreateZoneAntiAffinityElement(vmoName string, component string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: GetSpecID(vmoName, component),
						},
						TopologyKey: constants.K8sZoneLabel,
					},
				},
			},
		},
	}
}

// CreateNodeAntiAffinityElement return an Affinity resource for a given VMO instance and component
func CreateNodeAntiAffinityElement(vmoName string, component string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: GetSpecID(vmoName, component),
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
	}
}

// GetElasticsearchMasterInitContainer return an Elasticsearch Init container for the master.  This changes ownership of
// the ES directory permissions needed to access PV volume data.  Also set the max map count.
func GetElasticsearchMasterInitContainer() *corev1.Container {
	elasticsearchInitContainer := CreateContainerElement(nil, nil, config.ElasticsearchInit)
	elasticsearchInitContainer.Command =
		[]string{"sh", "-c", "chown -R 1000:1000 /usr/share/opensearch/data; sysctl -w vm.max_map_count=262144"}
	elasticsearchInitContainer.Ports = nil
	return &elasticsearchInitContainer
}

// GetElasticsearchInitContainer returns an Elasticsearch Init container object
func GetElasticsearchInitContainer() *corev1.Container {
	elasticsearchInitContainer := CreateContainerElement(nil, nil, config.ElasticsearchInit)
	elasticsearchInitContainer.Args = []string{"sysctl", "-w", "vm.max_map_count=262144"}
	elasticsearchInitContainer.Ports = nil
	return &elasticsearchInitContainer
}

// NewVal return a pointer to an int32 given an int32 value
func NewVal(value int32) *int32 {
	var val = value
	return &val
}

// New64Val return a pointer to an int64 given an int64 value
func New64Val(value int64) *int64 {
	var val = value
	return &val
}

// oidcProxyName returns OIDC Proxy name of the component. ex. es-ingest-oidc
func oidcProxyName(componentName string) string {
	return componentName + "-" + config.OidcProxy.Name
}

// OidcProxyMetaName returns OIDC Proxy meta name of the component. ex. vmi-system-es-ingest-oidc
func OidcProxyMetaName(vmoName string, component string) string {
	return GetMetaName(vmoName, oidcProxyName(component))
}

// AuthProxyMetaName returns Auth Proxy service name
// TESTING: should be passed in from hel chart as s
func AuthProxyMetaName() string {
	return os.Getenv("AUTH_PROXY_SERVICE_NAME")
}

// AuthProxyMetaName returns Auth Proxy service name
func AuthProxyPort() string {
	return os.Getenv("AUTH_PROXY_SERVICE_PORT")
}

// OidcProxyConfigName returns OIDC Proxy ConfigMap name of the component. ex. vmi-system-es-ingest-oidc-config
func OidcProxyConfigName(vmo string, component string) string {
	return OidcProxyMetaName(vmo, component) + "-config"
}

// OidcProxyIngressHost returns OIDC Proxy ingress host.
func OidcProxyIngressHost(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) string {
	host := component.Name
	if component.EndpointName != "" {
		host = component.EndpointName
	}
	return fmt.Sprintf("%s.%s", host, vmo.Spec.URI)
}

//CreateOidcProxy creates OpenID Connect (OIDC) proxy container and config Volume
func CreateOidcProxy(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, vmoResources *vmcontrollerv1.Resources, component *config.ComponentDetails) ([]corev1.Volume, *corev1.Container) {
	var volumes []corev1.Volume
	configName := OidcProxyConfigName(vmo.Name, component.Name)
	var defaultMode int32 = 0755
	configVolume := corev1.Volume{Name: configName, VolumeSource: corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: configName},
			DefaultMode:          &defaultMode,
		},
	}}
	oidcProxContainer := CreateContainerElement(nil, vmoResources, *component.OidcProxy)
	oidcProxContainer.Command = []string{"/bootstrap/startup.sh"}
	oidcProxContainer.VolumeMounts = []corev1.VolumeMount{{Name: configName, MountPath: "/bootstrap"}}
	if len(vmo.Labels[constants.ClusterNameData]) > 0 {
		secretVolume := corev1.Volume{Name: "secret", VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: constants.MCRegistrationSecret,
			},
		}}
		volumes = append(volumes, secretVolume)
		oidcProxContainer.VolumeMounts = append(oidcProxContainer.VolumeMounts, corev1.VolumeMount{Name: "secret", MountPath: "/secret"})
	}
	volumes = append(volumes, configVolume)
	return volumes, &oidcProxContainer
}

//OidcProxyService creates OidcProxy Service
func OidcProxyService(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          GetMetaLabels(vmo),
			Name:            OidcProxyMetaName(vmo.Name, component.Name),
			Namespace:       vmo.Namespace,
			OwnerReferences: GetOwnerReferences(vmo),
		},
		Spec: corev1.ServiceSpec{
			Type:     vmo.Spec.ServiceType,
			Selector: GetSpecID(vmo.Name, component.Name),
			Ports:    []corev1.ServicePort{{Name: "oidc", Port: int32(constants.OidcProxyPort)}},
		},
	}
}

// convertToRegexp converts index pattern to a regular expression pattern.
func ConvertToRegexp(pattern string) string {
	var result strings.Builder
	// Add ^ at the beginning
	result.WriteString("^")
	for i, literal := range strings.Split(pattern, "*") {

		// Replace * with .*
		if i > 0 {
			result.WriteString(".*")
		}

		// Quote any regular expression meta characters in the
		// literal text.
		result.WriteString(regexp.QuoteMeta(literal))
	}
	// Add $ at the end
	result.WriteString("$")
	return result.String()
}

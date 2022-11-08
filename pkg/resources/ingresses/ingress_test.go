// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package ingresses

import (
	"fmt"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
)

func TestVMONoIngress(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled: true,
			},
			Kibana: vmcontrollerv1.Kibana{
				Enabled: true,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
			},
		},
	}
	ingresses, err := New(vmo, map[string]*netv1.Ingress{})
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 0, len(ingresses), "Length of generated Ingresses")
}

func TestVMOWithIngresses(t *testing.T) {
	const vmiName = "test-vmi"
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			SecretName: "secret",
			URI:        "example.com",
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled: true,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
			},
		},
	}
	vmo.Name = vmiName
	ingresses, err := New(vmo, map[string]*netv1.Ingress{})
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 3, len(ingresses), "Length of generated Ingresses")
	assert.Equal(t, 1, len(ingresses[0].Spec.TLS), "Number of TLS elements in generated Ingress")
	assert.Equal(t, 1, len(ingresses[0].Spec.TLS[0].Hosts), "Number of hosts in generated Ingress")
	assert.Equal(t, "api.example.com", ingresses[0].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, "grafana.example.com", ingresses[1].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, "opensearch.example.com", ingresses[2].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, vmiName+"-tls-api", ingresses[0].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, vmiName+"-tls-grafana", ingresses[1].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, vmiName+"-tls-os-ingest", ingresses[2].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, "basic", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-type"], "Auth type")
	assert.Equal(t, "secret", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-secret"], "Auth secret")
	assert.Equal(t, "example.com auth", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-realm"], "Auth realm")
	assert.Equal(t, "true", ingresses[0].Annotations["nginx.ingress.kubernetes.io/service-upstream"], "Service upstream")
	assert.Equal(t, "${service_name}.${namespace}.svc.cluster.local", ingresses[0].Annotations["nginx.ingress.kubernetes.io/upstream-vhost"], "Upstream vhost")
	assert.Equal(t, "api.example.com", ingresses[0].Annotations["cert-manager.io/common-name"], "TLS cert CN")
	assert.Equal(t, "grafana.example.com", ingresses[1].Annotations["cert-manager.io/common-name"], "TLS cert CN")
	assert.Equal(t, "opensearch.example.com", ingresses[2].Annotations["cert-manager.io/common-name"], "TLS cert CN")
	assert.Equal(t, getIngressClassName(vmo), *ingresses[0].Spec.IngressClassName)
}

// TestIngressesToAddDeprecatedHosts updates the new OS and OSD ingresses
// With the rules and hosts of deprecated ES and Kibana ingresses.
// Tests VPO Upgrade scenario
func TestIngressesToAddDeprecatedHosts(t *testing.T) {
	const vmiName = "system"
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			SecretName: "secret",
			URI:        "example.com",
			Kibana: vmcontrollerv1.Kibana{
				Enabled: true,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
			},
		},
	}
	vmo.Name = vmiName
	ingressESHost := resources.OidcProxyIngressHost(vmo, &config.ElasticsearchIngest)
	ingressESRule := resources.GetIngressRule(ingressESHost)
	deprecatedESIngress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s%s-%s", constants.VMOServiceNamePrefix, vmo.Name, config.ElasticsearchIngest.Name),
			Namespace: vmo.Namespace,
		},
		Spec: netv1.IngressSpec{
			TLS: []netv1.IngressTLS{
				{
					Hosts:      []string{ingressESHost},
					SecretName: fmt.Sprintf("%s-tls-%s", vmo.Name, config.ElasticsearchIngest.Name),
				},
			},
			Rules: []netv1.IngressRule{ingressESRule},
		}}

	existingIngress := make(map[string]*netv1.Ingress)
	existingIngress[resources.GetMetaName("system", config.ElasticsearchIngest.Name)] = deprecatedESIngress

	ingressKibanaHost := resources.OidcProxyIngressHost(vmo, &config.Kibana)
	ingressKibanaRule := resources.GetIngressRule(ingressKibanaHost)
	deprecatedKibanaIngress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s%s-%s", constants.VMOServiceNamePrefix, vmo.Name, config.Kibana.Name),
			Namespace: vmo.Namespace,
		},
		Spec: netv1.IngressSpec{
			TLS: []netv1.IngressTLS{
				{
					Hosts:      []string{ingressKibanaHost},
					SecretName: fmt.Sprintf("%s-tls-%s", vmo.Name, config.Kibana.Name),
				},
			},
			Rules: []netv1.IngressRule{ingressKibanaRule},
		}}
	existingIngress[resources.GetMetaName("system", config.Kibana.Name)] = deprecatedKibanaIngress
	ingresses, err := New(vmo, existingIngress)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(ingresses[1].Spec.Rules[0].Host)
	assert.Equal(t, 2, len(ingresses[2].Spec.Rules), "Length of Opensearch Ingress Rules")
	assert.Equal(t, 2, len(ingresses[2].Spec.Rules), "Length of Opendashboards Ingress Rules")
	assert.Equal(t, "api.example.com", ingresses[0].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, "kibana.example.com", ingresses[1].Spec.TLS[0].Hosts[1], "TLS hosts")
	assert.Equal(t, "opensearchdashboards.example.com", ingresses[1].Spec.TLS[0].Hosts[0], "Deprecated TLS host")
	assert.Equal(t, "opensearch.example.com", ingresses[2].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, "elasticsearch.example.com", ingresses[2].Spec.TLS[0].Hosts[1], "Deprecated TLS host")
	assert.Equal(t, 3, len(ingresses), "Length of generated Ingresses")
	assert.Equal(t, 1, len(ingresses[0].Spec.TLS), "Number of TLS elements in generated Ingress")
	assert.Equal(t, 1, len(ingresses[0].Spec.TLS[0].Hosts), "Number of hosts in generated Ingress")
	assert.Equal(t, vmiName+"-tls-api", ingresses[0].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, vmiName+"-tls-os-ingest", ingresses[2].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, "basic", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-type"], "Auth type")
	assert.Equal(t, "secret", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-secret"], "Auth secret")
	assert.Equal(t, "example.com auth", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-realm"], "Auth realm")
	assert.Equal(t, "true", ingresses[0].Annotations["nginx.ingress.kubernetes.io/service-upstream"], "Service upstream")
	assert.Equal(t, "${service_name}.${namespace}.svc.cluster.local", ingresses[0].Annotations["nginx.ingress.kubernetes.io/upstream-vhost"], "Upstream vhost")
	assert.Equal(t, "api.example.com", ingresses[0].Annotations["cert-manager.io/common-name"], "TLS cert CN")
	assert.Equal(t, "opensearch.example.com", ingresses[2].Annotations["cert-manager.io/common-name"], "TLS cert CN")
	assert.Equal(t, getIngressClassName(vmo), *ingresses[0].Spec.IngressClassName)
}

// TestIngressesToPreserveAdditionalDeprecatedHosts tests reconcile with the existing OS and OSD ingresses
// And preserves the existing ES and Kibana ingress rules/hosts in OS/OSD ingress during upgrade.
// Tests VPO Upgrade scenario
func TestIngressesToPreserveAdditionalDeprecatedHosts(t *testing.T) {
	const vmiName = "system"
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			SecretName: "secret",
			URI:        "example.com",
			Kibana: vmcontrollerv1.Kibana{
				Enabled: true,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
			},
		},
	}
	vmo.Name = vmiName
	ingressESHost := resources.OidcProxyIngressHost(vmo, &config.ElasticsearchIngest)
	ingressESRule := resources.GetIngressRule(ingressESHost)
	ingressOSHost := resources.OidcProxyIngressHost(vmo, &config.OpensearchIngest)
	ingressOSRule := resources.GetIngressRule(ingressOSHost)
	deprecatedESIngress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s%s-%s", constants.VMOServiceNamePrefix, vmo.Name, config.ElasticsearchIngest.Name),
			Namespace: vmo.Namespace,
		},
		Spec: netv1.IngressSpec{
			TLS: []netv1.IngressTLS{
				{
					Hosts:      []string{ingressESHost, ingressOSHost},
					SecretName: fmt.Sprintf("%s-tls-%s", vmo.Name, config.ElasticsearchIngest.Name),
				},
			},
			Rules: []netv1.IngressRule{ingressESRule, ingressOSRule},
		}}

	existingIngress := make(map[string]*netv1.Ingress)
	existingIngress[resources.GetMetaName("system", config.ElasticsearchIngest.Name)] = deprecatedESIngress

	ingressKibanaHost := resources.OidcProxyIngressHost(vmo, &config.Kibana)
	ingressKibanaRule := resources.GetIngressRule(ingressKibanaHost)
	ingressOSDHost := resources.OidcProxyIngressHost(vmo, &config.OpenSearchDashboards)
	ingressOSDRule := resources.GetIngressRule(ingressOSDHost)
	deprecatedKibanaIngress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s%s-%s", constants.VMOServiceNamePrefix, vmo.Name, config.Kibana.Name),
			Namespace: vmo.Namespace,
		},
		Spec: netv1.IngressSpec{
			TLS: []netv1.IngressTLS{
				{
					Hosts:      []string{ingressKibanaHost, ingressOSDHost},
					SecretName: fmt.Sprintf("%s-tls-%s", vmo.Name, config.Kibana.Name),
				},
			},
			Rules: []netv1.IngressRule{ingressKibanaRule, ingressOSDRule},
		}}
	existingIngress[resources.GetMetaName("system", config.Kibana.Name)] = deprecatedKibanaIngress
	ingresses, err := New(vmo, existingIngress)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(ingresses[1].Spec.Rules[0].Host)
	assert.Equal(t, 2, len(ingresses[2].Spec.Rules), "Length of Opensearch Ingress Rules")
	assert.Equal(t, 2, len(ingresses[2].Spec.Rules), "Length of Opendashboards Ingress Rules")
	assert.Equal(t, "api.example.com", ingresses[0].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, "kibana.example.com", ingresses[1].Spec.TLS[0].Hosts[1], "TLS hosts")
	assert.Equal(t, "opensearchdashboards.example.com", ingresses[1].Spec.TLS[0].Hosts[0], "Deprecated TLS host")
	assert.Equal(t, "opensearch.example.com", ingresses[2].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, "elasticsearch.example.com", ingresses[2].Spec.TLS[0].Hosts[1], "Deprecated TLS host")
	assert.Equal(t, 3, len(ingresses), "Length of generated Ingresses")
	assert.Equal(t, 1, len(ingresses[0].Spec.TLS), "Number of TLS elements in generated Ingress")
	assert.Equal(t, 1, len(ingresses[0].Spec.TLS[0].Hosts), "Number of hosts in generated Ingress")
	assert.Equal(t, vmiName+"-tls-api", ingresses[0].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, vmiName+"-tls-os-ingest", ingresses[2].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, "basic", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-type"], "Auth type")
	assert.Equal(t, "secret", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-secret"], "Auth secret")
	assert.Equal(t, "example.com auth", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-realm"], "Auth realm")
	assert.Equal(t, "true", ingresses[0].Annotations["nginx.ingress.kubernetes.io/service-upstream"], "Service upstream")
	assert.Equal(t, "${service_name}.${namespace}.svc.cluster.local", ingresses[0].Annotations["nginx.ingress.kubernetes.io/upstream-vhost"], "Upstream vhost")
	assert.Equal(t, "api.example.com", ingresses[0].Annotations["cert-manager.io/common-name"], "TLS cert CN")
	assert.Equal(t, "opensearch.example.com", ingresses[2].Annotations["cert-manager.io/common-name"], "TLS cert CN")
	assert.Equal(t, getIngressClassName(vmo), *ingresses[0].Spec.IngressClassName)
}

func TestGetIngressClassName(t *testing.T) {
	ingressClassName := "foobar"
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			IngressClassName: &ingressClassName,
		},
	}
	assert.Equal(t, ingressClassName, getIngressClassName(vmo))
}

func TestVMOWithCascadingDelete(t *testing.T) {
	// With CascadingDelete
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			SecretName:      "secret",
			URI:             "example.com",
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled:  true,
				Replicas: 1,
			},
			AlertManager: vmcontrollerv1.AlertManager{
				Enabled: true,
			},
			Kibana: vmcontrollerv1.Kibana{
				Enabled: true,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
			},
		},
	}

	ingresses, err := New(vmo, map[string]*netv1.Ingress{})
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(ingresses) > 0, "Non-zero length generated ingresses")
	for _, ingress := range ingresses {
		assert.Equal(t, 1, len(ingress.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")
	}

	// Without CascadingDelete
	vmo.Spec.CascadingDelete = false
	ingresses, err = New(vmo, map[string]*netv1.Ingress{})
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(ingresses) > 0, "Non-zero length generated ingresses")
	for _, ingress := range ingresses {
		assert.Equal(t, 0, len(ingress.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
	}
}

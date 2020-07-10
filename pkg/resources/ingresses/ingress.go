// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package ingresses

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	extensions_v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func createIngressRuleElement(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, componentDetails config.ComponentDetails) extensions_v1beta1.IngressRule {
	serviceName := resources.GetMetaName(vmi.Name, componentDetails.Name)
	endpointName := componentDetails.EndpointName
	if endpointName == "" {
		endpointName = componentDetails.Name
	}
	fqdn := fmt.Sprintf("%s.%s", endpointName, vmi.Spec.URI)

	return extensions_v1beta1.IngressRule{
		Host: fqdn,
		IngressRuleValue: extensions_v1beta1.IngressRuleValue{
			HTTP: &extensions_v1beta1.HTTPIngressRuleValue{
				Paths: []extensions_v1beta1.HTTPIngressPath{
					{
						Path: "/",
						Backend: extensions_v1beta1.IngressBackend{
							ServiceName: serviceName,
							ServicePort: intstr.FromInt(componentDetails.Port),
						},
					},
				},
			},
		},
	}
}

func createIngressElementNoBasicAuth(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, hostName string, componentDetails config.ComponentDetails, ingressRule extensions_v1beta1.IngressRule) (*extensions_v1beta1.Ingress, error) {
	var hosts = []string{hostName}
	ingress := &extensions_v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{},
			Labels:          resources.GetMetaLabels(vmi),
			Name:            fmt.Sprintf("%s%s-%s", constants.VMIServiceNamePrefix, vmi.Name, componentDetails.Name),
			Namespace:       vmi.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmi),
		},
		Spec: extensions_v1beta1.IngressSpec{
			TLS: []extensions_v1beta1.IngressTLS{
				{
					Hosts:      hosts,
					SecretName: vmi.Name + "-tls",
				},
			},
			Rules: []extensions_v1beta1.IngressRule{ingressRule},
		},
	}

	ingress.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"] = constants.NginxClientMaxBodySize

	if len(vmi.Spec.IngressTargetDNSName) != 0 {
		ingress.Annotations["external-dns.alpha.kubernetes.io/target"] = vmi.Spec.IngressTargetDNSName
		ingress.Annotations["external-dns.alpha.kubernetes.io/ttl"] = strconv.Itoa(constants.ExternalDnsTTLSeconds)
	}
	// if we specify AutoSecret: true we attach an annotation that will create a cert
	if vmi.Spec.AutoSecret {
		// we must create a secret name too
		ingress.Annotations["kubernetes.io/tls-acme"] = "true"
	} else {
		ingress.Annotations["kubernetes.io/tls-acme"] = "false"
	}
	return ingress, nil
}

func addBasicAuthIngressAnnotations(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, ingress *extensions_v1beta1.Ingress, healthLocations string) {
	ingress.Annotations["nginx.ingress.kubernetes.io/auth-type"] = "basic"
	ingress.Annotations["nginx.ingress.kubernetes.io/auth-secret"] = vmi.Spec.SecretName
	ingress.Annotations["nginx.ingress.kubernetes.io/auth-realm"] = vmi.Spec.URI + " auth"
	//For custom location snippets k8s recommends we use server-snippet instead of configuration-snippet
	// With ingress controller 0.24.1 our code using configuration-snippet no longer works
	ingress.Annotations["nginx.ingress.kubernetes.io/server-snippet"] = healthLocations
}

func createIngressElement(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, hostName string, componentDetails config.ComponentDetails, ingressRule extensions_v1beta1.IngressRule, healthLocations string) (*extensions_v1beta1.Ingress, error) {
	ingress, err := createIngressElementNoBasicAuth(vmi, hostName, componentDetails, ingressRule)
	if err != nil {
		return ingress, err
	}
	addBasicAuthIngressAnnotations(vmi, ingress, healthLocations)
	return ingress, nil
}

// New will return a new Service for VMI that needs to executed for on Complete
func New(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) ([]*extensions_v1beta1.Ingress, error) {
	var ingresses []*extensions_v1beta1.Ingress

	// Only create ingress if URI and secret name specified
	if len(vmi.Spec.URI) <= 0 {
		glog.V(6).Info("URI not specified, skipping ingress creation")
		return ingresses, nil
	}

	// Create Ingress Rule for API Endpoint
	ingRule := createIngressRuleElement(vmi, config.Api)
	host := config.Api.Name + "." + vmi.Spec.URI
	healthLocations := noAuthOnHealthCheckSnippet(vmi, "", config.Api)
	ingress, err := createIngressElement(vmi, host, config.Api, ingRule, healthLocations)
	if err != nil {
		return ingresses, err
	}
	ingresses = append(ingresses, ingress)

	if vmi.Spec.Grafana.Enabled {
		// Create Ingress Rule for Grafana Endpoint
		ingRule = createIngressRuleElement(vmi, config.Grafana)
		host := config.Grafana.Name + "." + vmi.Spec.URI
		ingress, err := createIngressElementNoBasicAuth(vmi, host, config.Grafana, ingRule)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)
	}
	if vmi.Spec.Prometheus.Enabled {
		// Create Ingress Rule for Prometheus Endpoint
		ingRule = createIngressRuleElement(vmi, config.Prometheus)
		host = config.Prometheus.Name + "." + vmi.Spec.URI
		healthLocations = noAuthOnHealthCheckSnippet(vmi, "", config.Prometheus)
		ingress, err = createIngressElement(vmi, host, config.Prometheus, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)

		ingRule = createIngressRuleElement(vmi, config.PrometheusGW)
		host = config.PrometheusGW.Name + "." + vmi.Spec.URI
		healthLocations = noAuthOnHealthCheckSnippet(vmi, "", config.PrometheusGW)
		ingress, err = createIngressElement(vmi, host, config.PrometheusGW, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)
	}
	if vmi.Spec.AlertManager.Enabled {
		// Create Ingress Rule for AlertManager Endpoint
		ingRule = createIngressRuleElement(vmi, config.AlertManager)
		host = config.AlertManager.Name + "." + vmi.Spec.URI
		healthLocations = noAuthOnHealthCheckSnippet(vmi, "", config.AlertManager)
		ingress, err = createIngressElement(vmi, host, config.AlertManager, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)
	}
	if vmi.Spec.Kibana.Enabled {
		// Create Ingress Rule for Kibana Endpoint
		ingRule = createIngressRuleElement(vmi, config.Kibana)
		host := config.Kibana.Name + "." + vmi.Spec.URI
		healthLocations = noAuthOnHealthCheckSnippet(vmi, "", config.Kibana)

		ingress, err = createIngressElement(vmi, host, config.Kibana, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)
	}
	if vmi.Spec.Elasticsearch.Enabled {
		var ingress *extensions_v1beta1.Ingress
		ingRule = createIngressRuleElement(vmi, config.ElasticsearchIngest)
		host = config.ElasticsearchIngest.EndpointName + "." + vmi.Spec.URI
		healthLocations = noAuthOnHealthCheckSnippet(vmi, "", config.ElasticsearchIngest)
		ingress, err = createIngressElement(vmi, host, config.ElasticsearchIngest, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		ingress.Annotations["nginx.ingress.kubernetes.io/proxy-read-timeout"] = constants.NginxProxyReadTimeoutForKibana
		ingresses = append(ingresses, ingress)
	}

	return ingresses, nil
}

// noAuthOnHealthCheckSnippet returns an NGINX configuration snippet with Basic Authentication disabled for the the
// specified component's health check path.
func noAuthOnHealthCheckSnippet(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, disambiguationRoot string, componentDetails config.ComponentDetails) string {
	// Added = check so nginx matches only this path i.e. strict check
	return `location = ` + disambiguationRoot + componentDetails.LivenessHTTPPath + ` {
   auth_basic off;
   auth_request off;
   proxy_pass  ` + fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s", constants.VMIServiceNamePrefix+vmi.Name+"-"+componentDetails.Name, vmi.Namespace, componentDetails.Port, componentDetails.LivenessHTTPPath) + `;
}
`
}

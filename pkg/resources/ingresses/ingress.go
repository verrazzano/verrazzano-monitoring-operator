// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package ingresses

import (
	"fmt"
	"strconv"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"go.uber.org/zap"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createIngressRuleElement(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, componentDetails config.ComponentDetails) networkingv1.IngressRule {
	serviceName := resources.GetMetaName(vmo.Name, componentDetails.Name)
	endpointName := componentDetails.EndpointName
	if endpointName == "" {
		endpointName = componentDetails.Name
	}
	fqdn := fmt.Sprintf("%s.%s", endpointName, vmo.Spec.URI)

	return networkingv1.IngressRule{
		Host: fqdn,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path: "/",
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: serviceName,
								Port: networkingv1.ServiceBackendPort{
									Number: int32(componentDetails.Port),
								},
							},
						},
					},
				},
			},
		},
	}
}

func createIngressElementNoBasicAuth(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, hostName string, componentDetails config.ComponentDetails, ingressRule networkingv1.IngressRule) (*networkingv1.Ingress, error) {
	var hosts = []string{hostName}
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{},
			Labels:          resources.GetMetaLabels(vmo),
			Name:            fmt.Sprintf("%s%s-%s", constants.VMOServiceNamePrefix, vmo.Name, componentDetails.Name),
			Namespace:       vmo.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmo),
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      hosts,
					SecretName: vmo.Name + "-tls",
				},
			},
			Rules: []networkingv1.IngressRule{ingressRule},
		},
	}

	ingress.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"] = constants.NginxClientMaxBodySize

	if len(vmo.Spec.IngressTargetDNSName) != 0 {
		ingress.Annotations["external-dns.alpha.kubernetes.io/target"] = vmo.Spec.IngressTargetDNSName
		ingress.Annotations["external-dns.alpha.kubernetes.io/ttl"] = strconv.Itoa(constants.ExternalDNSTTLSeconds)
	}
	// if we specify AutoSecret: true we attach an annotation that will create a cert
	if vmo.Spec.AutoSecret {
		// we must create a secret name too
		ingress.Annotations["kubernetes.io/tls-acme"] = "true"
	} else {
		ingress.Annotations["kubernetes.io/tls-acme"] = "false"
	}
	return ingress, nil
}

func addBasicAuthIngressAnnotations(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, ingress *networkingv1.Ingress, healthLocations string) {
	ingress.Annotations["nginx.ingress.kubernetes.io/auth-type"] = "basic"
	ingress.Annotations["nginx.ingress.kubernetes.io/auth-secret"] = vmo.Spec.SecretName
	ingress.Annotations["nginx.ingress.kubernetes.io/auth-realm"] = vmo.Spec.URI + " auth"
	//For custom location snippets k8s recommends we use server-snippet instead of configuration-snippet
	// With ingress controller 0.24.1 our code using configuration-snippet no longer works
	ingress.Annotations["nginx.ingress.kubernetes.io/server-snippet"] = healthLocations
}

func createIngressElement(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, hostName string, componentDetails config.ComponentDetails, ingressRule networkingv1.IngressRule, healthLocations string) (*networkingv1.Ingress, error) {
	ingress, err := createIngressElementNoBasicAuth(vmo, hostName, componentDetails, ingressRule)
	if err != nil {
		return ingress, err
	}
	addBasicAuthIngressAnnotations(vmo, ingress, healthLocations)
	return ingress, nil
}

// New will return a new Service for VMO that needs to executed for on Complete
func New(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) ([]*networkingv1.Ingress, error) {
	var ingresses []*networkingv1.Ingress

	// Only create ingress if URI and secret name specified
	if len(vmo.Spec.URI) <= 0 {
		zap.S().Debugw("URI not specified, skipping ingress creation")
		return ingresses, nil
	}

	// Create Ingress Rule for API Endpoint
	if !config.API.Disabled {
		ingRule := createIngressRuleElement(vmo, config.API)
		host := config.API.Name + "." + vmo.Spec.URI
		healthLocations := noAuthOnHealthCheckSnippet(vmo, "", config.API)
		ingress, err := createIngressElement(vmo, host, config.API, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		setNginxRoutingAnnotations(ingress)
		ingresses = append(ingresses, ingress)
	}

	if vmo.Spec.Grafana.Enabled {
		if config.Grafana.OidcProxy != nil {
			ingresses = append(ingresses, newOidcProxyIngress(vmo, &config.Grafana))
		} else {
			// Create Ingress Rule for Grafana Endpoint
			ingRule := createIngressRuleElement(vmo, config.Grafana)
			host := config.Grafana.Name + "." + vmo.Spec.URI
			ingress, err := createIngressElementNoBasicAuth(vmo, host, config.Grafana, ingRule)
			if err != nil {
				return ingresses, err
			}
			ingresses = append(ingresses, ingress)
		}
	}
	if vmo.Spec.Prometheus.Enabled {
		if config.Prometheus.OidcProxy != nil {
			ingresses = append(ingresses, newOidcProxyIngress(vmo, &config.Prometheus))
		} else {
			// Create Ingress Rule for Prometheus Endpoint
			ingRule := createIngressRuleElement(vmo, config.Prometheus)
			host := config.Prometheus.Name + "." + vmo.Spec.URI
			healthLocations := noAuthOnHealthCheckSnippet(vmo, "", config.Prometheus)
			ingress, err := createIngressElement(vmo, host, config.Prometheus, ingRule, healthLocations)
			if err != nil {
				return ingresses, err
			}
			ingresses = append(ingresses, ingress)
		}
	}
	if vmo.Spec.AlertManager.Enabled {
		// Create Ingress Rule for AlertManager Endpoint
		ingRule := createIngressRuleElement(vmo, config.AlertManager)
		host := config.AlertManager.Name + "." + vmo.Spec.URI
		healthLocations := noAuthOnHealthCheckSnippet(vmo, "", config.AlertManager)
		ingress, err := createIngressElement(vmo, host, config.AlertManager, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)
	}
	if vmo.Spec.Kibana.Enabled {
		if config.Kibana.OidcProxy != nil {
			ingresses = append(ingresses, newOidcProxyIngress(vmo, &config.Kibana))
		} else {
			// Create Ingress Rule for Kibana Endpoint
			ingRule := createIngressRuleElement(vmo, config.Kibana)
			host := config.Kibana.Name + "." + vmo.Spec.URI
			healthLocations := noAuthOnHealthCheckSnippet(vmo, "", config.Kibana)

			ingress, err := createIngressElement(vmo, host, config.Kibana, ingRule, healthLocations)
			if err != nil {
				return ingresses, err
			}
			ingresses = append(ingresses, ingress)
		}
	}
	if vmo.Spec.Elasticsearch.Enabled {
		if config.ElasticsearchIngest.OidcProxy != nil {
			ingress := newOidcProxyIngress(vmo, &config.ElasticsearchIngest)
			ingress.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"] = "65M"
			ingresses = append(ingresses, ingress)
		} else {
			var ingress *networkingv1.Ingress
			ingRule := createIngressRuleElement(vmo, config.ElasticsearchIngest)
			host := config.ElasticsearchIngest.EndpointName + "." + vmo.Spec.URI
			healthLocations := noAuthOnHealthCheckSnippet(vmo, "", config.ElasticsearchIngest)
			ingress, err := createIngressElement(vmo, host, config.ElasticsearchIngest, ingRule, healthLocations)
			if err != nil {
				return ingresses, err
			}
			ingress.Annotations["nginx.ingress.kubernetes.io/proxy-read-timeout"] = constants.NginxProxyReadTimeoutForKibana
			ingresses = append(ingresses, ingress)
		}

	}

	return ingresses, nil
}

// setNginxRoutingAnnotations adds the nginx annotations required for routing via istio envoy
func setNginxRoutingAnnotations(ingress *networkingv1.Ingress) {
	ingress.Annotations["nginx.ingress.kubernetes.io/service-upstream"] = "true"
	ingress.Annotations["nginx.ingress.kubernetes.io/upstream-vhost"] = "${service_name}.${namespace}.svc.cluster.local"
}

// noAuthOnHealthCheckSnippet returns an NGINX configuration snippet with Basic Authentication disabled for the the
// specified component's health check path.
func noAuthOnHealthCheckSnippet(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, disambiguationRoot string, componentDetails config.ComponentDetails) string {
	// Added = check so nginx matches only this path i.e. strict check
	return `location = ` + disambiguationRoot + componentDetails.LivenessHTTPPath + ` {
   auth_basic off;
   auth_request off;
   proxy_pass  ` + fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s", constants.VMOServiceNamePrefix+vmo.Name+"-"+componentDetails.Name, vmo.Namespace, componentDetails.Port, componentDetails.LivenessHTTPPath) + `;
}
`
}

// newOidcProxyIngress creates the Ingress of the OidcProxy
func newOidcProxyIngress(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) *networkingv1.Ingress {
	resources.AuthProxyPort()
	port, err := strconv.ParseInt(resources.AuthProxyPort(), 10, 32)
	if err != nil {
		port = 8775
	}
	serviceName := resources.AuthProxyMetaName()
	ingressHost := resources.OidcProxyIngressHost(vmo, component)
	ingressRule := networkingv1.IngressRule{
		Host: ingressHost,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path: "/()(.*)",
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: serviceName,
								Port: networkingv1.ServiceBackendPort{
									Number: int32(port),
								},
							},
						},
					},
				},
			},
		},
	}
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{},
			Labels:          resources.GetMetaLabels(vmo),
			Name:            fmt.Sprintf("%s%s-%s", constants.VMOServiceNamePrefix, vmo.Name, component.Name),
			Namespace:       vmo.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmo),
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{ingressHost},
					SecretName: vmo.Name + "-tls",
				},
			},
			Rules: []networkingv1.IngressRule{ingressRule},
		},
	}
	ingress.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"] = constants.NginxClientMaxBodySize
	if len(vmo.Spec.IngressTargetDNSName) != 0 {
		ingress.Annotations["external-dns.alpha.kubernetes.io/target"] = vmo.Spec.IngressTargetDNSName
		ingress.Annotations["external-dns.alpha.kubernetes.io/ttl"] = strconv.Itoa(constants.ExternalDNSTTLSeconds)
	}
	if vmo.Spec.AutoSecret {
		ingress.Annotations["kubernetes.io/tls-acme"] = "true"
	} else {
		ingress.Annotations["kubernetes.io/tls-acme"] = "false"
	}
	ingress.Annotations["nginx.ingress.kubernetes.io/rewrite-target"] = "/$2"
	setNginxRoutingAnnotations(ingress)
	return ingress
}

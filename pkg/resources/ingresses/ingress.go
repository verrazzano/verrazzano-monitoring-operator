// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package ingresses

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	extensions_v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func createIngressRuleElement(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, componentDetails config.ComponentDetails) extensions_v1beta1.IngressRule {
	serviceName := resources.GetMetaName(sauron.Name, componentDetails.Name)
	endpointName := componentDetails.EndpointName
	if endpointName == "" {
		endpointName = componentDetails.Name
	}
	fqdn := fmt.Sprintf("%s.%s", endpointName, sauron.Spec.URI)

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

func createIngressElementNoBasicAuth(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, hostName string, componentDetails config.ComponentDetails, ingressRule extensions_v1beta1.IngressRule) (*extensions_v1beta1.Ingress, error) {
	var hosts = []string{hostName}
	ingress := &extensions_v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{},
			Labels:          resources.GetMetaLabels(sauron),
			Name:            fmt.Sprintf("%s%s-%s", constants.SauronServiceNamePrefix, sauron.Name, componentDetails.Name),
			Namespace:       sauron.Namespace,
			OwnerReferences: resources.GetOwnerReferences(sauron),
		},
		Spec: extensions_v1beta1.IngressSpec{
			TLS: []extensions_v1beta1.IngressTLS{
				{
					Hosts:      hosts,
					SecretName: sauron.Name + "-tls",
				},
			},
			Rules: []extensions_v1beta1.IngressRule{ingressRule},
		},
	}

	ingress.Annotations["nginx.ingress.kubernetes.io/proxy-body-size"] = constants.NginxClientMaxBodySize

	if len(sauron.Spec.IngressTargetDNSName) != 0 {
		ingress.Annotations["external-dns.alpha.kubernetes.io/target"] = sauron.Spec.IngressTargetDNSName
		ingress.Annotations["external-dns.alpha.kubernetes.io/ttl"] = strconv.Itoa(constants.ExternalDnsTTLSeconds)
	}
	// if we specify AutoSecret: true we attach an annotation that will create a cert
	if sauron.Spec.AutoSecret {
		// we must create a secret name too
		ingress.Annotations["kubernetes.io/tls-acme"] = "true"
	} else {
		ingress.Annotations["kubernetes.io/tls-acme"] = "false"
	}
	return ingress, nil
}

func addBasicAuthIngressAnnotations(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, ingress *extensions_v1beta1.Ingress, healthLocations string) {
	ingress.Annotations["nginx.ingress.kubernetes.io/auth-type"] = "basic"
	ingress.Annotations["nginx.ingress.kubernetes.io/auth-secret"] = sauron.Spec.SecretName
	ingress.Annotations["nginx.ingress.kubernetes.io/auth-realm"] = sauron.Spec.URI + " auth"
	//For custom location snippets k8s recommends we use server-snippet instead of configuration-snippet
	// With ingress controller 0.24.1 our code using configuration-snippet no longer works
	ingress.Annotations["nginx.ingress.kubernetes.io/server-snippet"] = healthLocations
}

func createIngressElement(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, hostName string, componentDetails config.ComponentDetails, ingressRule extensions_v1beta1.IngressRule, healthLocations string) (*extensions_v1beta1.Ingress, error) {
	ingress, err := createIngressElementNoBasicAuth(sauron, hostName, componentDetails, ingressRule)
	if err != nil {
		return ingress, err
	}
	addBasicAuthIngressAnnotations(sauron, ingress, healthLocations)
	return ingress, nil
}

// New will return a new Service for Sauron that needs to executed for on Complete
func New(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) ([]*extensions_v1beta1.Ingress, error) {
	//create log for new creating service
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "VerrazzanoMonitoringInstance").Str("name", sauron.Name).Logger()

	var ingresses []*extensions_v1beta1.Ingress

	// Only create ingress if URI and secret name specified
	if len(sauron.Spec.URI) <= 0 {
		logger.Debug().Msg("URI not specified, skipping ingress creation")
		return ingresses, nil
	}

	// Create Ingress Rule for API Endpoint
	ingRule := createIngressRuleElement(sauron, config.Api)
	host := config.Api.Name + "." + sauron.Spec.URI
	healthLocations := noAuthOnHealthCheckSnippet(sauron, "", config.Api)
	ingress, err := createIngressElement(sauron, host, config.Api, ingRule, healthLocations)
	if err != nil {
		return ingresses, err
	}
	ingresses = append(ingresses, ingress)

	if sauron.Spec.Grafana.Enabled {
		// Create Ingress Rule for Grafana Endpoint
		ingRule = createIngressRuleElement(sauron, config.Grafana)
		host := config.Grafana.Name + "." + sauron.Spec.URI
		ingress, err := createIngressElementNoBasicAuth(sauron, host, config.Grafana, ingRule)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)
	}
	if sauron.Spec.Prometheus.Enabled {
		// Create Ingress Rule for Prometheus Endpoint
		ingRule = createIngressRuleElement(sauron, config.Prometheus)
		host = config.Prometheus.Name + "." + sauron.Spec.URI
		healthLocations = noAuthOnHealthCheckSnippet(sauron, "", config.Prometheus)
		ingress, err = createIngressElement(sauron, host, config.Prometheus, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)

		ingRule = createIngressRuleElement(sauron, config.PrometheusGW)
		host = config.PrometheusGW.Name + "." + sauron.Spec.URI
		healthLocations = noAuthOnHealthCheckSnippet(sauron, "", config.PrometheusGW)
		ingress, err = createIngressElement(sauron, host, config.PrometheusGW, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)
	}
	if sauron.Spec.AlertManager.Enabled {
		// Create Ingress Rule for AlertManager Endpoint
		ingRule = createIngressRuleElement(sauron, config.AlertManager)
		host = config.AlertManager.Name + "." + sauron.Spec.URI
		healthLocations = noAuthOnHealthCheckSnippet(sauron, "", config.AlertManager)
		ingress, err = createIngressElement(sauron, host, config.AlertManager, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)
	}
	if sauron.Spec.Kibana.Enabled {
		// Create Ingress Rule for Kibana Endpoint
		ingRule = createIngressRuleElement(sauron, config.Kibana)
		host := config.Kibana.Name + "." + sauron.Spec.URI
		healthLocations = noAuthOnHealthCheckSnippet(sauron, "", config.Kibana)

		ingress, err = createIngressElement(sauron, host, config.Kibana, ingRule, healthLocations)
		if err != nil {
			return ingresses, err
		}
		ingresses = append(ingresses, ingress)
	}
	if sauron.Spec.Elasticsearch.Enabled {
		var ingress *extensions_v1beta1.Ingress
		ingRule = createIngressRuleElement(sauron, config.ElasticsearchIngest)
		host = config.ElasticsearchIngest.EndpointName + "." + sauron.Spec.URI
		healthLocations = noAuthOnHealthCheckSnippet(sauron, "", config.ElasticsearchIngest)
		ingress, err = createIngressElement(sauron, host, config.ElasticsearchIngest, ingRule, healthLocations)
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
func noAuthOnHealthCheckSnippet(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, disambiguationRoot string, componentDetails config.ComponentDetails) string {
	// Added = check so nginx matches only this path i.e. strict check
	return `location = ` + disambiguationRoot + componentDetails.LivenessHTTPPath + ` {
   auth_basic off;
   auth_request off;
   proxy_pass  ` + fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s", constants.SauronServiceNamePrefix+sauron.Name+"-"+componentDetails.Name, sauron.Namespace, componentDetails.Port, componentDetails.LivenessHTTPPath) + `;
}
`
}

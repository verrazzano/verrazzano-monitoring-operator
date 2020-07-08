// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package util

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewSecret creates a basic auth secret
func NewSecret(secretName, namespace string) *corev1.Secret {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			//OwnerReferences: resources.GetOwnerReferences(sauron),
		},
		Data: map[string][]byte{
			"username": []byte("sauron"),
			"password": []byte("changeme"),
		},
	}
}

// NewSecretWithTLS creates an auth secret with TLS keys
func NewSecretWithTLS(secretName, namespace string, tlsCrt, tlsKey []byte, user string, passwd string) *corev1.Secret {
	if len(tlsKey) == 0 || len(secretName) == 0 || len(tlsCrt) == 0 {
		return nil
	}
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			//OwnerReferences: resources.GetOwnerReferences(sauron),
		},
		Data: map[string][]byte{
			"username": []byte(user),
			"password": []byte(passwd),
			"tls.crt":  tlsCrt,
			"tls.key":  tlsKey,
		},
	}
}

// NewSecretWithTLS creates an auth secret with TLS keys for admin and Reporter Users
func NewSecretWithTLSWithMultiUser(secretName, namespace string, tlsCrt, tlsKey []byte, user string, passwd string, extraCreds []string) *corev1.Secret {
	if len(tlsKey) == 0 || len(secretName) == 0 || len(tlsCrt) == 0 {
		return nil
	}
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			//OwnerReferences: resources.GetOwnerReferences(sauron),
		},
		Data: map[string][]byte{
			"username":  []byte(user),
			"password":  []byte(passwd),
			"username2": []byte(extraCreds[0]),
			"password2": []byte(extraCreds[1]),
			"tls.crt":   tlsCrt,
			"tls.key":   tlsKey,
		},
	}
}

// NewSauron creates a new sauron
func NewSauron(genName, secretName string) *vmcontrollerv1.VerrazzanoMonitoringInstance {
	return &vmcontrollerv1.VerrazzanoMonitoringInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: vmcontrollerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: genName,
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			SecretsName:     secretName,
			ServiceType:     corev1.ServiceTypeNodePort,
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled:  true,
				Replicas: 1,
			},
			AlertManager: vmcontrollerv1.AlertManager{
				Enabled:  true,
				Replicas: 1,
			},
			Kibana: vmcontrollerv1.Kibana{
				Enabled:  true,
				Replicas: 1,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
				MasterNode: vmcontrollerv1.ElasticsearchNode{
					// Hack for tests
					Replicas: 3,
				},
				IngestNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 1,
				},
				DataNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 1,
				},
			},
			Api: vmcontrollerv1.Api{
				Replicas: 1,
			},
		},
	}
}

// NewGrafanaOnlySauron Sauron with Grafana Service only enabled
func NewGrafanaOnlySauron(genName, secretName string) *vmcontrollerv1.VerrazzanoMonitoringInstance {
	return &vmcontrollerv1.VerrazzanoMonitoringInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: vmcontrollerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: genName,
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			SecretsName:     secretName,
			ServiceType:     corev1.ServiceTypeNodePort,
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled: false,
			},
			AlertManager: vmcontrollerv1.AlertManager{
				Enabled: false,
			},
			Kibana: vmcontrollerv1.Kibana{
				Enabled: false,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: false,
			},
		},
	}
}

// Copyright (C) 2020, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestVMOEmptyDeploymentSize(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	operatorConfig := &config.OperatorConfig{}
	expected, err := New(vmo, fake.NewSimpleClientset(), operatorConfig, map[string]string{})
	if err != nil {
		t.Error(err)
	}
	deployments := expected.Deployments
	assert.Equal(t, 1, len(deployments), "Length of generated deployments")
}

func TestVMOFullDeploymentSize(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			OpensearchDashboards: vmcontrollerv1.OpensearchDashboards{
				Enabled: true,
			},
			Opensearch: vmcontrollerv1.Opensearch{
				Enabled:    true,
				IngestNode: vmcontrollerv1.ElasticsearchNode{Replicas: 1},
				MasterNode: vmcontrollerv1.ElasticsearchNode{Replicas: 1},
				DataNode:   vmcontrollerv1.ElasticsearchNode{Replicas: 1},
			},
		},
	}
	expected, err := New(vmo, fake.NewSimpleClientset(), &config.OperatorConfig{}, map[string]string{})
	if err != nil {
		t.Error(err)
	}
	deployments := expected.Deployments
	deployments = append(deployments, NewOpenSearchDashboardsDeployment(vmo))
	assert.Equal(t, 5, len(deployments), "Length of generated deployments")
	assert.Equal(t, constants.VMOKind, deployments[0].ObjectMeta.OwnerReferences[0].Kind, "OwnerReferences is not set by default")
}

func TestVMODevProfileFullDeploymentSize(t *testing.T) {

	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			OpensearchDashboards: vmcontrollerv1.OpensearchDashboards{
				Enabled: true,
			},
			Opensearch: vmcontrollerv1.Opensearch{
				Enabled:    true,
				IngestNode: vmcontrollerv1.ElasticsearchNode{Replicas: 0},
				MasterNode: vmcontrollerv1.ElasticsearchNode{
					Replicas: 1,
					Roles: []vmcontrollerv1.NodeRole{
						vmcontrollerv1.MasterRole,
						vmcontrollerv1.IngestRole,
						vmcontrollerv1.DataRole,
					},
				},
				DataNode: vmcontrollerv1.ElasticsearchNode{Replicas: 0},
			},
		},
	}
	assert.True(t, nodes.IsSingleNodeCluster(vmo), "Single node ES setup, expected IsDevProfile to be true")
	expected, err := New(vmo, fake.NewSimpleClientset(), &config.OperatorConfig{}, map[string]string{})
	if err != nil {
		t.Error(err)
	}
	deployments := expected.Deployments
	deployments = append(deployments, NewOpenSearchDashboardsDeployment(vmo))
	assert.Equal(t, 3, len(deployments), "Length of generated deployments")
	assert.Equal(t, constants.VMOKind, deployments[0].ObjectMeta.OwnerReferences[0].Kind, "OwnerReferences is not set by default")
}

func TestVMODevProfileInvalidESTopology(t *testing.T) {

	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			OpensearchDashboards: vmcontrollerv1.OpensearchDashboards{
				Enabled: true,
			},
			Opensearch: vmcontrollerv1.Opensearch{
				Enabled:    true,
				IngestNode: vmcontrollerv1.ElasticsearchNode{Replicas: 0},
				MasterNode: vmcontrollerv1.ElasticsearchNode{Replicas: 0},
				DataNode:   vmcontrollerv1.ElasticsearchNode{Replicas: 0},
			},
		},
	}
	assert.False(t, nodes.IsSingleNodeCluster(vmo), "Invalid single node ES setup, expected IsDevProfile to be false")
	_, err := New(vmo, fake.NewSimpleClientset(), &config.OperatorConfig{}, map[string]string{})
	assert.Nil(t, err)
}

func TestVMOWithCascadingDelete(t *testing.T) {
	// With CascadingDelete
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			OpensearchDashboards: vmcontrollerv1.OpensearchDashboards{
				Enabled: true,
			},
			Opensearch: vmcontrollerv1.Opensearch{
				Enabled:    true,
				MasterNode: vmcontrollerv1.ElasticsearchNode{Replicas: 1},
			},
		},
	}
	expected, err := New(vmo, fake.NewSimpleClientset(), &config.OperatorConfig{}, map[string]string{})
	if err != nil {
		t.Error(err)
	}
	deployments := expected.Deployments
	assert.True(t, len(deployments) > 0, "Non-zero length generated deployments")
	for _, deployment := range deployments {
		assert.Equal(t, 1, len(deployment.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")
	}

	// Without CascadingDelete
	vmo.Spec.CascadingDelete = false
	expected, err = New(vmo, fake.NewSimpleClientset(), &config.OperatorConfig{}, map[string]string{})
	if err != nil {
		t.Error(err)
	}
	deployments = expected.Deployments
	assert.True(t, len(deployments) > 0, "Non-zero length generated deployments")
	for _, deployment := range deployments {
		assert.Equal(t, 0, len(deployment.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
	}
}

func TestVMOWithResourceConstraints(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "system",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
				Resources: vmcontrollerv1.Resources{
					LimitCPU:      "500m",
					LimitMemory:   "120Mi",
					RequestCPU:    "200m",
					RequestMemory: "60Mi",
				},
			},
			OpensearchDashboards: vmcontrollerv1.OpensearchDashboards{
				Enabled: true,
				Resources: vmcontrollerv1.Resources{
					LimitCPU:      "0.53",
					LimitMemory:   "123M",
					RequestCPU:    "0.23",
					RequestMemory: "63M",
				},
			},
			Opensearch: vmcontrollerv1.Opensearch{
				Enabled: true,
				IngestNode: vmcontrollerv1.ElasticsearchNode{
					Name:     config.ElasticsearchIngest.Name,
					Replicas: 1,
					Resources: vmcontrollerv1.Resources{
						LimitCPU:      "0.54",
						LimitMemory:   "124M",
						RequestCPU:    "0.24",
						RequestMemory: "64M",
					},
					Roles: []vmcontrollerv1.NodeRole{vmcontrollerv1.IngestRole},
				},
				DataNode: vmcontrollerv1.ElasticsearchNode{
					Name:     config.ElasticsearchData.Name,
					Replicas: 1,
					Roles:    []vmcontrollerv1.NodeRole{vmcontrollerv1.DataRole},
				},
				MasterNode: vmcontrollerv1.ElasticsearchNode{Replicas: 1},
			},
		},
	}
	expected, err := New(vmo, fake.NewSimpleClientset(), &config.OperatorConfig{}, map[string]string{})
	if err != nil {
		t.Error(err)
	}

	for _, deployment := range expected.Deployments {
		esDataName := resources.GetMetaName(vmo.Name, config.ElasticsearchData.Name)
		if deployment.Name == resources.GetMetaName(vmo.Name, config.Grafana.Name) {
			assert.Equal(t, resource.MustParse("500m"), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu(), "Granfana Limit CPU")
			assert.Equal(t, resource.MustParse("120Mi"), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory(), "Granfana Limit Memory")
			assert.Equal(t, resource.MustParse("200m"), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu(), "Granfana Request CPU")
			assert.Equal(t, resource.MustParse("60Mi"), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory(), "Granfana Request Memory")
		} else if deployment.Name == resources.GetMetaName(vmo.Name, config.Kibana.Name) {
			assert.Equal(t, resource.MustParse("0.53"), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu(), "Granfana Limit CPU")
			assert.Equal(t, resource.MustParse("123M"), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory(), "Granfana Limit Memory")
			assert.Equal(t, resource.MustParse("0.23"), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu(), "Granfana Request CPU")
			assert.Equal(t, resource.MustParse("63M"), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory(), "Granfana Request Memory")
		} else if deployment.Name == resources.GetMetaName(vmo.Name, config.ElasticsearchIngest.Name) {
			assert.Equal(t, resource.MustParse("0.54"), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu(), "Granfana Limit CPU")
			assert.Equal(t, resource.MustParse("124M"), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory(), "Granfana Limit Memory")
			assert.Equal(t, resource.MustParse("0.24"), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu(), "Granfana Request CPU")
			assert.Equal(t, resource.MustParse("64M"), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory(), "Granfana Request Memory")
		} else if deployment.Name == resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name) {
			// No resources specified on this endpoint
		} else if strings.Contains(deployment.Name, resources.GetMetaName(vmo.Name, config.ElasticsearchData.Name)) {
			// No resources specified on this endpoint
		} else if deployment.Name == resources.GetMetaName(vmo.Name, config.API.Name) {
			// No resources specified on API endpoint
		} else if deployment.Name == resources.OidcProxyMetaName(vmo.Name, config.ElasticsearchIngest.Name) {
			// No resources specified on OIDC proxy
		} else {
			t.Log("ESDataName: " + esDataName)
			t.Error("Unknown Deployment Name: " + deployment.Name)
		}
	}
}

func TestVMOWithReplicas(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			API: vmcontrollerv1.API{
				Replicas: 2,
			},
			OpensearchDashboards: vmcontrollerv1.OpensearchDashboards{
				Enabled:  true,
				Replicas: 4,
			},
		},
	}
	expected, err := New(vmo, fake.NewSimpleClientset(), &config.OperatorConfig{}, map[string]string{})
	if err != nil {
		t.Error(err)
	}
	deployments := expected.Deployments
	deployments = append(deployments, NewOpenSearchDashboardsDeployment(vmo))
	assert.Equal(t, 2, len(deployments), "Length of generated deployments")
	for _, deployment := range deployments {
		if deployment.Name == resources.GetMetaName(vmo.Name, config.API.Name) {
			assert.Equal(t, *resources.NewVal(2), *deployment.Spec.Replicas, "Api replicas")
		} else if deployment.Name == resources.GetMetaName(vmo.Name, config.OpenSearchDashboards.Name) {
			assert.Equal(t, *resources.NewVal(4), *deployment.Spec.Replicas, "OSD replicas")
		}
	}
}

func TestGetAdForPvcIndex(t *testing.T) {
	m1 := map[string]string{}
	m2 := map[string]string{"pvc1": "ad1", "pvc2": "ad2"}
	s1 := vmcontrollerv1.Storage{
		Size:     "50Gi",
		PvcNames: []string{"pvc1", "pvc2", "pvc3"},
	}
	assert.Equal(t, "", getAvailabilityDomainForPvcIndex(nil, m1, 0), "With nil storage")
	assert.Equal(t, "", getAvailabilityDomainForPvcIndex(nil, m2, 0), "With nil storage")
	assert.Equal(t, "", getAvailabilityDomainForPvcIndex(&s1, m1, 0), "With empty AD map")
	assert.Equal(t, "ad1", getAvailabilityDomainForPvcIndex(&s1, m2, 0), "With valid PVC")
	assert.Equal(t, "ad2", getAvailabilityDomainForPvcIndex(&s1, m2, 1), "With valid PVC")
	assert.Equal(t, "", getAvailabilityDomainForPvcIndex(&s1, m2, 2), "With non-existent PVC")
	assert.Equal(t, "", getAvailabilityDomainForPvcIndex(&s1, m2, 3), "With PVC index out of bounds")
}

func TestAPIWithNatGatewayIPs(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "my-vmo",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			NatGatewayIPs: []string{"1.1.1.1", "2.1.1.1"},
		},
	}
	expected, err := New(vmo, fake.NewSimpleClientset(), &config.OperatorConfig{}, map[string]string{})
	if err != nil {
		t.Error(err)
	}
	deployments := expected.Deployments
	apiDeployment, err := getDeploymentByName(constants.VMOServiceNamePrefix+"my-vmo-api", deployments)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, []string{"--natGatewayIPs=1.1.1.1,2.1.1.1"}, apiDeployment.Spec.Template.Spec.Containers[0].Args, "API args with NAT GW")
}

// Returns the deployment with the given name from the given list of deployments, returning an error if not found
func getDeploymentByName(deploymentName string, deploymentList []*appsv1.Deployment) (*appsv1.Deployment, error) {
	for _, deployment := range deploymentList {
		if deployment.Name == deploymentName {
			return deployment, nil
		}
	}
	return nil, fmt.Errorf("deployment %s not found", deploymentName)
}

func TestGrafanaSMTPConfig(t *testing.T) {
	trueValue := true
	vmi := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "system",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
				SMTP: &vmcontrollerv1.SMTPInfo{
					Enabled:        &trueValue,
					Host:           "testhost:25",
					ExistingSecret: "smtp-secret",
					UserKey:        "user",
					PasswordKey:    "pass",
					CertFileKey:    "certificate.crt",
					KeyFileKey:     "key.file",
					SkipVerify:     &trueValue,
					FromAddress:    "test@test.com",
					FromName:       "test",
					EHLOIdentity:   "EHLO",
					StartTLSPolicy: "OpportunisticStartTLS",
				},
			},
		},
	}
	testEncoded := []byte(base64.StdEncoding.EncodeToString([]byte("test")))
	secret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name: vmi.Spec.Grafana.SMTP.ExistingSecret,
		},
		Data: map[string][]byte{
			vmi.Spec.Grafana.SMTP.UserKey:     testEncoded,
			vmi.Spec.Grafana.SMTP.PasswordKey: testEncoded,
			vmi.Spec.Grafana.SMTP.CertFileKey: testEncoded,
			vmi.Spec.Grafana.SMTP.KeyFileKey:  testEncoded,
		},
	}
	expected, err := New(vmi, fake.NewSimpleClientset(&secret), &config.OperatorConfig{}, map[string]string{})
	if err != nil {
		t.Error(err)
	}

	expectedEnvVars := map[string]string{
		"GF_SMTP_ENABLED":         fmt.Sprintf("%v", trueValue),
		"GF_SMTP_HOST":            vmi.Spec.Grafana.SMTP.Host,
		"GF_SMTP_CERT_FILE":       fmt.Sprintf("%s/%s", constants.GrafanaSMTPConfigVolumePath, vmi.Spec.Grafana.SMTP.CertFileKey),
		"GF_SMTP_KEY_FILE":        fmt.Sprintf("%s/%s", constants.GrafanaSMTPConfigVolumePath, vmi.Spec.Grafana.SMTP.KeyFileKey),
		"GF_SMTP_SKIP_VERIFY":     fmt.Sprintf("%v", trueValue),
		"GF_SMTP_FROM_ADDRESS":    vmi.Spec.Grafana.SMTP.FromAddress,
		"GF_SMTP_FROM_NAME":       vmi.Spec.Grafana.SMTP.FromName,
		"GF_SMTP_EHLO_IDENTITY":   vmi.Spec.Grafana.SMTP.EHLOIdentity,
		"GF_SMTP_STARTTLS_POLICY": string(vmi.Spec.Grafana.SMTP.StartTLSPolicy),
	}
	for _, deployment := range expected.Deployments {
		if deployment.Name == resources.GetMetaName(vmi.Name, config.Grafana.Name) {
			for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
				if value, ok := expectedEnvVars[env.Name]; ok {
					assert.Equal(t, value, env.Value)
					delete(expectedEnvVars, env.Name)
				}

				if env.Name == "GF_SMTP_USER" {
					assert.NotNil(t, env.ValueFrom)
					assert.Equal(t, vmi.Spec.Grafana.SMTP.UserKey, env.ValueFrom.SecretKeyRef.Key)
				}

				if env.Name == "GF_SMTP_PASSWORD" {
					assert.NotNil(t, env.ValueFrom)
					assert.Equal(t, vmi.Spec.Grafana.SMTP.PasswordKey, env.ValueFrom.SecretKeyRef.Key)
				}
			}
			assert.Len(t, expectedEnvVars, 0, fmt.Sprintf("Could not find %v env variables set in Grafana deployment", expectedEnvVars))

			volumeFound := false
			for _, volume := range deployment.Spec.Template.Spec.Volumes {
				if volume.Name == constants.GrafanaSMTPConfigVolumeName {
					assert.Equal(t, vmi.Spec.Grafana.SMTP.ExistingSecret, volume.Secret.SecretName)
					volumeFound = true
				}
			}
			assert.True(t, volumeFound, "secret volume not created")

			volumeMountFound := false
			for _, volumeMount := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
				if volumeMount.Name == constants.GrafanaSMTPConfigVolumeName {
					assert.Equal(t, constants.GrafanaSMTPConfigVolumePath, volumeMount.MountPath)
					volumeMountFound = true
				}
			}
			assert.True(t, volumeMountFound, "secret volume not mounted to container")

		}
	}
}

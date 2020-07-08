// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package integ

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"

	"strconv"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/test/integ/framework"
	testutil "github.com/verrazzano/verrazzano-monitoring-operator/test/integ/util"
	"gopkg.in/resty.v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	username = "sauron"
	password = "changeme"
)

const (
	reporterUsername = "vmi-reporter"
	reporterPassword = "changeme"
)
const (
	changeusername = "s@ur0n"
	changepassword = "ch@ngeMe"
)

// ********************************************************
// *** Scenarios covered by the Basic Integration Tests ***
// Setup
// - creation of a basic Sauron instance + validations below
// - creation of a Sauron instance with block volumes + validations below
// Sauron API Server validations
// - Verify service endpoint connectivity
// - Update/PUT Prometheus configuration via HTTP API /prometheus/config
// - GET Prometheus configuration via HTTP API /prometheus/config
// - Update/PUT Alertmanager configuration via HTTP API /alertmanager/config
// - GET Alertmanager configuration via HTTP API /alertmanager/config
// - Update/PUT Alert configuration via HTTP API /alertmanager/config
// - GET Alert configuration via HTTP API /alertmanager/config
// Prometheus Server validations
// - Verify service endpoint connectivity
// - Verify Elasticsearch configuration via Prometheus HTTP API
// - Use Sauron API to addd http://localhost:9090/metrics as a local scrape target
// - Verify special 'up' metric which is updated when it performs a scrape
// Grafana Server validations
// - Verify service endpoint connectivity
// - Upload dashboard via Grafana HTTP API
// - GET/DELETE dashboard via Grafana HTTP API
// Alertmanager Server validations
// - Verify service endpoint connectivity
// - Verify Prometheus has an Alertmanager defined
// Elasticsearch Server validations
// - Verify service endpoint connectivity
// - Upload new document via Elasticsearch HTTP API
// - GET/DELETE document via Elasticsearch HTTP API
// Kibana Server validations
// - Verify service endpoint connectivity
// - Upload new document via Elasticsearch HTTP API
// - search for document via Kibana HTTP API
// - GET/DELETE entire index via Elasticsearch HTTP API
// ********************************************************

func TestBasic1Sauron(t *testing.T) {
	f := framework.Global
	var sauronName string

	// Create secrets - domain only used with ingress
	secretName := f.RunID + "-sauron-secrets"
	testDomain := "ingress-test.example.com"
	secret, err := createTestSecrets(secretName, testDomain)
	if err != nil {
		t.Errorf("failed to create test secrets: %+v", err)
	}

	// Create sauron
	if f.Ingress {
		sauronName = f.RunID + "-ingress"
	} else {
		sauronName = f.RunID + "-sauron-basic"
	}
	sauron := testutil.NewSauron(sauronName, secretName)
	if f.Ingress {
		sauron.Spec.URI = testDomain
	}

	if testutil.RunBeforePhase(f) {
		// Create Sauron instance
		sauron, err = testutil.CreateSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret)
		if err != nil {
			t.Fatalf("Failed to create Sauron: %v", err)
		}
	} else {
		sauron, err = testutil.GetSauron(f.CRClient, f.Namespace, sauron)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete Sauron instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret); err != nil {
				t.Fatalf("Failed to clean up Sauron: %v", err)
			}
		}()
	}
	verifySauronDeployment(t, sauron)
}

func TestBasic2SauronWithDataVolumes(t *testing.T) {
	f := framework.Global

	// Create Secrets - domain only used with ingress
	secretName := f.RunID + "-sauron-secrets"
	testDomain := "ingress-test.example.com"
	secret, err := createTestSecrets(secretName, testDomain)
	if err != nil {
		t.Errorf("failed to create test secrets: %+v", err)
	}

	// Create Sauron
	sauron := testutil.NewSauron(f.RunID+"-sauron-data", secretName)
	sauron.Spec.Elasticsearch.Storage = vmcontrollerv1.Storage{Size: "50Gi"}
	sauron.Spec.Prometheus.Storage = vmcontrollerv1.Storage{Size: "50Gi"}
	sauron.Spec.Grafana.Storage = vmcontrollerv1.Storage{Size: "50Gi"}
	sauron.Spec.AlertManager.Replicas = 3
	sauron.Spec.Api.Replicas = 2
	if f.Ingress {
		sauron.Spec.URI = testDomain
	}

	if testutil.RunBeforePhase(f) {
		// Create Sauron instance
		sauron, err = testutil.CreateSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret)
		if err != nil {
			t.Fatalf("Failed to create Sauron: %v", err)
		}
	} else {
		sauron, err = testutil.GetSauron(f.CRClient, f.Namespace, sauron)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete Sauron instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret); err != nil {
				t.Fatalf("Failed to clean up Sauron: %v", err)
			}
		}()
	}

	verifySauronDeployment(t, sauron)
}

func TestBasic3GrafanaOnlySauronAPITokenOperations(t *testing.T) {
	f := framework.Global
	secretName := f.RunID + "-sauron-secrets"
	secret := testutil.NewSecret(secretName, f.Namespace)
	sauron := testutil.NewGrafanaOnlySauron(f.RunID+"-"+"grafana-only", secretName)
	var err error
	if testutil.RunBeforePhase(f) {
		// Create Sauron instance
		sauron, err = testutil.CreateSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret)
		if err != nil {
			t.Fatalf("Failed to create Sauron: %v", err)
		}
	} else {
		sauron, err = testutil.GetSauron(f.CRClient, f.Namespace, sauron)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete Sauron instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret); err != nil {
				t.Fatalf("Failed to clean up Sauron: %v", err)
			}
		}()
	}
	verifyGrafanaAPITokenOperations(t, sauron)
}

// Creates Simple sauron without canaries definied
// Use API server REST apis to create/update/delete canaries
func TestBasic4SauronMultiUserAuthn(t *testing.T) {
	f := framework.Global
	var err error
	testDomain := "multiuser-authn.example.com"

	hosts := "*." + testDomain + ",api." + testDomain + ",grafana." + testDomain + ",prometheus." + testDomain + ",prometheus-gw." +
		testDomain + ",kibana." + testDomain + ",elasticsearch." + testDomain + "," + f.ExternalIP

	err = testutil.GenerateKeys(hosts, testDomain, "", 365*24*time.Hour, true, 2048, "P256")
	if err != nil {
		t.Fatalf("error generating keys: %+v", err)
	}
	tCert, err := ioutil.ReadFile(os.TempDir() + "/tls.crt")
	if err != nil {
		fmt.Print(err)
	}
	tKey, err := ioutil.ReadFile(os.TempDir() + "/tls.key")
	if err != nil {
		fmt.Print(err)
	}

	secretName := f.RunID + "-sauron-secrets"
	extraCreds := []string{reporterUsername, reporterPassword}
	secret := testutil.NewSecretWithTLSWithMultiUser(secretName, f.Namespace, tCert, tKey, username, password, extraCreds)

	// Create Sauron
	sauron := testutil.NewSauron(f.RunID+"-"+"multiuser-authn", secretName)
	sauron.Spec.URI = testDomain

	if testutil.RunBeforePhase(f) {
		// Create Sauron instance
		sauron, err = testutil.CreateSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret)
		if err != nil {
			t.Fatalf("Failed to create Sauron: %v", err)
		}
	} else {
		sauron, err = testutil.GetSauron(f.CRClient, f.Namespace, sauron)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete Sauron instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret); err != nil {
				t.Fatalf("Failed to clean up Sauron: %v", err)
			}
		}()
	}
	verifyMultiUserAuthnOperations(t, sauron)
}

func TestBasic4SauronWithIngress(t *testing.T) {
	f := framework.Global

	testDomain := "ingress-test.example.com"
	hosts := "*." + testDomain + ",api." + testDomain + ",grafana." + testDomain + ",prometheus." + testDomain + ",prometheus-gw." +
		testDomain + ",kibana." + testDomain + ",elasticsearch." + testDomain + "," + f.ExternalIP

	err := testutil.GenerateKeys(hosts, testDomain, "", 365*24*time.Hour, true, 2048, "P256")
	if err != nil {
		t.Fatalf("error generating keys: %+v", err)
	}
	tCert, err := ioutil.ReadFile(os.TempDir() + "/tls.crt")
	if err != nil {
		fmt.Print(err)
	}
	tKey, err := ioutil.ReadFile(os.TempDir() + "/tls.key")
	if err != nil {
		fmt.Print(err)
	}

	secretName := f.RunID + "-sauron-secrets"
	secret := testutil.NewSecretWithTLS(secretName, f.Namespace, tCert, tKey, username, password)

	// Create Sauron
	sauron := testutil.NewSauron(f.RunID+"-ingress", secretName)
	sauron.Spec.URI = testDomain

	if testutil.RunBeforePhase(f) {
		// Create Sauron instance
		sauron, err = testutil.CreateSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret)
		if err != nil {
			t.Fatalf("Failed to create Sauron: %v", err)
		}
		fmt.Printf("Ingress SauronSpec: %v", sauron)
	} else {
		sauron, err = testutil.GetSauron(f.CRClient, f.Namespace, sauron)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete Sauron instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret); err != nil {
				t.Fatalf("Failed to clean up Sauron: %v", err)
			}
		}()
	}
	verifySauronDeploymentWithIngress(t, sauron, username, password)

	//update top-level secret new username/password
	fmt.Println("Updating sauron username/paswword")
	secret, err = f.KubeClient2.CoreV1().Secrets(sauron.Namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret %s doesn't exists in namespace %s : %v", secretName, sauron.Namespace, err)
	}
	secret.Data["username"] = []byte(changeusername)
	secret.Data["password"] = []byte(changepassword)
	secret, err = f.KubeClient2.CoreV1().Secrets(sauron.Namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Error when updating a secret %s, %v", secretName, err)
	}
	verifySauronDeploymentWithIngress(t, sauron, changeusername, changepassword)
}

func TestBasic4SauronOperatorMetricsServer(t *testing.T) {
	f := framework.Global
	operatorSvcPort := getPortFromService(t, f.OperatorNamespace, "verrazzano-monitoring-operator")
	if err := testutil.WaitForEndpointAvailable("verrazzano-monitoring-operator", f.ExternalIP, operatorSvcPort, "/metrics", http.StatusOK, testutil.DefaultRetry); err != nil {
		t.Fatal(err)
	}
}

func TestBasic2PrometheusMultipleReplicas(t *testing.T) {
	f := framework.Global
	var sauronName string

	// Create secrets - domain only used with ingress
	secretName := f.RunID + "-sauron-secrets"
	testDomain := "ingress-test.example.com"
	secret, err := createTestSecrets(secretName, testDomain)
	if err != nil {
		t.Errorf("failed to create test secrets: %+v", err)
	}

	// Create sauron
	if f.Ingress {
		sauronName = f.RunID + "-ingress"
	} else {
		sauronName = f.RunID + "-prom-2x"
	}
	sauron := testutil.NewSauron(sauronName, secretName)
	sauron.Spec.Prometheus.Storage = vmcontrollerv1.Storage{Size: "50Gi"}
	sauron.Spec.Prometheus.Replicas = 3
	if f.Ingress {
		sauron.Spec.URI = testDomain
	}

	if testutil.RunBeforePhase(f) {
		// Create Sauron instance
		sauron, err = testutil.CreateSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret)
		if err != nil {
			t.Fatalf("Failed to create Sauron: %v", err)
		}
	} else {
		sauron, err = testutil.GetSauron(f.CRClient, f.Namespace, sauron)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete Sauron instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteSauron(f.CRClient, f.KubeClient2, f.Namespace, sauron, secret); err != nil {
				t.Fatalf("Failed to clean up Sauron: %v", err)
			}
		}()
	}
	// Check additional prometheus deployments
	var deploymentNamesToReplicas = map[string]int32{
		constants.SauronServiceNamePrefix + sauron.Name + "-prometheus-1": 1,
		constants.SauronServiceNamePrefix + sauron.Name + "-prometheus-2": 1,
	}
	for deploymentName := range deploymentNamesToReplicas {
		err := testutil.WaitForDeploymentAvailable(f.Namespace, deploymentName,
			deploymentNamesToReplicas[deploymentName], testutil.DefaultRetry, f.KubeClient)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Log("  ==> Additional Prometheus deployments are available")

	verifySauronDeployment(t, sauron)

	// Verify PVC Locations of prometheus deployments to ensure they are on different ADs of a region
	deploymentList, _ := f.KubeClient.AppsV1().Deployments(f.Namespace).List(context.Background(), metav1.ListOptions{})
	// Keep track of unique prometheus PVC AD information. Test expectes this should be 3, each PVC on unique AD
	var prometheusADs []string
	for _, deployment := range deploymentList.Items {
		if !(deployment.Spec.Template.Labels["app"] == fmt.Sprintf("%s-%s", sauron.Name, "prometheus")) {
			continue
		}
		skip := false
		pvcName := deployment.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName
		pvc, err := f.KubeClient.CoreV1().PersistentVolumeClaims(f.Namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		AD := pvc.Spec.Selector.MatchLabels["oci-availability-domain"]
		for _, prometheusAD := range prometheusADs {
			if prometheusAD == AD {
				skip = true
				t.Fatalf(" ==> FAIL!!! PVC %s is got assigned to %s again..", pvcName, AD)
			}
		}
		if !skip {
			prometheusADs = append(prometheusADs, AD)
		}
	}
	assert.Equal(t, 3, len(prometheusADs), "Length of prometheus ADs")
}

// Create appropriate secrets file for the test
func createTestSecrets(secretName, testDomain string) (*corev1.Secret, error) {
	f := framework.Global

	if !f.Ingress {
		// Create simple secret
		return testutil.NewSecret(secretName, f.Namespace), nil
	}

	// Create TLS Secret
	hosts := "*." + testDomain + ",api." + testDomain + ",grafana." + testDomain + ",prometheus." + testDomain + ",prometheus-gw." +
		testDomain + ",kibana." + testDomain + ",elasticsearch." + testDomain + "," + f.ExternalIP

	err := testutil.GenerateKeys(hosts, testDomain, "", 365*24*time.Hour, true, 2048, "P256")
	if err != nil {
		return nil, err
	}
	tCert, err := ioutil.ReadFile(os.TempDir() + "/tls.crt")
	if err != nil {
		return nil, err
	}
	tKey, err := ioutil.ReadFile(os.TempDir() + "/tls.key")
	if err != nil {
		return nil, err
	}

	return testutil.NewSecretWithTLS(secretName, f.Namespace, tCert, tKey, username, password), nil
}

func verifyMultiUserAuthnOperations(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	var httpProtocol, myURL, host, body string
	var promPort, promGWPort, apiPort, esPort int32
	var resp *http.Response
	var err error
	var headers = map[string]string{}

	fmt.Println("======================================================")
	fmt.Printf("Testing Sauron %s components in namespace '%s'\n", sauron.Name, f.Namespace)
	if f.Ingress {
		fmt.Println("Mode: Testing via the Ingress Controller")
	} else {
		fmt.Println("This test can only run in ingress mode")
		return
	}

	// What port should we use?
	if f.Ingress {
		httpProtocol = "https://"
		promPort = getPortFromService(t, f.OperatorNamespace, f.IngressControllerSvcName)
		promGWPort = promPort
		apiPort = promPort
	} else {
		httpProtocol = "http://"
		promPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Prometheus.Name+"-0")
		promGWPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.PrometheusGW.Name)
		apiPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Api.Name)
		esPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.ElasticsearchIngest.Name)
	}

	// Verify service endpoint connectivity
	// Verify Prometheus availability
	waitForEndpoint(t, sauron, "Prometheus", promPort, "/-/healthy")

	// Verify Prometheus-GW availability
	waitForEndpoint(t, sauron, "Prometheus-GW", promGWPort, "/")

	// Verify API availability
	waitForEndpoint(t, sauron, "API", apiPort, "/healthcheck")
	fmt.Println("  ==> Service endpoint is available")

	//Test 1: Validate get Unauthorized for reporter user
	myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, apiPort, "/prometheus/config")
	host = "api." + sauron.Spec.URI
	resp, body, err = sendRequestWithUserPassword("GET", myURL, host, false, headers, "", reporterUsername, reporterPassword)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected response code %d from GET on %s but got %d: (%v)",
			http.StatusUnauthorized, "/prometheus/config", resp.StatusCode, body)
	}
	fmt.Println("Test 1: Validate get Unauthorized for reporter userd - PASSED")

	//Test 2: Validate that reporter user is able to send to Pushgateway
	pushGatewayJobName := strings.Replace(f.RunID, "-", "_", -1)
	pushGatewayJobURL := fmt.Sprintf("%s%s:%d/metrics/job/%s", httpProtocol, f.ExternalIP, promGWPort, pushGatewayJobName)
	metricData := pushGatewayJobName + " 3.14159\n"

	// Test 3: Validate reporter use is able to push to elasticsearch
	index := strings.ToLower("verifyElasticsearch") + f.RunID
	docPath := "/" + index + "/_doc/1"

	message := make(map[string]interface{})
	message["message"] = "verifyElasticsearch." + f.RunID

	jsonPayload, marshalErr := json.Marshal(message)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}

	myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, esPort, docPath)
	host = "elasticsearch." + sauron.Spec.URI
	headers["Content-Type"] = "application/json"
	resp, _, err = sendRequestWithUserPassword("POST", myURL, host, false, headers, string(jsonPayload), reporterUsername, reporterPassword)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected response code %d from POST but got %d: (%v)", http.StatusCreated, resp.StatusCode, resp)
	}
	fmt.Println("  ==> Document " + docPath + " created")

	// Push metric to the Push Gateway
	resp, _, err = sendRequestWithUserPassword("POST", pushGatewayJobURL, "prometheus-gw."+sauron.Spec.URI, false, headers, metricData, reporterUsername, reporterPassword)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected response code %d from POST on %s but got %d: (%v)", http.StatusAccepted, pushGatewayJobURL, resp.StatusCode, resp)
	}
	fmt.Println("Test 2: Validate that reporter user is able to send to Pushgateway - PASSED")
	fmt.Println("  ==> Metric pushed to Push Gateway")

}

func verifySauronDeployment(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	fmt.Println("======================================================")
	fmt.Printf("Testing Sauron %s components in namespace '%s'\n", sauron.Name, f.Namespace)
	if f.Ingress {
		fmt.Println("Mode: Testing via the Ingress Controller")
	} else {
		fmt.Println("Mode: Testing via NodePorts")
	}
	fmt.Println("======================================================")

	// Verify deployments
	fmt.Println("Step 1: Verify Sauron instance deployments")

	// Verify deployments
	var deploymentNamesToReplicas = map[string]int32{
		constants.SauronServiceNamePrefix + sauron.Name + "-" + config.Api.Name:                 sauron.Spec.Api.Replicas,
		constants.SauronServiceNamePrefix + sauron.Name + "-" + config.Grafana.Name:             1,
		constants.SauronServiceNamePrefix + sauron.Name + "-" + config.PrometheusGW.Name:        1,
		constants.SauronServiceNamePrefix + sauron.Name + "-" + config.AlertManager.Name:        sauron.Spec.AlertManager.Replicas,
		constants.SauronServiceNamePrefix + sauron.Name + "-" + config.Kibana.Name:              sauron.Spec.Kibana.Replicas,
		constants.SauronServiceNamePrefix + sauron.Name + "-" + config.ElasticsearchIngest.Name: sauron.Spec.Elasticsearch.IngestNode.Replicas,
		constants.SauronServiceNamePrefix + sauron.Name + "-" + config.ElasticsearchMaster.Name: sauron.Spec.Elasticsearch.MasterNode.Replicas,
	}
	for i := 0; i < int(sauron.Spec.Prometheus.Replicas); i++ {
		deploymentNamesToReplicas[constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Prometheus.Name+"-"+strconv.Itoa(i)] = 1
	}
	for i := 0; i < int(sauron.Spec.Elasticsearch.DataNode.Replicas); i++ {
		deploymentNamesToReplicas[constants.SauronServiceNamePrefix+sauron.Name+"-"+config.ElasticsearchData.Name+"-"+strconv.Itoa(i)] = 1
	}

	statefulSetComponents := []string{constants.SauronServiceNamePrefix + sauron.Name + "-" + config.AlertManager.Name}

	statefulSetComponents = append(statefulSetComponents, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.ElasticsearchMaster.Name)
	for deploymentName := range deploymentNamesToReplicas {
		var err error
		if resources.SliceContains(statefulSetComponents, deploymentName) {
			err = testutil.WaitForStatefulSetAvailable(f.Namespace, deploymentName,
				deploymentNamesToReplicas[deploymentName], testutil.DefaultRetry, f.KubeClient)
		} else {
			err = testutil.WaitForDeploymentAvailable(f.Namespace, deploymentName,
				deploymentNamesToReplicas[deploymentName], testutil.DefaultRetry, f.KubeClient)
		}

		if err != nil {
			t.Fatal(err)
		}
	}
	fmt.Println("  ==> All deployments are available")

	// Run the tests
	fmt.Println("Step 2: Verify API service")

	verifyAPI(t, sauron)
	fmt.Println("Step 3: Verify Prometheus")
	verifyPrometheus(t, sauron)
	fmt.Println("Step 4: Verify Grafana")
	verifyGrafana(t, sauron)
	fmt.Println("Step 5: Verify Alertmanager")
	verifyAlertmanager(t, sauron)
	fmt.Println("Step 6: Verify Elasticsearch")
	verifyElasticsearch(t, sauron, true, false)
	fmt.Println("Step 7: Verify Kibana")
	verifyKibana(t, sauron)

	fmt.Println("======================================================")
}

func verifyAPI(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	var apiPort, promPort int32

	// What ports should we use?
	if f.Ingress {
		apiPort = getPortFromService(t, f.OperatorNamespace, f.IngressControllerSvcName)
		promPort = apiPort
	} else {
		apiPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Api.Name)
		promPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Prometheus.Name)
	}

	// Wait for API availability
	waitForEndpoint(t, sauron, "API", apiPort, "/healthcheck")

	// Wait for Prometheus availability
	waitForEndpoint(t, sauron, "Prometheus", promPort, "/-/healthy")
	fmt.Println("  ==> Service endpoint is available")
}

func verifyPrometheus(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	var promPort, promGWPort, apiPort int32
	var resp *http.Response
	var httpProtocol, myURL, body, host, payload string
	var err error
	var headers = map[string]string{}

	if !sauron.Spec.Prometheus.Enabled {
		return
	}

	// What port should we use?
	if f.Ingress {
		promPort = getPortFromService(t, f.OperatorNamespace, f.IngressControllerSvcName)
		promGWPort = promPort
		apiPort = promPort
		httpProtocol = "https://"
	} else {
		promPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Prometheus.Name)
		promGWPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.PrometheusGW.Name)
		apiPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Api.Name)
		httpProtocol = "http://"
	}

	// Verify Prometheus availability
	waitForEndpoint(t, sauron, "Prometheus", promPort, "/-/healthy")

	// Verify Prometheus-GW availability
	waitForEndpoint(t, sauron, "Prometheus-GW", promGWPort, "/")

	// Verify API availability
	waitForEndpoint(t, sauron, "API", apiPort, "/healthcheck")
	fmt.Println("  ==> Service endpoint is available")

	// Add local Prometheus /metrics
	var prometheusConfig = `
 - job_name: '` + f.RunID + `'` + `
   static_configs:
   - targets: ["` + "localhost:9090" + `"]`

	pushGatewayJobName := strings.Replace(f.RunID, "-", "_", -1)
	pushGatewayJobURL := fmt.Sprintf("%s%s:%d/metrics/job/%s", httpProtocol, f.ExternalIP, promGWPort, pushGatewayJobName)
	metricName := pushGatewayJobName
	metricData := pushGatewayJobName + " 3.14159\n"
	prometheusURL := fmt.Sprintf("%s%s:%d", httpProtocol, f.ExternalIP, promPort)

	if testutil.RunBeforePhase(f) {

		// GET - /prometheus/config
		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, apiPort, "/prometheus/config")
		host = "api." + sauron.Spec.URI
		resp, body, err = sendRequest("GET", myURL, host, false, headers, "")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected response code %d from GET on %s but got %d: (%v)", http.StatusOK, "/prometheus/config", resp.StatusCode, resp)
		}

		// PUT -- /prometheus/config
		payload = body + prometheusConfig
		resp, _, err = sendRequest("PUT", myURL, host, false, headers, payload)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			t.Fatalf("Expected response code %d or %d from PUT on %s but got %d: (%v)", http.StatusOK, http.StatusAccepted, myURL, resp.StatusCode, resp)
		}
		fmt.Println("  ==> Prometheus config updated")
		fmt.Println("  ==> Local Prometheus scrape target defined")

		// Verify Push Gateway scrape target is present in Prometheus
		err = waitForKeyWithValueResponse("prometheus."+sauron.Spec.URI, f.ExternalIP, promPort, "/api/v1/targets", "job", "PushGateway")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("  ==> Push Gateway target defined ")

		// Push metric to the Push Gateway
		resp, _, err = sendRequest("POST", pushGatewayJobURL, "prometheus-gw."+sauron.Spec.URI, false, headers, metricData)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			t.Fatalf("Expected response code %d or %d from POST on %s but got %d: (%v)", http.StatusOK, http.StatusAccepted, pushGatewayJobURL, resp.StatusCode, resp)
		}
		fmt.Println("  ==> Metric pushed to Push Gateway")

		// Wait for metric to appear in Prometheus
		err = waitForPrometheusMetric(sauron.Spec.URI, prometheusURL, metricName)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("  ==> Metric propagated to Prometheus")

		// We delete the metric from GW here to ensure that by the time we are verifying the metric after
		// upgrade, we don't have any _new_ data coming in from the GW.  We want to be verifying old data at that point.

		resp, _, err = sendRequest("DELETE", pushGatewayJobURL, "prometheus-gw."+sauron.Spec.URI, false, headers, metricData)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("Expected response code %d from DELETE on %s but got %d: (%v)", http.StatusAccepted, pushGatewayJobURL, resp.StatusCode, resp)
		}
		fmt.Println("  ==> Metric deleted from Push Gateway")
	}

	if testutil.RunAfterPhase(f) {
		// Verify Prometheus config using Sauron API Server

		myURL := fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, apiPort, "/prometheus/config")
		resp, body, _ = sendRequest("GET", myURL, "api."+sauron.Spec.URI, false, headers, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected response code %d from GET on %s but got %d: (%v)", http.StatusOK, "/prometheus/config", resp.StatusCode, resp)
		}
		if !bytes.Contains([]byte(body), []byte(prometheusConfig)) {
			t.Fatalf("Expected response body %s but got %s", prometheusConfig, body)
		}

		// Verify local scrape target was added by the API server
		err = waitForKeyWithValueResponse("prometheus."+sauron.Spec.URI, f.ExternalIP, promPort, "/api/v1/targets", "job", f.RunID)
		if err != nil {
			t.Fatal(err)
		}

		err = waitForPrometheusMetric(sauron.Spec.URI, prometheusURL, metricName)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("  ==> Historical metric defined in Prometheus")
	}
}

func verifyGrafanaAPITokenOperations(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global

	if !sauron.Spec.Grafana.Enabled {
		return
	}

	grafanaSvc, err := testutil.WaitForService(f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-grafana", testutil.DefaultRetry, f.KubeClient)
	if err != nil {
		t.Fatal(err)
	}

	// Verify service endpoint connectivity
	if err := testutil.WaitForEndpointAvailable("Grafana", f.ExternalIP, grafanaSvc.Spec.Ports[0].NodePort, "/api/health", http.StatusOK, testutil.DefaultRetry); err != nil {
		t.Fatal(err)
	}

	fmt.Println("  ==> service endpoint available")

	if testutil.RunBeforePhase(f) {
		// Test1 - Accessing the grafana endpoint root URL using NGINX basic auth should return 200.
		//         This validates backward compatability, Users can continue to use Sauron basic auth
		fmt.Println("Dashboard API Token tests")
		resp, err := resty.R().SetBasicAuth(username, password).
			Get(fmt.Sprintf("http://%s:%d%s",
				f.ExternalIP, grafanaSvc.Spec.Ports[0].NodePort, "/"))
		if err != nil {
			t.Fatalf("Failed to get root URL for grafana endpoint %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("Expected response code %d from GET on %s but got %d: (%v)",
				http.StatusOK, "/", resp.StatusCode(), resp)
		}
		fmt.Println("Test1 - Accessing the grafana endpoint root URL using NGINX basic auth should return 200 - Passed")

		// Test2: Get a Admin API Token via Grafana HTTP API
		resp2, err2 := resty.R().SetHeader("Content-Type", "application/json").
			SetBody(`{"name":"adminTestApiKey", "role": "Admin"}`).SetBasicAuth(username, password).
			Post(fmt.Sprintf("http://%s:%d%s",
				f.ExternalIP, grafanaSvc.Spec.Ports[0].NodePort, "/api/auth/keys"))
		if err2 != nil {
			t.Fatal(err2)
		}
		if resp2.StatusCode() != http.StatusOK {
			t.Fatalf("Expected response code %d from POST on %s but got %d: (%v)",
				http.StatusOK, "/api/auth/keys", resp2.StatusCode(), resp2)
		}
		var jsonResp map[string]string
		if unmarshalErr := json.Unmarshal(resp2.Body(), &jsonResp); unmarshalErr != nil {
			t.Fatal(unmarshalErr)
		}
		var adminToken = jsonResp["key"]
		fmt.Println("Dashboard API admin token: " + adminToken)

		if adminToken == "" {
			t.Fatalf("Did not get a admin API token from grafan")
		}
		fmt.Println("Test2: Get a Admin API Token via Grafana HTTP API - PASSED")

		// Test3: Get a View API Token via Grafana HTTP API
		var jsonResp3 map[string]string
		resp3, err3 := resty.R().SetHeader("Content-Type", "application/json").
			SetBody(`{"name":"viewTestApiKey", "role": "Viewer"}`).SetBasicAuth(username, password).
			Post(fmt.Sprintf("http://%s:%d%s",
				f.ExternalIP, grafanaSvc.Spec.Ports[0].NodePort, "/api/auth/keys"))
		if err3 != nil {
			t.Fatal(err3)
		}
		if resp3.StatusCode() != http.StatusOK {
			t.Fatalf("Expected response code %d from POST on %s but got %d: (%v)",
				http.StatusOK, "/api/auth/keys", resp3.StatusCode(), resp3)
		}

		if unmarshalErr := json.Unmarshal(resp3.Body(), &jsonResp3); unmarshalErr != nil {
			t.Fatal(unmarshalErr)
		}
		var viewAPIToken = jsonResp3["key"]
		fmt.Println("Dashboard API view token: " + viewAPIToken)

		if viewAPIToken == "" {
			t.Fatalf("Did not get a view API token from grafana")
		}
		fmt.Println("Test3: Get a View API Token via Grafana HTTP API - PASSED")

		//Test4: Validate that any Grafana API call fails if No Valid API Token is passed
		resp4, err4 := resty.R().
			Get(fmt.Sprintf("http://%s:%d%s",
				f.ExternalIP, grafanaSvc.Spec.Ports[0].NodePort, "/api/org"))
		if err4 != nil {
			t.Fatal(err4)
		}
		if resp4.StatusCode() != http.StatusUnauthorized {
			t.Fatalf("Expected response code %d from GET on %s but got %d: (%v)",
				http.StatusUnauthorized, "/api/auth/keys", resp4.StatusCode(), resp4)
		}
		fmt.Println("Test4: Validate that any Grafana API call fails if No Valid API Token is passed - PASSED")

		//Test5: Validate that View Only operation PASSES with a Viewer role API Token.
		resp5, err5 := resty.R().SetHeader("Authorization", "Bearer "+viewAPIToken).
			Get(fmt.Sprintf("http://%s:%d%s",
				f.ExternalIP, grafanaSvc.Spec.Ports[0].NodePort, "/api/org"))
		if err5 != nil {
			t.Fatal(err5)
		}
		if resp5.StatusCode() != http.StatusOK {
			t.Fatalf("Expected response code %d from POST on %s but got %d: (%v)",
				http.StatusOK, "/api/auth/keys", resp5.StatusCode(), resp5)
		}
		fmt.Printf("Test5: Validate that View Only operation PASSES with a Viewer role API Token. (%v) - PASSED\n", resp5)

		//Test6: Validate that Admin Operation FAILS with a Viewer Role API Token
		var jsonResp6 map[string]string
		resp6, err6 := resty.R().SetHeader("Content-Type", "application/json").
			SetHeader("Authorization", "Bearer "+viewAPIToken).SetBody(`{"name":"Main org."}`).
			Put(fmt.Sprintf("http://%s:%d%s",
				f.ExternalIP, grafanaSvc.Spec.Ports[0].NodePort, "/api/org"))
		if err6 != nil {
			t.Fatal(err2)
		}
		if resp6.StatusCode() != http.StatusForbidden {
			t.Fatalf("Expected response code %d from POST on %s but got %d: (%v)",
				http.StatusForbidden, "/api/auth/keys", resp6.StatusCode(), resp6)
		}

		if unmarshalErr := json.Unmarshal(resp6.Body(), &jsonResp6); unmarshalErr != nil {
			t.Fatal(unmarshalErr)
		}
		var message = jsonResp6["message"]
		fmt.Println("Admin operation with View Only Token message: " + message)

		if message != "Permission denied" {
			t.Fatalf("Did not get Permission Denied message on Admin Operation with Viewer API Token, got (%v)", resp6)
		}
		fmt.Println("Test6: Validate that Admin Operation FAILS with a Viewer Role API Token - PASSED")

		//Test7: Validate that Admin Operation PASSES with ONLY a Admin Role API Token
		var jsonResp7 map[string]string
		resp7, err7 := resty.R().SetHeader("Content-Type", "application/json").
			SetHeader("Authorization", "Bearer "+adminToken).SetBody(`{"name":"Main org."}`).
			Put(fmt.Sprintf("http://%s:%d%s",
				f.ExternalIP, grafanaSvc.Spec.Ports[0].NodePort, "/api/org"))
		if err7 != nil {
			t.Fatal(err7)
		}
		if resp7.StatusCode() != http.StatusOK {
			t.Fatalf("Expected response code %d from POST on %s but got %d: (%v)",
				http.StatusOK, "/api/auth/keys", resp7.StatusCode(), resp7)
		}

		if unmarshalErr := json.Unmarshal(resp7.Body(), &jsonResp7); unmarshalErr != nil {
			t.Fatal(unmarshalErr)
		}
		message = jsonResp7["message"]
		fmt.Println("Admin operation with Admin role Token message: " + message)

		if message != "Organization updated" {
			t.Fatalf("Did not get Organization updated message on Admin Operation with Admin API Token")
		}
		fmt.Println("Test7: Validate that Admin Operation PASSES with a Admin Role API Token - PASSED")

	}
	if testutil.RunAfterPhase(f) {
		if !testutil.SkipTeardown(f) {
			//Delete The API token keys created.
			//Test 8: Delete the API Viewer Token
			resp8, err8 := resty.R().SetBasicAuth(username, password).
				Delete(fmt.Sprintf("http://%s:%d%s",
					f.ExternalIP, grafanaSvc.Spec.Ports[0].NodePort, "/api/auth/keys/1"))
			if err8 != nil {
				t.Fatal(err8)
			}
			if resp8.StatusCode() != http.StatusOK {
				t.Fatalf("Expected response code %d from POST on %s but got %d: (%v)",
					http.StatusOK, "/api/auth/keys", resp8.StatusCode(), resp8)
			}
			fmt.Println("Test8: Delete the API Viewer Token - PASSED")

			//Test 9: Delete API Admin Token
			resp9, err9 := resty.R().SetBasicAuth(username, password).
				Delete(fmt.Sprintf("http://%s:%d%s",
					f.ExternalIP, grafanaSvc.Spec.Ports[0].NodePort, "/api/auth/keys/1"))
			if err9 != nil {
				t.Fatal(err9)
			}
			if resp9.StatusCode() != http.StatusOK {
				t.Fatalf("Expected response code %d from POST on %s but got %d: (%v)",
					http.StatusOK, "/api/auth/keys", resp9.StatusCode(), resp9)
			}
			fmt.Println("Test 9: Delete API Admin Token - PASSED")

		}
	}
}

func verifyGrafana(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	var port int32
	var httpProtocol, myURL, host, body string
	var resp *http.Response
	var err error
	var headers = map[string]string{}

	externalDomainName := "localhost"
	if !sauron.Spec.Grafana.Enabled {
		return
	}

	// What port should we use?
	if f.Ingress {
		port = getPortFromService(t, f.OperatorNamespace, f.IngressControllerSvcName)
		httpProtocol = "https://"
		externalDomainName = "grafana." + sauron.Spec.URI
	} else {
		port = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Grafana.Name)
		httpProtocol = "http://"
	}

	// Verify Grafana availability
	waitForEndpoint(t, sauron, "Grafana", port, "/api/health")
	fmt.Println("  ==> Service endpoint is available")

	/* Verify domain and root_url */
	host = "grafana." + sauron.Spec.URI
	myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, port, "/api/admin/settings")
	resp, body, err = sendRequest("GET", myURL, host, true, headers, "")

	fmt.Println("Checking for domain and root_url in Grafana config")
	if err != nil {
		t.Fatalf("Failed to retrieve Grafana settings %s %v", f.RunID, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected response code %d from GET on %s but got %d: (%v)",
			http.StatusOK, "/api/admin/settings", resp.StatusCode, resp)
	}

	var adminRes map[string]interface{}
	if err := json.Unmarshal([]byte(body), &adminRes); err != nil {
		t.Fatalf("Expected Grafana config in output but found a different result:\n %s", body)
	}

	if len(adminRes) == 0 {
		t.Fatal("Expected a result in response but found none")
	}

	if _, ok := adminRes["server"]; !ok {
		t.Fatalf("Expected \"server\" element in result but found none\n")
	}

	grafanaServerConfig := adminRes["server"].(map[string]interface{})
	if domain, ok := grafanaServerConfig["domain"]; !ok {
		t.Fatalf("Expected 'domain' element in result but found none\n")
	} else if domain != externalDomainName {
		t.Fatalf("Actual domain value '%s' doesn't match expected value '%s'\n", domain, externalDomainName)
	} else {
		fmt.Printf("'domain' obtained from Grafana config is = %+v\n", domain)
	}
	if externalDomainName != "localhost" {
		if rootUrl, ok := grafanaServerConfig["root_url"]; !ok {
			t.Fatalf("Expected 'root_url' element in result but found none\n")
		} else if rootUrl != "https://"+externalDomainName {
			t.Fatalf("Actual root_url value '%s' doesn't match expected value '%s'\n", rootUrl, "https://"+externalDomainName)
		} else {
			fmt.Printf("'root_url' obtained from Grafana config is = %+v\n", rootUrl)
		}
	}

	/** End verify domain and root_url **/

	dashboardConfig, err := readTestDashboardConfig(f.RunID)
	if err != nil {
		t.Fatal(err)
	}

	if testutil.RunBeforePhase(f) {

		// Verify "sauron" user can retrieve the user list
		// This is a good check that authentication is being passedi properly with ingress
		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, port, "/api/users")
		host = "grafana." + sauron.Spec.URI
		resp, _, err = sendRequest("GET", myURL, host, true, headers, "")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected response code %d from GET on %s but got %d: (%v)",
				http.StatusOK, "/api/users", resp.StatusCode, resp)
		}
		fmt.Println("  ==> List of users received")

		// Upload a new dashboard via Grafana HTTP API
		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, port, "/api/dashboards/db")
		host = "grafana." + sauron.Spec.URI
		headers["Content-Type"] = "application/json"
		resp, _, err = sendRequest("POST", myURL, host, true, headers, dashboardConfig)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected response code %d from POST on %s but got %d: (%v)",
				http.StatusOK, "/api/dashboards/db", resp.StatusCode, resp)
		}
		fmt.Println("  ==> Dashboard " + f.RunID + " created")
	}
	if testutil.RunAfterPhase(f) {
		// GET dashboard via Grafana HTTP API
		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, port, "/api/search?type=dash-db&query="+f.RunID)
		host = "grafana." + sauron.Spec.URI
		fmt.Printf("URL: %s\n", myURL)
		resp, body, err = sendRequest("GET", myURL, host, true, headers, "")

		if err != nil {
			t.Fatalf("Failed to retrieve dashboard %s %v", f.RunID, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected response code %d from GET on %s but got %d: (%v)",
				http.StatusOK, "/api/search?type=dash-db&query="+f.RunID, resp.StatusCode, resp)
		}

		var res []map[string]interface{}
		if err := json.Unmarshal([]byte(body), &res); err != nil {
			t.Fatalf("Expected a list of dashboards but found a different result:\n %s", body)
		}

		if len(res) == 0 {
			t.Fatal("Expected a result in response but found none")
		}

		dashUID := ""
		for _, dash := range res {
			if dash["title"] == f.RunID {
				dashUID = dash["uid"].(string)
				break
			}
		}

		if dashUID == "" {
			t.Fatalf("Expected a dashboard with title %s but found none\n", f.RunID)
		}
		fmt.Println("  ==> Dashboard " + f.RunID + " retrieved")

		// DELETE dashboard via Grafana HTTP API if we are tearing down
		if !testutil.SkipTeardown(f) {
			myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, port, "/api/dashboards/uid/"+dashUID)
			host = "grafana." + sauron.Spec.URI
			fmt.Printf("URL: %s\n", myURL)
			resp, _, err = sendRequest("DELETE", myURL, host, true, headers, "")
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
				t.Fatalf("Expected response code %d or %d from DELETE on %s but got %d: (%v)", http.StatusOK, http.StatusAccepted, "/api/dashboards/uid/"+dashUID, resp.StatusCode, resp)
			}
			fmt.Println("  ==> Dashboard " + f.RunID + " deleted")
		}
	}
}

func verifyAlertmanager(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global

	if !sauron.Spec.AlertManager.Enabled {
		return
	}

	var alrtMgrConfig = `
route:
  receiver: ` + f.RunID + `
  group_by: ['alertname']
  group_wait: 1s
  group_interval: 1m
  repeat_interval: 1h
receivers:
- name: ` + f.RunID + `
  pagerduty_configs:
  - service_key: some-service-key
`

	var alrtDef = `
groups:
- name: ` + f.RunID + `
  rules:
  - alert: TargetUP
    expr: sum(up) > 0
    labels:
      severity: page
    annotations:
      summary: integ test run ` + f.RunID + ` target up
`

	var alertPort, apiPort, promPort int32
	var httpProtocol, myURL, host, payload, body string
	var resp *http.Response
	var err error
	var headers = map[string]string{}

	// What ports should we use?
	if f.Ingress {
		alertPort = getPortFromService(t, f.OperatorNamespace, f.IngressControllerSvcName)
		apiPort = alertPort
		promPort = alertPort
		httpProtocol = "https://"
	} else {
		alertPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.AlertManager.Name)
		apiPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Api.Name)
		promPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Prometheus.Name)
		httpProtocol = "http://"
	}

	//Verify service endpoint connectivity
	waitForEndpoint(t, sauron, "AlertManager", alertPort, "/#/status")
	fmt.Println("  ==> Service endpoint is available")

	// Get alertmanagers
	myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, promPort, "/api/v1/alertmanagers")
	host = "prometheus." + sauron.Spec.URI
	_, body, err = sendRequest("GET", myURL, host, false, headers, "")
	if err != nil {
		t.Fatalf("Failed to GET /api/v1/alertmanagers %v", err)
	}

	// Verify cluster state
	fmt.Printf("  ==> Verifying AlertManager cluster size reaches %d\n", sauron.Spec.AlertManager.Replicas)
	err = waitForStringInResponse("alertmanager."+sauron.Spec.URI, f.ExternalIP, alertPort, "/metrics", fmt.Sprintf("alertmanager_cluster_members %d", sauron.Spec.AlertManager.Replicas))
	if err != nil {
		t.Fatal(err)
	}

	// Get Alertmanager service - needed for target port
	alertMgrSvc, err := testutil.WaitForService(f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-alertmanager", testutil.DefaultRetry, f.KubeClient)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(body, alertMgrSvc.Spec.Ports[0].TargetPort.StrVal) {
		t.Fatalf("response body %s does not contain expected alertmanager", body)
	}
	fmt.Println("  ==> Alertmanager defined")

	if testutil.RunBeforePhase(f) {

		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, apiPort, "/alertmanager/config")
		host = "api." + sauron.Spec.URI
		resp, _, err = sendRequest("PUT", myURL, host, false, headers, alrtMgrConfig)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			t.Fatalf("Expected response code %d or %d from PUT on %s but got %d: (%v)", http.StatusOK, http.StatusAccepted, "/alertmanager/config", resp.StatusCode, resp)
		}
		fmt.Println("  ==> Alertmanager config set via API")

		//api server is causing trouble for immediate PUT requests, wait for api server url is ready before attempting to send another PUT request
		waitForEndpoint(t, sauron, "API", apiPort, "/prometheus/rules")

		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, apiPort, "/prometheus/rules/"+f.RunID+".rules")
		host = "api." + sauron.Spec.URI
		resp, _, err = sendRequest("PUT", myURL, host, false, headers, alrtDef)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			t.Fatalf("Expected response code %d or %d from PUT on %s but got %d: (%v)", http.StatusOK, http.StatusAccepted, "/prometheus/rules/"+f.RunID+".rules", resp.StatusCode, resp)
		}
		fmt.Println("  ==> Alert definition set via API")
	}
	if testutil.RunAfterPhase(f) {
		// Verify alert manager config
		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, apiPort, "/alertmanager/config")
		host = "api." + sauron.Spec.URI
		resp, body, _ = sendRequest("GET", myURL, host, false, headers, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected response code %d from GET on %s but got %d: (%v)", http.StatusOK, "/alertmanager/config", resp.StatusCode, resp)
		}
		if !strings.Contains(body, alrtMgrConfig) {
			t.Fatalf("Expected response body %s but got %s", alrtMgrConfig, body)
		}

		// Verify alert definition
		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, apiPort, "/prometheus/rules/"+f.RunID+".rules")
		host = "api." + sauron.Spec.URI
		resp, body, _ = sendRequest("GET", myURL, host, false, headers, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected response code %d from GET on %s but got %d: (%v)", http.StatusOK, "/prometheus/rules/"+f.RunID+".rules", resp.StatusCode, resp)
		}
		if !strings.Contains(body, alrtDef) {
			t.Fatalf("Expected response body %s but got %s", alrtDef, body)
		}

		// Wait for alert to trigger
		err = waitForKeyWithValueResponse("alertmanager."+sauron.Spec.URI, f.ExternalIP, alertPort, "/api/v1/alerts", "receivers", "["+f.RunID+"]")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("  ==> Test alert firing")

		// Clear Alertmanager config via API if we are tearing down
		if !testutil.SkipTeardown(f) {
			// Delete alertrule
			myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, apiPort, "/prometheus/rules/"+f.RunID+".rules")
			host = "api." + sauron.Spec.URI
			resp, _, err = sendRequest("DELETE", myURL, host, false, headers, alrtDef)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
				t.Fatalf("Expected response code %d or %d from DELETE on %s but got %d: (%v)", http.StatusOK, http.StatusAccepted, "/prometheus/rules/"+f.RunID+".rules", resp.StatusCode, resp)
			}
			fmt.Println("  ==> Alert definition unset via API")

			myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, apiPort, "/alertmanager/config")
			host = "api." + sauron.Spec.URI
			payload = `route:
   receiver: Test
receivers:
   - name: Test
`
			resp, _, err = sendRequest("PUT", myURL, host, false, headers, payload)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
				t.Fatalf("Expected response code %d or %d from PUT on %s but got %d: (%v)", http.StatusOK, http.StatusAccepted, "/alertmanager/config", resp.StatusCode, resp)
			}
		}
	}
}

func verifyElasticsearch(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, verifyIngestPipeline bool, verifySauronIndex bool) {
	f := framework.Global

	var httpProtocol, myURL, host string
	var esPort int32
	var resp *http.Response
	var err error
	var headers = map[string]string{}

	if !sauron.Spec.Elasticsearch.Enabled {
		return
	}

	index := strings.ToLower("verifyElasticsearch") + f.RunID
	docPath := "/" + index + "/_doc/"
	countPath := "/_cat/count/" + index
	ingestPipelinePath := "/_ingest/pipeline/" + f.RunID

	// What port should we use?
	if f.Ingress {
		esPort = getPortFromService(t, f.OperatorNamespace, f.IngressControllerSvcName)
		httpProtocol = "https://"
	} else {
		esPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.ElasticsearchIngest.Name)
		httpProtocol = "http://"
	}
	host = "elasticsearch." + sauron.Spec.URI

	// Verify service endpoint connectivity
	waitForEndpoint(t, sauron, "Elasticsearch", esPort, "/_cluster/health")
	waitForEndpoint(t, sauron, "Elasticsearch", esPort, "/_cat/indices")
	fmt.Println("  ==> Service endpoint is available")

	// Verify expected cluster size
	expectedClusterSize := sauron.Spec.Elasticsearch.MasterNode.Replicas + sauron.Spec.Elasticsearch.IngestNode.Replicas + sauron.Spec.Elasticsearch.DataNode.Replicas
	fmt.Printf("  ==> Verifying Elasticsearch cluster size is as expected (%d)\n", expectedClusterSize)
	err = waitForKeyWithValueResponse(host, f.ExternalIP, esPort, "/_cluster/stats", "successful", strconv.Itoa(int(expectedClusterSize)))
	if err != nil {
		t.Fatal(err)
	}

	if testutil.RunBeforePhase(f) {
		// Add a log record
		message := make(map[string]interface{})
		message["message"] = "verifyElasticsearch." + f.RunID

		jsonPayload, marshalErr := json.Marshal(message)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		for i := 1; i <= 100; i++ {

			myURL = fmt.Sprintf("%s%s:%d%s%d", httpProtocol, f.ExternalIP, esPort, docPath, i)
			headers["Content-Type"] = "application/json"
			resp, _, err = sendRequest("POST", myURL, host, false, headers, string(jsonPayload))
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("Expected response code %d from POST but got %d: (%v)", http.StatusCreated, resp.StatusCode, resp)
			}
		}
		fmt.Println("  ==> 100 Documents " + docPath + " created")

		if verifyIngestPipeline {
			// Add an ingest pipeline
			jsonPayload = []byte(`{"processors": [{"rename": {"field": "hostname","target_field": "host", "ignore_missing": true}}]}`)
			myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, esPort, ingestPipelinePath)
			resp, _, err = sendRequest("PUT", myURL, host, false, headers, string(jsonPayload))
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected response code %d from PUT but got %d: (%v)", http.StatusOK, resp.StatusCode, resp)
			}
			fmt.Println("  ==> Ingest Pipeline " + ingestPipelinePath + " created")
		}

		ElasticSearchService, err := testutil.WaitForService(f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.ElasticsearchIngest.Name, testutil.DefaultRetry, f.KubeClient)
		if err != nil {
			t.Fatal(err)
		}

		ElasticSearchIndexDocCountURL := fmt.Sprintf("http://%s:%d%s", f.ExternalIP, ElasticSearchService.Spec.Ports[0].NodePort, countPath)

		found, err := testutil.WaitForElasticSearchIndexDocCount(ElasticSearchIndexDocCountURL, "100", testutil.DefaultRetry)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Error("Docs in the index are not 100")
		}
	}

	if testutil.RunAfterPhase(f) {

		for i := 1; i <= 100; i++ {
			// Verify log record
			myURL = fmt.Sprintf("%s%s:%d%s%d", httpProtocol, f.ExternalIP, esPort, docPath, i)
			resp, _, err = sendRequest("GET", myURL, host, false, headers, "")
			if err != nil {
				t.Fatalf("Failed to retrieve document %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected response code %d from GET but got %d: (%v)", http.StatusOK, resp.StatusCode, resp)
			}
		}
		fmt.Println("  ==> 100 Documents " + docPath + " retrieved")

		//Verify Doc Count
		ElasticSearchService, err := testutil.WaitForService(f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.ElasticsearchIngest.Name, testutil.DefaultRetry, f.KubeClient)
		if err != nil {
			t.Fatal(err)
		}
		ElasticSearchIndexDocCountURL := fmt.Sprintf("http://%s:%d%s", f.ExternalIP, ElasticSearchService.Spec.Ports[0].NodePort, countPath)

		found, err := testutil.WaitForElasticSearchIndexDocCount(ElasticSearchIndexDocCountURL, "100", testutil.DefaultRetry)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Error("Docs in the restored index are not 100")
		}

		// Hack for upgrade from ES 6.x to 7.x
		if !strings.HasPrefix(f.ElasticsearchVersion, "7.") {
			if verifyIngestPipeline {
				// Verify ingest pipeline
				myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, esPort, ingestPipelinePath)
				resp, _, err = sendRequest("GET", myURL, host, false, headers, "")
				if err != nil {
					t.Fatalf("Failed to retrieve document %v", err)
				}
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("Expected response code %d from GET but got %d: (%v)", http.StatusOK, resp.StatusCode, resp)
				}
				fmt.Println("  ==> Ingest pipeline " + docPath + " retrieved")
			}
		}

		if verifySauronIndex {
			// Verify presence of .sauron index added by backup/restore process
			myURL = fmt.Sprintf("%s%s:%d/.sauron", httpProtocol, f.ExternalIP, esPort)
			resp, _, err = sendRequest("GET", myURL, host, false, headers, "")
			if err != nil {
				t.Fatalf("Failed to check for Sauron index %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected response code %d from GET but got %d: (%v)", http.StatusOK, resp.StatusCode, resp)
			}
			fmt.Println("  ==> .sauron index observed")
		}

		// DELETE message document via elasticsearch HTTP API if we are tearing down
		if !testutil.SkipTeardown(f) {
			// Delete log record
			for i := 1; i <= 100; i++ {
				myURL = fmt.Sprintf("%s%s:%d%s%d", httpProtocol, f.ExternalIP, esPort, docPath, i)
				resp, _, err = sendRequest("DELETE", myURL, host, false, headers, "")
				if err != nil {
					t.Fatal(err)
				}
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
					t.Fatalf("Expected response code %d or %d from DELETE on %s but got %d: (%v)", http.StatusOK, http.StatusAccepted, myURL, resp.StatusCode, resp)
				}
			}
			fmt.Println("  ==> 100 Documents from " + docPath + " deleted")
			if verifyIngestPipeline {
				// Delete ingest pipeline
				myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, esPort, ingestPipelinePath)
				resp, _, err = sendRequest("DELETE", myURL, host, false, headers, "")
				if err != nil {
					t.Fatal(err)
				}
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
					t.Fatalf("Expected response code %d or %d from DELETE on %s but got %d: (%v)", http.StatusOK, http.StatusAccepted, myURL, resp.StatusCode, resp)
				}
				fmt.Println("  ==> Ingest pipeline" + docPath + " deleted")
			}
		}
	}
}

func verifyKibana(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	var httpProtocol, myURL, host, body string
	var resp *http.Response
	var err error
	var kbPort, esPort int32
	var headers = map[string]string{}

	if !sauron.Spec.Kibana.Enabled {
		return
	}

	index := strings.ToLower("verifyKibana")
	docPath := "/" + index + "/doc"

	// What port should we use?
	if f.Ingress {
		kbPort = getPortFromService(t, f.OperatorNamespace, f.IngressControllerSvcName)
		esPort = kbPort
		httpProtocol = "https://"
	} else {
		kbPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.Kibana.Name)
		esPort = getPortFromService(t, f.Namespace, constants.SauronServiceNamePrefix+sauron.Name+"-"+config.ElasticsearchIngest.Name)
		httpProtocol = "http://"
	}

	// Verify service endpoint connectivity
	waitForEndpoint(t, sauron, "Kibana", kbPort, "/api/status")
	fmt.Println("  ==> Service endpoint is available")

	testMessage := "verifyKibana." + f.RunID
	if testutil.RunBeforePhase(f) {

		doc := make(map[string]interface{})
		doc["user"] = f.RunID
		doc["post_date"] = time.Now().Format(time.RFC3339)
		doc["message"] = testMessage
		jsonPayload, marshalErr := json.Marshal(doc)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}

		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, esPort, docPath)
		host = "elasticsearch." + sauron.Spec.URI
		headers["Content-Type"] = "application/json"
		resp, _, err = sendRequest("POST", myURL, host, false, headers, string(jsonPayload))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Expected response code %d from POST but got %d: (%v)", http.StatusCreated, resp.StatusCode, resp)
		}
		fmt.Println("  ==> Document " + docPath + " created")
		// Deal with possible Elasticsearch index delay on a newly created document
		time.Sleep(10 * time.Second)
	}
	if testutil.RunAfterPhase(f) {
		var query = `
{
  "query":{
    "query_string":{
      "fields":[
        "user"
      ],
      "query":"` + f.RunID + `"
    }
  }
}
`
		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, kbPort, "/elasticsearch/"+index+"/_search")
		host = "kibana." + sauron.Spec.URI
		headers["Content-Type"] = "application/json"
		headers["kbn-version"] = f.ElasticsearchVersion
		resp, body, err = sendRequest("POST", myURL, host, false, headers, query)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected response code %d from POST but got %d: (%v)", http.StatusOK, resp.StatusCode, resp)
		}

		var jsonResp map[string]interface{}
		if unmarshalErr := json.Unmarshal([]byte(body), &jsonResp); unmarshalErr != nil {
			t.Fatal(unmarshalErr)
		}

		if !containsKeyWithValue(jsonResp, "message", testMessage) {
			t.Fatalf("response body %s does not contain expected message value %s", body, testMessage)
		}

		// DELETE index via elasticsearch HTTP API if we are tearing down
		if !testutil.SkipTeardown(f) {

			myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, esPort, "/"+index)
			host = "elasticsearch." + sauron.Spec.URI
			resp, _, err = sendRequest("DELETE", myURL, host, false, headers, "")
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
				t.Fatalf("Expected response code %d or %d from DELETE on %s but got %d: (%v)", http.StatusOK, http.StatusAccepted, myURL, resp.StatusCode, resp)
			}
			fmt.Println("  ==> Index " + "/" + index + " deleted")
		}
	}
}

func verifySauronDeploymentWithIngress(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, username, password string) {
	f := framework.Global

	type ingress struct {
		componentName  string
		endpointName   string
		deploymentName string
		urlPath        string
		replicas       int32
	}
	ingressMappings := []ingress{
		{"api", "api", constants.SauronServiceNamePrefix + sauron.Name + "-" + config.Api.Name, "/prometheus/config", 1},
		{"grafana", "grafana", constants.SauronServiceNamePrefix + sauron.Name + "-" + config.Grafana.Name, "", 1},
		{"prometheus", "prometheus", constants.SauronServiceNamePrefix + sauron.Name + "-" + config.Prometheus.Name + "-0", "/graph", 1},
		{"prometheus-gw", "prometheus-gw", constants.SauronServiceNamePrefix + sauron.Name + "-" + config.PrometheusGW.Name, "", 1},
		{"alertmanager", "alertmanager", constants.SauronServiceNamePrefix + sauron.Name + "-" + config.AlertManager.Name, "", 1},
		{"elasticsearch-ingest", "elasticsearch", constants.SauronServiceNamePrefix + sauron.Name + "-" + config.ElasticsearchIngest.Name, "", 1},
		{"kibana", "kibana", constants.SauronServiceNamePrefix + sauron.Name + "-" + config.Kibana.Name, "/app/kibana", 1},
	}
	statefulSetComponents := []string{"alertmanager"}

	// Verify Sauron instance deployments

	fmt.Println("when junk credentials provided Mapping of Sauron Endpoints")
	for p := range ingressMappings {
		var err error
		if resources.SliceContains(statefulSetComponents, ingressMappings[p].componentName) {
			err = testutil.WaitForStatefulSetAvailable(f.Namespace, ingressMappings[p].deploymentName,
				ingressMappings[p].replicas, testutil.DefaultRetry, f.KubeClient)
		} else {
			err = testutil.WaitForDeploymentAvailable(f.Namespace, ingressMappings[p].deploymentName,
				ingressMappings[p].replicas, testutil.DefaultRetry, f.KubeClient)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	fmt.Println("Verifing Ingress Controller service.")
	ingressControllerSvc, err := testutil.WaitForService(f.OperatorNamespace, f.IngressControllerSvcName, testutil.DefaultRetry, f.KubeClient)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("IngressControllerSvc:%v", ingressControllerSvc)

	for i := range ingressMappings {
		// Verify service endpoint connectivity through ingress controller

		if ingressControllerSvc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			if err := testutil.WaitForEndpointAvailableWithAuth(
				ingressMappings[i].endpointName,
				sauron.Spec.URI,
				"",
				ingressControllerSvc.Spec.Ports[0].Port,
				ingressMappings[i].urlPath,
				http.StatusOK,
				testutil.DefaultRetry,
				username,
				password); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := testutil.WaitForEndpointAvailableWithAuth(
				ingressMappings[i].endpointName,
				sauron.Spec.URI,
				f.ExternalIP,
				ingressControllerSvc.Spec.Ports[0].NodePort,
				ingressMappings[i].urlPath,
				http.StatusOK,
				testutil.DefaultRetry,
				username,
				password); err != nil {
				t.Fatal(err)
			}
		}

		fmt.Println(ingressMappings[i].componentName + "  ==> service endpoint found through ingress-controller")
	}

	for i := range ingressMappings {
		// Verify 401 error returned by ingress controller when no credentials supplied

		if ingressControllerSvc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			if err := testutil.WaitForEndpointAvailableWithAuth(
				ingressMappings[i].endpointName,
				sauron.Spec.URI,
				"",
				ingressControllerSvc.Spec.Ports[0].Port,
				ingressMappings[i].urlPath,
				http.StatusUnauthorized,
				testutil.DefaultRetry,
				"",
				""); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := testutil.WaitForEndpointAvailableWithAuth(
				ingressMappings[i].endpointName,
				sauron.Spec.URI,
				f.ExternalIP,
				ingressControllerSvc.Spec.Ports[0].NodePort,
				ingressMappings[i].urlPath,
				http.StatusUnauthorized,
				testutil.DefaultRetry,
				"",
				""); err != nil {
				t.Fatal(err)
			}
		}

		fmt.Println(ingressMappings[i].componentName + "  ==> 401 error successfully returned from ingress-controller when junk credentials provided")
	}
}

// containsKeyWithValue recursively searches the specified JSON for the first matched key and returns the value as a string.
// If the key is not found, the empty string is returned.
func containsKeyWithValue(v interface{}, key, value string) bool {
	switch vv := v.(type) {
	case map[string]interface{}:
		for k, v := range vv {
			if k == key {
				// Base case
				return fmt.Sprintf("%v", v) == value
			}
			if containsKeyWithValue(v, key, value) {
				return true
			}
		}
	case []interface{}:
		for _, v := range vv {
			if containsKeyWithValue(v, key, value) {
				return true
			}
		}
	}
	return false
}

// waitForKeyWithValueResponse performs a GET and calls containsKeyWithValue each time.
func waitForKeyWithValueResponse(host, externalIP string, port int32, urlPath, key, value string) error {
	f := framework.Global
	var httpProtocol, body string
	var err error
	var headers = map[string]string{}

	// Always use https with Ingress Controller
	if f.Ingress {
		httpProtocol = "https://"
	} else {
		httpProtocol = "http://"
	}
	endpointUrl := fmt.Sprintf("%s%s:%d%s", httpProtocol, externalIP, port, urlPath)
	fmt.Printf("  ==> Verifying endpoint (%s) with key:%s and value:%s\n", endpointUrl, key, value)

	err = testutil.Retry(testutil.DefaultRetry, func() (bool, error) {
		_, body, err = sendRequest("GET", endpointUrl, host, false, headers, "")
		var jsonResp map[string]interface{}
		if err := json.Unmarshal([]byte(body), &jsonResp); err == nil {
			found := containsKeyWithValue(jsonResp, key, value)
			return found, nil
		}
		return false, err
	})
	return err
}

// waitForStringInResponse waits until the given URL contains a given expected string
func waitForStringInResponse(host string, externalIP string, port int32, urlPath, expectedString string) error {
	f := framework.Global
	var httpProtocol, body string
	var err error
	var headers = map[string]string{}

	// Always use https with Ingress Controller
	if f.Ingress {
		httpProtocol = "https://"
	} else {
		httpProtocol = "http://"
	}
	endpointUrl := fmt.Sprintf("%s%s:%d%s", httpProtocol, externalIP, port, urlPath)

	err = testutil.Retry(testutil.DefaultRetry, func() (bool, error) {
		_, body, err = sendRequest("GET", endpointUrl, host, false, headers, "")
		if err == nil {
			return strings.Contains(body, expectedString), nil
		}
		return false, nil
	})
	return err
}

// waitForPrometheusMetric waits until the given metric appears in Prometheus (from some time in the last day)
func waitForPrometheusMetric(domain, prometheusURL string, metricName string) error {
	var err error
	var body string
	var headers = map[string]string{}

	endpointUrl := fmt.Sprintf("%s/api/v1/query?query=avg_over_time(%s[1d])", prometheusURL, metricName)
	err = testutil.Retry(testutil.DefaultRetry, func() (bool, error) {

		_, body, err = sendRequest("GET", endpointUrl, "prometheus."+domain, false, headers, "")
		var jsonResp map[string]interface{}
		if err := json.Unmarshal([]byte(body), &jsonResp); err == nil {
			dataJSON := jsonResp["data"]
			// Need to convert the [data] element into an interface in order to process it
			switch vv := dataJSON.(type) {
			case map[string]interface{}:
				// A non-empty list means the metric exists
				return fmt.Sprint(vv["result"]) != "[]", nil
			case []interface{}:
				return false, nil
			}
		}
		return false, nil
	})
	return err

}

// readTestDashboardConfig returns a dashboard as a JSON-encoded string whose title is the specified id.
func readTestDashboardConfig(id string) (string, error) {
	dashboardConfig, readErr := ioutil.ReadFile("files/grafana.dashboard.json")
	if readErr != nil {
		return "", readErr
	}

	var dbData map[string]interface{}
	if err := json.Unmarshal(dashboardConfig, &dbData); err != nil {
		return "", err
	}
	dbData["title"] = id
	dbData["overwrite"] = "true"

	// Create and populate outer JSON object
	jsonPayload := make(map[string]interface{})
	jsonPayload["dashboard"] = dbData
	config, marshalErr := json.MarshalIndent(jsonPayload, "", "  ")
	if marshalErr != nil {
		return "", marshalErr
	}
	return string(config), nil
}

// Helper function to return the port for a given component
func getPortFromService(t *testing.T, namespace, name string) int32 {
	f := framework.Global

	// Get the Service
	mySvc, err := testutil.WaitForService(namespace, name, testutil.DefaultRetry, f.KubeClient)
	if err != nil {
		t.Fatal(err)
	}

	// Return the port
	return mySvc.Spec.Ports[0].NodePort
}

// Call appropriate Wait function depending on whether or not we have ingress
func waitForEndpoint(t *testing.T, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, componentName string, componentPort int32, componentURL string) {
	f := framework.Global

	if f.Ingress {
		if err := testutil.WaitForEndpointAvailableWithAuth(
			componentName,
			sauron.Spec.URI,
			f.ExternalIP,
			componentPort,
			componentURL,
			http.StatusOK,
			testutil.DefaultRetry,
			username,
			password); err != nil {
			t.Fatal(err)
		}

		// If not using ingress...
	} else {
		if err := testutil.WaitForEndpointAvailable(
			componentName,
			f.ExternalIP,
			componentPort,
			componentURL,
			http.StatusOK,
			testutil.DefaultRetry); err != nil {
			t.Fatal(err)
		}
	}
}

func sendRequest(action, myURL, host string, useAuth bool, headers map[string]string, payload string) (*http.Response, string, error) {
	return sendRequestWithUserPassword(action, myURL, host, useAuth, headers, payload, username, password)
}

// Put together an http.Request with or without ingress controller
// Returns an immediate response;  no waiting.
func sendRequestWithUserPassword(action, myURL, host string, useAuth bool, headers map[string]string, payload string, reqUserName string, reqPassword string) (*http.Response, string, error) {
	f := framework.Global
	var err error

	tr := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		TLSHandshakeTimeout: 10 * time.Second,
		// Match Cirith's default timeouts
		ResponseHeaderTimeout: 20 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &http.Client{Transport: tr, Timeout: 300 * time.Second}
	tURL := url.URL{}

	// Set proxy for http client - needs to be done unless using localhost
	proxyURL := os.Getenv("http_proxy")
	if proxyURL != "" {
		tURLProxy, _ := tURL.Parse(proxyURL)
		tr.Proxy = http.ProxyURL(tURLProxy)
	}

	// fmt.Printf(" --> Request: %s - %s\n", action, myURL)
	req, _ := http.NewRequest(action, myURL, strings.NewReader(payload))
	req.Header.Add("Accept", "*/*")

	// Add any headers to the request
	for k := range headers {
		req.Header.Add(k, headers[k])
	}

	// If using ingress, we need to set the host...
	if f.Ingress {
		req.Host = host
	}

	// Use Basic Auth if using ingress - or if requested...
	if f.Ingress || useAuth {
		req.SetBasicAuth(reqUserName, reqPassword)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// Extract the body
	body, _ := ioutil.ReadAll(resp.Body)
	// fmt.Printf(" --> Body: %s", string(body))

	return resp, string(body), err
}

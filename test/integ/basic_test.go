// Copyright (C) 2020, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package integ

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
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

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/test/integ/framework"
	testutil "github.com/verrazzano/verrazzano-monitoring-operator/test/integ/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	username         = "vmo"
	reporterUsername = "vmi-reporter"
	changeUsername   = "s@ur0n"
)

var password = generateRandomString()
var reporterPassword = generateRandomString()
var changePassword = generateRandomString()

// ********************************************************
// *** Scenarios covered by the Basic Integration Tests ***
// Setup
// - creation of a basic VMO instance + validations below
// - creation of a VMO instance with block volumes + validations below
// VMO API Server validations
// - Verify service endpoint connectivity
// Grafana Server validations
// - Verify service endpoint connectivity
// - Upload dashboard via Grafana HTTP API
// - GET/DELETE dashboard via Grafana HTTP API
// Opensearch Server validations
// - Verify service endpoint connectivity
// - Upload new document via Opensearch HTTP API
// - GET/DELETE document via Opensearch HTTP API
// Kibana Server validations
// - Verify service endpoint connectivity
// - Upload new document via Opensearch HTTP API
// - search for document via Kibana HTTP API
// - GET/DELETE entire index via Opensearch HTTP API
// ********************************************************

func TestBasic1VMO(t *testing.T) {
	f := framework.Global
	var vmoName string

	// Create secrets - domain only used with ingress
	secretName := f.RunID + "-vmo-secrets"
	testDomain := "ingress-test.example.com"
	secret, err := createTestSecrets(secretName, testDomain)
	if err != nil {
		t.Errorf("failed to create test secrets: %+v", err)
	}

	// Create vmo
	if f.Ingress {
		vmoName = f.RunID + "-ingress"
	} else {
		vmoName = f.RunID + "-vmo-basic"
	}
	vmo := testutil.NewVMO(vmoName, secretName)
	if f.Ingress {
		vmo.Spec.URI = testDomain
	}

	if testutil.RunBeforePhase(f) {
		// Create VMO instance
		vmo, err = testutil.CreateVMO(f.CRClient, f.KubeClient2, f.Namespace, vmo, secret)
		if err != nil {
			t.Fatalf("Failed to create VMO: %v", err)
		}
	} else {
		vmo, err = testutil.GetVMO(f.CRClient, f.Namespace, vmo)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete VMO instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteVMO(f.CRClient, f.KubeClient2, f.Namespace, vmo, secret); err != nil {
				t.Fatalf("Failed to clean up VMO: %v", err)
			}
		}()
	}
	verifyVMODeployment(t, vmo)
}

func TestBasic2VMOWithDataVolumes(t *testing.T) {
	f := framework.Global

	// Create Secrets - domain only used with ingress
	secretName := f.RunID + "-vmo-secrets"
	testDomain := "ingress-test.example.com"
	secret, err := createTestSecrets(secretName, testDomain)
	if err != nil {
		t.Errorf("failed to create test secrets: %+v", err)
	}

	// Create VMO
	vmo := testutil.NewVMO(f.RunID+"-vmo-data", secretName)
	vmo.Spec.Opensearch.Storage = vmcontrollerv1.Storage{Size: "50Gi"}
	vmo.Spec.Grafana.Storage = vmcontrollerv1.Storage{Size: "50Gi"}
	vmo.Spec.API.Replicas = 2
	if f.Ingress {
		vmo.Spec.URI = testDomain
	}

	if testutil.RunBeforePhase(f) {
		// Create VMO instance
		vmo, err = testutil.CreateVMO(f.CRClient, f.KubeClient2, f.Namespace, vmo, secret)
		if err != nil {
			t.Fatalf("Failed to create VMO: %v", err)
		}
	} else {
		vmo, err = testutil.GetVMO(f.CRClient, f.Namespace, vmo)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete VMO instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteVMO(f.CRClient, f.KubeClient2, f.Namespace, vmo, secret); err != nil {
				t.Fatalf("Failed to clean up VMO: %v", err)
			}
		}()
	}

	verifyVMODeployment(t, vmo)
}

func TestBasic3GrafanaOnlyVMOAPITokenOperations(t *testing.T) {
	f := framework.Global
	secretName := f.RunID + "-vmo-secrets"
	secret := testutil.NewSecret(secretName, f.Namespace)
	vmo := testutil.NewGrafanaOnlyVMO(f.RunID+"-"+"grafana-only", secretName)
	var err error
	if testutil.RunBeforePhase(f) {
		// Create VMO instance
		vmo, err = testutil.CreateVMO(f.CRClient, f.KubeClient2, f.Namespace, vmo, secret)
		if err != nil {
			t.Fatalf("Failed to create VMO: %v", err)
		}
	} else {
		vmo, err = testutil.GetVMO(f.CRClient, f.Namespace, vmo)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete VMO instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteVMO(f.CRClient, f.KubeClient2, f.Namespace, vmo, secret); err != nil {
				t.Fatalf("Failed to clean up VMO: %v", err)
			}
		}()
	}
	verifyGrafanaAPITokenOperations(t, vmo)
}

// Creates Simple vmo without canaries definied
// Use API server REST apis to create/update/delete canaries
func TestBasic4VMOMultiUserAuthn(t *testing.T) {
	f := framework.Global
	var err error
	testDomain := "multiuser-authn.example.com"

	hosts := "*." + testDomain + ",api." + testDomain + ",grafana." + testDomain +
		",kibana." + testDomain + ",elasticsearch." + testDomain + "," + f.ExternalIP

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

	secretName := f.RunID + "-vmo-secrets"
	extraCreds := []string{reporterUsername, reporterPassword}
	secret := testutil.NewSecretWithTLSWithMultiUser(secretName, f.Namespace, tCert, tKey, username, password, extraCreds)

	// Create VMO
	vmo := testutil.NewVMO(f.RunID+"-"+"multiuser-authn", secretName)
	vmo.Spec.URI = testDomain

	if testutil.RunBeforePhase(f) {
		// Create VMO instance
		vmo, err = testutil.CreateVMO(f.CRClient, f.KubeClient2, f.Namespace, vmo, secret)
		if err != nil {
			t.Fatalf("Failed to create VMO: %v", err)
		}
	} else {
		vmo, err = testutil.GetVMO(f.CRClient, f.Namespace, vmo)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete VMO instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteVMO(f.CRClient, f.KubeClient2, f.Namespace, vmo, secret); err != nil {
				t.Fatalf("Failed to clean up VMO: %v", err)
			}
		}()
	}
	verifyMultiUserAuthnOperations(t, vmo)
}

func TestBasic4VMOWithIngress(t *testing.T) {
	f := framework.Global

	testDomain := "ingress-test.example.com"
	hosts := "*." + testDomain + ",api." + testDomain + ",grafana." + testDomain +
		",kibana." + testDomain + ",elasticsearch." + testDomain + "," + f.ExternalIP

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

	secretName := f.RunID + "-vmo-secrets"
	secret := testutil.NewSecretWithTLS(secretName, f.Namespace, tCert, tKey, username, password)

	// Create VMO
	vmo := testutil.NewVMO(f.RunID+"-ingress", secretName)
	vmo.Spec.URI = testDomain

	if testutil.RunBeforePhase(f) {
		// Create VMO instance
		vmo, err = testutil.CreateVMO(f.CRClient, f.KubeClient2, f.Namespace, vmo, secret)
		if err != nil {
			t.Fatalf("Failed to create VMO: %v", err)
		}
		fmt.Printf("Ingress VMOSpec: %v", vmo)
	} else {
		vmo, err = testutil.GetVMO(f.CRClient, f.Namespace, vmo)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	// Delete VMO instance if we are tearing down
	if !testutil.SkipTeardown(f) {
		defer func() {
			if err := testutil.DeleteVMO(f.CRClient, f.KubeClient2, f.Namespace, vmo, secret); err != nil {
				t.Fatalf("Failed to clean up VMO: %v", err)
			}
		}()
	}
	verifyVMODeploymentWithIngress(t, vmo, username, password)

	//update top-level secret new username/password
	fmt.Println("Updating vmo username/paswword")
	secret, err = f.KubeClient2.CoreV1().Secrets(vmo.Namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret %s doesn't exists in namespace %s : %v", secretName, vmo.Namespace, err)
	}
	secret.Data["username"] = []byte(changeUsername)
	secret.Data["password"] = []byte(changePassword)
	secret, err = f.KubeClient2.CoreV1().Secrets(vmo.Namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Error when updating a secret %s, %v", secretName, err)
	}
	verifyVMODeploymentWithIngress(t, vmo, changeUsername, changePassword)
}

func TestBasic4VMOOperatorMetricsServer(t *testing.T) {
	f := framework.Global
	operatorSvcPort := getPortFromService(t, f.OperatorNamespace, "verrazzano-monitoring-operator")
	if err := testutil.WaitForEndpointAvailable("verrazzano-monitoring-operator", f.ExternalIP, operatorSvcPort, "/metrics", http.StatusOK, testutil.DefaultRetry); err != nil {
		t.Fatal(err)
	}
}

// Create appropriate secrets file for the test
func createTestSecrets(secretName, testDomain string) (*corev1.Secret, error) {
	f := framework.Global

	if !f.Ingress {
		// Create simple secret
		return testutil.NewSecret(secretName, f.Namespace), nil
	}

	// Create TLS Secret
	hosts := "*." + testDomain + ",api." + testDomain + ",grafana." + testDomain +
		",kibana." + testDomain + ",elasticsearch." + testDomain + "," + f.ExternalIP

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

func verifyMultiUserAuthnOperations(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	var httpProtocol, myURL, host string
	var apiPort, esPort int32
	var resp *http.Response
	var err error
	var headers = map[string]string{}

	fmt.Println("======================================================")
	fmt.Printf("Testing VMO %s components in namespace '%s'\n", vmo.Name, f.Namespace)
	if f.Ingress {
		fmt.Println("Mode: Testing via the Ingress Controller")
	} else {
		fmt.Println("This test can only run in ingress mode")
		return
	}

	// What port should we use?
	if f.Ingress {
		httpProtocol = "https://"
		apiPort = getPortFromService(t, f.OperatorNamespace, f.IngressControllerSvcName)
	} else {
		httpProtocol = "http://"
		apiPort = getPortFromService(t, f.Namespace, constants.VMOServiceNamePrefix+vmo.Name+"-"+config.API.Name)
		esPort = getPortFromService(t, f.Namespace, constants.VMOServiceNamePrefix+vmo.Name+"-"+config.ElasticsearchIngest.Name)
	}

	// Verify service endpoint connectivity
	// Verify API availability
	waitForEndpoint(t, vmo, "API", apiPort, "/healthcheck")
	fmt.Println("  ==> Service endpoint is available")

	// Test 1: Validate reporter use is able to push to elasticsearch
	index := strings.ToLower("verifyElasticsearch") + f.RunID
	docPath := "/" + index + "/_doc/1"

	message := make(map[string]interface{})
	message["message"] = "verifyElasticsearch." + f.RunID

	jsonPayload, marshalErr := json.Marshal(message)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}

	myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, esPort, docPath)
	host = "elasticsearch." + vmo.Spec.URI
	headers["Content-Type"] = "application/json"
	resp, _, err = sendRequestWithUserPassword("POST", myURL, host, false, headers, string(jsonPayload), reporterUsername, reporterPassword)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected response code %d from POST but got %d: (%v)", http.StatusCreated, resp.StatusCode, resp)
	}
	fmt.Println("  ==> Document " + docPath + " created")

}

func verifyVMODeployment(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	fmt.Println("======================================================")
	fmt.Printf("Testing VMO %s components in namespace '%s'\n", vmo.Name, f.Namespace)
	if f.Ingress {
		fmt.Println("Mode: Testing via the Ingress Controller")
	} else {
		fmt.Println("Mode: Testing via NodePorts")
	}
	fmt.Println("======================================================")

	// Verify deployments
	fmt.Println("Step 1: Verify VMO instance deployments")

	// Verify deployments
	var deploymentNamesToReplicas = map[string]int32{
		constants.VMOServiceNamePrefix + vmo.Name + "-" + config.API.Name:                 vmo.Spec.API.Replicas,
		constants.VMOServiceNamePrefix + vmo.Name + "-" + config.Grafana.Name:             1,
		constants.VMOServiceNamePrefix + vmo.Name + "-" + config.Kibana.Name:              vmo.Spec.Kibana.Replicas,
		constants.VMOServiceNamePrefix + vmo.Name + "-" + config.ElasticsearchIngest.Name: vmo.Spec.Opensearch.IngestNode.Replicas,
		constants.VMOServiceNamePrefix + vmo.Name + "-" + config.ElasticsearchMaster.Name: vmo.Spec.Opensearch.MasterNode.Replicas,
	}
	for i := 0; i < int(vmo.Spec.Opensearch.DataNode.Replicas); i++ {
		deploymentNamesToReplicas[constants.VMOServiceNamePrefix+vmo.Name+"-"+config.ElasticsearchData.Name+"-"+strconv.Itoa(i)] = 1
	}

	statefulSetComponents := []string{}

	statefulSetComponents = append(statefulSetComponents, constants.VMOServiceNamePrefix+vmo.Name+"-"+config.ElasticsearchMaster.Name)
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

	verifyAPI(t, vmo)
	fmt.Println("Step 3: Verify Grafana")
	verifyGrafana(t, vmo)
	fmt.Println("Step 4: Verify Opensearch")
	verifyElasticsearch(t, vmo, true, false)
	fmt.Println("Step 5: Verify Kibana")
	verifyKibana(t, vmo)

	fmt.Println("======================================================")
}

func verifyAPI(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	var apiPort int32

	// What ports should we use?
	if f.Ingress {
		apiPort = getPortFromService(t, f.OperatorNamespace, f.IngressControllerSvcName)
	} else {
		apiPort = getPortFromService(t, f.Namespace, constants.VMOServiceNamePrefix+vmo.Name+"-"+config.API.Name)
	}

	// Wait for API availability
	waitForEndpoint(t, vmo, "API", apiPort, "/healthcheck")
	fmt.Println("  ==> Service endpoint is available")
}

func verifyGrafanaAPITokenOperations(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global

	if !vmo.Spec.Grafana.Enabled {
		return
	}

	grafanaSvc, err := testutil.WaitForService(f.Namespace, constants.VMOServiceNamePrefix+vmo.Name+"-grafana", testutil.DefaultRetry, f.KubeClient)
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
		//         This validates backward compatability, Users can continue to use VMO basic auth
		fmt.Println("Dashboard API Token tests")
		restyClient := testutil.GetClient()
		resp, err := restyClient.R().SetBasicAuth(username, password).
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
		restyClient = testutil.GetClient()
		resp2, err2 := restyClient.R().SetHeader("Content-Type", "application/json").
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
		restyClient = testutil.GetClient()
		resp3, err3 := restyClient.R().SetHeader("Content-Type", "application/json").
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
		restyClient = testutil.GetClient()
		resp4, err4 := restyClient.R().
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
		restyClient = testutil.GetClient()
		resp5, err5 := restyClient.R().SetHeader("Authorization", "Bearer "+viewAPIToken).
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
		restyClient = testutil.GetClient()
		resp6, err6 := restyClient.R().SetHeader("Content-Type", "application/json").
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
		restyClient = testutil.GetClient()
		resp7, err7 := restyClient.R().SetHeader("Content-Type", "application/json").
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
			restyClient := testutil.GetClient()
			resp8, err8 := restyClient.R().SetBasicAuth(username, password).
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
			restyClient = testutil.GetClient()
			resp9, err9 := restyClient.R().SetBasicAuth(username, password).
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

func verifyGrafana(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	var port int32
	var httpProtocol, myURL, host, body string
	var resp *http.Response
	var err error
	var headers = map[string]string{}

	externalDomainName := "localhost"
	if !vmo.Spec.Grafana.Enabled {
		return
	}

	// What port should we use?
	if f.Ingress {
		port = getPortFromService(t, f.OperatorNamespace, f.IngressControllerSvcName)
		httpProtocol = "https://"
		externalDomainName = "grafana." + vmo.Spec.URI
	} else {
		port = getPortFromService(t, f.Namespace, constants.VMOServiceNamePrefix+vmo.Name+"-"+config.Grafana.Name)
		httpProtocol = "http://"
	}

	// Verify Grafana availability
	waitForEndpoint(t, vmo, "Grafana", port, "/api/health")
	fmt.Println("  ==> Service endpoint is available")

	/* Verify domain and root_url */
	host = "grafana." + vmo.Spec.URI
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
		if rootURL, ok := grafanaServerConfig["root_url"]; !ok {
			t.Fatalf("Expected 'root_url' element in result but found none\n")
		} else if rootURL != "https://"+externalDomainName {
			t.Fatalf("Actual root_url value '%s' doesn't match expected value '%s'\n", rootURL, "https://"+externalDomainName)
		} else {
			fmt.Printf("'root_url' obtained from Grafana config is = %+v\n", rootURL)
		}
	}

	/** End verify domain and root_url **/

	dashboardConfig, err := readTestDashboardConfig(f.RunID)
	if err != nil {
		t.Fatal(err)
	}

	if testutil.RunBeforePhase(f) {

		// Verify "vmo" user can retrieve the user list
		// This is a good check that authentication is being passedi properly with ingress
		myURL = fmt.Sprintf("%s%s:%d%s", httpProtocol, f.ExternalIP, port, "/api/users")
		host = "grafana." + vmo.Spec.URI
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
		host = "grafana." + vmo.Spec.URI
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
		host = "grafana." + vmo.Spec.URI
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
			host = "grafana." + vmo.Spec.URI
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

func verifyElasticsearch(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, verifyIngestPipeline bool, verifyVMOIndex bool) {
	f := framework.Global

	var httpProtocol, myURL, host string
	var esPort int32
	var resp *http.Response
	var err error
	var headers = map[string]string{}

	if !vmo.Spec.Opensearch.Enabled {
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
		esPort = getPortFromService(t, f.Namespace, constants.VMOServiceNamePrefix+vmo.Name+"-"+config.ElasticsearchIngest.Name)
		httpProtocol = "http://"
	}
	host = "elasticsearch." + vmo.Spec.URI

	// Verify service endpoint connectivity
	waitForEndpoint(t, vmo, "Opensearch", esPort, "/_cluster/health")
	waitForEndpoint(t, vmo, "Opensearch", esPort, "/_cat/indices")
	fmt.Println("  ==> Service endpoint is available")

	// Verify expected cluster size
	expectedClusterSize := vmo.Spec.Opensearch.MasterNode.Replicas + vmo.Spec.Opensearch.IngestNode.Replicas + vmo.Spec.Opensearch.DataNode.Replicas
	fmt.Printf("  ==> Verifying Opensearch cluster size is as expected (%d)\n", expectedClusterSize)
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
			fmt.Println("  ==> IngestNodes Pipeline " + ingestPipelinePath + " created")
		}

		ElasticSearchService, err := testutil.WaitForService(f.Namespace, constants.VMOServiceNamePrefix+vmo.Name+"-"+config.ElasticsearchIngest.Name, testutil.DefaultRetry, f.KubeClient)
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
		ElasticSearchService, err := testutil.WaitForService(f.Namespace, constants.VMOServiceNamePrefix+vmo.Name+"-"+config.ElasticsearchIngest.Name, testutil.DefaultRetry, f.KubeClient)
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
				fmt.Println("  ==> IngestNodes pipeline " + docPath + " retrieved")
			}
		}

		if verifyVMOIndex {
			// Verify presence of .vmo index added by backup/restore process
			myURL = fmt.Sprintf("%s%s:%d/.vmo", httpProtocol, f.ExternalIP, esPort)
			resp, _, err = sendRequest("GET", myURL, host, false, headers, "")
			if err != nil {
				t.Fatalf("Failed to check for VMO index %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected response code %d from GET but got %d: (%v)", http.StatusOK, resp.StatusCode, resp)
			}
			fmt.Println("  ==> .vmo index observed")
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
				fmt.Println("  ==> IngestNodes pipeline" + docPath + " deleted")
			}
		}
	}
}

func verifyKibana(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) {
	f := framework.Global
	var httpProtocol, myURL, host, body string
	var resp *http.Response
	var err error
	var kbPort, esPort int32
	var headers = map[string]string{}

	if !vmo.Spec.Kibana.Enabled {
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
		kbPort = getPortFromService(t, f.Namespace, constants.VMOServiceNamePrefix+vmo.Name+"-"+config.OpenSearchDashboards.Name)
		esPort = getPortFromService(t, f.Namespace, constants.VMOServiceNamePrefix+vmo.Name+"-"+config.ElasticsearchIngest.Name)
		httpProtocol = "http://"
	}

	// Verify service endpoint connectivity
	waitForEndpoint(t, vmo, "Kibana", kbPort, "/api/status")
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
		host = "elasticsearch." + vmo.Spec.URI
		headers["Content-Type"] = "application/json"
		resp, _, err = sendRequest("POST", myURL, host, false, headers, string(jsonPayload))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Expected response code %d from POST but got %d: (%v)", http.StatusCreated, resp.StatusCode, resp)
		}
		fmt.Println("  ==> Document " + docPath + " created")
		// Deal with possible Opensearch index delay on a newly created document
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
		host = "kibana." + vmo.Spec.URI
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
			host = "elasticsearch." + vmo.Spec.URI
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

func verifyVMODeploymentWithIngress(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, username, password string) {
	f := framework.Global

	type ingress struct {
		componentName  string
		endpointName   string
		deploymentName string
		urlPath        string
		replicas       int32
	}
	ingressMappings := []ingress{
		{"api", "api", constants.VMOServiceNamePrefix + vmo.Name + "-" + config.API.Name, "/prometheus/config", 1},
		{"grafana", "grafana", constants.VMOServiceNamePrefix + vmo.Name + "-" + config.Grafana.Name, "", 1},
		{"elasticsearch-ingest", "elasticsearch", constants.VMOServiceNamePrefix + vmo.Name + "-" + config.ElasticsearchIngest.Name, "", 1},
		{"kibana", "kibana", constants.VMOServiceNamePrefix + vmo.Name + "-" + config.Kibana.Name, "/app/kibana", 1},
	}

	// Verify VMO instance deployments

	fmt.Println("when junk credentials provided Mapping of VMO Endpoints")
	for p := range ingressMappings {
		err := testutil.WaitForDeploymentAvailable(f.Namespace, ingressMappings[p].deploymentName,
			ingressMappings[p].replicas, testutil.DefaultRetry, f.KubeClient)
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
				vmo.Spec.URI,
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
				vmo.Spec.URI,
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
				vmo.Spec.URI,
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
				vmo.Spec.URI,
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
	endpointURL := fmt.Sprintf("%s%s:%d%s", httpProtocol, externalIP, port, urlPath)
	fmt.Printf("  ==> Verifying endpoint (%s) with key:%s and value:%s\n", endpointURL, key, value)

	err = testutil.Retry(testutil.DefaultRetry, func() (bool, error) {
		_, body, err = sendRequest("GET", endpointURL, host, false, headers, "")
		var jsonResp map[string]interface{}
		if err := json.Unmarshal([]byte(body), &jsonResp); err == nil {
			found := containsKeyWithValue(jsonResp, key, value)
			return found, nil
		}
		return false, err
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
func waitForEndpoint(t *testing.T, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, componentName string, componentPort int32, componentURL string) {
	f := framework.Global

	if f.Ingress {
		if err := testutil.WaitForEndpointAvailableWithAuth(
			componentName,
			vmo.Spec.URI,
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
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec //#gosec G402
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

// generateRandomString returns a base64 encoded generated random string.
func generateRandomString() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

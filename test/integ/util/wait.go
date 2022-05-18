// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package util

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"crypto/tls"
	"net/http"
	"net/url"
	"os"
	"time"
)

// DefaultRetry is the default backoff for e2e tests.
var DefaultRetry = wait.Backoff{
	Steps:    150,
	Duration: 4 * time.Second,
	Factor:   1.0,
	Jitter:   0.1,
}

// Retry executes the provided function repeatedly, retrying until the function
// returns done = true, errors, or exceeds the given timeout.
func Retry(backoff wait.Backoff, fn wait.ConditionFunc) error {
	var lastErr error
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		done, err := fn()
		if err != nil {
			lastErr = err
		}
		return done, err
	})
	if err == wait.ErrWaitTimeout {
		if lastErr != nil {
			err = lastErr
		}
	}
	return err
}

// WaitForDeploymentAvailable waits for the given deployment to reach the given number of available replicas
func WaitForDeploymentAvailable(namespace string, deploymentName string, availableReplicas int32, backoff wait.Backoff, kubeClient kubernetes.Interface) error {
	var err error
	fmt.Printf("Waiting for deployment '%s' to reach %d available and total replicas...\n", deploymentName, availableReplicas)
	err = Retry(backoff, func() (bool, error) {
		deployments, err := kubeClient.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, deployment := range deployments.Items {
			if deployment.Name == deploymentName {
				if deployment.Status.AvailableReplicas == availableReplicas && deployment.Status.Replicas == availableReplicas {
					return true, nil
				}
			}
		}
		return false, nil
	})
	return err
}

// WaitForStatefulSetAvailable waits for the given statefulset to reach the given number of available replicas
func WaitForStatefulSetAvailable(namespace string, statefulSetName string, availableReplicas int32, backoff wait.Backoff, kubeClient kubernetes.Interface) error {
	var err error
	fmt.Printf("Waiting for statefulset '%s' to reach %d available and total replicas...\n", statefulSetName, availableReplicas)
	err = Retry(backoff, func() (bool, error) {
		statefulSets, err := kubeClient.AppsV1().StatefulSets(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, statefulSet := range statefulSets.Items {
			if statefulSet.Name == statefulSetName {
				if statefulSet.Status.CurrentReplicas == availableReplicas && statefulSet.Status.ReadyReplicas == availableReplicas {
					return true, nil
				}
			}
		}
		return false, nil
	})
	return err
}

// WaitForService waits for the given service to become available
func WaitForService(namespace string, serviceName string, backoff wait.Backoff,
	kubeClient kubernetes.Interface) (*apiv1.Service, error) {
	var latest *apiv1.Service
	var err error
	fmt.Printf("Waiting for service '%s'...\n", serviceName)
	err = Retry(backoff, func() (bool, error) {
		services, err := kubeClient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for i, service := range services.Items {
			if service.Name == serviceName {
				latest = &services.Items[i]
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return latest, nil

}

// WaitForElasticSearchIndexDocCount waits for doc count to show up in elasticsearch
func WaitForElasticSearchIndexDocCount(ElasticSearchIndexDocCountURL string, count string, backoff wait.Backoff) (bool, error) {
	var err error
	fmt.Println("Getting index doc count from elasticsearch url: " + ElasticSearchIndexDocCountURL)
	err = Retry(backoff, func() (bool, error) {
		resp, err := http.Get(ElasticSearchIndexDocCountURL) //nolint:gosec //#gosec G107
		if err != nil {
			return false, err
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		fmt.Println("Elasticsearch Index Doc Count output: \n" + string(body))

		docCount := string(body)[strings.LastIndex(string(body), " ")+1:]
		docCount = strings.TrimSuffix(docCount, "\n")
		if strings.Compare(docCount, count) == 0 {
			fmt.Println("  ==> Index has " + count + " docs.")
			return true, nil
		}
		fmt.Println("Index does not have " + count + " docs. It only has " + docCount + " docs. Waiting ...")
		return false, err
	})

	if err != nil {
		return false, err
	}
	return true, nil

}

// WaitForEndpointAvailableWithAuth waits for the given endpoint to become available
// if useIp is set, use the given worker ip to verify the endpoints/urlPath
func WaitForEndpointAvailableWithAuth(
	component string,
	domainName string,
	useIP string,
	port int32,
	path string,
	expectedStatusCode int,
	backoff wait.Backoff,
	username string,
	password string) error {

	var err error
	startTime := time.Now()

	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, //nolint:gosec //#gosec G402
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{Transport: tr, Timeout: 300 * time.Second}

	tURL := url.URL{}

	// Set proxy for http client
	if useIP != "localhost" {
		proxyURL := os.Getenv("http_proxy")
		if proxyURL != "" {
			fmt.Println("Setting proxy for http clients to :" + proxyURL)
			tURLProxy, _ := tURL.Parse(proxyURL)
			tr.Proxy = http.ProxyURL(tURLProxy)
		}
	}

	var myURL string

	// if requesting to use a specific worker IP to verify endpoint
	if useIP != "" {
		myURL = fmt.Sprintf("https://%s:%d%s", useIP, port, path)
	} else { // otherwise, use the dns name(component.domainName)
		myURL = fmt.Sprintf("https://%s:%d%s", component+"."+domainName, port, path)
	}

	req, _ := http.NewRequest("GET", myURL, nil)

	//Only fix the HEADER in when requesting to use a specific worker IP to verify endpoint
	if useIP != "" {
		req.Host = component + "." + domainName
	}

	req.Header.Add("Accept", "*/*")
	req.SetBasicAuth(username, password)

	fmt.Printf("Waiting for %s (%s) to reach status code %d...\n", component, myURL, expectedStatusCode)

	err = Retry(backoff, func() (bool, error) {
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == expectedStatusCode {
			return true, nil
		}
		return false, nil
	})
	fmt.Printf("Wait time: %s \n", time.Since(startTime))
	if err != nil {
		return err
	}
	return nil
}

// WaitForEndpointAvailable waits for the given endpoint to become available
func WaitForEndpointAvailable(name string, externalIP string, port int32, urlPath string, expectedStatusCode int, backoff wait.Backoff) error {
	var err error
	endpointURL := fmt.Sprintf("http://%s:%d%s", externalIP, port, urlPath)
	fmt.Printf("Waiting for %s (%s) to reach status code %d...\n", name, endpointURL, expectedStatusCode)
	restyClient := GetClient()
	restyClient.SetTimeout(10 * time.Second)
	startTime := time.Now()
	err = Retry(backoff, func() (bool, error) {
		resp, e := restyClient.R().Get(endpointURL)
		if e != nil {
			fmt.Printf("error requesting URL %s: %+v", endpointURL, e)
			return false, nil
		}
		observedStatusCode := resp.StatusCode()
		if observedStatusCode != expectedStatusCode {
			fmt.Printf("URL %s: expected status code %d, observed %d", endpointURL, expectedStatusCode, observedStatusCode)
			return false, nil
		}
		return true, nil
	})
	fmt.Printf("Wait time: %s \n", time.Since(startTime))
	if err != nil {
		return err
	}
	return nil
}

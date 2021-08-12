// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// This program checks the version and cluster status of an Elasticsearch
// instance every five seconds.  If the version is sufficient, the status is
// healthy, and enough data nodes are up, then it exits with status 0.
// Otherwise, it tries again.  When the maximum wait time has elapsed without
// success, it exits with a non-zero status.

// The Version struct is defined to match JSON schema returned by ES REST API endpoints
type Version struct { //nolint:deadcode
	Number string `json:"number"`
}

// The ESNode struct is defined to match JSON schema returned by ES REST API endpoints
// Note: the field tags need to be kept in sync with the h querystring value in the nodes URL below
type ESNode struct {
	Version string `json:"version"`
	Role    string `json:"node.role"`
	//NOTE: name included only as output sugar; not used in sufficiency decisions
	Name string `json:"name"`
}

// The ClusterHealth struct defines the status of the cluster
type ClusterHealth struct {
	Status string `json:"status"`
}

const (
	defaultDefaultWait = "1m"
	checkInterval      = 5
)

var (
	client http.Client

	defaultWait      time.Duration
	maxWait          time.Duration
	numDataNodes     int
	esURL            string
	clusterHealthURL string
	nodesURL         string
	version          string
)

func init() {
	defaultWait, _ = time.ParseDuration(defaultDefaultWait)
	client = http.Client{Timeout: time.Duration(checkInterval * time.Second)}
}

func (ch ClusterHealth) isSufficient() bool {
	return ch.Status != "red"
}

func (node ESNode) isSufficient(version string) bool {
	return node.Version == version
}

func (node ESNode) isDataRole() bool {
	// Check the role string for the data-node flag
	// - role string consists of 1-3 chars, "d" (data), "i" (ingest), "m" (master), so look that it contains "d"
	// - a node can be any combination of roles (e.g., single-node cluster would be "dim")
	return strings.Contains(node.Role, "d")
}

// Send GET request to given URL, expected JSON response
func getResponseBody(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Enable basic auth if username and password is provided
	username, password := os.Getenv("ES_USER"), os.Getenv("ES_PASSWORD")
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	req.Header.Add("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("received unsuccessful response: %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func getClusterHealth(url string) (*ClusterHealth, error) {
	body, err := getResponseBody(url)
	if err != nil {
		return nil, err
	}
	var clusterHealth ClusterHealth
	if err = json.Unmarshal(body, &clusterHealth); err != nil {
		return nil, err
	}
	return &clusterHealth, nil
}

func getNodes(url string) (*[]ESNode, error) {
	body, err := getResponseBody(url)

	if err != nil {
		return nil, err
	}
	nodes := make([]ESNode, 0)
	if err = json.Unmarshal(body, &nodes); err != nil {
		return nil, err
	}
	return &nodes, nil
}

func isSufficient(clusterHealth *ClusterHealth, nodes *[]ESNode, version string, requiredDataNodes int) bool {
	if nodes == nil || clusterHealth == nil {
		return false
	}
	if !clusterHealth.isSufficient() {
		return false
	}
	observedDataNodes := 0
	for _, node := range *nodes {
		if !node.isSufficient(version) {
			return false
		}
		//only data nodes already at the required version count
		if node.isDataRole() {
			observedDataNodes++
		}
	}
	return observedDataNodes >= requiredDataNodes
}

func notSufficient(format string, v ...interface{}) {
	log.Printf(format, v...)
	time.Sleep(time.Duration(5 * time.Second))
}

func main() {
	flag.IntVar(&numDataNodes, "number-of-data-nodes", -1, "number of Elasticsearch data nodes required")
	flag.DurationVar(&maxWait, "timeout", defaultWait, "how long to wait for target state before timing out default is 1m")
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 || numDataNodes < 1 {
		log.Fatalf("Usage: eswait [-number-of-data-nodes number-of-data-nodes -timeout wait-duration] elasticsearch-url elasticsearch-version")
	}

	baseURL, err := url.Parse(args[0])
	if err != nil {
		log.Fatalf("invalid ES URL: %v", err)
	}
	esURL = baseURL.String()

	clusterHealthPath, _ := url.Parse("/_cluster/health")
	clusterHealthURL = baseURL.ResolveReference(clusterHealthPath).String()
	//NOTE: the h values in the querystring need to be kept in sync with the JSON tags on ESNode struct fields above
	nodesPath, _ := url.Parse("/_cat/nodes?h=name,node.role,version")
	nodesURL = baseURL.ResolveReference(nodesPath).String()

	version = args[1]
	if len(version) == 0 {
		log.Fatalf("version is required")
	}

	log.Printf("will wait %v for %s to satisfy:", maxWait, esURL)
	log.Printf("* version %s", version)
	log.Printf("* at least %d data nodes", numDataNodes)
	log.Print("* at least cluster status \"yellow\"")

	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		clusterHealth, err := getClusterHealth(clusterHealthURL)
		if err != nil {
			notSufficient("error getting cluster health from %s: %v", clusterHealthURL, err)
			continue
		}
		nodes, err := getNodes(nodesURL)
		if err != nil {
			notSufficient("error getting nodes from %s: %+v", nodesURL, err)
		}
		if isSufficient(clusterHealth, nodes, version, numDataNodes) {
			log.Printf("SUCCESS: version: %s, health: %s, nodes: %v", version, clusterHealth.Status, *nodes)
			os.Exit(0)
		}
		notSufficient("insufficient, retrying: cluster health: %v, nodes: %v", *clusterHealth, *nodes)
	}
	log.Fatalf("FAILURE: could not reach version %s, healthy state and %d data nodes", version, numDataNodes)
}

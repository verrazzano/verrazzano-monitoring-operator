// Copyright (C) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	"net/http"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	appslistersv1 "k8s.io/client-go/listers/apps/v1"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
)

type (
	OSClient struct {
		httpClient        *http.Client
		DoHTTP            func(request *http.Request) (*http.Response, error)
		statefulSetLister appslistersv1.StatefulSetLister
	}
)

const (
	indexSettings     = `{"index_patterns": [".opendistro*"],"priority": 0,"template": {"settings": {"auto_expand_replicas": "0-1"}}}`
	applicationJSON   = "application/json"
	contentTypeHeader = "Content-Type"
)

func NewOSClient(statefulSetLister appslistersv1.StatefulSetLister) *OSClient {
	o := &OSClient{
		httpClient:        http.DefaultClient,
		statefulSetLister: statefulSetLister,
	}
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
		return o.httpClient.Do(request)
	}
	return o
}

// IsDataResizable returns an error unless these conditions of the OpenSearch cluster are met
// - at least 2 data nodes
// - 'green' health
// - all expected nodes are present in the cluster status
func (o *OSClient) IsDataResizable(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	dataNodes := nodes.DataNodes(vmo)
	if len(dataNodes) < MinDataNodesForResize {
		return fmt.Errorf("cannot resize OpenSearch with less than %d data nodes. Scale up your cluster to at least %d data nodes", MinDataNodesForResize, MinDataNodesForResize)
	}
	return o.opensearchHealth(vmo, true, true)
}

// IsUpdated returns an error unless these conditions of the OpenSearch cluster are met
// - 'green' health
// - all expected nodes are present in the cluster status
func (o *OSClient) IsUpdated(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	return o.opensearchHealth(vmo, true, true)
}

// IsGreen returns an error unless these conditions of the OpenSearch cluster are met
// - 'green' health
func (o *OSClient) IsGreen(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	return o.opensearchHealth(vmo, false, false)
}

// ConfigureISM sets up the ISM Policies
// The returned channel should be read for exactly one response, which tells whether ISM configuration succeeded.
func (o *OSClient) ConfigureISM(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) chan error {
	ch := make(chan error)
	// configuration is done asynchronously, as this does not need to be blocking
	go func() {
		if !vmi.Spec.Opensearch.Enabled {
			ch <- nil
			return
		}

		if !o.IsOpenSearchReady(vmi) {
			ch <- nil
			return
		}

		opensearchEndpoint := resources.GetOpenSearchHTTPEndpoint(vmi)
		for _, policy := range vmi.Spec.Opensearch.Policies {
			if err := o.createISMPolicy(opensearchEndpoint, policy); err != nil {
				ch <- err
				return
			}
		}

		ch <- o.cleanupPolicies(opensearchEndpoint, vmi.Spec.Opensearch.Policies)
	}()

	return ch
}

// DeleteDefaultISMPolicy deletes the default ISM policy if they exists
func (o *OSClient) DeleteDefaultISMPolicy(log vzlog.VerrazzanoLogger, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) chan error {
	ch := make(chan error)
	go func() {
		// if Elasticsearch.DisableDefaultPolicy is set to false, skip the deletion.
		if !vmi.Spec.Opensearch.Enabled || !vmi.Spec.Opensearch.DisableDefaultPolicy {
			ch <- nil
			return
		}

		if !o.IsOpenSearchReady(vmi) {
			ch <- nil
			return
		}

		openSearchEndpoint := resources.GetOpenSearchHTTPEndpoint(vmi)
		for policyName := range defaultISMPoliciesMap {
			resp, err := o.deletePolicy(openSearchEndpoint, policyName)
			// If policy doesn't exist, ignore the error, otherwise pass the error to channel.
			if (err != nil && resp == nil) || (err != nil && resp != nil && resp.StatusCode != http.StatusNotFound) {
				ch <- err
			}
			// Remove the policy from the current write indices of system and application data streams
			var pattern string
			if policyName == "vz-system" {
				pattern = "verrazzano-system"
			} else {
				pattern = "verrazzano-application-*"
			}
			indices, err := o.getWriteIndexForDataStream(log, openSearchEndpoint, pattern)
			if err != nil {
				ch <- err
			}
			for _, index := range indices {
				ok, err := o.shouldAddOrRemoveDefaultPolicy(openSearchEndpoint, index, policyName)
				if err != nil {
					ch <- err
				}
				if ok {
					err = o.removePolicyForIndex(openSearchEndpoint, index)
					if err != nil {
						ch <- err
					}
				}
			}
		}
		ch <- nil
	}()
	return ch
}

// SyncDefaultISMPolicy set up the default ISM Policies.
// The returned channel should be read for exactly one response, which tells whether default ISM policies are synced.
func (o *OSClient) SyncDefaultISMPolicy(log vzlog.VerrazzanoLogger, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) chan error {
	ch := make(chan error)
	go func() {
		if !vmi.Spec.Opensearch.Enabled || vmi.Spec.Opensearch.DisableDefaultPolicy {
			ch <- nil
			return
		}

		if !o.IsOpenSearchReady(vmi) {
			ch <- nil
			return
		}
		openSearchEndpoint := resources.GetOpenSearchHTTPEndpoint(vmi)
		log.Debugf("calling createOrUpdateDefaultISMPolicy")
		_, err := o.createOrUpdateDefaultISMPolicy(log, openSearchEndpoint)
		ch <- err
	}()

	return ch
}

// SetAutoExpandIndices updates the default index settings to auto expand replicas (max 1) when nodes are added to the cluster
func (o *OSClient) SetAutoExpandIndices(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) chan error {
	ch := make(chan error)

	// configuration is done asynchronously, as this does not need to be blocking
	go func() {
		if !vmi.Spec.Opensearch.Enabled {
			ch <- nil
			return
		}
		if !nodes.IsSingleNodeCluster(vmi) {
			ch <- nil
			return
		}

		if !o.IsOpenSearchReady(vmi) {
			ch <- nil
			return
		}

		opensearchEndpoint := resources.GetOpenSearchHTTPEndpoint(vmi)
		settingsURL := fmt.Sprintf("%s/_index_template/ism-plugin-template", opensearchEndpoint)
		req, err := http.NewRequest("PUT", settingsURL, bytes.NewReader([]byte(indexSettings)))
		if err != nil {
			ch <- err
			return
		}
		req.Header.Add(contentTypeHeader, applicationJSON)
		resp, err := o.DoHTTP(req)
		if err != nil {
			ch <- err
			return
		}
		if resp.StatusCode != http.StatusOK {
			ch <- fmt.Errorf("got status code %d when updating default settings of index, expected %d", resp.StatusCode, http.StatusOK)
			return
		}
		var updatedIndexSettings map[string]bool
		err = json.NewDecoder(resp.Body).Decode(&updatedIndexSettings)
		if err != nil {
			ch <- err
			return
		}
		if !updatedIndexSettings["acknowledged"] {
			ch <- fmt.Errorf("expected acknowldegement for index settings update but did not get. Actual response  %v", updatedIndexSettings)
			return
		}
		ch <- nil
	}()

	return ch
}

// IsOpenSearchReady returns true when all OpenSearch pods are ready, false otherwise
func (o *OSClient) IsOpenSearchReady(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) bool {
	selector := labels.SelectorFromSet(map[string]string{constants.VMOLabel: vmi.Name, constants.ComponentLabel: constants.ComponentOpenSearchValue})
	statefulSets, err := o.statefulSetLister.StatefulSets(vmi.Namespace).List(selector)
	if err != nil {
		zap.S().Errorf("error fetching OpenSearch statefulset, error: %s", err.Error())
		return false
	}

	if len(statefulSets) == 0 {
		zap.S().Warn("waiting for OpenSearch statefulset to be created.")
		return false
	}

	if len(statefulSets) > 1 {
		zap.S().Errorf("invalid number of OpenSearch statefulset created %v.", len(statefulSets))
		return false
	}

	return statefulSets[0].Status.ReadyReplicas == statefulSets[0].Status.Replicas
}

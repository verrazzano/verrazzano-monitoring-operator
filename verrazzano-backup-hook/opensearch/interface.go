// Copyright (c) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"context"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/types"
	"go.uber.org/zap"
	"io"
	"net/http"
)

// Opensearch Interface implements methods needed for backup and restore of Opensearch
// These methods are used with the hook to save and restore Opensearch data
type Opensearch interface {
	// HTTPHelper Http wrapper to make REST based method calls
	HTTPHelper(ctx context.Context, method, requestURL string, body io.Reader, data interface{}) error

	// EnsureOpenSearchIsReachable Keep alive check with retry
	EnsureOpenSearchIsReachable() error

	// EnsureOpenSearchIsHealthy Health status check with retry
	EnsureOpenSearchIsHealthy() error

	// ReloadOpensearchSecureSettings updates Opensearch keystore with credentials
	ReloadOpensearchSecureSettings() error

	// RegisterSnapshotRepository creates a new S3 based repository
	RegisterSnapshotRepository() error

	// TriggerSnapshot starts the snapshot(backup) of the Opensearch data streams
	TriggerSnapshot() error

	// CheckSnapshotProgress checks the status of the backup process
	CheckSnapshotProgress() error

	// DeleteData deletes all data streams and indices
	DeleteData() error

	// TriggerRestore starts the snapshot restore of the Opensearch data streams
	TriggerRestore() error

	// CheckRestoreProgress checks the progress of the restore progress
	CheckRestoreProgress() error

	// Backup Toplevel method to start the backup operation
	Backup() error

	// Restore Toplevel method to start the restore operation
	Restore() error

	// BasicAuthRequired tells whether to use basic auth while performing http requests or not
	BasicAuthRequired() bool

	// GetCredential returns the username and password required for basic auth
	GetCredential() (string, string)
}

// OpensearchImpl struct for Opensearch interface
type OpensearchImpl struct {
	Client     *http.Client
	Timeout    string //Timeout for HTTP calls
	BaseURL    string
	SecretData *types.ConnectionData
	Log        *zap.SugaredLogger
	BasicAuth  *BasicAuth
}

// BasicAuth for BasicAuth interface
type BasicAuth struct {
	required bool
	username string
	password string
}

// NewBasicAuth returns a new BasicAuth struct
func NewBasicAuth(required bool, username, password string) *BasicAuth {
	return &BasicAuth{
		required: required,
		username: username,
		password: password,
	}
}

// New Opensearch Impl constructor
func New(baseURL string, timeout string, client *http.Client, secretData *types.ConnectionData, log *zap.SugaredLogger, basicAuth *BasicAuth) *OpensearchImpl {
	return &OpensearchImpl{
		Client:     client,
		Timeout:    timeout,
		BaseURL:    baseURL,
		SecretData: secretData,
		Log:        log,
		BasicAuth:  basicAuth,
	}
}

type OpensearchVar struct {
	// OpenSearchURL Opensearch url used internally
	OpenSearchURL string
	// Namespace for Opensearch pods
	Namespace string
	// OpenSearchDataPodContainerName Opensearch data pod container name
	OpenSearchDataPodContainerName string
	// OpenSearchMasterPodContainerName Opensearch master pod container name
	OpenSearchMasterPodContainerName string
	// IngestResourceName Opensearch ingest deployment/sts name
	IngestResourceName string
	// IngestLabelSelector Opensearch ingest pod label selector
	IngestLabelSelector string
	// OSDDeploymentName OSD deployment name
	OSDDeploymentName string
	// OSDLabelSelector Label selector for OSD pod
	OSDLabelSelector string
	// OSDDeploymentLabelSelector OSD deployment label selector
	OSDDeploymentLabelSelector string
	// OperatorDeploymentName Deployment name for Opensearch Operator
	OperatorDeploymentName string
	// OperatorDeploymentLabelSelector Label selector for Opensearch Operator
	OperatorDeploymentLabelSelector string
	// OpenSearchMasterLabel Label selector for OpenSearch master pods
	OpenSearchMasterLabel string
	// OpenSearchDataLabel Label selector for OpenSearch data pods
	OpenSearchDataLabel string
}

func NewOpensearchVar(isLegacyOS bool) *OpensearchVar {
	if isLegacyOS {
		return &OpensearchVar{
			OpenSearchURL:                    "http://127.0.0.1:9200",
			Namespace:                        constants.VerrazzanoSystemNamespace,
			OpenSearchDataPodContainerName:   "es-data",
			OpenSearchMasterPodContainerName: "es-master",
			IngestResourceName:               "vmi-system-es-ingest",
			IngestLabelSelector:              "app=system-es-ingest",
			OSDDeploymentName:                "vmi-system-osd",
			OSDLabelSelector:                 "app=system-osd",
			OSDDeploymentLabelSelector:       "verrazzano-component=osd",
			OperatorDeploymentName:           "verrazzano-monitoring-operator",
			OperatorDeploymentLabelSelector:  "k8s-app=verrazzano-monitoring-operator",
			OpenSearchMasterLabel:            "opensearch.verrazzano.io/role-master=true",
			OpenSearchDataLabel:              "opensearch.verrazzano.io/role-data=true",
		}
	} else {
		return &OpensearchVar{
			OpenSearchURL:                    "https://127.0.0.1:9200",
			Namespace:                        constants.VerrazzanoLoggingNamespace,
			OpenSearchDataPodContainerName:   "opensearch",
			OpenSearchMasterPodContainerName: "opensearch",
			IngestResourceName:               "opensearch-es-ingest",
			IngestLabelSelector:              "opster.io/opensearch-nodepool=es-ingest",
			OSDDeploymentName:                "opensearch-dashboards",
			OSDLabelSelector:                 "opensearch.cluster.dashboards=opensearch",
			OSDDeploymentLabelSelector:       "opensearch.cluster.dashboards=opensearch",
			OperatorDeploymentName:           "opensearch-operator-controller-manager",
			OperatorDeploymentLabelSelector:  "control-plane=controller-manager",
			OpenSearchMasterLabel:            "opensearch.role=cluster_manager",
			OpenSearchDataLabel:              "opster.io/opensearch-nodepool=es-data",
		}
	}
}

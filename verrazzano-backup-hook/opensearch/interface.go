// Copyright (c) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"context"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/types"
	"go.uber.org/zap"
	"io"
	"net/http"
	"time"
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
}

// OpensearchImpl struct for Opensearch interface
type OpensearchImpl struct {
	Client     *http.Client
	Timeout    time.Duration //Timeout for HTTP calls
	BaseURL    string
	SecretData *types.ConnectionData
	Log        *zap.SugaredLogger
}

// New Opensearch Impl constructor
func New(baseURL string, timeout time.Duration, client *http.Client, secretData *types.ConnectionData, log *zap.SugaredLogger) *OpensearchImpl {
	return &OpensearchImpl{
		Client:     client,
		Timeout:    timeout,
		BaseURL:    baseURL,
		SecretData: secretData,
		Log:        log,
	}
}

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

// OpenSearch interface implements all the method utilized in the application
type Opensearch interface {
	HTTPHelper(ctx context.Context, method, requestURL string, body io.Reader, data interface{}) error
	Backup() error
	Restore() error
	EnsureOpenSearchIsReachable() error
	EnsureOpenSearchIsHealthy() error
	ReloadOpensearchSecureSettings() error
	RegisterSnapshotRepository() error
	TriggerSnapshot() error
	CheckSnapshotProgress() error
	DeleteData() error
	TriggerRestore() error
	CheckRestoreProgress() error
}

type OpensearchImpl struct {
	Client     *http.Client
	Timeout    time.Duration //Timeout for HTTP calls
	BaseURL    string
	SecretData *types.ConnectionData
	Log        *zap.SugaredLogger
}

func New(baseURL string, timeout time.Duration, client *http.Client, secretData *types.ConnectionData, log *zap.SugaredLogger) *OpensearchImpl {
	return &OpensearchImpl{
		Client:     client,
		Timeout:    timeout,
		BaseURL:    baseURL,
		SecretData: secretData,
		Log:        log,
	}
}

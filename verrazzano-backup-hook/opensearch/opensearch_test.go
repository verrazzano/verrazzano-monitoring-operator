// Copyright (c) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch_test

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/log"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/opensearch"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/types"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const (
	healthURL         = "/_cluster/health"
	secureSettingsURL = "/_nodes/reload_secure_settings"
	dataStreamsURL    = "/_data_stream"
	snapshotURL       = "/_snapshot"
	timeOutGlobal     = "10m"
)

var (
	fakeBasicAuth = opensearch.NewBasicAuth(false, "", "")
)

func logHelper() (*zap.SugaredLogger, string) {
	file, err := os.CreateTemp(os.TempDir(), fmt.Sprintf("verrazzano-%s-hook-*.log", strings.ToLower("TEST")))
	if err != nil {
		fmt.Printf("Unable to create temp file")
		os.Exit(1)
	}
	defer file.Close()
	log, _ := log.Logger(file.Name())
	return log, file.Name()
}

var (
	httpServer *httptest.Server
	openSearch opensearch.Opensearch
)

func mockEnsureOpenSearchIsReachable(error bool, w http.ResponseWriter, r *http.Request) {
	fmt.Println("Reachable ...")
	w.Header().Add("Content-Type", constants.HTTPContentType)
	if error {
		w.WriteHeader(http.StatusGatewayTimeout)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	var osinfo types.OpenSearchClusterInfo
	osinfo.ClusterName = "foo"
	json.NewEncoder(w).Encode(osinfo)
}

func mockEnsureOpenSearchIsHealthy(error bool, w http.ResponseWriter, r *http.Request) {
	fmt.Println("Healthy ...")
	w.Header().Add("Content-Type", constants.HTTPContentType)
	var oshealth types.OpenSearchHealthResponse
	oshealth.ClusterName = "bar"
	if error {
		w.WriteHeader(http.StatusGatewayTimeout)
		oshealth.Status = "red"
	} else {
		w.WriteHeader(http.StatusOK)
		oshealth.Status = "green"
	}
	json.NewEncoder(w).Encode(oshealth)
}

func mockOpenSearchOperationResponse(error bool, w http.ResponseWriter, r *http.Request) {
	fmt.Println("Snapshot register ...")
	w.Header().Add("Content-Type", constants.HTTPContentType)
	var registerResponse types.OpenSearchOperationResponse
	if error {
		w.WriteHeader(http.StatusGatewayTimeout)
		registerResponse.Acknowledged = false
	} else {
		w.WriteHeader(http.StatusOK)
		registerResponse.Acknowledged = true
	}

	json.NewEncoder(w).Encode(registerResponse)
}

func mockReloadOpensearchSecureSettings(error bool, w http.ResponseWriter, r *http.Request) {
	fmt.Println("Reload secure settings")
	w.Header().Add("Content-Type", constants.HTTPContentType)
	var reloadsettings types.OpenSearchSecureSettingsReloadStatus
	w.WriteHeader(http.StatusOK)
	reloadsettings.ClusterNodes.Total = 3
	if error {
		reloadsettings.ClusterNodes.Failed = 1
		reloadsettings.ClusterNodes.Successful = 3
	} else {
		reloadsettings.ClusterNodes.Failed = 0
		reloadsettings.ClusterNodes.Successful = 3
	}
	json.NewEncoder(w).Encode(reloadsettings)
}

func mockTriggerSnapshotRepository(error bool, w http.ResponseWriter, r *http.Request) {
	fmt.Println("Snapshot ...")
	w.Header().Add("Content-Type", constants.HTTPContentType)

	if error {
		w.WriteHeader(http.StatusGatewayTimeout)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	switch r.Method {
	case http.MethodPost:
		var triggerSnapshot types.OpenSearchSnapshotResponse
		triggerSnapshot.Accepted = true
		json.NewEncoder(w).Encode(triggerSnapshot)

	case http.MethodGet:
		var snapshotInfo types.OpenSearchSnapshotStatus
		var snapshots []types.Snapshot
		var snapshot types.Snapshot
		snapshot.Snapshot = "foo"
		snapshot.State = constants.OpenSearchSnapShotSuccess
		snapshot.Indices = []string{"alpha", "beta", "gamma"}
		snapshot.DataStreams = []string{"mono", "di", "tri"}
		snapshots = append(snapshots, snapshot)
		snapshotInfo.Snapshots = snapshots
		json.NewEncoder(w).Encode(snapshotInfo)
	}

}

func mockRestoreProgress(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Restore progress ...")
	w.Header().Add("Content-Type", constants.HTTPContentType)
	w.WriteHeader(http.StatusOK)
	var dsInfo types.OpenSearchDataStreams
	var arrayDs []types.DataStreams
	var ds types.DataStreams
	ds.Name = "foo"
	ds.Status = constants.DataStreamGreen
	arrayDs = append(arrayDs, ds)
	ds.Name = "bar"
	arrayDs = append(arrayDs, ds)
	dsInfo.DataStreams = arrayDs
	json.NewEncoder(w).Encode(dsInfo)

}

func TestMain(m *testing.M) {
	log, f := logHelper()
	defer os.Remove(f)

	conData := types.ConnectionData{
		BackupName:    "mango",
		VeleroTimeout: "1s",
	}

	fmt.Println("Starting mock server")
	httpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case "/":
			mockEnsureOpenSearchIsReachable(false, w, r)
		case healthURL:
			mockEnsureOpenSearchIsHealthy(false, w, r)
		case fmt.Sprintf("%s/%s", snapshotURL, constants.OpenSearchSnapShotRepoName), fmt.Sprintf("%s/*", dataStreamsURL), "/*":
			mockOpenSearchOperationResponse(false, w, r)
		case secureSettingsURL:
			mockReloadOpensearchSecureSettings(false, w, r)
		case fmt.Sprintf("/_snapshot/%s/%s", constants.OpenSearchSnapShotRepoName, "mango"), fmt.Sprintf("/_snapshot/%s/%s/_restore", constants.OpenSearchSnapShotRepoName, "mango"):
			mockTriggerSnapshotRepository(false, w, r)
		case dataStreamsURL:
			mockRestoreProgress(w, r)

		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer httpServer.Close()

	fmt.Println("mock opensearch handler")
	openSearch = opensearch.New(httpServer.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)

	fmt.Println("Start tests")
	m.Run()
}

// Test_EnsureOpenSearchIsReachable tests the EnsureOpenSearchIsReachable method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with OpenSearch URL
// THEN verifies whether OpenSearch is reachable or not
func Test_EnsureOpenSearchIsReachable(t *testing.T) {
	log, f := logHelper()
	defer os.Remove(f)

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case "/":
			mockEnsureOpenSearchIsReachable(false, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server1.Close()

	conData := types.ConnectionData{
		BackupName:    "mango",
		VeleroTimeout: "1s",
	}
	o := opensearch.New(server1.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err := o.EnsureOpenSearchIsReachable()
	assert.Nil(t, err)

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case "/":
			mockEnsureOpenSearchIsReachable(true, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server2.Close()

	o = opensearch.New(server2.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err = o.EnsureOpenSearchIsReachable()
	assert.Nil(t, err)

}

// Test_EnsureOpenSearchIsHealthy tests the EnsureOpenSearchIsHealthy method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with snapshot name
// THEN checks if opensearch cluster is healthy
func Test_EnsureOpenSearchIsHealthy(t *testing.T) {
	log, f := logHelper()
	defer os.Remove(f)

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case healthURL:
			mockEnsureOpenSearchIsHealthy(false, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server1.Close()

	conData := types.ConnectionData{
		BackupName:    "mango",
		VeleroTimeout: "1s",
	}
	o := opensearch.New(server1.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err := o.EnsureOpenSearchIsHealthy()
	assert.Nil(t, err)

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case healthURL:
			mockEnsureOpenSearchIsHealthy(true, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server2.Close()

	o = opensearch.New(server2.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err = o.EnsureOpenSearchIsHealthy()
	assert.NotNil(t, err)
}

// Test_EnsureOpenSearchIsHealthy tests the EnsureOpenSearchIsHealthy method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with snapshot name
// THEN checks if opensearch cluster is healthy
func Test_RegisterSnapshotRepository(t *testing.T) {
	log, f := logHelper()
	defer os.Remove(f)

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case fmt.Sprintf("%s/%s", snapshotURL, constants.OpenSearchSnapShotRepoName):
			mockOpenSearchOperationResponse(false, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server1.Close()

	var objsecret types.ObjectStoreSecret
	objsecret.SecretName = "alpha"
	objsecret.SecretKey = "cloud"
	objsecret.ObjectAccessKey = "alphalapha"
	objsecret.ObjectSecretKey = "betabetabeta"

	conData := types.ConnectionData{
		BackupName:    "mango",
		VeleroTimeout: "1s",
		RegionName:    "region",
		Endpoint:      constants.OpenSearchURL,
		Secret:        objsecret,
	}

	o := opensearch.New(server1.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err := o.RegisterSnapshotRepository()
	assert.Nil(t, err)

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case fmt.Sprintf("%s/%s", snapshotURL, constants.OpenSearchSnapShotRepoName):
			mockOpenSearchOperationResponse(true, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server2.Close()

	o = opensearch.New(server2.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err = o.RegisterSnapshotRepository()
	assert.NotNil(t, err)

}

// Test_ReloadOpensearchSecureSettings tests the ReloadOpensearchSecureSettings method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with snapshot name
// THEN updates opensearch keystore creds
func Test_ReloadOpensearchSecureSettings(t *testing.T) {
	log, f := logHelper()
	defer os.Remove(f)
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case secureSettingsURL:
			mockReloadOpensearchSecureSettings(false, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server1.Close()

	conData := types.ConnectionData{
		BackupName:    "mango",
		VeleroTimeout: "1s",
		RegionName:    "region",
	}
	o := opensearch.New(server1.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err := o.ReloadOpensearchSecureSettings()
	assert.Nil(t, err)

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case secureSettingsURL:
			mockReloadOpensearchSecureSettings(true, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server2.Close()
	o = opensearch.New(server2.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err = o.ReloadOpensearchSecureSettings()
	assert.NotNil(t, err)
}

// TestTriggerSnapshot tests the TriggerSnapshot method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with snapshot name
// THEN creates a snapshot in object store
func Test_TriggerSnapshot(t *testing.T) {
	log, f := logHelper()
	defer os.Remove(f)

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case fmt.Sprintf("%s/%s/%s", snapshotURL, constants.OpenSearchSnapShotRepoName, "mango"):
			mockTriggerSnapshotRepository(false, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server1.Close()

	conData := types.ConnectionData{
		BackupName:    "mango",
		VeleroTimeout: "1s",
		RegionName:    "region",
	}
	o := opensearch.New(server1.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err := o.TriggerSnapshot()
	assert.Nil(t, err)

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case fmt.Sprintf("%s/%s/%s", snapshotURL, constants.OpenSearchSnapShotRepoName, "mango"):
			mockTriggerSnapshotRepository(true, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server2.Close()
	o = opensearch.New(server2.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err = o.TriggerSnapshot()
	assert.NotNil(t, err)

}

// TestCheckSnapshotProgress tests the CheckSnapshotProgress method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with snapshot name
// THEN tracks snapshot progress towards completion
func TestCheckSnapshotProgress(t *testing.T) {
	log, f := logHelper()
	defer os.Remove(f)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case fmt.Sprintf("%s/%s/%s", snapshotURL, constants.OpenSearchSnapShotRepoName, "mango"):
			mockTriggerSnapshotRepository(false, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server.Close()

	conData := types.ConnectionData{
		BackupName:    "mango",
		VeleroTimeout: "1s",
		RegionName:    "region",
	}
	o := opensearch.New(server.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err := o.CheckSnapshotProgress()
	assert.Nil(t, err)
}

// Test_DeleteDataStreams tests the DeleteData method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with logger
// THEN deletes data from Opensearch cluster
func Test_DeleteDataStreams(t *testing.T) {
	log, f := logHelper()
	defer os.Remove(f)

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case fmt.Sprintf("%s/*", dataStreamsURL), "/*":
			mockOpenSearchOperationResponse(false, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server1.Close()

	conData := types.ConnectionData{
		BackupName:    "mango",
		VeleroTimeout: "1s",
		RegionName:    "region",
	}
	o := opensearch.New(server1.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err := o.DeleteData()
	assert.Nil(t, err)

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case fmt.Sprintf("%s/*", dataStreamsURL), "/*":
			mockOpenSearchOperationResponse(true, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server2.Close()

	o = opensearch.New(server2.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err = o.DeleteData()
	assert.NotNil(t, err)
}

// Test_TriggerSnapshot tests the TriggerRestore method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with snapshot name
// THEN creates a restore from object store from given snapshot name
func Test_TriggerRestore(t *testing.T) {
	log, f := logHelper()
	defer os.Remove(f)

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case fmt.Sprintf("%s/%s/%s/_restore", snapshotURL, constants.OpenSearchSnapShotRepoName, "mango"):
			mockTriggerSnapshotRepository(false, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server1.Close()

	conData := types.ConnectionData{
		BackupName:    "mango",
		VeleroTimeout: "1s",
		RegionName:    "region",
	}
	o := opensearch.New(server1.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err := o.TriggerRestore()
	assert.Nil(t, err)

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case fmt.Sprintf("%s/%s/%s/_restore", snapshotURL, constants.OpenSearchSnapShotRepoName, "mango"):
			mockTriggerSnapshotRepository(true, w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server2.Close()
	o = opensearch.New(server2.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err = o.TriggerRestore()
	assert.NotNil(t, err)
}

// Test_CheckRestoreProgress tests the CheckRestoreProgress method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with snapshot name
// THEN tracks snapshot restore towards completion
func Test_CheckRestoreProgress(t *testing.T) {
	log, f := logHelper()
	defer os.Remove(f)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case dataStreamsURL:
			mockRestoreProgress(w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	defer server.Close()

	conData := types.ConnectionData{
		BackupName:    "mango",
		VeleroTimeout: "1s",
		RegionName:    "region",
	}
	o := opensearch.New(server.URL, timeOutGlobal, http.DefaultClient, &conData, log, fakeBasicAuth)
	err := o.CheckRestoreProgress()
	assert.Nil(t, err)
}

// Test_Backup tests the Backup method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with snapshot name
// THEN takes the opensearch backup
func Test_Backup(t *testing.T) {
	err := openSearch.Backup()
	assert.Nil(t, err)
}

// Test_Restore tests the Restore method for the following use case.
// GIVEN OpenSearch object
// WHEN invoked with snapshot name
// THEN restores the opensearch from a given backup
func Test_Restore(t *testing.T) {
	err := openSearch.Restore()
	assert.Nil(t, err)
}

// Copyright (c) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/types"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/utilities"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

// HTTPHelper supports net/http calls of type GET/POST/DELETE
func (o *OpensearchImpl) HTTPHelper(ctx context.Context, method, requestURL string, body io.Reader, data interface{}) error {
	o.Log.Debugf("Invoking HTTP '%s' request with url '%s'", method, requestURL)
	var response *http.Response
	var request *http.Request
	var err error
	//client := &http.Client{}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	switch method {
	case "GET":
		request, err = http.NewRequestWithContext(ctx, http.MethodGet, requestURL, body)
	case "POST":
		request, err = http.NewRequestWithContext(ctx, http.MethodPost, requestURL, body)
	case "DELETE":
		request, err = http.NewRequestWithContext(ctx, http.MethodDelete, requestURL, body)
	}
	if err != nil {
		o.Log.Error("Error creating request ", zap.Error(err))
		return err
	}

	request.Header.Add("Content-Type", constants.HTTPContentType)
	response, err = o.Client.Do(request)
	if err != nil {
		o.Log.Errorf("HTTP '%s' failure while invoking url '%s' due to '%v'", method, requestURL, zap.Error(err))
		return err
	}
	defer response.Body.Close()

	bdata, err := ioutil.ReadAll(response.Body)
	if err != nil {
		o.Log.Errorf("Unable to read response body ", zap.Error(err))
		return err
	}

	if response.StatusCode != 200 {
		o.Log.Errorf("Response code is not 200 OK!. Actual response code '%v' with response body '%v'", response.StatusCode, string(bdata))
		return err
	}

	err = json.Unmarshal(bdata, &data)
	if err != nil {
		o.Log.Errorf("json unmarshalling error %v", err)
		return err
	}

	return nil
}

// EnsureOpenSearchIsReachable is used determine whether OpenSearch cluster is reachable
func (o *OpensearchImpl) EnsureOpenSearchIsReachable() error {
	o.Log.Infof("Checking if cluster is reachable")
	var osinfo types.OpenSearchClusterInfo
	done := false
	var timeSeconds float64

	if utilities.GetEnvWithDefault(constants.DevKey, constants.FalseString) == constants.TruthString {
		// if UT flag is set, skip to avoid retry logic
		return nil
	}

	timeParse, err := time.ParseDuration(o.SecretData.Timeout)
	if err != nil {
		o.Log.Errorf("Unable to parse time duration ", zap.Error(err))
		return err
	}
	totalSeconds := timeParse.Seconds()

	for !done {
		err := o.HTTPHelper(context.Background(), "GET", o.BaseURL, nil, &osinfo)
		if err != nil {
			if timeSeconds < totalSeconds {
				message := "Cluster is not reachable"
				duration, err := utilities.WaitRandom(message, o.SecretData.Timeout, o.Log)
				if err != nil {
					return err
				}
				timeSeconds = timeSeconds + float64(duration)
			} else {
				o.Log.Errorf("Timeout '%s' exceeded. Cluster not reachable", o.SecretData.Timeout)
				return err
			}
		} else {
			done = true
		}
	}

	o.Log.Infof("Cluster '%s' is reachable", osinfo.ClusterName)

	return nil
}

//EnsureOpenSearchIsHealthy ensures OpenSearch cluster is healthy
func (o *OpensearchImpl) EnsureOpenSearchIsHealthy() error {
	o.Log.Infof("Checking if cluster is healthy")
	var clusterHealth types.OpenSearchHealthResponse
	err := o.EnsureOpenSearchIsReachable()
	if err != nil {
		return err
	}

	healthURL := fmt.Sprintf("%s/_cluster/health", o.BaseURL)
	healthReachable := false
	var timeSeconds float64

	timeParse, err := time.ParseDuration(o.SecretData.Timeout)
	if err != nil {
		o.Log.Errorf("Unable to parse time duration ", zap.Error(err))
		return err
	}
	totalSeconds := timeParse.Seconds()

	if utilities.GetEnvWithDefault(constants.DevKey, constants.FalseString) == constants.TruthString {
		// if UT flag is set, skip to avoid retry logic
		return nil
	}

	for !healthReachable {
		err = o.HTTPHelper(context.Background(), "GET", healthURL, nil, &clusterHealth)
		if err != nil {
			if timeSeconds < totalSeconds {
				message := "Cluster health endpoint is not reachable"
				duration, err := utilities.WaitRandom(message, o.SecretData.Timeout, o.Log)
				if err != nil {
					return err
				}
				timeSeconds = timeSeconds + float64(duration)
			} else {
				o.Log.Errorf("Timeout '%s' exceeded. Cluster health endpoint is not reachable", o.SecretData.Timeout)
				return err
			}
		} else {
			o.Log.Infof("Cluster health endpoint is reachable now")
			healthReachable = true
		}
	}

	healthGreen := false

	for !healthGreen {
		err = o.HTTPHelper(context.Background(), "GET", healthURL, nil, &clusterHealth)
		if err != nil {
			if timeSeconds < totalSeconds {
				message := "Json unmarshalling error"
				duration, err := utilities.WaitRandom(message, o.SecretData.Timeout, o.Log)
				if err != nil {
					return err
				}
				timeSeconds = timeSeconds + float64(duration)
				continue
			} else {
				return fmt.Errorf("Timeout '%s' exceeded. Json unmarshalling error while checking cluster health %v", o.SecretData.Timeout, zap.Error(err))
			}
		}

		if clusterHealth.Status != "green" {
			if timeSeconds < totalSeconds {
				message := fmt.Sprintf("Cluster health is '%s'", clusterHealth.Status)
				duration, err := utilities.WaitRandom(message, o.SecretData.Timeout, o.Log)
				if err != nil {
					return err
				}
				timeSeconds = timeSeconds + float64(duration)
			} else {
				return fmt.Errorf("Timeout '%s' exceeded. Cluster health expected 'green' , current state '%s'", o.SecretData.Timeout, clusterHealth.Status)
			}
		} else {
			healthGreen = true
		}
	}

	if healthReachable && healthGreen {
		o.Log.Infof("Cluster is reachable and healthy with status as '%s'", clusterHealth.Status)
		return nil
	}

	return err
}

// ReloadOpensearchSecureSettings used to reload secure settings once object store keys are updated
func (o *OpensearchImpl) ReloadOpensearchSecureSettings() error {
	var secureSettings types.OpenSearchSecureSettingsReloadStatus
	url := fmt.Sprintf("%s/_nodes/reload_secure_settings", o.BaseURL)

	err := o.HTTPHelper(context.Background(), "POST", url, nil, &secureSettings)
	if err != nil {
		return err
	}

	if secureSettings.ClusterNodes.Failed == 0 && secureSettings.ClusterNodes.Total == 0 && secureSettings.ClusterNodes.Successful == 0 {
		return fmt.Errorf("Invalid cluster settings detected. Check the connection")
	}

	if secureSettings.ClusterNodes.Failed == 0 && secureSettings.ClusterNodes.Total == secureSettings.ClusterNodes.Successful {
		o.Log.Infof("Secure settings reloaded sucessfully across all '%v' nodes of the cluster", secureSettings.ClusterNodes.Total)
		return nil
	}
	return fmt.Errorf("Not all nodes were updated successfully. Total = '%v', Failed = '%v' , Successful = '%v'", secureSettings.ClusterNodes.Total, secureSettings.ClusterNodes.Failed, secureSettings.ClusterNodes.Successful)
}

// RegisterSnapshotRepository registers an object store with OpenSearch using the s3-plugin
func (o *OpensearchImpl) RegisterSnapshotRepository() error {
	o.Log.Infof("Registering s3 backend repository '%s'", constants.OpenSearchSnapShotRepoName)
	var snapshotPayload types.OpenSearchSnapshotRequestPayload
	var registerResponse types.OpenSearchOperationResponse
	snapshotPayload.Type = "s3"
	snapshotPayload.Settings.Bucket = o.SecretData.BucketName
	snapshotPayload.Settings.Region = o.SecretData.RegionName
	snapshotPayload.Settings.Client = "default"
	snapshotPayload.Settings.Endpoint = o.SecretData.Endpoint
	snapshotPayload.Settings.PathStyleAccess = true

	postBody, err := json.Marshal(snapshotPayload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/_snapshot/%s", o.BaseURL, constants.OpenSearchSnapShotRepoName)

	err = o.HTTPHelper(context.Background(), "POST", url, bytes.NewBuffer(postBody), &registerResponse)
	if err != nil {
		return err
	}

	if registerResponse.Acknowledged {
		o.Log.Infof("Snapshot registered successfully !")
		return nil
	}
	return fmt.Errorf("Snapshot registration unsuccessful. Response = %v", registerResponse)
}

// TriggerSnapshot this triggers a snapshot/backup of all the data streams/indices
func (o *OpensearchImpl) TriggerSnapshot() error {
	o.Log.Infof("Triggering snapshot with name '%s'", o.SecretData.BackupName)
	var snapshotResponse types.OpenSearchSnapshotResponse
	snapShotURL := fmt.Sprintf("%s/_snapshot/%s/%s", o.BaseURL, constants.OpenSearchSnapShotRepoName, o.SecretData.BackupName)

	err := o.HTTPHelper(context.Background(), "POST", snapShotURL, nil, &snapshotResponse)
	if err != nil {
		return err
	}

	if !snapshotResponse.Accepted {
		return fmt.Errorf("Snapshot registration failure. Response = %v ", snapshotResponse)
	}
	o.Log.Infof("Snapshot registered successfully !")
	return nil
}

// CheckSnapshotProgress checks the data backup progress.
func (o *OpensearchImpl) CheckSnapshotProgress() error {
	o.Log.Infof("Checking snapshot progress with name '%s'", o.SecretData.BackupName)
	snapShotURL := fmt.Sprintf("%s/_snapshot/%s/%s", o.BaseURL, constants.OpenSearchSnapShotRepoName, o.SecretData.BackupName)
	var snapshotInfo types.OpenSearchSnapshotStatus

	if utilities.GetEnvWithDefault(constants.DevKey, constants.FalseString) == constants.TruthString {
		// if UT flag is set, skip to avoid retry logic
		return nil
	}

	var timeSeconds float64
	timeParse, err := time.ParseDuration(o.SecretData.Timeout)
	if err != nil {
		o.Log.Errorf("Unable to parse time duration ", zap.Error(err))
		return err
	}
	totalSeconds := timeParse.Seconds()

	done := false
	for !done {
		err := o.HTTPHelper(context.Background(), "GET", snapShotURL, nil, &snapshotInfo)
		if err != nil {
			return err
		}
		switch snapshotInfo.Snapshots[0].State {
		case constants.OpenSearchSnapShotInProgress:
			if timeSeconds < totalSeconds {
				message := fmt.Sprintf("Snapshot '%s' is in progress", o.SecretData.BackupName)
				duration, err := utilities.WaitRandom(message, o.SecretData.Timeout, o.Log)
				if err != nil {
					return err
				}
				timeSeconds = timeSeconds + float64(duration)
			} else {
				return fmt.Errorf("Timeout '%s' exceeded. Snapshot '%s' state is still IN_PROGRESS", o.SecretData.Timeout, o.SecretData.BackupName)
			}
		case constants.OpenSearchSnapShotSuccess:
			o.Log.Infof("Snapshot '%s' complete", o.SecretData.BackupName)
			done = true
		default:
			return fmt.Errorf("Snapshot '%s' state is invalid '%s'", o.SecretData.BackupName, snapshotInfo.Snapshots[0].State)
		}
	}

	o.Log.Infof("Number of shards backed up = %v", snapshotInfo.Snapshots[0].Shards.Total)
	o.Log.Infof("Number of successfull shards backed up = %v", snapshotInfo.Snapshots[0].Shards.Total)
	o.Log.Infof("Indices backed up = %v", snapshotInfo.Snapshots[0].Indices)
	o.Log.Infof("Data streams backed up = %v", snapshotInfo.Snapshots[0].DataStreams)

	return nil
}

// DeleteData used to delete data streams before restore.
func (o *OpensearchImpl) DeleteData() error {
	o.Log.Infof("Deleting data streams followed by index ..")
	dataStreamURL := fmt.Sprintf("%s/_data_stream/*", o.BaseURL)
	dataIndexURL := fmt.Sprintf("%s/*", o.BaseURL)
	var deleteResponse types.OpenSearchOperationResponse

	err := o.HTTPHelper(context.Background(), "DELETE", dataStreamURL, nil, &deleteResponse)
	if err != nil {
		return err
	}

	if !deleteResponse.Acknowledged {
		return fmt.Errorf("Data streams deletion failure. Response = %v ", deleteResponse)
	}

	err = o.HTTPHelper(context.Background(), "DELETE", dataIndexURL, nil, &deleteResponse)
	if err != nil {
		return err
	}

	if !deleteResponse.Acknowledged {
		return fmt.Errorf("Data index deletion failure. Response = %v ", deleteResponse)
	}

	o.Log.Infof("Data streams and data indexes deleted successfully !")
	return nil
}

// TriggerRestore Triggers a restore from a specified snapshot
func (o *OpensearchImpl) TriggerRestore() error {
	o.Log.Infof("Triggering restore with name '%s'", o.SecretData.BackupName)
	restoreURL := fmt.Sprintf("%s/_snapshot/%s/%s/_restore", o.BaseURL, constants.OpenSearchSnapShotRepoName, o.SecretData.BackupName)
	var restoreResponse types.OpenSearchSnapshotResponse

	err := o.HTTPHelper(context.Background(), "POST", restoreURL, nil, &restoreResponse)
	if err != nil {
		return err
	}

	if !restoreResponse.Accepted {
		return fmt.Errorf("Snapshot restore trigger failed. Response = %v ", restoreResponse)
	}
	o.Log.Infof("Snapshot restore triggered successfully !")
	return nil
}

// CheckRestoreProgress checks progress of restore process, by monitoring all the data streams
func (o *OpensearchImpl) CheckRestoreProgress() error {
	o.Log.Infof("Checking restore progress with name '%s'", o.SecretData.BackupName)
	dsURL := fmt.Sprintf("%s/_data_stream", o.BaseURL)
	var snapshotInfo types.OpenSearchDataStreams

	if utilities.GetEnvWithDefault(constants.DevKey, constants.FalseString) == constants.TruthString {
		// if UT flag is set, skip to avoid retry logic
		return nil
	}

	var timeSeconds float64
	timeParse, err := time.ParseDuration(o.SecretData.Timeout)
	if err != nil {
		o.Log.Errorf("Unable to parse time duration ", zap.Error(err))
		return err
	}
	totalSeconds := timeParse.Seconds()
	done := false
	notGreen := false

	for !done {
		err := o.HTTPHelper(context.Background(), "GET", dsURL, nil, &snapshotInfo)
		if err != nil {
			return err
		}
		for _, ds := range snapshotInfo.DataStreams {
			o.Log.Infof("Data stream '%s' restore status '%s'", ds.Name, ds.Status)
			switch ds.Status {
			case constants.DataStreamGreen:
				o.Log.Infof("Data stream '%s' restore complete", ds.Name)
			default:
				notGreen = true
			}
		}

		if notGreen {
			if timeSeconds < totalSeconds {
				message := "Restore is in progress"
				duration, err := utilities.WaitRandom(message, o.SecretData.Timeout, o.Log)
				if err != nil {
					return err
				}
				timeSeconds = timeSeconds + float64(duration)
				notGreen = false
			} else {
				return fmt.Errorf("Timeout '%s' exceeded. Restore '%s' state is still IN_PROGRESS", o.SecretData.Timeout, o.SecretData.BackupName)
			}
		} else {
			// This section is hit when all data streams are green
			// exit feedback loop
			done = true
		}

	}

	o.Log.Infof("All streams are healthy")
	return nil
}

// Backup - Toplevel method to invoke OpenSearch backup
func (o *OpensearchImpl) Backup() error {
	o.Log.Info("Start backup steps ....")
	err := o.RegisterSnapshotRepository()
	if err != nil {
		return err
	}

	err = o.TriggerSnapshot()
	if err != nil {
		return err
	}

	err = o.CheckSnapshotProgress()
	if err != nil {
		return err
	}

	return nil
}

// Restore - Top level method to invoke opensearch restore
func (o *OpensearchImpl) Restore() error {
	o.Log.Info("Start restore steps ....")
	err := o.RegisterSnapshotRepository()
	if err != nil {
		return err
	}

	err = o.DeleteData()
	if err != nil {
		return err
	}

	err = o.TriggerRestore()
	if err != nil {
		return err
	}

	err = o.CheckRestoreProgress()
	if err != nil {
		return err
	}

	return nil
}

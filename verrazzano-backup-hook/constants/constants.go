// Copyright (c) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package constants

// General constants
const (
	// VerrazzanoSystemNamespace is the Namespace where Opensearch components are installed
	VerrazzanoSystemNamespace = "verrazzano-system"

	// VerrazzanoLoggingNamespace is the namespace where new Opensearch components are installed
	VerrazzanoLoggingNamespace = "verrazzano-logging"

	// VerrazzanoNameSpaceName Namespace where Velero components are installed
	VeleroNameSpace = "verrazzano-backup"

	// BackupOperation backup operation expected value
	BackupOperation = "backup"

	// RestoreOperation restore operation expected value
	RestoreOperation = "restore"

	// Min value used in WaitRandom
	Min = 10

	// Max value used in WaitRandom
	Max = 25

	// DevKey used in setting env values for dev
	DevKey = "dev"

	// TrueString is expected value for true
	TrueString = "true"

	// FalseString is expected value for false
	FalseString = "false"

	// RetryCount Default retry count for various operations
	RetryCount = 50

	// OpenSearchHealthCheckTimeoutKey Env key for Opensearch health check
	OpenSearchHealthCheckTimeoutKey = "HEALTH_CHECK"

	// OpenSearchHealthCheckTimeoutDefaultValue Env value for key OpenSearchHealthCheckTimeoutKey for Opensearch health check
	OpenSearchHealthCheckTimeoutDefaultValue = "10m"

	// DisableSecurityPluginOS Env key to disable Security Plugin
	DisableSecurityPluginOS = "DISABLE_SECURITY_PLUGIN"
)

const (
	// AwsAccessKeyString AWS access key id string
	AwsAccessKeyString = "aws_access_key_id" //nolint:gosec //#gosec G101

	// AwsSecretAccessKeyString AWS secret access key id string
	AwsSecretAccessKeyString = "aws_secret_access_key" //nolint:gosec //#gosec G101
)

// OpenSearch constants
const (
	// OpenSearchURL Opensearch url used internally
	OpenSearchURL = "http://127.0.0.1:9200"

	// OpenSearchDataPodContainerName Opensearch data pod container name
	OpenSearchDataPodContainerName = "es-data"

	// OpenSearchMasterPodContainerName Opensearch master pod container name
	OpenSearchMasterPodContainerName = "es-master"

	OpenSearchClusterName = "opensearch"

	// HTTPContentType content type in http request/response
	HTTPContentType = "application/json"

	// OpenSearchSnapShotRepoName Opensearch snapshot name in remote repository
	OpenSearchSnapShotRepoName = "verrazzano-backup"

	// IngestDeploymentName Opensearch ingest deployment name
	IngestDeploymentName = "vmi-system-es-ingest"

	// IngestLabelSelector Opensearch ingest pod label selector
	IngestLabelSelector = "app=system-es-ingest"
	// KibanaDeploymentName Kibana deployment name
	KibanaDeploymentName = "vmi-system-osd"

	// KibanaLabelSelector Label selector for Kibana pod
	KibanaLabelSelector = "app=system-osd"

	// KibanaDeploymentLabelSelector Kibana deployment label selector
	KibanaDeploymentLabelSelector = "verrazzano-component=osd"

	// VMODeploymentName Deployment name for Verrazzano Monitoring Operator
	VMODeploymentName = "verrazzano-monitoring-operator"

	// VMOLabelSelector Label selector for Verrazzano Monitoring Operator
	VMOLabelSelector = "k8s-app=verrazzano-monitoring-operator"

	// OpenSearchSnapShotSuccess Success status message expected value
	OpenSearchSnapShotSuccess = "SUCCESS"

	// OpenSearchSnapShotInProgress In progress status message expected value
	OpenSearchSnapShotInProgress = "IN_PROGRESS"

	// DataStreamGreen Data stream green status expected value
	DataStreamGreen = "GREEN"

	// OpenSearchKeystoreAccessKeyCmd Opensearch cmd to add s3 access key
	OpenSearchKeystoreAccessKeyCmd = "/usr/share/opensearch/bin/opensearch-keystore add --stdin --force s3.client.default.access_key" //nolint:gosec //#nosec G204

	// OpenSearchKeystoreSecretAccessKeyCmd Opensearch cmd to add s3 secret access key
	OpenSearchKeystoreSecretAccessKeyCmd = "/usr/share/opensearch/bin/opensearch-keystore add --stdin --force s3.client.default.secret_key" //nolint:gosec //#nosec G204

	// OpenSearchMasterLabel Label selector for OpenSearch master pods
	OpenSearchMasterLabel = "opensearch.verrazzano.io/role-master=true"

	// OpenSearchDataLabel Label selector for OpenSearch data pods
	OpenSearchDataLabel = "opensearch.verrazzano.io/role-data=true"
)

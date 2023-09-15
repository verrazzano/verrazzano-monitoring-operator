// Copyright (c) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	vmov1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/constants"
	model "github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/types"
	futil "github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/utilities"
	vmofake "github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/utilities/k8s/fake"
	"go.uber.org/zap"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"sync"
	"time"
)

// PopulateConnData creates the connection object that's used to communicate to object store.
func (k *K8sImpl) PopulateConnData(veleroNamespace, backupName string) (*model.ConnectionData, error) {
	k.Log.Infof("Populating connection data from backup '%v' in namespace '%s'", backupName, veleroNamespace)

	backup, err := k.GetBackup(veleroNamespace, backupName)
	if err != nil {
		return nil, err
	}

	if backup.Spec.StorageLocation == "default" {
		k.Log.Infof("Default creds not supported. Custom credentaisl needs to be created before creating backup storage location")
		return nil, err
	}

	k.Log.Infof("Detected Velero backup storage location '%s' in namespace '%s' used by backup '%s'", backup.Spec.StorageLocation, veleroNamespace, backupName)
	bsl, err := k.GetBackupStorageLocation(veleroNamespace, backup.Spec.StorageLocation)
	if err != nil {
		return nil, err
	}

	secretData, err := k.GetObjectStoreCreds(bsl.Spec.Credential.Name, bsl.Metadata.Namespace, bsl.Spec.Credential.Key)
	if err != nil {
		return nil, err
	}

	var conData model.ConnectionData
	conData.Secret = *secretData
	conData.RegionName = bsl.Spec.Config.Region
	conData.Endpoint = bsl.Spec.Config.S3URL
	conData.BucketName = bsl.Spec.ObjectStorage.Bucket
	conData.BackupName = backupName
	// For now, we will look at the first POST hook in the first Hook in Velero Backup
	conData.VeleroTimeout = backup.Spec.Hooks.Resources[0].Post[0].Exec.Timeout

	return &conData, nil

}

// GetObjectStoreCreds fetches credentials from Velero Backup object store location.
// This object will be pre-created before the execution of this hook
func (k *K8sImpl) GetObjectStoreCreds(secretName, namespace, secretKey string) (*model.ObjectStoreSecret, error) {
	secret := v1.Secret{}
	if err := k.K8sClient.Get(context.TODO(), crtclient.ObjectKey{Name: secretName, Namespace: namespace}, &secret); err != nil {
		k.Log.Errorf("Failed to retrieve secret '%s' due to : %v", secretName, err)
		return nil, err
	}

	file, err := futil.CreateTempFileWithData(secret.Data[secretKey])
	if err != nil {
		return nil, err
	}
	defer os.Remove(file)

	accessKey, secretAccessKey, err := futil.ReadTempCredsFile(file, k.CredentialProfile)
	if err != nil {
		k.Log.Error("Error while reading creds from file ", zap.Error(err))
		return nil, err
	}

	var secretData model.ObjectStoreSecret
	secretData.SecretName = secretName
	secretData.SecretKey = secretKey
	secretData.ObjectAccessKey = accessKey
	secretData.ObjectSecretKey = secretAccessKey
	return &secretData, nil
}

// GetBackupStorageLocation retrieves data from the Velero backup storage location
func (k *K8sImpl) GetBackupStorageLocation(veleroNamespace, bslName string) (*model.VeleroBackupStorageLocation, error) {
	k.Log.Infof("Fetching Velero backup storage location '%s' in namespace '%s'", bslName, veleroNamespace)
	gvr := schema.GroupVersionResource{
		Group:    "velero.io",
		Version:  "v1",
		Resource: "backupstoragelocations",
	}
	bslRecievd, err := k.DynamicK8sInterface.Resource(gvr).Namespace(veleroNamespace).Get(context.Background(), bslName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if bslRecievd == nil {
		k.Log.Infof("No Velero backup storage location in namespace '%s' was detected", veleroNamespace)
		return nil, err
	}

	var bsl model.VeleroBackupStorageLocation
	bdata, err := json.Marshal(bslRecievd)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bdata, &bsl)
	if err != nil {
		return nil, err
	}
	return &bsl, nil
}

// GetBackup Retrieves Velero backup object from the cluster
func (k *K8sImpl) GetBackup(veleroNamespace, backupName string) (*model.VeleroBackup, error) {
	k.Log.Infof("Fetching Velero backup '%s' in namespace '%s'", backupName, veleroNamespace)
	gvr := schema.GroupVersionResource{
		Group:    "velero.io",
		Version:  "v1",
		Resource: "backups",
	}
	backupFetched, err := k.DynamicK8sInterface.Resource(gvr).Namespace(veleroNamespace).Get(context.Background(), backupName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if backupFetched == nil {
		k.Log.Infof("No Velero backup in namespace '%s' was detected", veleroNamespace)
		return nil, err
	}

	var backup model.VeleroBackup
	bdata, err := json.Marshal(backupFetched)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bdata, &backup)
	if err != nil {
		return nil, err
	}
	return &backup, nil
}

// ScaleDeployment is used to scale a deployment to specific replica count
// labelSelectors, namespace, deploymentName are used to identify deployments
// and specific pods associated with them.
func (k *K8sImpl) ScaleDeployment(labelSelector, namespace, deploymentName string, replicaCount int32) error {
	k.Log.Infof("Scale deployment '%s' in namespace '%s", deploymentName, namespace)
	var wg sync.WaitGroup
	depPatch := apps.Deployment{}
	if err := k.K8sClient.Get(context.TODO(), types.NamespacedName{Name: deploymentName, Namespace: namespace}, &depPatch); err != nil {
		return err
	}
	currentValue := *depPatch.Spec.Replicas
	desiredValue := replicaCount

	if desiredValue == currentValue {
		k.Log.Infof("Deployment scaling skipped as desired replicas is same as current replicas")
		return nil
	}

	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	pods, err := k.K8sInterface.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return err
	}
	wg.Add(len(pods.Items))

	mergeFromDep := client.MergeFrom(depPatch.DeepCopy())
	depPatch.Spec.Replicas = &replicaCount
	if err := k.K8sClient.Patch(context.TODO(), &depPatch, mergeFromDep); err != nil {
		k.Log.Error("Unable to patch !!")
		return err
	}

	timeout := futil.GetEnvWithDefault(constants.OpenSearchHealthCheckTimeoutKey, constants.OpenSearchHealthCheckTimeoutDefaultValue)

	if desiredValue > currentValue {
		//log.Info("Scaling up pods ...")
		message := "Wait for pods to come up"
		_, err := futil.WaitRandom(message, timeout, k.Log)
		if err != nil {
			return err
		}

		for _, item := range pods.Items {
			k.Log.Debugf("Firing go routine to check on pod '%s'", item.Name)
			go k.CheckPodStatus(item.Name, namespace, "up", timeout, &wg)
		}
	}

	if desiredValue < currentValue {
		k.Log.Info("Scaling down pods ...")
		for _, item := range pods.Items {
			k.Log.Debugf("Firing go routine to check on pod '%s'", item.Name)
			go k.CheckPodStatus(item.Name, namespace, "down", timeout, &wg)
		}
	}

	wg.Wait()
	k.Log.Infof("Successfully scaled deployment '%s' in namespace '%s' from '%v' to '%v' replicas ", deploymentName, namespace, currentValue, replicaCount)
	return nil

}

// CheckDeployment checks the existence of a deployment in anamespace
func (k *K8sImpl) CheckDeployment(labelSelector, namespace string) (bool, error) {
	k.Log.Infof("Checking deployment with labelselector '%v' exists in namespace '%s", labelSelector, namespace)
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	deployment, err := k.K8sInterface.AppsV1().Deployments(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return false, err
	}

	// There should be one deployment of kibana
	if len(deployment.Items) == 1 {
		return true, nil
	}
	return false, nil
}

// ScaleSTS scales down the STS to zero replicas
func (k *K8sImpl) ScaleSTS(stsName, namespace string, replicaCount int32) error {
	scale, err := k.K8sInterface.AppsV1().StatefulSets(namespace).GetScale(context.TODO(), stsName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	scaleDown := *scale
	scaleDown.Spec.Replicas = replicaCount

	k.Log.Infof("Scaling down sts %s in namespace %s to zero replicas", stsName, namespace)
	_, err = k.K8sInterface.AppsV1().StatefulSets(namespace).UpdateScale(context.TODO(), stsName, &scaleDown, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// IsPodReady checks whether pod is Ready
func (k *K8sImpl) IsPodReady(pod *v1.Pod) (bool, error) {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			k.Log.Infof("Pod '%s' in namespace '%s' is in '%s' state", pod.Name, pod.Namespace, condition.Type)
			return true, nil
		}
	}
	k.Log.Infof("Pod '%s' in namespace '%s' is still not Ready", pod.Name, pod.Namespace)
	return false, nil
}

// CheckPodStatus checks the state of the pod depending on checkFlag
func (k *K8sImpl) CheckPodStatus(podName, namespace, checkFlag string, timeout string, wg *sync.WaitGroup) error {
	k.Log.Infof("Checking Pod '%s' status in namespace '%s", podName, namespace)
	var timeSeconds float64
	defer wg.Done()
	timeParse, err := time.ParseDuration(timeout)
	if err != nil {
		k.Log.Errorf("Unable to parse time duration ", zap.Error(err))
		return err
	}
	totalSeconds := timeParse.Seconds()
	done := false
	wait := false

	for !done {
		pod, err := k.K8sInterface.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if pod == nil && checkFlag == "down" {
			// break loop when scaling down condition is met
			k.Log.Infof("Pod '%s' has scaled down successfully", podName)
			done = true
		}

		// If pod is found
		if pod != nil {
			switch checkFlag {
			case "up":
				// Check status and apply retry logic
				if pod.Status.Phase != "Running" {
					// Pod is not Running state so we need to wait.
					wait = true
				} else {
					// break loop when scaling up condition is met
					k.Log.Infof("Pod '%s' is in 'Running' state", pod.Name)
					ok, err := k.IsPodReady(pod)
					if err != nil {
						return err
					}
					if ok {
						// break loop pod is Running and pod is in Ready.
						done = true
					} else {
						// Pod is in Running state but still not ready. Hence, we will wait.
						wait = true
					}
				}

			case "down":
				wait = true
			}

			if wait {
				fmt.Printf("timeSeconds = %v, totalSeconds = %v ", timeSeconds, totalSeconds)
				if timeSeconds < totalSeconds {
					message := fmt.Sprintf("Pod '%s' is in '%s' state", pod.Name, pod.Status.Phase)
					duration, err := futil.WaitRandom(message, timeout, k.Log)
					if err != nil {
						return err
					}
					timeSeconds = timeSeconds + float64(duration)

				} else {
					return fmt.Errorf("Timeout '%s' exceeded. Pod '%s' is still not in running state", timeout, pod.Name)
				}
				// change wait to false after each wait
				wait = false
			}
		}
	}
	return nil
}

// CheckAllPodsAfterRestore checks presence of pods part of Opensearch cluster implementation after restore
func (k *K8sImpl) CheckAllPodsAfterRestore() error {
	timeout := futil.GetEnvWithDefault(constants.OpenSearchHealthCheckTimeoutKey, constants.OpenSearchHealthCheckTimeoutDefaultValue)

	message := "Waiting for OpenSearch Operator to come up"
	_, err := futil.WaitRandom(message, timeout, k.Log)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	namespace := constants.VerrazzanoLoggingNamespace
	ingestLabel := constants.NewIngestLabelSelector
	osdLabel := constants.OSDDeploymentLabelSelector

	ok, err := k.IsLegacyOS()
	if err != nil {
		return err
	}

	if ok {
		namespace = constants.VerrazzanoSystemNamespace
		ingestLabel = constants.IngestLabelSelector
		osdLabel = constants.KibanaLabelSelector
	}

	k.Log.Infof("Checking pods with labelselector '%v' in namespace '%s", ingestLabel, namespace)
	listOptions := metav1.ListOptions{LabelSelector: ingestLabel}
	ingestPods, err := k.K8sInterface.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	wg.Add(len(ingestPods.Items))
	for _, pod := range ingestPods.Items {
		k.Log.Debugf("Firing go routine to check on pod '%s'", pod.Name)
		go k.CheckPodStatus(pod.Name, namespace, "up", timeout, &wg)
	}

	k.Log.Infof("Checking pods with labelselector '%v' in namespace '%s", osdLabel, namespace)
	listOptions = metav1.ListOptions{LabelSelector: osdLabel}
	kibanaPods, err := k.K8sInterface.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	wg.Add(len(kibanaPods.Items))
	for _, pod := range kibanaPods.Items {
		k.Log.Debugf("Firing go routine to check on pod '%s'", pod.Name)
		go k.CheckPodStatus(pod.Name, namespace, "up", timeout, &wg)
	}

	wg.Wait()
	return nil
}

// ExecPod runs a remote command a pod, returning the stdout and stderr of the command.
func (k *K8sImpl) ExecPod(pod *v1.Pod, container string, command []string) (string, string, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	request := k.K8sInterface.
		CoreV1().
		RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	var executor remotecommand.Executor
	var err error
	if futil.GetEnvWithDefault(constants.DevKey, constants.FalseString) == constants.TrueString {
		executor, err = vmofake.NewPodExecutor(k.K8sConfig, "POST", request.URL())
	} else {
		executor, err = NewPodExecutor(k.K8sConfig, "POST", request.URL())
	}

	if err != nil {
		return "", "", err
	}
	err = executor.Stream(remotecommand.StreamOptions{
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return "", "", fmt.Errorf("error running command %s on %v/%v: %v", command, pod.Namespace, pod.Name, err)
	}

	return stdout.String(), stderr.String(), nil
}

// UpdateKeystore Update Opensearch keystore with object store creds
func (k *K8sImpl) UpdateKeystore(connData *model.ConnectionData, timeout string) (bool, error) {

	var accessKeyCmd, secretKeyCmd []string
	accessKeyCmd = append(accessKeyCmd, "/bin/sh", "-c", fmt.Sprintf("echo %s | %s", strconv.Quote(connData.Secret.ObjectAccessKey), constants.OpenSearchKeystoreAccessKeyCmd))
	secretKeyCmd = append(secretKeyCmd, "/bin/sh", "-c", fmt.Sprintf("echo %s | %s", strconv.Quote(connData.Secret.ObjectSecretKey), constants.OpenSearchKeystoreSecretAccessKeyCmd))

	ok, err := k.IsLegacyOS()
	if err != nil {
		return false, err
	}

	namespace := constants.VerrazzanoLoggingNamespace

	masterLabel := constants.NewOpenSearchMasterLabel
	masterPodContainerName := constants.OpenSearchPodContainerName

	dataLabel := constants.NewOpenSearchDataLabel
	dataPodContainerName := constants.OpenSearchPodContainerName

	// If VMO based OS use original values
	if ok {
		namespace = constants.VerrazzanoSystemNamespace

		masterLabel = constants.OpenSearchMasterLabel
		masterPodContainerName = constants.OpenSearchMasterPodContainerName

		dataLabel = constants.OpenSearchDataLabel
		dataPodContainerName = constants.OpenSearchDataPodContainerName
	}

	// Updating keystore in other masters
	listOptions := metav1.ListOptions{LabelSelector: masterLabel}
	esMasterPods, err := k.K8sInterface.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	if err != nil {
		k.Log.Errorf("Unable to fetch list of opensearch master pods")
		return false, err
	}
	for _, pod := range esMasterPods.Items {
		err = k.ExecRetry(&pod, masterPodContainerName, timeout, accessKeyCmd) //nolint:gosec //#gosec G601
		if err != nil {
			k.Log.Errorf("Unable to exec into pod %s due to %v", pod.Name, err)
			return false, err
		}

		err = k.ExecRetry(&pod, masterPodContainerName, timeout, secretKeyCmd) //nolint:gosec //#gosec G601
		if err != nil {
			k.Log.Errorf("Unable to exec into pod %s due to %v", pod.Name, err)
			return false, err
		}
	}

	// Updating keystore in data nodes
	listOptions = metav1.ListOptions{LabelSelector: dataLabel}
	esDataPods, err := k.K8sInterface.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	if err != nil {
		k.Log.Errorf("Unable to fetch list of opensearch data pods")
		return false, err
	}

	for _, pod := range esDataPods.Items {
		err = k.ExecRetry(&pod, dataPodContainerName, timeout, accessKeyCmd) //nolint:gosec //#gosec G601
		if err != nil {
			k.Log.Errorf("Unable to exec into pod %s due to %v", pod.Name, err)
			return false, err
		}

		err = k.ExecRetry(&pod, dataPodContainerName, timeout, secretKeyCmd) //nolint:gosec //#gosec G601
		if err != nil {
			k.Log.Errorf("Unable to exec into pod %s due to %v", pod.Name, err)
			return false, err
		}
	}

	return true, nil

}

func (k *K8sImpl) ExecRetry(pod *v1.Pod, container, timeout string, execCmd []string) error {
	var timeSeconds float64
	done := false

	timeParse, err := time.ParseDuration(timeout)
	if err != nil {
		k.Log.Errorf("Unable to parse time duration ", zap.Error(err))
		return err
	}
	totalSeconds := timeParse.Seconds()

	for !done {
		k.Log.Infof("Updating keystore in pod '%s'", pod.Name)
		_, _, err = k.ExecPod(pod, container, execCmd) //nolint:gosec //#gosec G601
		if err != nil {
			if timeSeconds < totalSeconds {
				message := fmt.Sprintf("Unable to exec into pod '%s'", pod.Name)
				duration, err := futil.WaitRandom(message, timeout, k.Log)
				if err != nil {
					return err
				}
				timeSeconds = timeSeconds + float64(duration)
			} else {
				k.Log.Errorf("Global timeout '%s' exceeded. Unable to exec into pod", timeout)
				return err
			}
		} else {
			done = true
		}
	}
	return nil
}

// IsLegacyOS returns true if VMO based OpenSearch is running, false otherwise
func (k *K8sImpl) IsLegacyOS() (bool, error) {
	k.Log.Infof("Fetching VMI in verrazzano-system")
	gvr := schema.GroupVersionResource{
		Group:    "verrazzano.io",
		Version:  "v1",
		Resource: "verrazzanomonitoringinstances",
	}
	vmiFetched, err := k.DynamicK8sInterface.Resource(gvr).Namespace(constants.VerrazzanoSystemNamespace).Get(context.Background(), "system", metav1.GetOptions{})
	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if vmiFetched == nil {
		k.Log.Infof("No vmi found. Considering Legacy OS to be disabled")
		return false, nil
	}

	var vmi vmov1.VerrazzanoMonitoringInstance
	data, err := json.Marshal(vmiFetched)
	if err != nil {
		return false, err
	}
	err = json.Unmarshal(data, &vmi)
	if err != nil {
		return false, err
	}

	if vmi.Spec.Opensearch.Enabled {
		return true, nil
	}
	return false, nil
}

// Copyright (c) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package k8s_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/log"
	kutil "github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/utilities/k8s"
	vmofake "github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/utilities/k8s/fake"
	"go.uber.org/zap"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"sync"
	"testing"
)

var (
	TestPod = v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo",
			Namespace:   "foo",
			Annotations: map[string]string{},
		},
		Status: v1.PodStatus{
			Phase: "Running",
			Conditions: []v1.PodCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
				{
					Type:   "NotReady",
					Status: "True",
				},
			},
		},
	}
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

// TestPopulateConnData tests the PopulateConnData method for the following use case.
// GIVEN a Velero backup name
// WHEN Velero backup is in progress
// THEN fetches the secret associate with Velero backup
func TestPopulateConnData(t *testing.T) {
	t.Parallel()
	log, f := logHelper()
	defer os.Remove(f)

	var clientk client.Client
	fc := fake.NewSimpleClientset()
	dclient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	k8s := kutil.New(dclient, clientk, fc, nil, "default", log)
	conData, err := k8s.PopulateConnData(constants.VeleroNameSpace, "Foo")
	assert.Nil(t, conData)
	assert.NotNil(t, err)
}

// TestGetBackupStorageLocation tests the GetBackupStorageLocation method for the following use case.
// GIVEN a Velero backup storage location name
// WHEN invoked
// THEN fetches backup storage location object
func TestGetBackupStorageLocation(t *testing.T) {
	t.Parallel()
	log, f := logHelper()
	defer os.Remove(f)

	var clientk client.Client
	fc := fake.NewSimpleClientset()
	dclient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	k8s := kutil.New(dclient, clientk, fc, nil, "default", log)
	_, err := k8s.GetBackupStorageLocation("system", "fsl")
	assert.NotNil(t, err)
}

// TestGetBackup tests the GetBackup method for the following use case.
// GIVEN a Velero backup name
// WHEN invoked
// THEN fetches backup object
func TestGetBackup(t *testing.T) {
	t.Parallel()
	log, f := logHelper()
	defer os.Remove(f)

	var clientk client.Client
	fc := fake.NewSimpleClientset()
	dclient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	k8s := kutil.New(dclient, clientk, fc, nil, "default", log)

	_, err := k8s.GetBackup("system", "foo")
	assert.NotNil(t, err)
}

// TestCheckPodStatus tests the CheckPodStatus method for the following use case.
// GIVEN a pod name
// WHEN invoked
// THEN fetches pod status and monitors it depending on the checkFlag
func TestCheckPodStatus(t *testing.T) {
	t.Parallel()
	log, f := logHelper()
	defer os.Remove(f)
	var clientk client.Client
	fc := fake.NewSimpleClientset()
	dclient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	k8s := kutil.New(dclient, clientk, fc, nil, "default", log)

	var wg sync.WaitGroup

	wg.Add(1)
	err := k8s.CheckPodStatus("foo", "foo", "up", "10m", &wg)
	log.Infof("%v", err)
	assert.NotNil(t, err)
	wg.Wait()

	fc = fake.NewSimpleClientset(&TestPod)
	k8stest := kutil.New(dclient, clientk, fc, nil, "default", log)
	wg.Add(1)
	err = k8stest.CheckPodStatus("foo", "foo", "up", "10m", &wg)
	log.Infof("%v", err)
	assert.Nil(t, err)
	wg.Wait()

	wg.Add(1)
	err = k8stest.CheckPodStatus("foo", "foo", "down", "1s", &wg)
	log.Infof("%v", err)
	assert.NotNil(t, err)
	wg.Wait()
}

// TestCheckAllPodsAfterRestore tests the CheckAllPodsAfterRestore method for the following use case.
// GIVEN k8s client
// WHEN restore is complete
// THEN checks kibana and ingest pods are Ready after reboot
func TestCheckAllPodsAfterRestore(t *testing.T) {
	t.Parallel()
	IngestLabel := make(map[string]string)
	KibanaLabel := make(map[string]string)
	IngestLabel["app"] = "system-es-ingest"
	KibanaLabel["app"] = "system-osd"
	IngestPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo",
			Namespace:   constants.VerrazzanoSystemNamespace,
			Annotations: map[string]string{},
			Labels:      IngestLabel,
		},
		Status: v1.PodStatus{
			Phase: "Running",
			Conditions: []v1.PodCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
				{
					Type:   "NotReady",
					Status: "True",
				},
			},
		},
	}

	KibanaPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "bar",
			Namespace:   constants.VerrazzanoSystemNamespace,
			Annotations: map[string]string{},
			Labels:      KibanaLabel,
		},
		Status: v1.PodStatus{
			Phase: "Running",
			Conditions: []v1.PodCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
				{
					Type:   "NotReady",
					Status: "True",
				},
			},
		},
	}

	log, f := logHelper()
	defer os.Remove(f)

	var clientk client.Client
	fc := fake.NewSimpleClientset()
	dclient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	k8s := kutil.New(dclient, clientk, fc, nil, "default", log)

	os.Setenv(constants.OpenSearchHealthCheckTimeoutKey, "1s")

	err := k8s.CheckAllPodsAfterRestore()
	log.Infof("%v", err)
	assert.Nil(t, err)

	fc = fake.NewSimpleClientset(&IngestPod, &KibanaPod)
	k8snew := kutil.New(dclient, clientk, fc, nil, "default", log)
	err = k8snew.CheckAllPodsAfterRestore()
	log.Infof("%v", err)
	assert.Nil(t, err)

}

// TestCheckDeployment tests the CheckDeployment method for the following use case.
// GIVEN k8s client
// WHEN restore is complete
// THEN checks kibana deployment is present on system
func TestCheckDeployment(t *testing.T) {
	t.Parallel()
	KibanaLabel := make(map[string]string)
	KibanaLabel["verrazzano-component"] = "osd"
	PrimaryDeploy := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo",
			Namespace:   constants.VerrazzanoSystemNamespace,
			Annotations: map[string]string{},
			Labels:      KibanaLabel,
		},
	}

	SecondaryDeploy := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "bar",
			Namespace:   constants.VerrazzanoSystemNamespace,
			Annotations: map[string]string{},
			Labels:      KibanaLabel,
		},
	}

	log, f := logHelper()
	defer os.Remove(f)

	var clientk client.Client
	fc := fake.NewSimpleClientset()
	dclient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	k8s := kutil.New(dclient, clientk, fc, nil, "default", log)
	os.Setenv(constants.OpenSearchHealthCheckTimeoutKey, "1s")

	fmt.Println("Deployment not found")
	ok, err := k8s.CheckDeployment(constants.KibanaDeploymentLabelSelector, constants.VerrazzanoSystemNamespace)
	log.Infof("%v", err)
	assert.Nil(t, err)
	assert.Equal(t, ok, false)

	fmt.Println("Deployment found")
	fc = fake.NewSimpleClientset(&PrimaryDeploy)
	k8sPrimary := kutil.New(dclient, clientk, fc, nil, "default", log)
	ok, err = k8sPrimary.CheckDeployment(constants.KibanaDeploymentLabelSelector, constants.VerrazzanoSystemNamespace)
	log.Infof("%v", err)
	assert.Nil(t, err)
	assert.Equal(t, ok, true)

	fmt.Println("Multiple Deployments found")
	fc = fake.NewSimpleClientset(&PrimaryDeploy, &SecondaryDeploy)
	k8sPrimarySec := kutil.New(dclient, clientk, fc, nil, "default", log)
	ok, err = k8sPrimarySec.CheckDeployment(constants.KibanaDeploymentLabelSelector, constants.VerrazzanoSystemNamespace)
	log.Infof("%v", err)
	assert.Nil(t, err)
	assert.Equal(t, ok, false)
}

// TestIsPodReady tests the IsPodReady method for the following use case.
// GIVEN k8s client
// WHEN restore is complete
// THEN checks is pod is in ready state
func TestIsPodReady(t *testing.T) {
	t.Parallel()

	ReadyPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo",
			Namespace:   constants.VerrazzanoSystemNamespace,
			Annotations: map[string]string{},
		},
		Status: v1.PodStatus{
			Phase: "Running",
			Conditions: []v1.PodCondition{
				{
					Type:   "Initialized",
					Status: "True",
				},
				{
					Type:   "Ready",
					Status: "True",
				},
				{
					Type:   "ContainersReady",
					Status: "True",
				},
				{
					Type:   "PodScheduled",
					Status: "True",
				},
			},
		},
	}

	NotReadyPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo",
			Namespace:   constants.VerrazzanoSystemNamespace,
			Annotations: map[string]string{},
		},
		Status: v1.PodStatus{
			Phase: "Running",
			Conditions: []v1.PodCondition{
				{
					Type:   "Initialized",
					Status: "True",
				},
				{
					Type:   "ContainersReady",
					Status: "True",
				},
				{
					Type:   "PodScheduled",
					Status: "True",
				},
			},
		},
	}

	log, f := logHelper()
	defer os.Remove(f)

	os.Setenv(constants.OpenSearchHealthCheckTimeoutKey, "1s")

	var clientk client.Client
	fc := fake.NewSimpleClientset()
	dclient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	k8s := kutil.New(dclient, clientk, fc, nil, "default", log)
	ok, err := k8s.IsPodReady(&ReadyPod)
	log.Infof("%v", err)
	assert.Nil(t, err)
	assert.Equal(t, ok, true)

	ok, err = k8s.IsPodReady(&NotReadyPod)
	log.Infof("%v", err)
	assert.Nil(t, err)
	assert.Equal(t, ok, false)

}

// TestExecRetry tests the ExecRetry method for the following use case.
// GIVEN k8s client
// WHEN exec command fails
// THEN there is a retry of the exec command

func TestExecRetry(t *testing.T) {

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "foo",
		},
	}

	log, f := logHelper()
	defer os.Remove(f)
	defer os.Unsetenv(constants.DevKey)

	os.Setenv(constants.OpenSearchHealthCheckTimeoutKey, "1s")
	os.Setenv(constants.DevKey, constants.TrueString)

	var clientk client.Client
	config, fc := vmofake.NewClientsetConfig()
	dclient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	k8s := kutil.New(dclient, clientk, fc, config, "default", log)

	var accessKeyCmd []string
	accessKeyCmd = append(accessKeyCmd, "/bin/sh", "-c", fmt.Sprintf("echo %s | %s", strconv.Quote("ACCESS_KEY"), constants.OpenSearchKeystoreAccessKeyCmd))

	err := k8s.ExecRetry(pod, constants.OpenSearchDataPodContainerName, "1s", accessKeyCmd)
	assert.Nil(t, err)
}

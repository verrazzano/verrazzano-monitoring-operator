// Copyright (c) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package k8s

import (
	model "github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/types"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

type K8s interface {
	PopulateConnData(veleroNamespace, backupName string) (*model.ConnectionData, error)
	GetObjectStoreCreds(secretName, namespace, secretKey string) (*model.ObjectStoreSecret, error)
	GetBackup(veleroNamespace, backupName string) (*model.VeleroBackup, error)
	GetBackupStorageLocation(veleroNamespace, bslName string) (*model.VeleroBackupStorageLocation, error)
	ScaleDeployment(labelSelector, namespace, deploymentName string, replicaCount int32, log *zap.SugaredLogger) error
	CheckDeployment(labelSelector, namespace string) (bool, error)
	CheckPodStatus(podName, namespace, checkFlag string, timeout string, wg *sync.WaitGroup) error
	CheckAllPodsAfterRestore() error
	IsPodReady(pod *v1.Pod) (bool, error)
	ExecPod(cfg *rest.Config, pod *v1.Pod, container string, command []string) (string, string, error)
	UpdateKeystore(connData *model.ConnectionData, timeout string) (bool, error)
	ExecRetry(pod *v1.Pod, container, timeout string, execCmd []string) error
}

type K8sImpl struct {
	DynamicK8sInterface dynamic.Interface
	K8sClient           client.Client
	K8sInterface        kubernetes.Interface
	K8sConfig           *rest.Config
	CredentialProfile   string //default value `default`
	Log                 *zap.SugaredLogger
}

func New(dclient dynamic.Interface, kclient client.Client, kclientInterface kubernetes.Interface, cfg *rest.Config, credentialProfile string, log *zap.SugaredLogger) *K8sImpl {
	return &K8sImpl{
		DynamicK8sInterface: dclient,
		K8sClient:           kclient,
		K8sInterface:        kclientInterface,
		K8sConfig:           cfg,
		CredentialProfile:   credentialProfile,
		Log:                 log,
	}
}

// NewPodExecutor is to be overridden during unit tests
var NewPodExecutor = remotecommand.NewSPDYExecutor

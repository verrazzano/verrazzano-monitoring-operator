// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package util

import (
	"context"

	"k8s.io/client-go/kubernetes"

	"fmt"

	"github.com/golang/glog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/test/integ/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateVMI(
	crClient client.VMICR,
	clientSet kubernetes.Clientset,
	ns string,
	vmi *vmcontrollerv1.VerrazzanoMonitoringInstance,
	secret *corev1.Secret) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error) {

	fmt.Printf("Creating secret '%s' in namespace '%s'\n", vmi.Name, ns)
	_, err := clientSet.CoreV1().Secrets(ns).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		glog.Errorf("Cannot create a vmi secret: %s, due to err: %v", secret.Name, err)
		return nil, err
	}

	fmt.Printf("Creating vmi '%s' in namespace '%s'\n", vmi.Name, ns)
	vmi.Namespace = ns
	res, err := crClient.Create(context.TODO(), vmi)
	if err != nil {
		glog.Errorf("Unable to create vmi '%s' in namespace '%s'\n", vmi.Name, ns)
		return nil, err
	}
	fmt.Printf("Successfully created vmi '%s' in namespace '%s'\n", res.Name, res.Namespace)
	return res, nil
}

func GetVMI(
	crClient client.VMICR,
	ns string,
	vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error) {

	fmt.Printf("Getting existing vmi '%s' in namespace '%s'\n", vmi.Name, ns)
	res, err := crClient.Get(context.TODO(), ns, vmi.Name)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Successfully got existing vmi '%s' in namespace '%s'\n", res.Name, res.Namespace)
	return res, nil
}

func DeleteVMI(
	crClient client.VMICR,
	clientSet kubernetes.Clientset,
	ns string,
	vmi *vmcontrollerv1.VerrazzanoMonitoringInstance,
	secret *corev1.Secret) error {
	fmt.Printf("Deleting vmi '%s' in namespace '%s'\n", vmi.Name, vmi.Namespace)
	err := crClient.Delete(context.TODO(), ns, vmi.Name)
	if err != nil {
		return err
	}
	fmt.Printf("Deleting secret '%s' in namespace '%s'\n", vmi.Name, ns)
	err = clientSet.CoreV1().Secrets(ns).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf("Cannot delete secret: %s, due to err: %v", secret.Name, err)
		return err
	}

	fmt.Printf("Successfully deleted vmi '%s' in namespace '%s'\n", vmi.Name, vmi.Namespace)
	return nil
}

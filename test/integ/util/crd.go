// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package util

import (
	"context"
	"os"

	"k8s.io/client-go/kubernetes"

	"fmt"

	"github.com/rs/zerolog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/test/integ/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateVMO creates a secret and verrazzanoMonitoringInstance k8s resources
func CreateVMO(
	crClient client.VMOCR,
	clientSet kubernetes.Clientset,
	ns string,
	vmo *vmcontrollerv1.VerrazzanoMonitoringInstance,
	secret *corev1.Secret) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error) {

	//create log for creation of VMO
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "VerrazzanoMonitoringInstance").Str("name", vmo.Name).Logger()

	fmt.Printf("Creating secret '%s' in namespace '%s'\n", vmo.Name, ns)
	_, err := clientSet.CoreV1().Secrets(ns).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		logger.Error().Msgf("Cannot create a vmo secret: %s, due to err: %v", secret.Name, err)
		return nil, err
	}

	fmt.Printf("Creating vmo '%s' in namespace '%s'\n", vmo.Name, ns)
	vmo.Namespace = ns
	res, err := crClient.Create(context.TODO(), vmo)
	if err != nil {
		logger.Error().Msgf("Unable to create vmo '%s' in namespace '%s'\n", vmo.Name, ns)
		return nil, err
	}
	fmt.Printf("Successfully created vmo '%s' in namespace '%s'\n", res.Name, res.Namespace)
	return res, nil
}

// GetVMO returns a verrazzanoMonitoringInstance k8s resource
func GetVMO(
	crClient client.VMOCR,
	ns string,
	vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error) {

	fmt.Printf("Getting existing vmo '%s' in namespace '%s'\n", vmo.Name, ns)
	res, err := crClient.Get(context.TODO(), ns, vmo.Name)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Successfully got existing vmo '%s' in namespace '%s'\n", res.Name, res.Namespace)
	return res, nil
}

// DeleteVMO delete a secret and verrazzanoMonitoringInstance k8s resource
func DeleteVMO(
	crClient client.VMOCR,
	clientSet kubernetes.Clientset,
	ns string,
	vmo *vmcontrollerv1.VerrazzanoMonitoringInstance,
	secret *corev1.Secret) error {
	//create log for deletion of VMO
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "VerrazzanoMonitoringInstance").Str("name", vmo.Name).Logger()

	fmt.Printf("Deleting vmo '%s' in namespace '%s'\n", vmo.Name, vmo.Namespace)
	err := crClient.Delete(context.TODO(), ns, vmo.Name)
	if err != nil {
		return err
	}
	fmt.Printf("Deleting secret '%s' in namespace '%s'\n", vmo.Name, ns)
	err = clientSet.CoreV1().Secrets(ns).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error().Msgf("Cannot delete secret: %s, due to err: %v", secret.Name, err)
		return err
	}

	fmt.Printf("Successfully deleted vmo '%s' in namespace '%s'\n", vmo.Name, vmo.Namespace)
	return nil
}

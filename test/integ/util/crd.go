// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
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

func CreateSauron(
	crClient client.SauronCR,
	clientSet kubernetes.Clientset,
	ns string,
	sauron *vmcontrollerv1.VerrazzanoMonitoringInstance,
	secret *corev1.Secret) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error) {

	//create log for creation of Sauron
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "VerrazzanoMonitoringInstance").Str("name", sauron.Name).Logger()

	fmt.Printf("Creating secret '%s' in namespace '%s'\n", sauron.Name, ns)
	_, err := clientSet.CoreV1().Secrets(ns).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		logger.Error().Msgf("Cannot create a sauron secret: %s, due to err: %v", secret.Name, err)
		return nil, err
	}

	fmt.Printf("Creating sauron '%s' in namespace '%s'\n", sauron.Name, ns)
	sauron.Namespace = ns
	res, err := crClient.Create(context.TODO(), sauron)
	if err != nil {
		logger.Error().Msgf("Unable to create sauron '%s' in namespace '%s'\n", sauron.Name, ns)
		return nil, err
	}
	fmt.Printf("Successfully created sauron '%s' in namespace '%s'\n", res.Name, res.Namespace)
	return res, nil
}

func GetSauron(
	crClient client.SauronCR,
	ns string,
	sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error) {

	fmt.Printf("Getting existing sauron '%s' in namespace '%s'\n", sauron.Name, ns)
	res, err := crClient.Get(context.TODO(), ns, sauron.Name)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Successfully got existing sauron '%s' in namespace '%s'\n", res.Name, res.Namespace)
	return res, nil
}

func DeleteSauron(
	crClient client.SauronCR,
	clientSet kubernetes.Clientset,
	ns string,
	sauron *vmcontrollerv1.VerrazzanoMonitoringInstance,
	secret *corev1.Secret) error {
	//create log for deletion of Sauron
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "VerrazzanoMonitoringInstance").Str("name", sauron.Name).Logger()

	fmt.Printf("Deleting sauron '%s' in namespace '%s'\n", sauron.Name, sauron.Namespace)
	err := crClient.Delete(context.TODO(), ns, sauron.Name)
	if err != nil {
		return err
	}
	fmt.Printf("Deleting secret '%s' in namespace '%s'\n", sauron.Name, ns)
	err = clientSet.CoreV1().Secrets(ns).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error().Msgf("Cannot delete secret: %s, due to err: %v", secret.Name, err)
		return err
	}

	fmt.Printf("Successfully deleted sauron '%s' in namespace '%s'\n", sauron.Name, sauron.Namespace)
	return nil
}

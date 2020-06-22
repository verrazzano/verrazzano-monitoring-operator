// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	// "fmt"
	"errors"
	"reflect"

	"github.com/golang/glog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/secrets"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// PasswordSeparator separates passwords from hashes
	PasswordSeparator = ":"
	// LineSeparator separates password records
	LineSeparator = "\n"
)

// HashedPasswords name => hash
type HashedPasswords map[string]string

func hashSha(password string) string {
	s := sha1.New()
	s.Write([]byte(password))
	passwordSum := s.Sum(nil)
	return base64.StdEncoding.EncodeToString(passwordSum)
}

// Bytes bytes representation
func (hp HashedPasswords) Bytes() (passwordBytes []byte) {
	passwordBytes = []byte{}
	for name, hash := range hp {
		passwordBytes = append(passwordBytes, []byte(name+PasswordSeparator+hash+LineSeparator)...)
	}
	return passwordBytes
}

// SetPassword set a password for a user with a hashing algo
func (hp HashedPasswords) SetPassword(name, password string) (err error) {
	prefix := "{SHA}"
	hash := hashSha(password)
	hp[name] = prefix + hash
	return nil
}

func GetAuthSecrets(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) (string, string, error) {
	// setup username/passwords and secrets

	username, err := controller.loadSecretData(sauron.Namespace,
		sauron.Spec.SecretsName, constants.SauronSecretUsername)
	if err != nil {
		glog.Errorf("problem getting username, error: %v", err)
		return "", "", err
	}

	password, err := controller.loadSecretData(sauron.Namespace,
		sauron.Spec.SecretsName, constants.SauronSecretPassword)
	if err != nil {
		glog.Errorf("problem getting password, error: %v", err)
		return "", "", err
	}
	return string(username), string(password), nil
}

func CreateOrUpdateAuthSecrets(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, credsMap map[string]string) error {

	passwords := HashedPasswords(map[string]string{})
	for k, v := range credsMap {
		if err := passwords.SetPassword(k, v); err != nil {
			return err
		}
	}
	auth := fmt.Sprintf("%s", passwords.Bytes())
	// glog.V(4).Infof("Debug Auth '%s' ", auth)

	secretData := make(map[string][]byte)
	secretData["auth"] = []byte(auth)
	secret, err := controller.secretLister.Secrets(sauron.Namespace).Get(sauron.Spec.SecretName)
	//When secret exists, k8s api returns a nil err obj.
	if err == nil {
		isEqual := reflect.DeepEqual(secretData, secret.Data)
		if !isEqual {
			secret.Data = secretData
			_, err = controller.kubeclientset.CoreV1().Secrets(sauron.Namespace).Update(secret)
			if err != nil {
				glog.Errorf("caught an error trying to update a basic auth secret, err: %v", err)
				return err
			}
		}
		return nil
	}
	// set a name for our Sauron secret
	// create the secret based on the Username/Password passed in the spec
	secret, err = secrets.New(sauron, sauron.Spec.SecretName, []byte(auth))
	if err != nil {
		glog.Errorf("got an error trying to create a password hash, err: %v", err)
		return err
	}
	secretOut, err := controller.kubeclientset.CoreV1().Secrets(sauron.Namespace).Create(secret)
	if err != nil {
		glog.Errorf("caught an error trying to create a secret, err: %v", err)
		return err
	}
	glog.V(6).Infof("Created secret: %s", secretOut.Name)

	// Delete Auth secrets if it is not supposed to exists

	secretsNames := []string{secret.Name, sauron.Name + "-tls"}
	selector := labels.SelectorFromSet(map[string]string{constants.SauronLabel: sauron.Name})
	secretList, err := controller.secretLister.Secrets(sauron.Namespace).List(selector)
	if err != nil {
		return err
	}
	for _, existedSecret := range secretList {
		if !contains(secretsNames, existedSecret.Name) {
			glog.V(6).Infof("Deleting secret %s", existedSecret.Name)
			err := controller.kubeclientset.CoreV1().Secrets(sauron.Namespace).Delete(existedSecret.Name, &metav1.DeleteOptions{})
			if err != nil {
				glog.Errorf("Failed to delete secret %s, for the reason (%v)", existedSecret.Name, err)
				return err
			}
		}
	}

	return nil
}

func CreateOrUpdateTLSSecrets(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) error {

	if sauron.Spec.AutoSecret {
		glog.V(6).Info("Not explicitly creating TLS secret, we expect it to be auto-generated")
		// by setting the AutoSecret to true we ask that a certificate be made for us
		// currently the mechanism relies on ingressShim part of cert-manager to notice the
		// annotation we set on the ingress rule which is set off by AutoSecret being true
		return nil
	}
	//get tlsCrt from sauronSecrets
	tlsCrt, err := controller.loadSecretData(sauron.Namespace,
		sauron.Spec.SecretsName, constants.TLSCRTName)
	if err != nil {
		glog.Errorf("problem getting tls.crt data using name: %s, from secret: %s, error: %s",
			constants.TLSCRTName, sauron.Spec.SecretsName, err)
		return err
	}
	//get tlsKey from sauronSecrets
	tlsKey, err := controller.loadSecretData(sauron.Namespace,
		sauron.Spec.SecretsName, constants.TLSKeyName)
	if err != nil {
		glog.Errorf("problem getting tls.key data using name: %s, from secret: %s, error: %s",
			constants.TLSKeyName, sauron.Spec.SecretsName, err)
		return err
	}

	if len(tlsCrt) != 0 && len(tlsKey) != 0 {
		secretData := make(map[string][]byte)
		secret, err := controller.secretLister.Secrets(sauron.Namespace).Get(sauron.Name + "-tls")
		//When secret exists, k8s api returns a nil err obj.
		if err == nil {
			secretData["tls.crt"] = tlsCrt
			secretData["tls.key"] = tlsKey
			isSecretDataEqual := reflect.DeepEqual(secretData, secret.Data)
			if !isSecretDataEqual {
				secret.Data = secretData
				_, err = controller.kubeclientset.CoreV1().Secrets(sauron.Namespace).Update(secret)
				if err != nil {
					glog.Errorf("caught an error trying to update a basic auth secret, err: %v", err)
					return err
				}
			}
			return nil
		}
		secret, err = secrets.NewTLS(sauron, sauron.Namespace, sauron.Name+"-tls", secretData)
		if err != nil {
			glog.Errorf("got an error trying to create a password hash, err: %s", err)
			return err
		}
		secretOut, err := controller.kubeclientset.CoreV1().Secrets(sauron.Namespace).Create(secret)
		if err != nil {
			glog.Errorf("caught an error trying to create a secret, err: %s", err)
			return err
		}
		glog.V(6).Infof("Create TLS secret: %s", secretOut.Name)

		secret, err = secrets.NewTLS(sauron, constants.MonitoringNamespace, sauron.Name+"-tls", secretData)
		if err != nil {
			glog.Errorf("got an error trying to create the TLS secret object: %s", err)
			return err
		}
		secretOut, err = controller.kubeclientset.CoreV1().Secrets(sauron.Namespace).Create(secret)
		if err != nil {
			glog.Errorf("Error trying to create a TLS secret in monitoring namespace, err: %s", err)
			return err
		}
		glog.V(6).Infof("Create TLS secret in monitoring namespace: %s", secretOut.Name)

	}
	return nil
}

func (c *Controller) loadSecretData(ns, secretName, secretField string) ([]byte, error) {
	secret, err := c.secretLister.Secrets(ns).Get(secretName)
	if err != nil {
		return nil, err
	}

	if data, ok := secret.Data[secretField]; ok {
		return data, nil
	}
	return nil, nil
}

func (c *Controller) loadAllAuthSecretData(ns, secretName string) (map[string]string, error) {
	secret, err := c.secretLister.Secrets(ns).Get(secretName)
	if err != nil {
		return nil, err
	}

	dataMap := secret.Data

	_, ok := dataMap["username"]
	if !ok {
		return nil, errors.New("error: The default username is not defined in Sauron secrets")
	}

	m := make(map[string]string)
	re := regexp.MustCompile("[0-9]+")
	var pwd []byte
	for key, value := range dataMap {
		if !strings.Contains(strings.ToUpper(key), strings.ToUpper("username")) {
			continue
		}
		//Below Regular Expression returns any number present in the string after username
		userIndex := re.FindAllString(key, -1)
		if len(userIndex) == 0 {
			//Default User does not have any number appended
			pwd, ok = dataMap["password"]
			if !ok {
				return nil, errors.New("error: The default password is not defined in Sauron secrets")
			}
			m[string(value)] = string(pwd)
		} else if len(userIndex) == 1 {
			//Other users in the format username1,username2 etc, Have a integer appended at the end
			pwd, ok = dataMap["password"+userIndex[0]]
			if !ok {
				return nil, errors.New("error: The password is in the wrong format in Sauron secrets, should be i.e. password:p1, password2:u2, password3:u3 etc")
			}
			m[string(value)] = string(pwd)
		} else {
			// We should never reach here if the usernames are defined correctly in the secret file
			return nil, errors.New("error: The username is in the wrong format in Sauron secrets, More than 1 number in map key")
		}
	}

	return m, nil
}

// The prometheus pusher needs to access the ca.ctl cert in system-tls secret from within the pod.  The secret must
// be in the monitoring namespace to access it as a volume.  Copy the secret from the verrazzano-system
// namespace.
func CopyTLSSecretToMonitoringNS(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) error {

	// The secret must be this name since the name is hardcoded in monitoring/deployments.do of verrazzano operator.
	const secretName = "system-tls"
	secret, err := controller.kubeclientset.CoreV1().Secrets(sauron.Namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Error getting TLS secret %s from namespace %s, err: %s", secretName, sauron.Namespace, err)
		return err
	}

	_, err = controller.kubeclientset.CoreV1().Secrets(constants.MonitoringNamespace).Create(secret)
	if err != nil {
		glog.Errorf("caught an error trying to create a secret, err: %s", err)
		return err
	}
	glog.V(6).Infof("Created TLS secret %s in namespace %s", secretName, constants.MonitoringNamespace)

	return nil
}

// Copyright (c) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCreateCertificates tests that the certificates needed for webhooks are created
// GIVEN an output directory for certificates
//
//	WHEN I call CreateCertificates
//	THEN all the needed certificate artifacts are created
func TestCreateCertificates(t *testing.T) {
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "certs")
	if err != nil {
		assert.Nil(err, "error should not be returned creating temporary directory")
	}
	defer os.RemoveAll(dir)
	caBundle, err := CreateCertificates(dir)
	assert.Nil(err, "error should not be returned setting up certificates")
	assert.NotNil(caBundle, "CA bundle should be returned")

	crtFile := fmt.Sprintf("%s/%s", dir, "tls.crt")
	keyFile := fmt.Sprintf("%s/%s", dir, "tls.key")
	assert.FileExists(crtFile, dir, "tls.crt", "expected tls.crt file not found")
	assert.FileExists(keyFile, dir, "tls.key", "expected tls.key file not found")

	crtBytes, err := ioutil.ReadFile(crtFile)
	if assert.NoError(err) {
		block, _ := pem.Decode(crtBytes)
		assert.NotEmptyf(block, "failed to decode PEM block containing public key")
		assert.Equal("CERTIFICATE", block.Type)
		cert, err := x509.ParseCertificate(block.Bytes)
		if assert.NoError(err) {
			assert.NotEmpty(cert.DNSNames, "Certificate DNSNames SAN field should not be empty")
			assert.Equal("verrazzano-monitoring-operator.verrazzano-system.svc", cert.DNSNames[0])
		}
	}
}

// TestCreateWebhookCertificatesFail tests that the certificates needed for webhooks are not created
// GIVEN an invalid output directory for certificates
//
//	WHEN I call CreateCertificates
//	THEN all the needed certificate artifacts are not created
func TestCreateWebhookCertificatesFail(t *testing.T) {
	assert := assert.New(t)

	_, err := CreateCertificates("/bad-dir")
	assert.Error(err, "error should be returned setting up certificates")
}

// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package util

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"github.com/rs/zerolog"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	//create log for pem block for key
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "IntegTestUtil").Str("name", "pemBlockForKey").Logger()

	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			logger.Error().Msgf("Unable to marshal ECDSA private key: %v", err)
			return nil
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

// GenerateKeys this generates keys
//	host  	   : string "Comma-separated hostnames and IPs to generate a certificate for"
//	validFrom  : string "Creation date formatted as Jan 1 15:04:05 2011"
//	validFor   : time.Duration 365*24*time.Hour, "Duration that certificate is valid for"
//	isCA       : bool "whether this cert should be its own Certificate Authority"
//	rsaBits    : int "Size of RSA key to generate. Ignored if --ecdsa-curve is set"
//	ecdsaCurve : string "ECDSA curve to use to generate a key. Valid values are P224, P256 (recommended), P384, P521")
func GenerateKeys(host string, domain string, validFrom string, validFor time.Duration, isCA bool, rsaBits int, ecdsaCurve string) error {
	//create log for generating key
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "IntegTestUtil").Str("name", host).Logger()

	if len(host) == 0 {
		logger.Error().Msg("Missing required host argument")
	}

	var priv interface{}
	var err error
	switch ecdsaCurve {
	case "":
		priv, err = rsa.GenerateKey(rand.Reader, rsaBits)
	case "P224":
		priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case "P256":
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "P384":
		priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "P521":
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		logger.Error().Msgf("Unrecognized elliptic curve: %q", ecdsaCurve)
		return err
	}
	if err != nil {
		logger.Error().Msgf("failed to generate private key: %s", err)
	}

	var notBefore time.Time
	if len(validFrom) == 0 {
		notBefore = time.Now()
	} else {
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", validFrom)
		if err != nil {
			logger.Error().Msgf("Failed to parse creation date: %s\n", err)
			return err
		}
	}

	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		logger.Error().Msgf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:            []string{"US"},
			Locality:           []string{"Portland"},
			Organization:       []string{"Sauron"},
			OrganizationalUnit: []string{"PDX"},
			CommonName:         domain,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		logger.Error().Msgf("Failed to create certificate: %s", err)
	}

	certOut, err := os.Create(os.TempDir() + "/tls.crt")
	if err != nil {
		logger.Error().Msgf("failed to open"+os.TempDir()+"/tls.crt for writing: %s", err)
	}
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		logger.Error().Msgf("Error encoding certificate: %s", err)
	}

	err = certOut.Close()
	if err != nil {
		logger.Error().Msgf("Error closing cert file: %s", err)
	}

	fmt.Print("generated " + os.TempDir() + "/tls.crt\n")

	keyOut, err := os.OpenFile(os.TempDir()+"/tls.key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		logger.Error().Msgf("failed to open "+os.TempDir()+"/tls.key for writing:", err)
		return err
	}
	err = pem.Encode(keyOut, pemBlockForKey(priv))
	if err != nil {
		logger.Error().Msgf("Error encoding pem key: %s", err)
	}
	err = keyOut.Close()
	if err != nil {
		logger.Error().Msgf("Error closing key: %s", err)
	}
	fmt.Print("generated " + os.TempDir() + "/tls.key\n")

	return nil
}

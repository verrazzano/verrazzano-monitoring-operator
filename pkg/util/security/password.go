// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package security

import (
	"crypto/rand"
	b64 "encoding/base64"
	"fmt"
	"regexp"
)

//GeneratePassword will generate a password of length
func GeneratePassword(length int) (string, error) {
	if length < 1 {
		return "", fmt.Errorf("cannot create password of length %d", length)
	}
	// Enlarge buffer so plenty of room is left when special characters are stripped out
	b := make([]byte, length*3)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	pw := b64.StdEncoding.EncodeToString(b)
	pw, err = makeAlphaNumeric(pw)
	if err != nil {
		return "", err
	}
	return pw[:length], nil
}

// makeAlphaNumeric removes all special characters from a password string
func makeAlphaNumeric(input string) (string, error) {
	// Make a Regex to say we only want letters and numbers
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		return "", err
	}
	return reg.ReplaceAllString(input, ""), nil
}

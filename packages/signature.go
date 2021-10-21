// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"os"
	"strings"

	"github.com/pkg/errors"
)

func readSignature(basePath string) (string, error) {
	signatureFile := basePath + ".sig"
	signature, err := os.ReadFile(signatureFile)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil // signature file isn't present
	} else if err != nil {
		return "", errors.Wrap(err, "can't read signature file")
	}
	return strings.TrimSpace(string(signature)), nil
}

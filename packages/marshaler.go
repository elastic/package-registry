// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"encoding/json"
)

type MarshallerOption func(packages *Packages) error

func MarshalJSON(pkgs *Packages) ([]byte, error) {
	return json.MarshalIndent(pkgs, " ", " ")
}

func UnmarshalJSON(content []byte, pkgs *Packages, options ...MarshallerOption) error {
	err := json.Unmarshal(content, pkgs)
	if err != nil {
		return err
	}

	for _, opt := range options {
		err = opt(pkgs)
		if err != nil {
			return err
		}
	}
	return nil
}

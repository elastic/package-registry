// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"encoding/json"
)

func MarshalJSON(pkgs *Packages) ([]byte, error) {
	return json.MarshalIndent(pkgs, " ", " ")
}

func UnmarshalJSON(content []byte, pkgs *Packages) error {
	return json.Unmarshal(content, pkgs)
}

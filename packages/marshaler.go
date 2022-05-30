// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"encoding/json"
)

var (
	_ json.Marshaler   = new(Package)
	_ json.Unmarshaler = new(Package)
)

func (p *Package) MarshalJSON() ([]byte, error) {
	return json.MarshalIndent(*p, " ", " ")
}

func (p *Package) UnmarshalJSON(data []byte) error {
	type Alias Package
	aux := &struct {
		*Alias
	}{
		(*Alias)(p),
	}
	err := json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}
	return ((*Package)(aux.Alias)).setRuntimeFields()
}

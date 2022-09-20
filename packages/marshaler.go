// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"bytes"
	"encoding/json"

	"github.com/elastic/package-registry/util"
	jsoniter "github.com/json-iterator/go"
)

var (
	_ json.Marshaler   = new(Package)
	_ json.Unmarshaler = new(Package)
)

func (p *Package) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	err := util.WriteJSONPretty(&buf, *p)
	return buf.Bytes(), err
}

func (p *Package) UnmarshalJSON(data []byte) error {
	type Alias Package
	aux := &struct {
		*Alias
	}{
		(*Alias)(p),
	}
	err := jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(data, &aux)
	if err != nil {
		return err
	}
	((*Package)(aux.Alias)).setBasePolicyTemplates()
	return ((*Package)(aux.Alias)).setRuntimeFields()
}

// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"bytes"
	"encoding/json"
	"io"
)

// MarshalJSONPretty marshals a value to "pretty" JSON without HTML escaping.
func MarshalJSONPretty(v interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := WriteJSONPretty(buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteJSONPretty writes a value as "pretty" JSON without HTML escaping.
func WriteJSONPretty(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

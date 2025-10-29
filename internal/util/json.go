// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package util

import (
	"bytes"
	"encoding/json"
	"io"
)

// MarshalJSON marshals a value to compact JSON without HTML escaping.
func MarshalJSON(v interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder.Encode adds a trailing newline, trim it for compact output
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// WriteJSON writes a value as compact JSON without HTML escaping.
func WriteJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

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

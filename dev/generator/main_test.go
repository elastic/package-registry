// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeSavedObject(t *testing.T) {
	file := "../../testdata/package/example/0.0.2/kibana/dashboard/0c610510-5cbd-11e9-8477-077ec9664dbd.json"

	data, err := ioutil.ReadFile(file)
	assert.NoError(t, err)

	_, changed, err := encodedSavedObject(data)
	assert.NoError(t, err)
	assert.True(t, changed)
}

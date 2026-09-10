// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

//go:build mage

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoVersionFilesInSync(t *testing.T) {
	assert.NoError(t, checkGoVersionFilesInSync())
}

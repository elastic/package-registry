package main

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeSavedObject(t *testing.T) {
	file := "../package-examples/auditd-2.0.4/kibana/dashboard/7de391b0-c1ca-11e7-8995-936807a28b16-ecs.json"

	data, err := ioutil.ReadFile(file)
	assert.NoError(t, err)

	_, err = encodedSavedObject(data)
	assert.NoError(t, err)
}

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

var (
	responseToDecode = []string{
		"attributes.uiStateJSON",
		"attributes.visState",
		"attributes.optionsJSON",
		"attributes.panelsJSON",
		"attributes.kibanaSavedObjectMeta.searchSourceJSON",
	}
)

// DecodeExported decodes an exported dashboard
//func EncodeKibanaAssets(result common.MapStr) common.MapStr {

func TestFoo(t *testing.T) { // Read file from json
	file := "../package-examples/auditd-2.0.4/kibana/dashboard/7de391b0-c1ca-11e7-8995-936807a28b16-ecs.json"

	out, err := encodedSavedObject(file)
	fmt.Println(out)
	assert.NoError(t, err)
}

func encodedSavedObject(file string) (string, error) {

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}

	savedObject := common.MapStr{}
	json.Unmarshal(data, &savedObject)

	for _, v := range responseToDecode {
		out, err := savedObject.GetValue(v)
		// This means the key did not exists, no conversion needed
		if err != nil {
			continue
		}

		r, err := json.Marshal(&out)
		if err != nil {
			return "", err
		}
		savedObject.Put(v, string(r))
	}

	return savedObject.StringToPrint(), nil
}

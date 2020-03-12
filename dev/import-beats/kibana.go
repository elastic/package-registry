// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/pkg/errors"
)

var (
	encodedFields = []string{
		"attributes.uiStateJSON",
		"attributes.visState",
		"attributes.optionsJSON",
		"attributes.panelsJSON",
		"attributes.kibanaSavedObjectMeta.searchSourceJSON",
	}
)

type kibanaContent struct {
	files map[string]map[string][]byte
}

type kibanaMigrator struct {
	hostPort string
}

type kibanaDocuments struct {
	Objects []mapStr `json:"objects"`
}

func newKibanaMigrator(hostPort string) *kibanaMigrator {
	return &kibanaMigrator{
		hostPort: hostPort,
	}
}

func (km *kibanaMigrator) migrateDashboardFile(dashboardFile []byte) ([]byte, error) {
	dashboardFile, err := prepareDashboardFile(dashboardFile)
	if err != nil {
		return nil, errors.Wrapf(err, "preparing file failed")
	}

	request, err := http.NewRequest("POST",
		fmt.Sprintf("%s/api/kibana/dashboards/import?force=true", km.hostPort),
		bytes.NewReader(dashboardFile))
	if err != nil {
		return nil, errors.Wrapf(err, "creating POST request failed")
	}
	request.Header.Add("kbn-xsrf", "8.0.0")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, errors.Wrapf(err, "making POST request to Kibana failed")
	}
	defer response.Body.Close()

	saved, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "reading saved object failed")
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("making POST request failed: %s", string(saved))
	}
	return saved, nil
}

func prepareDashboardFile(dashboardFile []byte) ([]byte, error) {
	var documents kibanaDocuments

	// Rename indices (metricbeat, filebeat)
	dashboardFile = bytes.ReplaceAll(dashboardFile, []byte(`metricbeat-*`), []byte(`metrics-*`))
	dashboardFile = bytes.ReplaceAll(dashboardFile, []byte(`filebeat-*`), []byte(`logs-*`))

	err := json.Unmarshal(dashboardFile, &documents)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling dashboard file failed")
	}

	for i, object := range documents.Objects {
		object, err = encodeFields(object)
		if err != nil {
			return nil, errors.Wrapf(err, "encoding fields failed")
		}
		documents.Objects[i] = object
	}

	data, err := json.Marshal(&documents)
	if err != nil {
		return nil, errors.Wrapf(err, "marshalling dashboard file failed")
	}
	return data, nil
}

func encodeFields(ms mapStr) (mapStr, error) {
	for _, field := range encodedFields {
		v, err := ms.getValue(field)
		if err == errKeyNotFound {
			continue
		} else if err != nil {
			return mapStr{}, errors.Wrapf(err, "retrieving value failed (key: %s)", field)
		}

		ve, err := json.Marshal(v)
		if err != nil {
			return mapStr{}, errors.Wrapf(err, "marshalling value failed (key: %s)", field)
		}

		_, err = ms.put(field, string(ve))
		if err != nil {
			return mapStr{}, errors.Wrapf(err, "putting value failed (key: %s)", field)
		}
	}
	return ms, nil
}

func createKibanaContent(kibanaMigrator *kibanaMigrator, modulePath string) (kibanaContent, error) {
	moduleDashboardPath := path.Join(modulePath, "_meta", "kibana", "7", "dashboard")
	moduleDashboards, err := ioutil.ReadDir(moduleDashboardPath)
	if os.IsNotExist(err) {
		log.Printf("\tno dashboards present, skipped (modulePath: %s)", modulePath)
		return kibanaContent{}, nil
	}
	if err != nil {
		return kibanaContent{}, errors.Wrapf(err, "reading module dashboard directory failed (path: %s)",
			moduleDashboardPath)
	}

	kibana := kibanaContent{
		files: map[string]map[string][]byte{},
	}
	for _, moduleDashboard := range moduleDashboards {
		log.Printf("\tdashboard found: %s", moduleDashboard.Name())

		dashboardFilePath := path.Join(moduleDashboardPath, moduleDashboard.Name())
		dashboardFile, err := ioutil.ReadFile(dashboardFilePath)
		if err != nil {
			return kibanaContent{}, errors.Wrapf(err, "reading dashboard file failed (path: %s)",
				dashboardFilePath)
		}

		migrated, err := kibanaMigrator.migrateDashboardFile(dashboardFile)
		if err != nil {
			return kibanaContent{}, errors.Wrapf(err, "migrating dashboard file failed (path: %s)",
				dashboardFilePath)
		}

		extracted, err := extractKibanaObjects(migrated)
		if err != nil {
			return kibanaContent{}, errors.Wrapf(err, "extracting kibana dashboards failed")
		}

		for objectType, objects := range extracted {
			if _, ok := kibana.files[objectType]; !ok {
				kibana.files[objectType] = map[string][]byte{}
			}

			for k, v := range objects {
				kibana.files[objectType][k] = v
			}
		}
	}
	return kibana, nil
}

func extractKibanaObjects(dashboardFile []byte) (map[string]map[string][]byte, error) {
	var documents kibanaDocuments

	err := json.Unmarshal(dashboardFile, &documents)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling migrated dashboard file failed")
	}

	extracted := map[string]map[string][]byte{}
	for _, object := range documents.Objects {
		err = object.delete("updated_at")
		if err != nil {
			return nil, errors.Wrapf(err, "removing field updated_at failed")
		}

		err = object.delete("version")
		if err != nil {
			return nil, errors.Wrapf(err, "removing field version failed")
		}

		object, err = decodeFields(object)
		if err != nil {
			return nil, errors.Wrapf(err, "decoding fields failed")
		}

		data, err := json.MarshalIndent(object, "", "    ")
		if err != nil {
			return nil, errors.Wrapf(err, "marshalling object failed")
		}

		aType, err := object.getValue("type")
		if err != nil {
			return nil, errors.Wrapf(err, "retrieving type failed")
		}

		id, err := object.getValue("id")
		if err != nil {
			return nil, errors.Wrapf(err, "retrieving id failed")
		}

		if _, ok := extracted[aType.(string)]; !ok {
			extracted[aType.(string)] = map[string][]byte{}
		}
		extracted[aType.(string)][id.(string)+".json"] = data
	}

	return extracted, nil
}

func decodeFields(ms mapStr) (mapStr, error) {
	for _, field := range encodedFields {
		v, err := ms.getValue(field)
		if err == errKeyNotFound {
			continue
		} else if err != nil {
			return mapStr{}, errors.Wrapf(err, "retrieving value failed (key: %s)", field)
		}

		var vd interface{}
		err = json.Unmarshal([]byte(v.(string)), &vd)
		if err != nil {
			return mapStr{}, errors.Wrapf(err, "unmarshalling value failed (key: %s)", field)
		}

		_, err = ms.put(field, vd)
		if err != nil {
			return mapStr{}, errors.Wrapf(err, "putting value failed (key: %s)", field)
		}
	}
	return ms, nil
}

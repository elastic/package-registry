// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/pkg/errors"
)

type kibanaContent struct {
	dashboardFiles     map[string][]byte
	visualizationFiles map[string][]byte
}

type kibanaMigrator struct {
	hostPort string
}

func newKibanaMigrator(hostPort string) *kibanaMigrator {
	return &kibanaMigrator{
		hostPort: hostPort,
	}
}

func (km *kibanaMigrator) migrateDashboardFile(dashboardFile []byte) ([]byte, error) {
	request, err := http.NewRequest("POST",
		fmt.Sprintf("http://%s/api/kibana/dashboards/import?force=true", km.hostPort),
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

func createKibanaContent(kibanaMigrator *kibanaMigrator, modulePath string) (kibanaContent, error) {
	moduleDashboardPath := path.Join(modulePath, "_meta", "kibana", "7", "dashboard")
	moduleDashboards, err := ioutil.ReadDir(moduleDashboardPath)
	if os.IsNotExist(err) {
		log.Printf("\tno dashboards present, skipped (modulePath: %s)", modulePath)
		return kibanaContent{}, nil
	} else if err != nil {
		return kibanaContent{}, errors.Wrapf(err, "reading module dashboard directory failed (path: %s)",
			moduleDashboardPath)
	}

	kibana := kibanaContent{
		dashboardFiles:     map[string][]byte{},
		visualizationFiles: map[string][]byte{},
	}
	for _, moduleDashboard := range moduleDashboards {
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

		extractedDashboards, err := extractKibanaObjects(migrated, "dashboard")
		if err != nil {
			return kibanaContent{}, errors.Wrapf(err, "extracting kibana dashboards failed")
		}

		for k, v := range extractedDashboards {
			kibana.dashboardFiles[k] = v
		}

		extractedVisualizations, err := extractKibanaObjects(migrated, "visualization")
		if err != nil {
			return kibanaContent{}, errors.Wrapf(err, "extracting kibana visualizations failed")
		}

		for k, v := range extractedVisualizations {
			kibana.visualizationFiles[k] = v
		}
	}
	return kibana, nil
}

func extractKibanaObjects(dashboardFile []byte, objectType string) (map[string][]byte, error) {
	return nil, errors.New("not implemented")
}

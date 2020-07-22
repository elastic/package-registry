// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// +build integration

package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/magefile/mage/sh"
	"github.com/stretchr/testify/assert"
)

// TestSetup tests if Kibana can be run against the current registry
// and the setup command works as expected.
func TestSetup(t *testing.T) {

	// Mage fetchPackageStorage is needed to pull in the packages from package-storage
	err := sh.Run("mage", "fetchPackageStorage")
	if err != nil {
		t.Error(err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}
	defer os.Chdir(currentDir)
	err = os.Chdir("testing/environments")
	if err != nil {
		t.Error(err)
	}

	// Make sure services are shut down again at the end of the test
	defer func() {
		err = sh.Run("docker-compose", "-f", "snapshot.yml", "-f", "local.yml", "down", "-v")
		if err != nil {
			t.Error(err)
		}

	}()
	// Spin up services
	go func() {
		err = sh.Run("docker-compose", "-f", "snapshot.yml", "pull")
		if err != nil {
			t.Error(err)
		}

		err = sh.Run("docker-compose", "-f", "snapshot.yml", "-f", "local.yml", "up", "--force-recreate", "--remove-orphans", "--build")
		if err != nil {
			t.Error(err)
		}
	}()

	// Check for 5 minutes if service is available
	for i := 0; i < 5*60; i++ {
		output, _ := sh.Output("docker-compose", "-f", "snapshot.yml", "-f", "local.yml", "ps")
		if err != nil {
			// Log errors but do not act on it as at first it might not be ready yet
			log.Println(err)
		}
		// 3 services must report healthy
		c := strings.Count(output, "healthy")
		if c == 3 {
			break
		}

		// Wait 1 second between each iteration
		time.Sleep(1 * time.Second)
	}

	// Run setup in ingest_manager against registry to see if no errors are returned
	req, err := http.NewRequest("POST", "http://elastic:changeme@localhost:5601/api/ingest_manager/setup", nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Add("kbn-xsrf", "ingest_manager")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
	}

	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()
	assert.Equal(t, 200, resp.StatusCode)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	// Leaving this in as it could become useful for debugging purpose
	log.Println(string(body))
}

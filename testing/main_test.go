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

	err := os.Chdir("environments")
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
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	log.Println(string(body))
}

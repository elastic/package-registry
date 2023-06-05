// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/trigger-jenkins-buildkite-plugin/jenkins"
)

const (
	updatePackageRemoteJob    = "package_storage/job/update-package-registry-job-remote"
	updatePackageRemoteJobKey = "update-package-registry"
)

var allowedJenkinsJobs = map[string]string{
	updatePackageRemoteJobKey: updatePackageRemoteJob,
}

var (
	jenkinsHost  = os.Getenv("JENKINS_HOST_SECRET")
	jenkinsUser  = os.Getenv("JENKINS_USERNAME_SECRET")
	jenkinsToken = os.Getenv("JENKINS_TOKEN")
)

func jenkinsJobOptions() []string {
	keys := make([]string, 0, len(allowedJenkinsJobs))
	for k := range allowedJenkinsJobs {
		keys = append(keys, k)
	}
	return keys
}

func main() {
	jenkinsJob := flag.String("jenkins-job", "", fmt.Sprintf("Jenkins job to trigger. Allowed values: %s", strings.Join(jenkinsJobOptions(), " ,")))
	waitingTime := flag.Duration("waiting-time", 5*time.Second, fmt.Sprintf("Waiting period between each retry"))
	growthFactor := flag.Float64("growth-factor", 1.25, fmt.Sprintf("Growth-Factor used for exponential backoff delays"))
	retries := flag.Int("retries", 20, fmt.Sprintf("Number of retries to trigger the job"))
	maxWaitingTime := flag.Duration("max-waiting-time", 60*time.Minute, fmt.Sprintf("Maximum waiting time per each retry"))

	version := flag.String("version", "", "Package registry version")
	dryRun := flag.Bool("dry-run", true, "Dry run true by default")
	draft := flag.Bool("draft", true, "Create draft PRs. True by default")

	async := flag.Bool("async", false, "Run async the Jenkins job")
	flag.Parse()

	if _, ok := allowedJenkinsJobs[*jenkinsJob]; !ok {
		log.Fatal("Invalid jenkins job")
	}

	log.Printf("Triggering job: %s", allowedJenkinsJobs[*jenkinsJob])

	ctx := context.Background()
	client, err := jenkins.NewJenkinsClient(ctx, jenkinsHost, jenkinsUser, jenkinsToken)
	if err != nil {
		log.Fatalf("error creating jenkins client: %v", err)
	}

	opts := jenkins.Options{
		WaitingTime:    *waitingTime,
		Retries:        *retries,
		GrowthFactor:   *growthFactor,
		MaxWaitingTime: *maxWaitingTime,
	}

	switch *jenkinsJob {
	case updatePackageRemoteJobKey:
		err = runUpdateJob(ctx, client, *async, allowedJenkinsJobs[*jenkinsJob],
			*version, strconv.FormatBool(*draft), strconv.FormatBool(*dryRun), opts)
	default:
		log.Fatal("unsupported jenkins job")
	}

	if err != nil {
		log.Fatalf("Error: %s", err)
	}
}

func runUpdateJob(ctx context.Context, client *jenkins.JenkinsClient, async bool, jobName string,
	version, draft, dryRun string, opts jenkins.Options) error {
	if version == "" {
		return fmt.Errorf("missing parameter --version")
	}
	params := map[string]string{
		"dry_run": draft,
		"draft":   dryRun,
		"version": version,
	}
	return client.RunJob(ctx, jobName, async, params, opts)
}

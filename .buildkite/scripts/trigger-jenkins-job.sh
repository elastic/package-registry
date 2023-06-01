#!/bin/bash

cd .buildkite/scripts/triggerJenkinsJob

go run main.go \
    --jenkins-job "update-package-registry" \
    --version "3" \
    --dry-run "true" \
    --draft "true"

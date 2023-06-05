#!/bin/bash

cd .buildkite/scripts/triggerJenkinsJob

go run main.go \
    --jenkins-job="update-package-registry" \
    --version="${BUILDKITE_TAG}" \
    --dry-run="${DRY_RUN}" \
    --draft="${DRAFT_PR}"

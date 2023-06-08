#!/bin/bash
set -euo pipefail

source .buildkite/scripts/tooling.sh

transformTagAndValidate() {
    version=$1
    version=${version//\v/}
    if [[ $version =~ ^[0-9]+.[0-9]+.[0-9]+(-[A-Za-z0-9_]+)?$ ]]; then
        echo "valid version: ${version}"
    else
        echo "unvalid version: ${version}"
        exit 1
    fi
}

DOCKER_TAG_VERSION=$(buildkite-agent meta-data get DOCKER_TAG_VERSION)
transformTagAndValidate "$DOCKER_TAG_VERSION"
buildkite-agent meta-data set DOCKER_TAG_VERSION "${DOCKER_TAG_VERSION}"

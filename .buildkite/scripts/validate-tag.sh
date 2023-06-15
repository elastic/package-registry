#!/bin/bash
set -euo pipefail

source .buildkite/scripts/tooling.sh

transformTagAndValidate() {
    local version=$1
    version=${version/#v/}

    if [[ $version =~ ^[0-9]+.[0-9]+.[0-9]+(-[A-Za-z0-9_]+)?$ ]]; then
        echo "valid version: ${version}"
        DOCKER_TAG="${version}"
    else
        echo "invalid version: ${version}"
        echo "unsupported docker tag, please use the major.minor.path(-prerelease)? format (for example: 1.2.3 or 1.2.3-alpha)."
        exit 1
    fi
}

set +e
DOCKER_TAG=$(buildkite-agent meta-data get DOCKER_TAG)
set -e
if [ -z "${DOCKER_TAG:-}" ]; then
    echo "erorr: DOCKER_TAG is not set up, Please setup, DOCKER_TAG"
    exit 1
fi

echo "Validating tag parameter: ${DOCKER_TAG}..."
transformTagAndValidate "$DOCKER_TAG"
buildkite-agent meta-data set DOCKER_TAG "${DOCKER_TAG}"

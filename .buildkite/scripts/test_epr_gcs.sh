#!/bin/bash

cleanup () {
    echo "~~~ Cleaning up..."
    if [[ "${DOCKER_COMPOSE_EPR_PATH:-""}" != "" ]]; then
        echo "Stopping docker-compose projects and removing volumes"
        docker-compose -f "${DOCKER_COMPOSE_EPR_PATH}" down -v || true
        docker-compose -f "${DOCKER_COMPOSE_EPR_GCS_PATH}" down -v || true
    fi

    if [[ "${LOCAL_BUCKET_PATH:-""}" != "" ]]; then
        echo "Removing local bucket folder: ${LOCAL_BUCKET_PATH}"
        rm -rf "${LOCAL_BUCKET_PATH}" || true
    fi
}

trap cleanup EXIT

source .buildkite/scripts/pre-install-command.sh

set -euo pipefail

if running_on_buildkite ; then
    add_bin_path
    with_jq
    with_go
    with_mage
fi

test_service() {
    if ! curl -s "http://localhost:8080/" ; then
        echo "EPR service is not running"
        return 1
    fi
    return 0
}

test_packages() {
    local expected="$1"

    total_packages=$(curl -s  "http://localhost:8080/search?prerelease=true&all=true" | jq -r '.| length')
    echo "Found ${total_packages} packages in the EPR instance"
    if [[ "${total_packages}" -ne "${expected}" ]]; then
        return 1
    fi
    return 0
}

test_services() {
    local compose_file="$1"
    if ! docker-compose -f "${compose_file}" up -d ; then
        echo "Failed to start docker-compose project: ${compose_file}"
        return 1
    fi

    if ! test_service; then
        return 1
    fi

    if ! test_packages "${NUMBER_OF_PACKAGES}"; then
        echo "Test failed: Expected ${NUMBER_OF_PACKAGES} packages, but found a different number in the EPR"
        return 1
    fi

    if ! docker-compose -f "${compose_file}" down -v ; then
        echo "Failed to stop docker-compose project: ${compose_file}"
        return 1
    fi
    return 0
}

cd "${WORKSPACE}"
echo "--- Building EPR Docker image"
mage dockerBuild latest

cd "${WORKSPACE}/dev/"
DOCKER_COMPOSE_EPR_PATH="$(pwd)/docker-compose-epr.yml"
DOCKER_COMPOSE_EPR_GCS_PATH="$(pwd)/docker-compose-epr-gcs.yml"

export BUCKET_NAME="example"
export LOCAL_BUCKET_PATH="${WORKSPACE}/build/fakeserver/"

FAKE_GCS_SERVER_VERSION="$(grep fake-gcs-server ../go.mod | awk '{print $2}' | tr -d 'v')"
export FAKE_GCS_SERVER_VERSION


export SEARCH_INDEX_PATH="../storage/testdata/search-index-all-full.json"
NUMBER_OF_PACKAGES=$( jq -r '.packages | length' "${SEARCH_INDEX_PATH}" )

echo "--- Running EPR service with fakce GCS server running within EPR"

test_services "${DOCKER_COMPOSE_EPR_PATH}"


echo "--- Running EPR and fake GCS server as services"

./setup_bucket_gcs.sh -b "${BUCKET_NAME}" -c 1 -i ../storage/testdata/search-index-all-full.json -p "${LOCAL_BUCKET_PATH}"

test_services "${DOCKER_COMPOSE_EPR_GCS_PATH}"

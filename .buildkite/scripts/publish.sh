#!/bin/bash
set -euo pipefail

source .buildkite/scripts/tooling.sh

pushDockerImage() {
    docker build \
        -t "${DOCKER_IMG_TAG}" \
        --label BRANCH_NAME="${TAG_NAME}" \
        --label GIT_SHA="${BUILDKITE_COMMIT}" \
        --label GO_VERSION="${SETUP_GOLANG_VERSION}" \
        --label TIMESTAMP="$(date +%Y-%m-%d_%H:%M)" \
        .
    retry 3 docker push "${DOCKER_IMG_TAG}"
    echo "Docker image pushed: ${DOCKER_IMG_TAG}"
    docker tag "${DOCKER_IMG_TAG}" "${DOCKER_IMG_TAG_BRANCH}"
    retry 3 docker push "${DOCKER_IMG_TAG_BRANCH}"
    echo "Docker image pushed: ${DOCKER_IMG_TAG_BRANCH}"
}

if [[ -n "${BUILDKITE_PULL_REQUEST:-}" ]]; then
    DOCKER_NAMESPACE="${DOCKER_IMG_PR}"
    TAG_NAME="PR-${BUILDKITE_PULL_REQUEST}"
else
    DOCKER_NAMESPACE="${DOCKER_IMG}"
    TAG_NAME="${BUILDKITE_BRANCH}"  # e.g. main
fi

# if tag exists use tag instead
if [ -n "${BUILDKITE_TAG:-}" ]; then
    TAG_NAME="${BUILDKITE_TAG}"
fi

DOCKER_IMG_TAG="${DOCKER_NAMESPACE}:${BUILDKITE_COMMIT}"
DOCKER_IMG_TAG_BRANCH="${DOCKER_NAMESPACE}:${TAG_NAME}"

pushDockerImage

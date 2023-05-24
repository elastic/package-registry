#!/bin/bash
set -euo pipefail

pushDockerImage() {
    docker build \
        -t "${DOCKER_IMG_TAG}" \
        --label BRANCH_NAME="${GITHUB_PR_BRANCH}" \
        --label GIT_SHA="${BUILDKITE_COMMIT}" \
        --label GO_VERSION="${SETUP_GOLANG_VERSION}" \
        --label TIMESTAMP="$(date +%Y-%m-%d_%H:%M)" \
        .
    docker push "${DOCKER_IMG_TAG}"
    docker tag "${DOCKER_IMG_TAG}" "${DOCKER_IMG_TAG_BRANCH}"
    docker push "${DOCKER_IMG_TAG_BRANCH}"
}

if [[ "${BUILDKITE_PULL_REQUEST}" == "false" ]]; then
    DOCKER_NAMESPACE="${DOCKER_IMG}"
else
    DOCKER_NAMESPACE="${DOCKER_IMG_PR}"
fi

DOCKER_IMG_TAG="${DOCKER_NAMESPACE}:${BUILDKITE_COMMIT}"
DOCKER_IMG_TAG_BRANCH="${DOCKER_NAMESPACE}:${GITHUB_PR_BRANCH}"

pushDockerImage

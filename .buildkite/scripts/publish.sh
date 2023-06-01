#!/bin/bash
set -euo pipefail

source .buildkite/scripts/tooling.sh

pushDockerImage() {
    docker build \
        -t "${DOCKER_IMG_TAG}" \
        --label BRANCH_NAME="${GITHUB_PR_BRANCH}" \
        --label GIT_SHA="${BUILDKITE_COMMIT}" \
        --label GO_VERSION="${SETUP_GOLANG_VERSION}" \
        --label TIMESTAMP="$(date +%Y-%m-%d_%H:%M)" \
        .
    retry 3 docker push "${DOCKER_IMG_TAG}"
    docker tag "${DOCKER_IMG_TAG}" "${DOCKER_IMG_TAG_BRANCH}"
    retry 3 docker push "${DOCKER_IMG_TAG_BRANCH}"
}

if [[ "${BUILDKITE_PULL_REQUEST}" == "false" ]]; then
    DOCKER_NAMESPACE="${DOCKER_IMG}"
else
    DOCKER_NAMESPACE="${DOCKER_IMG_PR}"
fi

TAG_NAME="${GITHUB_PR_BRANCH}"
# if tag exists use tag instead
if [[ -n ${BUILDKITE_TAG} ]]; then
    TAG_NAME=${BUILDKITE_TAG}
fi

DOCKER_IMG_TAG="${DOCKER_NAMESPACE}:${BUILDKITE_COMMIT}"
DOCKER_IMG_TAG_BRANCH="${DOCKER_NAMESPACE}:${TAG_NAME}"

pushDockerImage

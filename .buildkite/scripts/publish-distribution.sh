#!/bin/bash
# DRY_RUN is true by default - script will not push to registry
source .buildkite/scripts/tooling.sh

set -euo pipefail

DOCKER_TAG=$(buildkite-agent meta-data get DOCKER_TAG)
echo "version for tagging: ${DOCKER_TAG}"

DOCKER_IMG_SOURCE="${DOCKER_IMAGE}:${TAG_NAME}"

if [[ "${TAG_NAME}" == "production" ]]; then
    DOCKER_IMG_TARGET="${DOCKER_IMAGE}:${DOCKER_TAG}"
else
    DOCKER_IMG_TARGET="${DOCKER_IMAGE}:${TAG_NAME}-${DOCKER_TAG}"
fi

echo "Docker retag"
docker buildx create --use

# do not push if DRY_RUN is true
if [[ ${DRY_RUN:-true} == "true" ]]; then
    docker buildx imagetools create --dry-run -t "${DOCKER_IMG_TARGET}" "${DOCKER_IMG_SOURCE}"
    docker buildx imagetools create --dry-run -t "${DOCKER_IMG_TARGET}-ubi" "${DOCKER_IMG_SOURCE}-ubi"
else
    retry 3 docker buildx imagetools create -t "${DOCKER_IMG_TARGET}" "${DOCKER_IMG_SOURCE}"
    retry 3 docker buildx imagetools create -t "${DOCKER_IMG_TARGET}-ubi" "${DOCKER_IMG_SOURCE}-ubi"
    echo "Docker image pushed: ${DOCKER_IMG_TARGET}"
fi

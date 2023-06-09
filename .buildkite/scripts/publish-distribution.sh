#!/bin/bash
# DRY_RUN is true by default - script will not push to registry
set -euo pipefail

source .buildkite/scripts/tooling.sh

DOCKER_TAG=$(buildkite-agent meta-data get DOCKER_TAG)
echo "version for tagging: ${DOCKER_TAG}"
DOCKER_IMG_SOURCE="${DOCKER_REGISTRY}/package-registry/distribution:${TAG_NAME}"
DOCKER_IMG_TARGET="${DOCKER_REGISTRY}/package-registry/distribution:${TAG_NAME}-${DOCKER_TAG}"

echo "Docker pull"
retry 3 docker pull "${DOCKER_IMG_SOURCE}"
echo "Docker retag"
docker tag "${DOCKER_IMG_SOURCE}" "${DOCKER_IMG_TARGET}"

# do not push if DRY_RUN is true
if [[ ${DRY_RUN:-true} == "true" ]]; then
    echo "Docker push command will be: retry 3 docker push ${DOCKER_IMG_TARGET}"
else
    echo "Docker push:"
    retry 3 docker push "${DOCKER_IMG_TARGET}"
fi

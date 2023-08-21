#!/bin/bash
set -euo pipefail

source .buildkite/scripts/tooling.sh

docker pull docker.elastic.co/observability-ci/package-registry:v1.21.0
docker tag docker.elastic.co/observability-ci/package-registry:v1.21.0 docker.elastic.co/package-registry/package-registry:v1.21.0

echo "docker push docker.elastic.co/package-registry/package-registry:v1.21.0"


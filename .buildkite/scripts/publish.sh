#!/bin/bash
source .buildkite/scripts/tooling.sh
source .buildkite/scripts/publish.helpers.sh

set -euo pipefail

set_image_tags

build_push_docker_image "docker.elastic.co/wolfi/chainguard-base" ""
build_push_docker_image "registry.access.redhat.com/ubi9/ubi-minimal:9.6" "-ubi"
build_push_docker_image "docker.elastic.co/wolfi/chainguard-base-fips" "-fips"

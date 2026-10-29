#!/usr/bin/env bats

# bats file_tags=tag:publish

# These tests run the whole publish.sh script with a stub of docker.

setup() {
    load 'setup_command'
    _common_setup

    _set_publish_env
    _mock_docker
}

@test "publish builds and pushes the three flavours in order" {
    _set_buildkite_env main false

    run bash .buildkite/scripts/publish.sh
    assert_success

    run grep "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_line --index 0 --partial "-t ${DOCKER_IMG}:abc123 -t ${DOCKER_IMG}:main --build-arg"
    assert_line --index 0 --partial "RUNNER_IMAGE=docker.elastic.co/wolfi/chainguard-base --build-arg FIPS=0"
    assert_line --index 1 --partial "-t ${DOCKER_IMG}:abc123-ubi -t ${DOCKER_IMG}:main-ubi --build-arg"
    assert_line --index 1 --partial "RUNNER_IMAGE=registry.access.redhat.com/ubi9/ubi-minimal:9.6 --build-arg FIPS=0"
    assert_line --index 2 --partial "-t ${DOCKER_IMG}:abc123-fips -t ${DOCKER_IMG}:main-fips --build-arg"
    assert_line --index 2 --partial "RUNNER_IMAGE=docker.elastic.co/wolfi/chainguard-base-fips --build-arg FIPS=1"
    assert_equal "${#lines[@]}" "3"
}

@test "publish pushes pull requests to the PR namespace" {
    _set_buildkite_env feature 123

    run bash .buildkite/scripts/publish.sh
    assert_success

    run grep "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_line --index 0 --partial "-t ${DOCKER_IMG_PR}:abc123 -t ${DOCKER_IMG_PR}:PR-123 --build-arg"
    assert_line --index 1 --partial "-t ${DOCKER_IMG_PR}:abc123-ubi -t ${DOCKER_IMG_PR}:PR-123-ubi --build-arg"
    assert_line --index 2 --partial "-t ${DOCKER_IMG_PR}:abc123-fips -t ${DOCKER_IMG_PR}:PR-123-fips --build-arg"
    refute_output --partial "${DOCKER_IMG}:"
}

@test "publish uses the git tag as image tag when building a tag" {
    _set_buildkite_env main false v1.2.3

    run bash .buildkite/scripts/publish.sh
    assert_success

    run grep "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_line --index 0 --partial "-t ${DOCKER_IMG}:abc123 -t ${DOCKER_IMG}:v1.2.3 --build-arg"
    assert_line --index 1 --partial "-t ${DOCKER_IMG}:abc123-ubi -t ${DOCKER_IMG}:v1.2.3-ubi --build-arg"
    assert_line --index 2 --partial "-t ${DOCKER_IMG}:abc123-fips -t ${DOCKER_IMG}:v1.2.3-fips --build-arg"
}

@test "publish stops at the first flavour whose build keeps failing" {
    _set_buildkite_env main false
    export DOCKER_STUB_BUILD_FAILURES=100

    run bash .buildkite/scripts/publish.sh
    assert_failure 1

    run grep -c "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_output "3"
    run grep -c -- "-ubi" "${DOCKER_CALLS_FILE}"
    assert_output "0"
}

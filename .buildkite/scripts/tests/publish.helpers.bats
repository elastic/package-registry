#!/usr/bin/env bats

# bats file_tags=tag:publish-helpers

setup() {
    load 'setup_command'
    _common_setup
    _set_publish_env

    sources="source .buildkite/scripts/tooling.sh; source .buildkite/scripts/publish.helpers.sh;"
}

# Write the command file that calls set_image_tags and prints the resulting variables, then run it.
_run_set_image_tags() {
    cat <<EOF > "${COMMAND_FILE}"
${sources}
set_image_tags
echo "\${DOCKER_NAMESPACE} \${TAG_NAME} \${DOCKER_IMG_TAG} \${DOCKER_IMG_TAG_BRANCH}"
EOF
    run ${test_command}
}

# Write the command file that builds a flavour (set_image_tags + build_push_docker_image), then run.
#
# Usage: _run_build_push <runner_image> [<tag_suffix>]
_run_build_push() {
    cat <<EOF > "${COMMAND_FILE}"
${sources}
set_image_tags
build_push_docker_image "$1" "${2:-}"
EOF
    run ${test_command}
}

# Print the "docker buildx build" call of a flavour, identified by the suffix of its tags.
_build_call() {
    grep -F -- "-t ${DOCKER_IMG}:${BUILDKITE_COMMIT}$1 -t " "${DOCKER_CALLS_FILE}"
}

@test "set_image_tags uses the branch name for branch builds" {
    _set_buildkite_env main false
    _run_set_image_tags
    assert_success
    assert_output "${DOCKER_IMG} main ${DOCKER_IMG}:abc123 ${DOCKER_IMG}:main"
}

@test "set_image_tags uses the PR namespace and tag for pull requests" {
    _set_buildkite_env feature 123
    _run_set_image_tags
    assert_success
    assert_output "${DOCKER_IMG_PR} PR-123 ${DOCKER_IMG_PR}:abc123 ${DOCKER_IMG_PR}:PR-123"
}

@test "set_image_tags uses the git tag when building a tag" {
    _set_buildkite_env main false v1.2.3
    _run_set_image_tags
    assert_success
    assert_output "${DOCKER_IMG} v1.2.3 ${DOCKER_IMG}:abc123 ${DOCKER_IMG}:v1.2.3"
}

@test "docker fails loudly when a test calls it without _mock_docker" {
    run docker images
    assert_failure
    assert_output --partial "ERROR: 'docker' called in a bats test without _mock_docker"
}

@test "build_push_docker_image builds the base flavour for amd64 and arm64" {
    _set_buildkite_env main false
    _mock_docker
    _run_build_push "docker.elastic.co/wolfi/chainguard-base" ""
    assert_success
    assert_output --partial "Docker images pushed: ${DOCKER_IMG}:abc123 ${DOCKER_IMG}:main"
    run grep -c "^buildx create --use" "${DOCKER_CALLS_FILE}"
    assert_output "1"
    run _build_call ""
    assert_output --partial "buildx build --push --platform linux/amd64,linux/arm64/v8 --progress plain"
    assert_output --partial "-t ${DOCKER_IMG}:abc123 -t ${DOCKER_IMG}:main --build-arg"
    assert_output --partial "--build-arg GO_VERSION=$(cat .go-version)"
    assert_output --partial "--build-arg BUILDER_IMAGE=docker.elastic.co/wolfi/go"
    assert_output --partial "--build-arg RUNNER_IMAGE=docker.elastic.co/wolfi/chainguard-base --build-arg FIPS=0"
    assert_output --partial "--label BRANCH_NAME=main --label GIT_SHA=abc123 --label GO_VERSION=$(cat .go-version) --label TIMESTAMP="
}

@test "build_push_docker_image adds the suffix to the tags of the ubi flavour" {
    _set_buildkite_env main false
    _mock_docker
    _run_build_push "registry.access.redhat.com/ubi9/ubi-minimal:9.6" "-ubi"
    assert_success
    run _build_call "-ubi"
    assert_output --partial "-t ${DOCKER_IMG}:abc123-ubi -t ${DOCKER_IMG}:main-ubi --build-arg"
    assert_output --partial "--build-arg RUNNER_IMAGE=registry.access.redhat.com/ubi9/ubi-minimal:9.6 --build-arg FIPS=0"
}

@test "build_push_docker_image enables FIPS in the fips flavour" {
    _set_buildkite_env main false
    _mock_docker
    _run_build_push "docker.elastic.co/wolfi/chainguard-base-fips" "-fips"
    assert_success
    run _build_call "-fips"
    assert_output --partial "--build-arg RUNNER_IMAGE=docker.elastic.co/wolfi/chainguard-base-fips --build-arg FIPS=1"
}

@test "build_push_docker_image labels the image with the git tag when building a tag" {
    _set_buildkite_env main false v1.2.3
    _mock_docker
    _run_build_push "docker.elastic.co/wolfi/chainguard-base" ""
    assert_success
    run _build_call ""
    assert_output --partial "-t ${DOCKER_IMG}:abc123 -t ${DOCKER_IMG}:v1.2.3 --build-arg"
    assert_output --partial "--label BRANCH_NAME=v1.2.3"
}

@test "build_push_docker_image retries a failed build" {
    _set_buildkite_env main false
    _mock_docker
    export DOCKER_STUB_BUILD_FAILURES=2
    _run_build_push "docker.elastic.co/wolfi/chainguard-base" ""
    assert_success
    run grep -c "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_output "3"
}

@test "build_push_docker_image fails when the build keeps failing" {
    _set_buildkite_env main false
    _mock_docker
    export DOCKER_STUB_BUILD_FAILURES=100
    cat <<EOF > "${COMMAND_FILE}"
${sources}
set_image_tags
build_push_docker_image "docker.elastic.co/wolfi/chainguard-base" ""
echo "must not be reached"
EOF

    run ${test_command}
    assert_failure 1
    refute_output --partial "must not be reached"
    refute_output --partial "Docker images pushed"
    run grep -c "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_output "3"
}

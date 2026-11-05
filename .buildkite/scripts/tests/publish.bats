#!/usr/bin/env bats

# bats file_tags=tag:publish

# These tests run the whole publish.sh script with a stub of docker.

setup() {
    load 'setup_command'
    _common_setup

    _set_publish_env
    _mock_docker

    # publish.sh uploads a build-unique image ref per flavour (digest resolution moved to the separate
    # resolve_image_digests.sh step); mock buildkite-agent so the upload never reaches the network.
    _mock_buildkite_agent
}

# publish.sh writes the image ref files in the working directory (the repository root); remove them so
# they do not linger as untracked files.
teardown() {
    rm -f image-ref-*.txt
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

    # a single buildx builder is created for the whole run, not one per flavour
    run grep -c "^buildx create --use" "${DOCKER_CALLS_FILE}"
    assert_output "1"
}

@test "publish pushes pull requests to the PR namespace" {
    _set_buildkite_env feature 123

    run bash .buildkite/scripts/publish.sh
    assert_success

    run grep "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_line --index 0 --partial "-t ${DOCKER_IMG_PR}:abc123 -t ${DOCKER_IMG_PR}:PR-123 --build-arg"
    assert_line --index 1 --partial "-t ${DOCKER_IMG_PR}:abc123-ubi -t ${DOCKER_IMG_PR}:PR-123-ubi --build-arg"
    assert_line --index 2 --partial "-t ${DOCKER_IMG_PR}:abc123-fips -t ${DOCKER_IMG_PR}:PR-123-fips --build-arg"
    assert_equal "${#lines[@]}" "3"
    refute_output --partial "${DOCKER_IMG}:"
}

@test "publish uses the git tag as image tag when building a tag" {
    _set_buildkite_env main false v1.2.3

    run bash .buildkite/scripts/publish.sh
    assert_success

    run grep "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_line --index 0 --partial "-t ${DOCKER_IMG}:v1.2.3 --build-arg"
    assert_line --index 1 --partial "-t ${DOCKER_IMG}:v1.2.3-ubi --build-arg"
    assert_line --index 2 --partial "-t ${DOCKER_IMG}:v1.2.3-fips --build-arg"
    # the commit tag is not re-pushed on tag builds (it already exists from the branch build)
    refute_output --partial "${DOCKER_IMG}:abc123"
}

@test "publish uploads one build-unique image ref per flavour for signing" {
    _set_buildkite_env main false

    run bash .buildkite/scripts/publish.sh
    assert_success

    # one ref file per flavour, referencing the commit tag (the build-unique ref on branch builds)
    assert_equal "$(cat image-ref-base.txt)" "${DOCKER_IMG}:abc123"
    assert_equal "$(cat image-ref-ubi.txt)" "${DOCKER_IMG}:abc123-ubi"
    assert_equal "$(cat image-ref-fips.txt)" "${DOCKER_IMG}:abc123-fips"

    # each ref file uploaded as a buildkite artifact
    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_line "artifact upload image-ref-base.txt"
    assert_line "artifact upload image-ref-ubi.txt"
    assert_line "artifact upload image-ref-fips.txt"
}

# Tag builds forward the release tag (:<tag>) as the build-unique ref: only the tag build pushes it and
# it is the released image. Branch builds forward the commit tag instead.
@test "publish uploads the release-tag ref on tag builds" {
    _set_buildkite_env main false v1.2.3

    run bash .buildkite/scripts/publish.sh
    assert_success

    assert_equal "$(cat image-ref-base.txt)" "${DOCKER_IMG}:v1.2.3"
    assert_equal "$(cat image-ref-ubi.txt)" "${DOCKER_IMG}:v1.2.3-ubi"
    assert_equal "$(cat image-ref-fips.txt)" "${DOCKER_IMG}:v1.2.3-fips"
}

# PR builds upload refs too, referencing the PR namespace, so the resolve step exercises the full path
# on every PR. The resolve step names their digest files so the signing pipeline cannot collect them.
@test "publish uploads PR image refs referencing the PR namespace" {
    _set_buildkite_env feature 123

    run bash .buildkite/scripts/publish.sh
    assert_success

    assert_equal "$(cat image-ref-base.txt)" "${DOCKER_IMG_PR}:abc123"
    assert_equal "$(cat image-ref-ubi.txt)" "${DOCKER_IMG_PR}:abc123-ubi"
    assert_equal "$(cat image-ref-fips.txt)" "${DOCKER_IMG_PR}:abc123-fips"

    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_line "artifact upload image-ref-base.txt"
    assert_line "artifact upload image-ref-ubi.txt"
    assert_line "artifact upload image-ref-fips.txt"
}

@test "publish stops at the first flavour whose build keeps failing" {
    _set_buildkite_env main false
    export DOCKER_STUB_BUILD_FAILURES=100

    run bash .buildkite/scripts/publish.sh
    assert_failure 1

    # 3 attempts of the base flavour, the number of attempts of "retry" in publish.helpers.sh
    run grep "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_success
    assert_equal "${#lines[@]}" "3"
    refute_output --partial "abc123-ubi"
}

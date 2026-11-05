#!/usr/bin/env bats

# bats file_tags=tag:publish

# These tests run the whole publish.sh script with a stub of docker.

setup() {
    load 'setup_command'
    _common_setup

    _set_publish_env
    _mock_docker

    # non-PR builds install crane and upload digest files for signing; mock both so the push path
    # never reaches the network, and crane resolves to a fixed digest.
    export WORKSPACE="${BATS_TEST_TMPDIR}/workspace"
    mkdir -p "${WORKSPACE}"
    export CRANE_STUB_DIGEST="sha256:deadbeef"
    _install_fake_crane
    _mock_buildkite_agent
}

# publish.sh writes the digest files in the working directory (the repository root); remove them so
# they do not linger as untracked files.
teardown() {
    rm -f package-registry-*-images-digest.txt pr-package-registry-*-images-digest.txt
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

@test "publish signs non-PR builds by uploading one digest file per flavour" {
    _set_buildkite_env main false

    run bash .buildkite/scripts/publish.sh
    assert_success

    # one digest file per flavour, referencing the commit tag and the resolved digest
    assert_equal "$(cat package-registry-base-images-digest.txt)" "${DOCKER_IMG}:abc123@sha256:deadbeef"
    assert_equal "$(cat package-registry-ubi-images-digest.txt)" "${DOCKER_IMG}:abc123-ubi@sha256:deadbeef"
    assert_equal "$(cat package-registry-fips-images-digest.txt)" "${DOCKER_IMG}:abc123-fips@sha256:deadbeef"

    # each digest file uploaded as a buildkite artifact
    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_line "artifact upload package-registry-base-images-digest.txt"
    assert_line "artifact upload package-registry-ubi-images-digest.txt"
    assert_line "artifact upload package-registry-fips-images-digest.txt"
}

# Tag builds sign by the release tag (:<tag>): only the tag build pushes it and it is the released
# image, so it is a reliable, build-unique reference. Branch builds sign by the commit tag instead.
@test "publish signs tag builds using the git tag in the digest reference" {
    _set_buildkite_env main false v1.2.3

    run bash .buildkite/scripts/publish.sh
    assert_success

    assert_equal "$(cat package-registry-base-images-digest.txt)" "${DOCKER_IMG}:v1.2.3@sha256:deadbeef"
    assert_equal "$(cat package-registry-ubi-images-digest.txt)" "${DOCKER_IMG}:v1.2.3-ubi@sha256:deadbeef"
    assert_equal "$(cat package-registry-fips-images-digest.txt)" "${DOCKER_IMG}:v1.2.3-fips@sha256:deadbeef"
}

# PR builds exercise the signing-input path too, but write and upload the digest files under a pr-
# prefixed name that cannot match the signing pipeline's ARTIFACT_PATTERN, so the signing pipeline
# would never collect them (and its trigger steps are gated off for PRs anyway).
@test "publish uploads PR digest files under a name the signing pipeline cannot collect" {
    _set_buildkite_env feature 123

    run bash .buildkite/scripts/publish.sh
    assert_success

    # the digest files are written under the pr- prefixed name, referencing the PR namespace...
    assert_equal "$(cat pr-package-registry-base-images-digest.txt)" "${DOCKER_IMG_PR}:abc123@sha256:deadbeef"
    assert_equal "$(cat pr-package-registry-ubi-images-digest.txt)" "${DOCKER_IMG_PR}:abc123-ubi@sha256:deadbeef"
    assert_equal "$(cat pr-package-registry-fips-images-digest.txt)" "${DOCKER_IMG_PR}:abc123-fips@sha256:deadbeef"

    # ...and not under the signing name (which matches ARTIFACT_PATTERN)
    assert_file_not_exists package-registry-base-images-digest.txt
    assert_file_not_exists package-registry-ubi-images-digest.txt
    assert_file_not_exists package-registry-fips-images-digest.txt

    # each pr- prefixed file is uploaded as a buildkite artifact
    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_line "artifact upload pr-package-registry-base-images-digest.txt"
    assert_line "artifact upload pr-package-registry-ubi-images-digest.txt"
    assert_line "artifact upload pr-package-registry-fips-images-digest.txt"
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

#!/usr/bin/env bats

# bats file_tags=tag:resolve-image-digests

# These tests run the whole resolve_image_digests.sh script with stubs of crane and buildkite-agent.

setup() {
    load 'setup_command'
    _common_setup

    _set_publish_env

    # the script installs crane (with_crane) and reads digests from the registry; install a fake crane
    # so nothing reaches the network and crane resolves to a fixed digest.
    export WORKSPACE="${BATS_TEST_TMPDIR}/workspace"
    mkdir -p "${WORKSPACE}"
    export CRANE_STUB_DIGEST="sha256:deadbeef"
    _install_fake_crane
    _mock_buildkite_agent
}

# the script downloads image refs and writes digest files in the working directory (the repository
# root); remove them so they do not linger as untracked files.
teardown() {
    rm -f image-ref-*.txt package-registry-*-images-digest.txt pr-package-registry-*-images-digest.txt
}

@test "resolve_image_digests.sh downloads the refs and resolves them into digest files" {
    _set_buildkite_env main false
    # the publish step's artifacts, already present (buildkite-agent download is stubbed as a no-op)
    echo "${DOCKER_IMG}:abc123" > image-ref-base.txt
    echo "${DOCKER_IMG}:abc123-ubi" > image-ref-ubi.txt
    echo "${DOCKER_IMG}:abc123-fips" > image-ref-fips.txt

    run bash .buildkite/scripts/resolve_image_digests.sh
    assert_success

    # it downloaded the refs with the expected glob
    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_line "artifact download image-ref-*.txt ."

    assert_equal "$(cat package-registry-base-images-digest.txt)" "${DOCKER_IMG}:abc123@sha256:deadbeef"
    assert_equal "$(cat package-registry-ubi-images-digest.txt)" "${DOCKER_IMG}:abc123-ubi@sha256:deadbeef"
    assert_equal "$(cat package-registry-fips-images-digest.txt)" "${DOCKER_IMG}:abc123-fips@sha256:deadbeef"

    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_line "artifact upload package-registry-base-images-digest.txt"
    assert_line "artifact upload package-registry-ubi-images-digest.txt"
    assert_line "artifact upload package-registry-fips-images-digest.txt"
}

@test "resolve_image_digests.sh writes PR digest files under the non-collectable pr- name" {
    _set_buildkite_env feature 123
    echo "${DOCKER_IMG_PR}:abc123" > image-ref-base.txt

    run bash .buildkite/scripts/resolve_image_digests.sh
    assert_success
    assert_equal "$(cat pr-package-registry-base-images-digest.txt)" "${DOCKER_IMG_PR}:abc123@sha256:deadbeef"
    assert_file_not_exists package-registry-base-images-digest.txt
}

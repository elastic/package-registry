#!/usr/bin/env bats

# bats file_tags=tag:publish-distribution

# These tests run the whole publish-distribution.sh script with stubs of docker, crane and
# buildkite-agent.

setup() {
    load 'setup_command'
    load 'setup_distribution_command'
    _common_setup

    _mock_docker

    export DOCKER_IMAGE="docker.elastic.co/package-registry/distribution"
    export DOCKER_IMAGE_RENAMED="docker.elastic.co/package-registry/package-registry-distribution"
    export TAG_NAME="production"
    export DRY_RUN="false"

    # pushing installs crane and uploads digest files for signing; mock them so the script never
    # reaches the network, and crane resolves to a fixed digest.
    export WORKSPACE="${BATS_TEST_TMPDIR}/workspace"
    mkdir -p "${WORKSPACE}"
    export CRANE_STUB_DIGEST="sha256:deadbeef"
    _install_fake_crane

    export BUILDKITE_AGENT_CALLS_FILE="${BATS_TEST_TMPDIR}/buildkite_agent_calls"
    : > "${BUILDKITE_AGENT_CALLS_FILE}"
    _create_stub buildkite-agent <<'EOF'
echo "$*" >> "${BUILDKITE_AGENT_CALLS_FILE}"
if [[ "$1 $2" == "meta-data get" && "$3" == "DOCKER_TAG" ]]; then
    echo "1.2.3"
elif [[ "$1 $2" == "meta-data get" && "$3" == "SKIP_RETAG" ]]; then
    echo "${SKIP_RETAG_META:-false}"
fi
EOF
}

# publish-distribution.sh writes the digest files in the working directory (the repository root);
# remove them so they do not linger as untracked files.
teardown() {
    rm -f package-registry-distribution-*-images-digest.txt
}

@test "publish-distribution signs the renamed images with a digest file per distribution" {
    run bash .buildkite/scripts/publish-distribution.sh
    assert_success

    # only the renamed repository is signed, one reference per pushed image
    run cat package-registry-distribution-production-images-digest.txt
    assert_line --index 0 "${DOCKER_IMAGE_RENAMED}:1.2.3@sha256:deadbeef"
    assert_line --index 1 "${DOCKER_IMAGE_RENAMED}:1.2.3-ubi@sha256:deadbeef"
    assert_line --index 2 "${DOCKER_IMAGE_RENAMED}:1.2.3-fips@sha256:deadbeef"
    assert_equal "${#lines[@]}" "3"

    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_line "artifact upload package-registry-distribution-production-images-digest.txt"
}

@test "publish-distribution signs the fips image" {
    run bash .buildkite/scripts/publish-distribution.sh
    assert_success

    run cat package-registry-distribution-production-images-digest.txt
    assert_line "${DOCKER_IMAGE_RENAMED}:1.2.3-fips@sha256:deadbeef"
}

@test "publish-distribution names the digest file after the distribution" {
    export TAG_NAME="lite-slim"

    run bash .buildkite/scripts/publish-distribution.sh
    assert_success

    run cat package-registry-distribution-lite-slim-images-digest.txt
    assert_line --index 0 "${DOCKER_IMAGE_RENAMED}:lite-1.2.3-slim@sha256:deadbeef"
}

@test "publish-distribution does not sign the retagged distribution image" {
    run bash .buildkite/scripts/publish-distribution.sh
    assert_success

    # distribution/* is already signed by package-storage-infra, only a retag happens here
    run cat package-registry-distribution-production-images-digest.txt
    refute_output --partial "${DOCKER_IMAGE}:"

    # only the renamed repo is signed, one line per flavour
    assert_line "${DOCKER_IMAGE_RENAMED}:1.2.3@sha256:deadbeef"
    assert_line "${DOCKER_IMAGE_RENAMED}:1.2.3-ubi@sha256:deadbeef"
    assert_line "${DOCKER_IMAGE_RENAMED}:1.2.3-fips@sha256:deadbeef"
    assert_equal "${#lines[@]}" "3"
}

@test "publish-distribution also signs the distribution repo when SIGN_DISTRIBUTION_IMAGES is true" {
    export SIGN_DISTRIBUTION_IMAGES="true"

    run bash .buildkite/scripts/publish-distribution.sh
    assert_success

    # both repos signed: the original distribution repo and the renamed repo, one line per flavour
    run cat package-registry-distribution-production-images-digest.txt
    assert_line "${DOCKER_IMAGE}:1.2.3@sha256:deadbeef"
    assert_line "${DOCKER_IMAGE}:1.2.3-ubi@sha256:deadbeef"
    assert_line "${DOCKER_IMAGE}:1.2.3-fips@sha256:deadbeef"
    assert_line "${DOCKER_IMAGE_RENAMED}:1.2.3@sha256:deadbeef"
    assert_line "${DOCKER_IMAGE_RENAMED}:1.2.3-ubi@sha256:deadbeef"
    assert_line "${DOCKER_IMAGE_RENAMED}:1.2.3-fips@sha256:deadbeef"
    assert_equal "${#lines[@]}" "6"
}

@test "publish-distribution signs without retagging when SKIP_RETAG is true" {
    export SKIP_RETAG_META="true"

    run bash .buildkite/scripts/publish-distribution.sh
    assert_success

    # no retag happened: imagetools create was never called (only setup_buildx's "buildx create")
    run cat "${DOCKER_CALLS_FILE}"
    refute_output --partial "imagetools create"

    # the digests of the already-pushed images are still resolved and uploaded for signing
    run cat package-registry-distribution-production-images-digest.txt
    assert_line "${DOCKER_IMAGE_RENAMED}:1.2.3@sha256:deadbeef"
    assert_line "${DOCKER_IMAGE_RENAMED}:1.2.3-ubi@sha256:deadbeef"
    assert_line "${DOCKER_IMAGE_RENAMED}:1.2.3-fips@sha256:deadbeef"
    assert_equal "${#lines[@]}" "3"

    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_line "artifact upload package-registry-distribution-production-images-digest.txt"
}

@test "publish-distribution does not sign anything on a dry run" {
    export DRY_RUN="true"

    run bash .buildkite/scripts/publish-distribution.sh
    assert_success

    assert_file_not_exists package-registry-distribution-production-images-digest.txt
    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    refute_output --partial "artifact upload"
}

@test "publish-distribution does not sign when there is no renamed image" {
    unset DOCKER_IMAGE_RENAMED

    run bash .buildkite/scripts/publish-distribution.sh
    assert_success

    assert_file_not_exists package-registry-distribution-production-images-digest.txt
}

@test "publish-distribution fails when the digest cannot be resolved" {
    # the fake crane reports "null", which write_image_digest_file treats as unresolvable
    CRANE_STUB_DIGEST="null" _install_fake_crane

    run bash .buildkite/scripts/publish-distribution.sh
    assert_failure
    assert_output --partial "could not resolve digest"
}

# The signing trigger step in .buildkite/release-package-registry-distribution.yml collects the digest
# files with a glob (ARTIFACT_PATTERN). This checks that every artifact the script actually uploads
# matches that glob, so the two sides cannot drift apart silently.
@test "the signing ARTIFACT_PATTERN matches the artifacts that publish-distribution uploads" {
    for tag_name in lite production lite-slim production-slim; do
        TAG_NAME="${tag_name}" run bash .buildkite/scripts/publish-distribution.sh
        assert_success
    done

    local pattern uploaded
    pattern="$(grep -m1 'ARTIFACT_PATTERN:' .buildkite/release-package-registry-distribution.yml | sed -E 's/.*"([^"]*)".*/\1/')"
    refute [ -z "${pattern}" ]

    run grep "^artifact upload" "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_equal "${#lines[@]}" "4"
    for line in "${lines[@]}"; do
        uploaded="${line#artifact upload }"
        case "${uploaded}" in
            ${pattern}) ;;
            *) fail "pipeline ARTIFACT_PATTERN '${pattern}' does not match uploaded artifact '${uploaded}'" ;;
        esac
    done
}

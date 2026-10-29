#!/usr/bin/env bats

# bats file_tags=tag:publish-helpers

setup() {
    load 'setup_command'
    _common_setup

    source "${ROOT_DIR}/.buildkite/scripts/tooling.sh"
    source "${ROOT_DIR}/.buildkite/scripts/publish.helpers.sh"

    # do not wait between retries
    sleep() { :; }

    DIGEST_FILE="${BATS_TEST_TMPDIR}/package-registry-images-digest.txt"
    IMAGE_REF="docker.elastic.co/package-registry/package-registry:main"
}

# Setup only needed by the tests that resolve digests: replace crane with a function.
#   CRANE_STUB_DIGEST: digest printed by "crane digest" (empty prints nothing)
#   CRANE_STUB_FAILURES: number of calls that fail before "crane digest" succeeds
_mock_crane() {
    export CRANE_STUB_DIGEST="${1:-}"
    export CRANE_STUB_FAILURES="${2:-0}"
    export CRANE_CALLS_FILE="${BATS_TEST_TMPDIR}/crane_calls"
    echo 0 > "${CRANE_CALLS_FILE}"

    crane() {
        local calls
        calls="$(cat "${CRANE_CALLS_FILE}")"
        echo $((calls + 1)) > "${CRANE_CALLS_FILE}"
        echo "$*" >> "${CRANE_CALLS_FILE}.args"
        if [[ "${calls}" -lt "${CRANE_STUB_FAILURES}" ]]; then
            return 1
        fi
        echo "${CRANE_STUB_DIGEST}"
    }
}

@test "write_image_digest_file writes the image reference with its digest" {
    _mock_crane "sha256:abc123"

    run write_image_digest_file "${IMAGE_REF}" "${DIGEST_FILE}"
    assert_success
    assert_output --partial "${IMAGE_REF}@sha256:abc123"
    assert_equal "$(cat "${DIGEST_FILE}")" "${IMAGE_REF}@sha256:abc123"
    assert_equal "$(cat "${CRANE_CALLS_FILE}.args")" "digest ${IMAGE_REF}"
}

@test "write_image_digest_file overwrites an existing digest file" {
    _mock_crane "sha256:abc123"
    echo "old content" > "${DIGEST_FILE}"

    run write_image_digest_file "${IMAGE_REF}" "${DIGEST_FILE}"
    assert_success
    assert_equal "$(cat "${DIGEST_FILE}")" "${IMAGE_REF}@sha256:abc123"
}

@test "write_image_digest_file retries when crane fails temporarily" {
    _mock_crane "sha256:abc123" 2

    run write_image_digest_file "${IMAGE_REF}" "${DIGEST_FILE}"
    assert_success
    assert_equal "$(head -1 "${CRANE_CALLS_FILE}")" "3"
    assert_equal "$(cat "${DIGEST_FILE}")" "${IMAGE_REF}@sha256:abc123"
}

@test "write_image_digest_file fails when crane keeps failing" {
    _mock_crane "sha256:abc123" 100

    run write_image_digest_file "${IMAGE_REF}" "${DIGEST_FILE}"
    assert_failure
    assert_output --partial "ERROR: could not resolve digest for ${IMAGE_REF}"
    assert_equal "$(head -1 "${CRANE_CALLS_FILE}")" "5"
    assert_file_not_exists "${DIGEST_FILE}"
}

@test "write_image_digest_file fails when the digest is empty" {
    _mock_crane ""

    run write_image_digest_file "${IMAGE_REF}" "${DIGEST_FILE}"
    assert_failure
    assert_output --partial "ERROR: could not resolve digest for ${IMAGE_REF}"
    assert_file_not_exists "${DIGEST_FILE}"
}

@test "write_image_digest_file fails when the digest is null" {
    _mock_crane "null"

    run write_image_digest_file "${IMAGE_REF}" "${DIGEST_FILE}"
    assert_failure
    assert_output --partial "ERROR: could not resolve digest for ${IMAGE_REF}"
    assert_file_not_exists "${DIGEST_FILE}"
}

@test "write_image_digest_file fails when crane is not installed" {
    # no crane function and a PATH without the crane binary
    PATH="${BATS_TEST_TMPDIR}/empty" run write_image_digest_file "${IMAGE_REF}" "${DIGEST_FILE}"
    assert_failure
    assert_output --partial "ERROR: crane command not found"
    assert_file_not_exists "${DIGEST_FILE}"
}

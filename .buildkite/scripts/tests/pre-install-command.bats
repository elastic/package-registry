#!/usr/bin/env bats

# bats file_tags=tag:pre-install-command

setup() {
    load 'setup_command'
    _common_setup

    # pre-install-command.sh sources tooling.sh using a path relative to the repository root
    cd "${ROOT_DIR}"
    # the sourced script enables strict mode, restore the options changed by it afterwards
    # (errexit must stay enabled: bats relies on it to detect failed assertions)
    local pipefail_option
    pipefail_option="$(shopt -po pipefail || true)"
    source .buildkite/scripts/pre-install-command.sh
    set +u
    eval "${pipefail_option}"

    export WORKSPACE="${BATS_TEST_TMPDIR}/workspace"
    mkdir -p "${WORKSPACE}"
    ORIGINAL_PATH="${PATH}"
}

teardown() {
    PATH="${ORIGINAL_PATH}"
}

# Setup only needed by the with_crane tests: pin the version and replace curl with a function that
# records its arguments and writes, in the path given with -o, a tarball with a fake crane binary.
_mock_crane_download() {
    export CRANE_VERSION="v0.0.1"
    export CURL_ARGS_FILE="${BATS_TEST_TMPDIR}/curl_args"

    curl() {
        echo "$*" > "${CURL_ARGS_FILE}"
        local output=""
        while [[ $# -gt 0 ]]; do
            if [[ "$1" == "-o" ]]; then
                output="$2"
            fi
            shift
        done
        local fake_dir="${BATS_TEST_TMPDIR}/fake_crane"
        mkdir -p "${fake_dir}"
        printf '#!/bin/bash\necho "fake crane ${CRANE_VERSION}"\n' > "${fake_dir}/crane"
        chmod +x "${fake_dir}/crane"
        tar -C "${fake_dir}" -czf "${output}" crane
    }
}

@test "with_crane downloads the pinned version for linux amd64" {
    _mock_crane_download
    platform_type="Linux"
    hw_type="x86_64"

    run with_crane
    assert_success
    assert_output --partial "fake crane v0.0.1"
    run cat "${CURL_ARGS_FILE}"
    assert_output --partial "https://github.com/google/go-containerregistry/releases/download/v0.0.1/go-containerregistry_Linux_x86_64.tar.gz"
}

@test "with_crane downloads the pinned version for linux arm64" {
    _mock_crane_download
    platform_type="Linux"
    hw_type="aarch64"

    run with_crane
    assert_success
    run cat "${CURL_ARGS_FILE}"
    assert_output --partial "go-containerregistry_Linux_arm64.tar.gz"
}

@test "with_crane downloads the pinned version for darwin arm64" {
    _mock_crane_download
    platform_type="Darwin"
    hw_type="arm64"

    run with_crane
    assert_success
    run cat "${CURL_ARGS_FILE}"
    assert_output --partial "go-containerregistry_Darwin_arm64.tar.gz"
}

@test "with_crane installs an executable crane available in PATH" {
    _mock_crane_download
    platform_type="Linux"
    hw_type="x86_64"

    with_crane > /dev/null

    assert_file_executable "${WORKSPACE}/bin/crane"
    assert_equal "$(command -v crane)" "${WORKSPACE}/bin/crane"
}

@test "with_crane does not leave temporary files behind" {
    _mock_crane_download
    platform_type="Linux"
    hw_type="x86_64"

    with_crane > /dev/null

    assert_file_not_exists "${WORKSPACE}/tmp"
}

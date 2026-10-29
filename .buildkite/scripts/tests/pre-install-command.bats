#!/usr/bin/env bats

# bats file_tags=tag:pre-install-command

setup() {
    load 'setup_command'
    _common_setup

    export WORKSPACE="${BATS_TEST_TMPDIR}/workspace"
    mkdir -p "${WORKSPACE}"
}

# Setup only needed by the with_crane tests: pin the version, stub uname to emulate the platform and
# replace curl with a stub that records its arguments and writes, in the path given with -o, a
# tarball with a fake crane binary.
#
# Usage: _mock_crane_download <system> <machine>
_mock_crane_download() {
    export UNAME_STUB_SYSTEM="$1"
    export UNAME_STUB_MACHINE="$2"
    export CRANE_VERSION="v0.0.1"
    export CURL_ARGS_FILE="${BATS_TEST_TMPDIR}/curl_args"

    _create_stub uname <<'EOF'
if [[ "${1:-}" == "-m" ]]; then
    echo "${UNAME_STUB_MACHINE}"
else
    echo "${UNAME_STUB_SYSTEM}"
fi
EOF
    _create_stub curl <<'EOF'
echo "$*" > "${CURL_ARGS_FILE}"
while [[ $# -gt 0 ]]; do
    if [[ "$1" == "-o" ]]; then
        output="$2"
    fi
    shift
done
fake_dir="$(mktemp -d)"
printf '#!/bin/bash\necho "fake crane ${CRANE_VERSION}"\n' > "${fake_dir}/crane"
chmod +x "${fake_dir}/crane"
tar -C "${fake_dir}" -czf "${output}" crane
rm -rf "${fake_dir}"
EOF
}


# Write the command file that sources pre-install-command.sh and runs with_crane, then run it.
_run_with_crane() {
    cat <<EOF > "${COMMAND_FILE}"
source .buildkite/scripts/pre-install-command.sh
with_crane
EOF
    run ${test_command}
}

@test "with_crane downloads the pinned version for linux amd64" {
    _mock_crane_download "Linux" "x86_64"
    _run_with_crane
    assert_success
    assert_output --partial "fake crane v0.0.1"
    run cat "${CURL_ARGS_FILE}"
    assert_output --partial "https://github.com/google/go-containerregistry/releases/download/v0.0.1/go-containerregistry_Linux_x86_64.tar.gz"
}

@test "with_crane downloads the pinned version for linux arm64" {
    _mock_crane_download "Linux" "aarch64"
    _run_with_crane
    assert_success
    run cat "${CURL_ARGS_FILE}"
    assert_output --partial "go-containerregistry_Linux_arm64.tar.gz"
}

@test "with_crane downloads the pinned version for darwin arm64" {
    _mock_crane_download "Darwin" "arm64"
    _run_with_crane
    assert_success
    run cat "${CURL_ARGS_FILE}"
    assert_output --partial "go-containerregistry_Darwin_arm64.tar.gz"
}

@test "with_crane installs an executable crane available in PATH" {
    _mock_crane_download "Linux" "x86_64"
    cat <<EOF > "${COMMAND_FILE}"
source .buildkite/scripts/pre-install-command.sh
with_crane > /dev/null
command -v crane
EOF

    run ${test_command}
    assert_success
    assert_output "${WORKSPACE}/bin/crane"
    assert_file_executable "${WORKSPACE}/bin/crane"
}

@test "with_crane does not leave temporary files behind" {
    _mock_crane_download "Linux" "x86_64"
    _run_with_crane
    assert_success
    assert_file_not_exists "${WORKSPACE}/tmp"
}

#!/usr/bin/env bats

# bats file_tags=tag:pre-install-command

setup() {
    load 'setup_command'
    _common_setup

    export WORKSPACE="${BATS_TEST_TMPDIR}/workspace"
    mkdir -p "${WORKSPACE}"
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
    _install_fake_crane "Linux" "x86_64"
    _run_with_crane
    assert_success
    assert_output --partial "fake crane v0.0.1"
    run cat "${CURL_ARGS_FILE}"
    assert_output --partial "https://github.com/google/go-containerregistry/releases/download/v0.0.1/go-containerregistry_Linux_x86_64.tar.gz"
    # --fail is required so HTTP errors reach the retry loop instead of being saved as the tarball.
    assert_output --partial "--fail"
}

@test "with_crane downloads the pinned version for linux arm64" {
    _install_fake_crane "Linux" "aarch64"
    _run_with_crane
    assert_success
    run cat "${CURL_ARGS_FILE}"
    assert_output --partial "go-containerregistry_Linux_arm64.tar.gz"
}

@test "with_crane downloads the pinned version for darwin arm64" {
    _install_fake_crane "Darwin" "arm64"
    _run_with_crane
    assert_success
    run cat "${CURL_ARGS_FILE}"
    assert_output --partial "go-containerregistry_Darwin_arm64.tar.gz"
}

@test "with_crane installs an executable crane available in PATH" {
    _install_fake_crane "Linux" "x86_64"
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
    _install_fake_crane "Linux" "x86_64"
    _run_with_crane
    assert_success
    assert_file_not_exists "${WORKSPACE}/tmp"
}

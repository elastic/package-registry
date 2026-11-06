#!/usr/bin/env bats

# bats file_tags=tag:pre-install-command

setup() {
    load 'setup_command'
    _common_setup

    export WORKSPACE="${BATS_TEST_TMPDIR}/workspace"
    mkdir -p "${WORKSPACE}"
}

@test "with_jq installs jq into WORKSPACE and makes it available on PATH" {
    _mock_jq_install
    cat <<EOF > "${COMMAND_FILE}"
source .buildkite/scripts/pre-install-command.sh
add_bin_path
with_jq > /dev/null
command -v jq
EOF

    run ${test_command}
    assert_success
    assert_output "${WORKSPACE}/bin/jq"
    assert_file_executable "${WORKSPACE}/bin/jq"
}

@test "with_jq installs a working jq" {
    _mock_jq_install
    cat <<EOF > "${COMMAND_FILE}"
source .buildkite/scripts/pre-install-command.sh
add_bin_path
with_jq > /dev/null
echo '{"k":"v"}' | jq -r '.k'
EOF

    run ${test_command}
    assert_success
    assert_output "v"
}

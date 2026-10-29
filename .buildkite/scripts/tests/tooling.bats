#!/usr/bin/env bats

# bats file_tags=tag:tooling

setup() {
    load 'setup_command'
    _common_setup
}

@test "retry succeeds at the first attempt" {
    cat <<EOF > "${COMMAND_FILE}"
source .buildkite/scripts/tooling.sh
retry 3 true
EOF
    run ${test_command}
    assert_success
    assert_output ""
}

@test "retry succeeds after some failed attempts" {
    local counter_file="${BATS_TEST_TMPDIR}/counter"
    echo 0 > "${counter_file}"
    cat <<EOF > "${COMMAND_FILE}"
source .buildkite/scripts/tooling.sh
flaky() {
    local count
    count="\$(cat "${counter_file}")"
    echo \$((count + 1)) > "${counter_file}"
    [[ "\${count}" -ge 2 ]]
}
retry 5 flaky
EOF
    run ${test_command}
    assert_success
    assert_equal "$(cat "${counter_file}")" "3"
    assert_output --partial "Retry 1/5 exited 1"
    assert_output --partial "Retry 2/5 exited 1"
}

@test "retry returns the exit code of the command when attempts are exhausted" {
    cat <<EOF > "${COMMAND_FILE}"
source .buildkite/scripts/tooling.sh
failing() { return 7; }
retry 3 failing
EOF
    run ${test_command}
    assert_failure 7
    assert_output --partial "no more retries left"
}

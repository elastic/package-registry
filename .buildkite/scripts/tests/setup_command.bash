#!/usr/bin/env bash

_common_setup() {
    load '../../../bats_tests/test_helper/bats-support/load'
    load '../../../bats_tests/test_helper/bats-assert/load'
    load '../../../bats_tests/test_helper/bats-mock/stub'
    load '../../../bats_tests/test_helper/bats-file/load'

    # search for the .git folder to know the workspace root
    # assume that this script is located somewhere under the git repository
    ROOT_DIR="$(git rev-parse --show-toplevel)"
}

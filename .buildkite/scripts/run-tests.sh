#!/bin/bash
set -euo pipefail

# Pre install:
source .buildkite/scripts/pre-install-command.sh
add_bin_path
with_mage
with_go_junit_report

set +e
mage -debug test | tee tests-report-linux.txt
exit_code=$?

# Create Junit report for junit annotation plugin
go-junit-report > tests-report-linux.xml < tests-report-linux.txt
exit $exit_code

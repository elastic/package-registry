#!/bin/bash
set -euo pipefail

# Pre install:
source .buildkite/scripts/pre-install-command.sh
add_bin_path
with_mage
with_go_junit_report

set +e
mage -debug test > tests-report-linux.txt
exit_code=$?
set -e

# Buildkite collapse logs under --- symbols
# need to change --- to anything else or switch off collapsing (note: not available at the moment of this commit)
awk '{gsub("---", "----"); print }' tests-report-linux.txt

# Create Junit report for junit annotation plugin
go-junit-report > tests-report-linux.xml < tests-report-linux.txt
exit $exit_code

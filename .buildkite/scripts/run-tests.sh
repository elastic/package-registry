#!/bin/bash
source .buildkite/scripts/pre-install-command.sh
set -euo pipefail

# Pre install:
add_bin_path
with_mage
with_go_junit_report

platform_type="$(uname | tr '[:upper:]' '[:lower:]')"
tests_report_txt_file="tests-report-${platform_type}.txt"
tests_report_xml_file="tests-report-${platform_type}.xml"
echo "--- Run Unit tests"
set +e
mage -debug test > "${tests_report_txt_file}"
exit_code=$?
set -e

echo "--- Check go version"
go env |grep GOTOOLCHAIN
go version

# Buildkite collapse logs under --- symbols
# need to change --- to anything else or switch off collapsing (note: not available at the moment of this commit)
awk '{gsub("---", "----"); print }' "${tests_report_txt_file}"

echo "Create Junit report for junit annotation plugin ${tests_report_xml_file}"
go-junit-report > "${tests_report_xml_file}" < "${tests_report_txt_file}"
exit $exit_code

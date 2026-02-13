#!/bin/bash
source .buildkite/scripts/pre-install-command.sh
set -euo pipefail

# Pre install:
add_bin_path
with_mage
with_go_junit_report

platform_type="$(uname | tr '[:upper:]' '[:lower:]')"
tests_report_txt_file="tests-report-fips-${platform_type}.txt"
tests_report_xml_file="tests-report-fips-${platform_type}.xml"
echo "--- Run FIPS Unit tests"
set +e
mage -debug testFIPS > "${tests_report_txt_file}"
exit_code=$?
set -e

# Buildkite collapse logs under --- symbols
# need to change --- to anything else or switch off collapsing (note: not available at the moment of this commit)
awk '{gsub("---", "----"); print }' "${tests_report_txt_file}"

echo "Create Junit report for junit annotation plugin ${tests_report_xml_file}"
go-junit-report > "${tests_report_xml_file}" < "${tests_report_txt_file}"
exit $exit_code

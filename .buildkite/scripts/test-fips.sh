#!/bin/bash
# Pre install:
source .buildkite/scripts/pre-install-command.sh

set -euo pipefail

add_bin_path
with_mage

echo "--- Build FIPS 140-3 binary"
mage -debug buildFIPS

echo "--- Verify FIPS 140-3 build settings"
build_info="$(go version -m package-registry)"
echo "${build_info}"

fips_version="$(echo "${build_info}" | grep -oE 'GOFIPS140=\S+' | cut -d= -f2-)"
default_godebug="$(echo "${build_info}" | grep -oE 'DefaultGODEBUG=\S+' | cut -d= -f2-)"

if [[ "${fips_version}" != v1.0.0* ]]; then
	echo "Expected GOFIPS140 to reference the certified v1.0.0 module, got: '${fips_version}'"
	exit 1
fi

if [[ "${default_godebug}" != *"fips140=on"* ]]; then
	echo "Expected DefaultGODEBUG to contain fips140=on, got: '${default_godebug}'"
	exit 1
fi

echo "FIPS 140-3 binary verified: GOFIPS140=${fips_version} DefaultGODEBUG=${default_godebug}"

echo "--- Run unit tests against the FIPS 140-3 crypto module"
mage -debug testFIPS

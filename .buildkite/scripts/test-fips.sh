#!/bin/bash
# Pre install:
source .buildkite/scripts/pre-install-command.sh

set -euo pipefail

add_bin_path
with_jq
with_mage

echo "--- Build FIPS 140-3 binary"
mage -debug buildFIPS

echo "--- Verify FIPS 140-3 build settings"
# GOFIPS140 includes a commit hash suffix (e.g. v1.0.0-c2097c7c), not just v1.0.0.
fips_version=$(go version -m -json package-registry | jq -r '.Settings[] | select(.Key == "GOFIPS140") | .Value')
default_godebug=$(go version -m -json package-registry | jq -r '.Settings[] | select(.Key == "DefaultGODEBUG") | .Value')

go version -m package-registry

if [[ ! "${fips_version}" =~ ^v1\.0\.0(-[a-f0-9]{5,40})?$ ]]; then
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

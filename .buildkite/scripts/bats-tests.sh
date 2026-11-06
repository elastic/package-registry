#!/bin/bash
source .buildkite/scripts/pre-install-command.sh

set -euo pipefail

# Pre install:
# The signing helpers parse the buildx metadata with jq and the bats tests exercise them, so jq must
# be on PATH for the test run. The golang agent image has no jq, so install the pinned one (the same
# with_jq that publish.sh uses) before running bats.
add_bin_path
with_jq

make bats-tester-ci

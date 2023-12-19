#!/bin/bash
# Pre install:
source .buildkite/scripts/pre-install-command.sh

set -euo pipefail

add_bin_path
with_mage

mage -debug build

#!/bin/bash
source .buildkite/scripts/pre-install-command.sh

set -euo pipefail

# Pre install:
add_bin_path
with_mage

mage -debug check

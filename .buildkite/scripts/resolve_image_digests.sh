#!/bin/bash
source .buildkite/scripts/tooling.sh
source .buildkite/scripts/pre-install-command.sh
source .buildkite/scripts/publish.helpers.sh

set -euo pipefail

# Resolve the signing digests for the package-registry images pushed by the publish step, in a
# dedicated step so a transient crane/registry failure does not fail an already-successful push (which
# would leave the pushed image unsigned). The publish step uploaded a build-unique image-ref-<flavour>.txt
# artifact per flavour; here we resolve each with crane and write + upload the
# package-registry-<flavour>-images-digest.txt file the signing pipeline collects (prefixed with "pr-"
# on PR builds so it is not collected).
#
# This step is safe to retrigger on its own: the refs are build-scoped artifacts that survive a retry,
# the images are already pushed under their unique tags, and cosign signs by digest (idempotent) -- so
# a retry resolves the same images with no rebuild or re-push.

with_crane

echo "--- Download image refs uploaded by the publish step"
# Rely on Buildkite's latest-attempt default. Do NOT pass --include-retried-jobs: it would surface
# every attempt's ref and make an exact-path query error on "Multiple artifacts were found".
retry 5 buildkite-agent artifact download "image-ref-*.txt" .

echo "--- Resolve image digests for signing"
resolve_image_digests

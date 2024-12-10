#!/bin/bash
# The version-controlled catalog template in catalog-template.json is
# currently auto-generated from the production catalog (in turn built by
# CPaaS/IIB). This makes it hard to modify (because a refresh would wipe manual
# modifications).
#
# This script:
# 1. creates a temporary catalog template from the version controlled template
# by injecting a bundle for a Konflux build, together with a rhacs-4.6 channel
# 2. renders this template into a version-controlled catalog.
#
# TODO: Once we stop building operator indexes using CPaaS/IIB, the entries for
# the Konflux build can be moved into the version-controlled template and the
# script can be removed in favor of the render-template command.

set -euo pipefail

OPM="$1"
shift

# A recent (more or less) successful konflux build.
# To find this value:
# - look in https://github.com/stackrox/stackrox/commits/master for a commit with a successful "Red Hat Konflux / operator-bundle-on-push" status,
# - go to "Details" next to it,
# - go to "build-container" under "Task Statuses",
# - go to the "Details" tab in the main pane,
# - scroll down to "Results",
# - take the tag from the "IMAGE_URL row"
# - take the whole value from the "IMAGE_DIGEST" row,
# - save and run make.
version="v4.7.0-163-g8f9cf23be2-fast"
digest="sha256:e05042a079ef49d3198297a5203420d165f44f23f7baf9c5f1bfb6c345771418"

# This
latest_legacy_version="$(jq -r '.entries[]|select(.schema=="olm.channel" and .name == "stable") | .entries|.[-1] | .name' < catalog-template.json)"

tmp_template="$(mktemp)"
trap 'rm -f $tmp_template' EXIT
jq --slurpfile channel channel-4.6.json '.entries += $channel
 | .entries += [{"schema": "olm.bundle", "image": "quay.io/rhacs-eng/stackrox-operator-bundle@'${digest}'"}]
 | .entries |= map(
   if .schema == "olm.channel" and .name == "stable"
   then .entries += [{"name": "rhacs-operator.'${version}'", "replaces": "'"${latest_legacy_version}"'", "skipRange": ">= 4.5.0 < 4.6.0"}]
   else .
   end)
 | .entries |= map(
   if .schema == "olm.channel" and .name == "rhacs-4.6"
   then .entries += [{"name": "rhacs-operator.'${version}'", "replaces": "rhacs-operator.v4.5.0", "skipRange": ">= 4.5.0 < 4.6.0"}]
   else .
   end)
' catalog-template.json > "$tmp_template"
echo >&2 "Running template rendering, this can take a few minutes..."

# Render catalog and post-process result by rewriting the image repository:
#
#     quay.io/rhacs-eng/stackrox-operator-bundle
#  -> registry.redhat.io/advanced-cluster-security/rhacs-operator-bundle
"${OPM}" alpha render-template basic "$@" "${tmp_template}" \
  | jq 'walk(
      if type == "string" and startswith("quay.io/rhacs-eng/stackrox-operator-bundle@")
      then
          "registry.redhat.io/advanced-cluster-security/rhacs-operator-bundle@" + (. | ltrimstr("quay.io/rhacs-eng/stackrox-operator-bundle@"))
      else
          .
      end
    )'

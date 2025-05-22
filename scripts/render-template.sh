#!/bin/bash
# The version-controlled catalog template in catalog-template.json is
# currently auto-generated from the production catalog (in turn built by
# CPaaS/IIB). This makes it hard to modify (because a refresh would wipe manual
# modifications).
#
# This script:
# 1. creates a temporary catalog template from the version controlled template
# by injecting a bundle for a Konflux build, together with a rhacs-4.8 channel
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
version="v4.8.0-749-gb5ee3108f8-fast"
digest="sha256:d325aacb04787efccd5ca1188d72bfc8db27646eaa6b9b700741616c6c301d6c"

# This
latest_legacy_version="$(jq -r '.entries[]|select(.schema=="olm.channel" and .name == "stable") | .entries|.[-1] | .name' < catalog-template.json)"

tmp_template="$(mktemp)"
trap 'rm -f $tmp_template' EXIT
jq --slurpfile channel channel-4.8.json '.entries += $channel
 | .entries += [{"schema": "olm.bundle", "image": "quay.io/rhacs-eng/stackrox-operator-bundle@'${digest}'"}]
 | .entries |= map(
   if .schema == "olm.channel" and .name == "stable"
   then .entries += [{"name": "rhacs-operator.'${version}'", "replaces": "'"${latest_legacy_version}"'", "skipRange": ">= 4.7.0 < 4.8.0"}]
   else .
   end)
 | .entries |= map(
   if .schema == "olm.channel" and .name == "rhacs-4.8"
   then .entries += [{"name": "rhacs-operator.'${version}'", "replaces": "rhacs-operator.v4.7.0", "skipRange": ">= 4.7.0 < 4.8.0"}]
   else .
   end)
' catalog-template.json > "$tmp_template"
echo >&2 "Running template rendering, this can take a few minutes..."

# Render catalog and post-process result by rewriting the image repository from either of
#
#     quay.io/rhacs-eng/stackrox-operator-bundle
#     registry-proxy.engineering.redhat.com/rh-osbs/rhacs-operator-bundle
#     brew.registry.redhat.io/rh-osbs/rhacs-operator-bundle
#
#  to registry.redhat.io/advanced-cluster-security/rhacs-operator-bundle
"${OPM}" alpha render-template basic "$@" "${tmp_template}" \
  | jq 'walk(
      if type == "string" and (sub("@.*"; "") | in(
          {"quay.io/rhacs-eng/stackrox-operator-bundle": true,
           "registry-proxy.engineering.redhat.com/rh-osbs/rhacs-operator-bundle": true,
           "brew.registry.redhat.io/rh-osbs/rhacs-operator-bundle": true}))
      then
          "registry.redhat.io/advanced-cluster-security/rhacs-operator-bundle@" + (. | sub("^[^@]*@"; ""))
      else
          .
      end
    )'

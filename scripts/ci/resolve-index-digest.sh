#!/usr/bin/env bash
#
# Resolve the digest of a pushed multi-arch manifest list (OCI image index).
#
# Usage: resolve-index-digest.sh <image-ref>
#   e.g. resolve-index-digest.sh ghcr.io/wave-engineering/swarm-cd:1.2.3
#
# Prints the digest to stdout, and appends `digest=<sha256:...>` to $GITHUB_OUTPUT
# when running under GitHub Actions.
#
# WHY THIS EXISTS: `docker buildx imagetools create` assembles and pushes an index but
# does not report the resulting digest, so it has to be read back from the registry.
# Callers need the digest rather than the tag because cosign signatures are bound to an
# immutable digest -- signing a tag would sign whatever that tag later points at.
#
# WHY `{{json .Manifest}}` AND NOT `{{.Manifest.Digest}}`: on buildx 0.21.3 the latter
# silently falls back to printing imagetools' entire default inspect dump instead of the
# field value. A *bogus* field name errors, but that valid-looking path does not -- so the
# failure mode is a plausible-looking command that emits multi-line garbage. The sha256
# regex below is what converts any such regression into a loud failure.
set -euo pipefail

ref="${1:?usage: resolve-index-digest.sh <image-ref>}"

manifest="$(docker buildx imagetools inspect "${ref}" --format '{{json .Manifest}}')"

# Assert it really IS an index. Without this the "sign the manifest list, never a per-arch
# child" guarantee holds only incidentally -- the sha256 regex below accepts any well-formed
# digest, so a single-arch image manifest would sail through and get signed, and `cosign
# verify <tag>` against a genuinely multi-arch tag would then find nothing.
media_type="$(jq -er '.mediaType' <<<"${manifest}")"
if [[ "${media_type}" != "application/vnd.oci.image.index.v1+json" \
   && "${media_type}" != "application/vnd.docker.distribution.manifest.list.v2+json" ]]; then
  echo "resolve-index-digest: ${ref} is not a manifest list/index (mediaType: ${media_type})" >&2
  exit 1
fi

digest="$(jq -er '.digest' <<<"${manifest}")"

if [[ ! "${digest}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
  echo "resolve-index-digest: expected a sha256 digest for ${ref}, got: ${digest}" >&2
  exit 1
fi

echo "${digest}"

# `if`, not `[[ ... ]] && echo`: as the script's last command the latter's false branch
# becomes the script's exit status, so a local run (no GITHUB_OUTPUT) would exit 1 on success.
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  echo "digest=${digest}" >> "${GITHUB_OUTPUT}"
fi

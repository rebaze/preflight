#!/usr/bin/env bash
set -euo pipefail

version=0.70.1
platform="$(uname -s)_$(uname -m)"
case "$platform" in
  Linux_x86_64) sha=613d124b8f6c1f3cee890491f7ab19114cca5a2102ca47cb2e6c35b4c23f9c8a ;;
  Linux_aarch64|Linux_arm64) platform=Linux_arm64; sha=8eb914755cb1b3c610d557019d5f373d5528b366ba706961d1a23f2ec55eab57 ;;
  Darwin_arm64) sha=b8eae5ce6c7c3a768a9b9c6c12b0c01f8fc065a99267c94646d309c479b218dd ;;
  Darwin_x86_64) sha=ccfe6dfb526a8d200d91d1a9f234f07d814f56f93a1ae8f47a5cb30f567dc97f ;;
  *) echo "Unsupported Conftest platform: $platform" >&2; exit 1 ;;
esac

directory=$(mktemp -d "${RUNNER_TEMP:?}/preflight-conftest.XXXXXX")
archive="conftest_${version}_${platform}.tar.gz"
curl --fail --silent --show-error --location --retry 3 \
  "https://github.com/open-policy-agent/conftest/releases/download/v${version}/${archive}" \
  --output "$directory/$archive"
printf '%s  %s\n' "$sha" "$directory/$archive" | shasum -a 256 --check
tar -xzf "$directory/$archive" -C "$directory" conftest
chmod +x "$directory/conftest"
"$directory/conftest" --version
shasum -a 256 "$directory/conftest"
printf '%s\n' "$directory" >> "${GITHUB_PATH:?}"
printf 'PREFLIGHT_CONFTEST=%s/conftest\n' "$directory" >> "${GITHUB_ENV:?}"

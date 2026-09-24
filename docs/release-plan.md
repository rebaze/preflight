# Release automation implementation plan

Goal: prepare Preflight's GitHub CI, releases and Homebrew distribution using Rio's existing approach.

Design: [release-design.md](release-design.md). Execute inline; owner requested autonomous preparation. Keep the work reviewable on `ci/release-automation`.

## Tasks

- [x] Add a test for side-effect-free version output, observe failure, implement build metadata; migrate imports to the selected GitHub module path. Verify with the Go suite.
- [x] Adapt Rio's release-publication fixtures/tests first, then its publication guard. Add tested strict stable-version/checksum handling for formula generation. Preserve staged-byte, attestation, draft and upload checks.
- [x] Add checksum-pinned Conftest setup, reusable CI, four-platform GoReleaser archives, release and optional tap jobs, App verification, CodeQL, Scorecard and Dependabot. Validate with actionlint, shellcheck, GoReleaser and a snapshot release.
- [x] Update README badges/install steps, release operations, architecture, development and roadmap; append actual verification evidence. Record credentials/settings that cannot be supplied locally.
- [x] Run standard checks, packaging smoke tests, offline publication and formula tests; independently review the complete diff.
- [x] Commit and push the branch, open [PR #1](https://github.com/rebaze/preflight/pull/1) and inspect GitHub CI. The signed commit succeeded after the owner approved the 1Password prompt.
- [ ] Confirm hosted CI after fixing the import-order failure caught by its formatting check.

## Review focus

Verify supporting data survives packaging and Homebrew installation; tags cannot publish from unrelated history; prereleases cannot update the stable tap; failed verification cannot publish; missing credentials are visible; release reruns cannot overwrite existing assets. Keep actual-container/pilot verification distinct from packaging verification.

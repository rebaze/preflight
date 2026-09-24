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
- [x] Confirm hosted CI after fixing the import-order failure: [run 36002876946](https://github.com/rebaze/preflight/actions/runs/36002876946) passed on `759227f`.

## Review focus

Verify supporting data survives packaging and Homebrew installation; tags cannot publish from unrelated history; prereleases cannot update the stable tap; failed verification cannot publish; missing credentials are visible; release reruns cannot overwrite existing assets. Keep actual-container/pilot verification distinct from packaging verification.

## PR #1 review corrections

The owner requested fixes and replies to the assessed findings on 2026-09-24.

- [x] Disable CodeQL checkout credential persistence.
- [x] Bind inventory and provenance to the triggering SHA; peel annotated tags and re-check remote identity around publication.
- [x] Compare draft asset digests before publication; verify immutable state and locked bytes afterward. Report post-request failures as possible public incidents and block downstream Homebrew.
- [x] Download and independently verify all four immutable release archives before minting tap credentials.
- [x] Enable/read back immutable releases for Preflight; document the single-writer assumption and absence of an atomic draft-asset publication API.
- [x] Preserve the clean-checkout build and correct Rio attribution; those review-summary concerns did not reproduce.

Regression coverage includes changes after the verification download and during publication, moved lightweight tags, nested annotated tags, wrong-source attestations, mutable releases and missing/replaced Homebrew archives. Local verification is recorded in `verification.md`; hosted results and responses are retained on PR #1.

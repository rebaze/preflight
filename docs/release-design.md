# Release automation design

Selected 2026-09-24 at the owner's request to prepare GitHub delivery using Rio as the reference.

Preflight's own distribution pipeline is separate from the local-feedback authority of the CLI. It must not change the pilot runner, policy, runtime pins or private-state boundaries.

## Delivery contract

- License: Apache-2.0, explicitly selected by the owner; include LICENSE and NOTICE in release archives.
- GitHub repository and Go module: `github.com/rebaze/preflight`.
- CI runs Go race tests, vet, formatting, real Conftest verification and release-packaging checks. Tests explicitly select Conftest 0.70.1 from checksum-pinned platform archives. Release tags run the same verification again before obtaining publication permissions.
- Release archives cover macOS/Linux, amd64/arm64, with a static CLI and the version-matched policy, profile, schemas, runtime and documentation. Windows distribution waits for Windows execution tests.
- `version` / `--version` expose version, commit and build date; they do not inspect or initialize project state.
- Use Rio's staged-publication boundary with explicit source-commit binding: inventory v2 records the triggering SHA; attestations must match it; lightweight/annotated remote tags are checked before draft creation, before publication and after locking. Existing releases or drafts are never overwritten.
- Require immutable releases and a single-writer operating policy. Compare downloaded draft bytes and remote asset digests before publication, then verify the immutable result and download its locked assets before declaring success. GitHub has no documented atomic draft-asset compare-and-swap: another privileged writer during publication can cause a public incident, which must fail the run and withhold Homebrew. Concurrency serializes this workflow, not arbitrary repository writers.
- Stable immutable releases can update a Homebrew formula in `rebaze/homebrew-tap` with a narrowly scoped GitHub App token. Before minting it, download all assets, verify the four archives against checksums, and verify checksum/archive provenance against the source SHA. Formula installation includes supporting data under `share/preflight`; Git and Conftest are dependencies, Docker remains an explicit runtime prerequisite. Prereleases never update the tap. No signing-key or token material is committed.
- Tap publication is explicitly enabled by a repository variable after App verification. Missing tap credentials must not be concealed as a successful tap update.
- CodeQL, Scorecard and Dependabot follow Rio's pinned-action conventions. No automatic PR merges or branch-protection changes.

## Completion and remaining decisions

Validate locally, build all four archive variants, inspect their contents and run a native packaged CLI. Push a reviewable branch and open a PR; observe its real GitHub checks. Do not create a version tag or first public release as part of preparation. Document outstanding App credentials, release version and repository-settings decisions with exact names and commands.

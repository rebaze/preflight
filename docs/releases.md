# GitHub releases and Homebrew

Preflight is hosted at [rebaze/preflight](https://github.com/rebaze/preflight). This pipeline distributes the tool itself; its local reports remain local feedback, never release authorization.

## What is prepared

| Workflow | Trigger and purpose |
| --- | --- |
| CI | PRs, main, manual runs, weekly scans, and reuse by release tags. Go race tests/vet, Conftest policy tests, formatting, module checks, govulncheck, offline publication tests, all four snapshot archives and a native package smoke test. |
| Release | `v*` tags. Repeat CI, bind the triggering SHA and remote tag, build/sign/attest archives, verify draft bytes and asset metadata, publish, then verify immutable state and locked bytes before reporting success. |
| Publish Homebrew | Stable immutable release with `PUBLISH_HOMEBREW=true`, or manual retry. Download/check all four archives and verify their provenance and source SHA before minting the tap token, generating the formula and pushing it. |
| Verify Release App | Manual: mint a token explicitly requesting Contents:write, require its installation repository scope to be exactly the tap, and read tap contents. Does not test branch-rule bypass by writing a branch. |
| Synthetic Docker and Vitest | Manual, opt-in networked dependency preparation followed by the existing offline synthetic container tests and real Vitest CLI demo. No customer checkout. |
| CodeQL / Scorecard | Scheduled and main runs; CodeQL also analyzes PRs. |

GoReleaser 2.18.2 builds macOS/Linux amd64/arm64 archives. Each contains `preflight`, README/documentation and version-matched `share/preflight/{policy,profiles,schemas,runtime}`. No Windows support is claimed. The binary has no third-party Go dependencies; Git, Conftest and Docker remain external prerequisites. `preflight version` reports version, commit and build time.

CI explicitly downloads Conftest 0.70.1 and verifies a platform-specific archive SHA256 committed in `.github/actions/setup-conftest/install.sh`. It prints the extracted binary's actual digest. This does not change existing initialized state or turn the historical macOS executable hash into a cross-platform pin.

## Repository setup remaining

Inspected 2026-09-24: Actions is enabled, default workflow token permission is read-only, there are no releases, no rulesets and no main branch protection. The existing tap has Rio and scat casks, with no Preflight package.

1. **Release App secret:** `RELEASE_APP_ID=3542938` has been set on Preflight to match Rio. Add `RELEASE_APP_PRIVATE_KEY` under [Actions secrets](https://github.com/rebaze/preflight/settings/secrets/actions), or expose the existing organization secret to Preflight. GitHub cannot export Rio's stored secret. Keep the key out of files committed here and out of logs.
2. **App installation:** ensure the existing release App is installed on `rebaze/homebrew-tap` with repository Contents read/write. The token request is limited to that repository. Run **Verify Release App** after the workflow reaches main. If tap rules require PRs, decide how the release App is permitted to update `Formula/preflight.rb`; this workflow does a normal push and cannot bypass rules by itself.
3. **Enable tap updates:** after verification, set `PUBLISH_HOMEBREW=true` in [Actions variables](https://github.com/rebaze/preflight/settings/variables/actions). Until then, releases explicitly report that Homebrew publication is disabled. No initial formula or placeholder checksum needs to be added manually: the first enabled stable release creates `Formula/preflight.rb`.
4. **License selected:** Apache-2.0, explicitly chosen by the owner to match Rio. The repository and release archives include `LICENSE` and `NOTICE`; the Homebrew formula declares Apache-2.0.
5. **First version:** choose a tag only after the automation is merged and its checks pass. Start with a prerelease such as `v0.1.0-rc.1` to exercise real signing/publication without updating Homebrew. A stable `v0.1.0` subsequently creates the formula when enabled. No tag is created by the preparation task.

Immutable releases were enabled and read back on `rebaze/preflight` on 2026-09-24 while addressing PR #1. Keep that setting enabled: the release and Homebrew helpers require an immutable published result. The administration-only settings endpoint is checked during repository setup; the release job does not receive an administration credential and verifies the actual published object's `immutable` field instead. Disabling the setting can therefore cause an incident after publication, not an early setup refusal.

Recommended remaining settings: require the CI test, packaging and vulnerability checks plus review on main, and restrict who can create/move release tags. This work does not change branch/tag protection or organization rules. CodeQL has passed on the PR; real OIDC signing and release publication still await the first tag.

## Concurrent-writer boundary

Use this workflow as the sole automated writer of Preflight release assets. Do not edit its drafts manually while it runs. Any additional release automation must share concurrency group `preflight-release-publication`. GitHub concurrency coordinates cooperating workflows; it does not block another workflow, a collaborator or an administrator from changing a draft.

The API offers no documented atomic operation that publishes only if a draft's asset hashes still match. The helper compares the current remote asset names/states/digests immediately before publication and verifies the immutable result and downloaded locked bytes afterward. A mutation observed before publication leaves a draft. A mutation during publication, unexpected mutable result, or uncertain publication response produces `INCIDENT`, fails the workflow, and prevents automatic Homebrew publication. The release may already be public. Immutability prevents subsequent modification; it does not prevent the earlier race or authorize a claim that bad bytes were never exposed.

For an incident, inspect the release/tag and investigate the other writer. Do not blindly retry, rewrite assets, move tags or downgrade the checks. Corrected assets need a new version. Manual Homebrew retries independently reject mutable releases, altered archives and wrong-commit attestations.

The `GITHUB_TOKEN` handles this repository's releases and attestations, with write permissions restricted to the release job. It does not need a personal access token. The release App is used only for the tap.

## Release procedure

1. Merge verified changes to main. Run **Synthetic Docker and Vitest** when changing runtime/execution behavior; the normal suite is not container evidence.
2. Select a version and tag the intended main commit. For example, after choosing the version deliberately:

   ```sh
   git switch main
   git pull --ff-only
   git tag -a v0.1.0-rc.1 -m 'Preflight v0.1.0-rc.1'
   git push origin v0.1.0-rc.1
   ```

3. Inspect the Release workflow and downloadable assets. Archives have checksums, a cosign bundle, provenance/SBOM attestations and a source CycloneDX document. A source SBOM is not proof of all runtime/container dependency contents.
4. For an enabled stable release, inspect Publish Homebrew and the resulting tap commit. Install with `brew install rebaze/tap/preflight`; inspect `preflight version` and the supporting files in `$(brew --prefix)/share/preflight`.

The release guard refuses an existing release **or draft** for a tag. If a failed run left a draft, inspect it and resolve it explicitly before retrying; the automation never deletes or replaces it. Do not move already-published tags. Prefer a new version for corrected assets.

If only tap publication failed or was disabled, fix its setup and run **Publish Homebrew** with the existing stable tag. It requires an immutable release, downloads all four archives and verifies checksums and provenance against the exact release workflow, tag and checked-out source SHA before the App token is minted. It refuses a downgrade and never rebuilds a release. A successful App permission inspection does not prove branch protection permits its eventual push.

## Local verification

```sh
make check
make packaging
make security
```

`make packaging` requires actionlint, shellcheck and GoReleaser 2.18.2. It builds snapshot archives without signing, publishing or touching Homebrew, checks their contents and executes the native packaged CLI. `make security` downloads/verifies the separate scanner module and queries the current vulnerability database. Offline publication tests exercise failure ordering using synthetic services; real OIDC signing and GitHub publication are verified only by a tag-triggered run.

For consumer verification, see [GitHub's artifact-attestation verification documentation](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations/verifying-the-provenance-of-binaries). A release archive can be checked with `gh attestation verify ARCHIVE --repo rebaze/preflight`, additionally constraining `--source-digest EXPECTED_COMMIT_SHA`, `--source-ref refs/tags/VERSION` and `--cert-identity https://github.com/rebaze/preflight/.github/workflows/release.yaml@refs/tags/VERSION` for the selected release. See also [GitHub's immutable-release semantics](https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases).

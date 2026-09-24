# Candidate build records

## Current rc.4 tester candidate

Plugin **0.1.0-rc.4** carries the pre-merge Copilot corrections and the standalone [tester onboarding page](try-preflight.html). It retains CLI skill protocol 1 and discovery v1. Build the four local archives from a clean reviewed checkout with `make packaging`; the CLI snapshot version includes that checkout's commit. Read `preflight capabilities --format json` and the generated `dist/checksums.txt` to identify the exact build you are testing. The onboarding page accepts the supplied native archive path and prepares installation commands.

The review corrections cover incomplete worktree identity, complete citations, legacy producer identity, merge-candidate uncertainty, duplicate CI event mappings and strict compatibility metadata. The page's copy/path/timer controls were verified in a real browser. Earlier agent timing measurements below remain tied to rc.2/rc.3; no human rc.4 usability result is claimed. Local snapshots remain unsigned. Tagging, signed publication and marketplace submission remain separate owner decisions.

## Historical rc.3 candidate

Built from clean source `ed870aadbcfb796c18039f267d2eeedc64f3872b`: plugin **0.1.0-rc.3**, CLI **0.0.0-SNAPSHOT-ed870aa**, skill protocol 1 and discovery v1. It adds the registered-path bootstrap guard after the rc.2 serial lookup miss. `make packaging` passed in 2.773 seconds using the already-installed toolchain/build cache; all four archives and native smoke checks passed. Fresh-harness results are recorded separately in [stage 5 evaluation](../evaluation/stage-5-results.md).

| Archive | SHA256 |
| --- | --- |
| `preflight_0.0.0-SNAPSHOT-ed870aa_darwin_amd64.tar.gz` | `765e2d7d5b4817a860faa9a04be541440b734aada90e27b2a117960a9c115831` |
| `preflight_0.0.0-SNAPSHOT-ed870aa_darwin_arm64.tar.gz` | `047c42e4f9628d347ec532bf53355341f941ec1ed2a9048677edf0207a043036` |
| `preflight_0.0.0-SNAPSHOT-ed870aa_linux_amd64.tar.gz` | `e85916cc26a8f80f9c9a90ef59db5766bded19b526b50f75fe13ad4d7632c299` |
| `preflight_0.0.0-SNAPSHOT-ed870aa_linux_arm64.tar.gz` | `3178643a38dcb8d5c40fa5318e0705920bf6f074a5614943dca7a77c98c6ac63` |

Use [the archive installation commands](plugin.md#install-the-reviewed-candidate). The binary and public plugin are both included; no source checkout or original conversation is needed at runtime. This is a local unsigned snapshot. Actual signatures, attestations and immutable publication remain the existing owner-triggered release workflow.

To rebuild this exact source candidate with the documented tools already installed:

```sh
git clone https://github.com/rebaze/preflight.git preflight-candidate
cd preflight-candidate
git checkout --detach ed870aadbcfb796c18039f267d2eeedc64f3872b
make packaging
```

Build-time metadata can change rebuilt archive bytes. Verify new local checksums; the table above identifies the actual measured files, not byte-reproducibility across future builds. No release tag was created.

## Historical rc.2 candidate

This rc.2 record is preserved after its unassisted serial-investigation lookup missed the work budget. The registered-path lookup fix advances the plugin to rc.3; the current build record is above and actual evaluations remain version-specific.

The unpublished rc.2 candidate was built from clean source commit `9662a51226b5e0023720f032b9623b69e1e9bc99` on 2026-09-24 using Go1.27.1 and GoReleaser2.18.2. Plugin version: **0.1.0-rc.2**. CLI version: **0.0.0-SNAPSHOT-9662a51**; skill protocol1, discovery `preflight.discovery/v1`. No version tag, release or marketplace submission was made.

All four archives passed the exact inventory/content checker. The macOS ARM binary passed native version/capabilities/empty-discovery checks and was extracted and installed for the actual fresh-harness evaluations. Other platform binaries were cross-built and inspected, not executed on this host. Local snapshots are unsigned; these checksums identify the measured local artifacts and are not release attestations.

| Archive | SHA256 |
| --- | --- |
| `preflight_0.0.0-SNAPSHOT-9662a51_darwin_amd64.tar.gz` | `a7e9315233c324693817d0a5139fcc1b59e45e8198be1c05385a96db096d3046` |
| `preflight_0.0.0-SNAPSHOT-9662a51_darwin_arm64.tar.gz` | `a68aeb43d8743d8aa14f73248e49901803cc6f452564601646349475e59f6f29` |
| `preflight_0.0.0-SNAPSHOT-9662a51_linux_amd64.tar.gz` | `727832f4f3b22f5536f894b647d0567f36a054920c5b0ed147b2b120db7c3318` |
| `preflight_0.0.0-SNAPSHOT-9662a51_linux_arm64.tar.gz` | `80c4aa90f665f633319673e846c7f27e2b984e95f7c38ac644f05b27c068a647` |

The archive and plugin require no Go toolchain at runtime. Use the delivered matching archive and [the exact installation commands](plugin.md#install-the-reviewed-candidate). Installation needs no original checkout or conversation. The CLI and plugin are both in that archive; no project dependencies or fixed execution profile are prerequisites for inspection.

To reproduce the package from published source history, with the explicitly documented [build/package tools](releases.md#local-verification) already installed:

```sh
git clone https://github.com/rebaze/preflight.git preflight-candidate
cd preflight-candidate
git checkout --detach 9662a51226b5e0023720f032b9623b69e1e9bc99
make packaging
```

This rebuilds the same source candidate; build-time metadata can change archive bytes, so verify the new local checksums rather than expecting the measured archive digests above. Do not silently replace a published signed release with a local build. Actual release signature/provenance/SBOM verification remains the guarded tag workflow described in [release operations](releases.md).

The first archive attempt from `da765d7` failed plugin-path checks and was not installed. Its failure and correction remain in [verification](verification.md). Actual timing and usability evidence belongs in [the evaluation record](../evaluation/README.md); agent runs are distinct from human observations. Owner actions remain review/merge, a deliberate release tag, guarded publication and explicit marketplace submission.

# Candidate build records

## Historical rc.2 candidate

This rc.2 record is preserved after its unassisted serial-investigation lookup missed the work budget. The registered-path lookup fix advances the plugin to rc.3; its actual build and evaluation are recorded separately once executed.

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

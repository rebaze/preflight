# Release and CI tools

These maintainer tools are separate from the Go CLI and run only when explicitly invoked or in GitHub Actions. They do not grant a Preflight report release authority.

- `release-publish.py`: stage an exact inventory bound to a source SHA, verify provenance/SBOM attestations and the checksum signature, create and verify a draft, publish, then verify immutable state and locked bytes. Requires Python 3.9+, GitHub CLI and cosign. Never overwrites an existing draft/release. A publication-time race is reported as a possible public incident, not prevented exposure; see the single-writer operating policy in the release guide.
- `release_common.py`: strict commit/tag resolution, release asset metadata and checksum validation shared by the publication and tap helpers. It performs no writes to GitHub.
- `verify-homebrew.py`: require a stable immutable release; download all assets and verify checksums plus source-bound checksum/archive provenance before any tap credentials are minted.
- `homebrew-formula.py`: render a stable four-platform formula from verified checksums. Refuses incomplete/duplicate checksums, prereleases and downgrades when given `--current`.
- `check-packages.py`: inspect all four archives, verify their checksums and required supporting files, and run the host's packaged executable.
- `security/`: isolated, pinned govulncheck module. The application module still uses only the Go standard library. Dependabot maintains scanner dependencies.
- `testdata/release/`: synthetic offline service doubles, not cryptographic verification or evidence of a real GitHub publication.

Run `python3 -m unittest discover -s tools -p '*_test.py'`, `make packaging` and `make security`. See [release operations](../docs/releases.md).

The publication guard, its fixtures/tests, scanner pins and workflow conventions were adapted from [rebaze/rio at 56741f3](https://github.com/rebaze/rio/tree/56741f32e5ba0d2ad8bd01833c0b9c54fdbf9542), with Preflight-specific packaging and stricter four-platform inventory validation. Rio's source and Preflight are Apache-2.0 licensed. The owner explicitly selected the same project license; see [LICENSE](../LICENSE) and [NOTICE](../NOTICE).

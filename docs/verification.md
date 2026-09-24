# Verification record

Date: 2026-09-23. All commands run from `/Users/tonit/devel/rebaze/preflight` unless noted. No project commits or publication.

| Command | Actual result |
| --- | --- |
| `go test -race ./...` | PASS: CLI, integration and internal/preflight packages; no race failures |
| `go vet ./...` | PASS, exit 0 |
| `conftest verify --policy policy` | PASS: 2 policy tests, 0 failures/warnings/skips |
| `go build -o bin/preflight ./cmd/preflight` | PASS, binary produced |
| `bin/preflight --help` | PASS, usage only |
| `PREFLIGHT_DOCKER_TESTS=1 go test ./internal/preflight -run '^TestDockerSyntheticReports$' -v -count=1` | PASS: real container pass/fail/missing evidence, 3.219 s package time |
| `PREFLIGHT_SYNTHETIC_DEMO=1 go test ./integration -run '^TestSyntheticCLIWorkflow$' -v -count=1` | PASS: full real-Vitest synthetic workflow, 23.25 s scenario time |
| `PREFLIGHT_WRITE_EXAMPLES=1 go test ./integration -v` | PASS: public CLI exits 0/1/2/3, common-model examples generated |
| Real pilot `init`, `explain`, `prepare`, `check --all`, `status` | Completed; final check/status exit 1 with current, preserved baseline failure; details in pilot-results.md |
| Direct pinned Conftest on final pilot inputs | Exit 1; exact same `eer.tests/tests_failed` violation |
| Pilot NUL-delimited Git status before/after | Byte-identical |
| Project `git diff --check` | Not applicable: standalone directory has no Git metadata or remote; whitespace/local-link/JSON checks used instead |

Ordinary `go test -race ./...` deliberately skips the opt-in networked/container demonstrations; both were run explicitly as listed. Synthetic fixture Git commits are confined to disposable test repositories, with local fixture identity and hooks/signing disabled. They are not commits in this project or the pilot.

Tests observed failing before implementation covered missing contracts, snapshot selection, policy integration, isolation/evidence handling, workflow orchestration and CLI errors. Additional concrete red/green regressions addressed actual Conftest metadata/YAML behavior, strict receipts, runtime pin reuse, multiple XML roots, expected-rule completeness, preserving known failures alongside evaluator/collection errors, source containment, changed test review and JSON output for argument errors.

Independent code review reproduced three adapter defects with temporary overlays. All three were fixed, the original reproductions and permanent regressions passed, and scoped re-review found them addressed without a material new issue. The policy package stayed unchanged during those fixes; the final real report uses the initialized governing policy.

Implementation details and intermediate targeted commands are recorded in `task-1-results.md`, `task-3-results.md`, `task-4-results.md` and `task-5-demo.md`. The public CLI integration test and snapshot/app suites cover Tasks 2 and 5. No baseline failure was removed to make a demonstration green.

## 2026-09-24 — Independent repository handoff

Initialized the standalone Git repository on `main`; no commit, remote, push or publication. Added repository-owned design/context, AGENTS.md, local verification skill, development guide and prioritized roadmap. The sibling design record is now historical provenance, not a checkout dependency.

A fresh-collaborator exercise first exposed hardcoded Homebrew evaluator fixtures, prior-host image assumptions and absent local design authority. Fixture-only changes now select `PREFLIGHT_CONFTEST` or PATH, pin the chosen executable, require an explicitly supplied immutable local demo image, inspect its availability, and generate synthetic manifests using the host UID/GID. No production CLI/policy/runner behavior changed.

Copied all Git-eligible source/docs into a separate directory, excluding `.git`, `bin`, private state and old reports. On that independent copy, using the existing macOS/arm64 toolchain:

| Check | Actual result |
| --- | --- |
| `go test -race ./...` | PASS; 43.837 seconds wall time |
| `go vet ./...` | PASS |
| `conftest verify --policy policy` | PASS; 2 policy tests |
| Build and CLI help | PASS |
| Explicit Docker reporter/bootstrap exercise | PASS; all three report states; 7.065 seconds wall time |
| Real-Vitest synthetic workflow with the freshly observed image ID | PASS; 26.771 seconds wall time |
| Final host-UID/image-validation regressions and updated real demo | PASS; updated real demo 27.89 seconds; source worktree run |
| Updated integration race checks and vet in the independent copy | PASS |
| Skill frontmatter, local Markdown links/anchors, JSON and whitespace | PASS |
| Ignore rules | Binary, state and `.env` excluded; skill and synthetic examples remain eligible |

Private validation logs/commands: `/tmp/preflight-standalone-validation-20260924`. Final updated demo artifacts: `/tmp/preflight-synthetic-demo-3969426294`. These local paths are supplementary evidence, not dependencies for future runs.

The validated image was `sha256:8b20521bb87c0faff185daa61be542bdf5ef2a45948fc62f547945df6effafe1`, base `node@sha256:6f7b03f7c2c8e2e784dcf9295400527b9b1270fd37b7e9a7285cf83b6951452d`, arm64. Future users obtain their own actual image identity using the development guide.

Independent read-only handoff review found and rechecked the host-ownership fix, immutable/local image contract and missing CGO/compiler prerequisite. Scoped re-review found all addressed. Native Linux was not executed; the ownership fix is regression-tested and exercised on macOS, not a claim of a complete cross-platform support matrix. No new real pilot run was needed or performed for this handoff; its original failed baseline remains unchanged in the historical record.

## 2026-09-24 — Initial commit checks

Before the user-authorized initial commit, reran `go test -race ./...`, `go vet ./...`, `conftest verify --policy policy` and the CLI build: all passed (Go test results reused the valid cache; Rego reported 2 passing tests). Checked 31 representative generated/private paths are ignored and 12 representative source, skill, schema and synthetic-example paths remain eligible. Staged whitespace checks passed. No new Docker or real-pilot run was needed for the ignore-rule/documentation changes; prior execution evidence remains above. No remote or publication action is included.

## 2026-09-24 — GitHub release automation preparation

Prepared GitHub Actions using Rio commit `56741f32e5ba0d2ad8bd01833c0b9c54fdbf9542` as a read-only reference. The selected module is now `github.com/rebaze/preflight`; CLI changes add version metadata only. No pilot runner/policy/runtime pins changed.

Local macOS/arm64 results:

- `go test -race ./...`, `go vet ./...`, `conftest verify --policy policy`, build: passed; 2 Rego tests. This first run selected the existing Homebrew Conftest reporting `dev`, OPA 1.20.2.
- `python3 -m unittest discover -s tools -p '*_test.py'`: 20 tests passed, including publication refusal on changed/missing assets, invalid verification, incomplete platform inventory, existing drafts and Homebrew prerelease/downgrade rejection. Synthetic service doubles do not authenticate real signatures.
- actionlint, shellcheck, `goreleaser check`: passed. GoReleaser 2.18.2 built all four macOS/Linux amd64/arm64 snapshot archives; the archive checker verified checksums, supporting files and the native packaged CLI/version.
- The initial archive check failed because the documentation glob omitted top-level documents. Explicit documentation/example globs fixed the packaging; the same archive check passed afterwards.
- Generated Homebrew formula: Ruby syntax passed. This is not a completed Homebrew installation from a published release.
- Pinned govulncheck module: tidy/verification passed; no vulnerabilities found in the application or scanner at this run's database state.
- Local Markdown file links and Git whitespace checks passed.

No new real pilot or Docker/Vitest run was required for these distribution changes. A manual synthetic workflow is prepared. Real OIDC signing, release upload verification and tap publication require a selected version tag and configured credentials and have not been claimed as executed. The historical real-pilot failure above remains unchanged.

The CI setup script was also executed locally against the official Conftest 0.70.1 Darwin arm64 archive. Archive SHA256 `b8eae5ce6c7c3a768a9b9c6c12b0c01f8fc065a99267c94646d309c479b218dd` verified; extracted executable SHA256 `a2971ccc84390569b202a853a1aa75f0923590bb8f1a0a673324f17c98f3aeed`, reporting Conftest 0.70.1 / OPA 1.20.2. The full Go race suite and both Rego tests passed with this explicitly selected evaluator as well.

Independent read-only review found no important correctness issues. It additionally evaluated the generated formula with Homebrew Ruby and installed a synthetic binary plus policy/profile/runtime/schema directories into a temporary prefix successfully. It confirmed the GitHub CLI attestation JSON and CycloneDX predicate shapes against official implementation sources. Real published-archive installation, GitHub OIDC, release upload and tap push remain unexecuted.

The owner subsequently selected Apache-2.0. Added the license, Rio attribution notice, README badge and Homebrew license field; rebuilt and inspected all four archives to require both LICENSE and NOTICE. All 20 helper tests and the updated package checks passed.

First hosted CI run [36002716083](https://github.com/rebaze/preflight/actions/runs/36002716083) failed the formatting gate on both test runners: the GitHub module-path migration changed import sorting in `integration/pilot_test.go`. Applying gofmt fixed the import order; the integration suite passed locally. This failure is retained; the subsequent hosted run must establish its own result.

## 2026-09-24 — PR #1 release-integrity corrections

Assessed all three inline Copilot findings and the summary-only build/Homebrew/attribution concerns against commit `759227f`. Reproduced the original remote-asset race with synthetic services: modifying a draft archive after its verification download still reported publication success. A fresh exported checkout with no `bin/` directory built successfully, and the Rio attribution commit resolved; those two summary concerns required no code change.

Added source-SHA inventory/attestation constraints, bounded annotated-tag resolution and remote tag checks; disabled CodeQL checkout credential persistence. Publication now compares draft asset digests before the request and verifies immutable state, tag identity and downloaded locked bytes afterward. Failures after the publication request explicitly report a possible public incident and prevent downstream Homebrew. A new Homebrew helper verifies immutable state, all four actual archives, their checksums and source-bound provenance before the App token step.

Regression tests first demonstrated acceptance of moved tags, wrong-commit attestations and the after-download/during-publication asset races in the old guard. Final verification:

- `make check`: Go race suite, vet, 2 Conftest policy tests, helper tests and build passed.
- Final `python3 -m unittest discover -s tools -p '*_test.py'`: 37 tests passed, including the additional tag-change-during-publication incident case.
- `make packaging`: actionlint, shellcheck, GoReleaser configuration, four real snapshot archives and native packaged CLI checks passed.
- Local Markdown file links and `git diff --check`: passed.
- Independent read-only review found no important correctness issues and confirmed the REST API assumptions against official GitHub documentation. Its independent run passed the 36 tests present before the final additional tag-race test.
- Enabled GitHub immutable releases on `rebaze/preflight` and read back `enabled: true`, `enforced_by_owner: false`. No release/tag was created; no branch/tag rules or organization policy changed.

These tests use explicit offline service doubles for the race and publication cases. Actual signing/publication and Homebrew installation still await a release. Immutability begins at publication; there is no claim of atomically preventing another privileged writer from changing a draft. The single-writer operating policy and incident response are documented in `releases.md`. Hosted validation for the new commit is tracked on PR #1; the earlier failed run remains in this record.

## 2026-09-24 — Release App secret and live verification

Stored `RELEASE_APP_PRIVATE_KEY` from the user-selected 1Password reference directly through stdin to GitHub's secret store. No plaintext key was printed or committed. The first manual verification run, [36010428624](https://github.com/rebaze/preflight/actions/runs/36010428624), successfully minted the tap-scoped token requesting Contents:write, but failed because the checker incorrectly treated repository user-role `permissions.push` as the installation token's permission grant. GitHub returned false user-role flags despite successful explicit token minting.

The checker now relies on explicit permission selection at minting, verifies the installation token's repository list contains exactly `rebaze/homebrew-tap`, and reads the tap contents. This remains read-only and does not claim that branch rules permit a future push. actionlint and whitespace checks passed; live verification of the corrected workflow is recorded in its GitHub run.

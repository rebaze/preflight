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

The first corrected branch run, [36010675331](https://github.com/rebaze/preflight/actions/runs/36010675331), exposed a GitHub CLI argument incompatibility: `--slurp` cannot be combined with its built-in `--jq`. The workflow now pipes paginated JSON through standalone jq under Bash pipefail. This failure did not indicate a key or App permission problem.

## 2026-09-24 — Neutral example identifiers and project description

Removed source-project and workspace identities from current source, schemas, fixtures, examples, packaging and public documentation. The bundled profile is `frontend-vitest`; its synthetic npm workspace is `@example/frontend` under `applications/frontend/apps/web`. The compiled runner and inspectable runtime script changed together. The README and design describe Preflight as a small, focused project. Historical reference-checkout paths and revision identifiers were omitted; the original eight suite-loading failures and zero assertions remain recorded as failures. Historical labels are anonymized, not claims that the current neutral names were the original input identities.

Verification:

- Profile naming test first failed against the previous profile, then passed with the neutral profile. The explicit receipt regression rejects the previous runner entrypoint digest; compiled/runtime equality also passed.
- `make check`: Go race tests, vet, 2 Conftest policy tests, 37 release-helper tests and CLI build passed.
- Docker reporter exercise: pass, fail and missing-report cases all passed. Freshly observed runner image: `sha256:be68af644897c1368b94ffbe3f6c703308f4fa0fa25fbcc94e84a0fa1cc5793f`.
- Full real-Vitest synthetic CLI workflow using that run's explicit image: passed in 29.67 seconds. Private synthetic run directory: `/tmp/preflight-synthetic-demo-2658113732`.
- `make packaging`: actionlint, shellcheck, GoReleaser config, all four archives and native packaged CLI passed; package checks require the renamed profile file.
- Regenerated common-model text/JSON examples. Current file names/content have no source-project identities or withdrawn project-stage labels; JSON parsing, local Markdown links/anchors and whitespace checks passed.
- Independent review confirmed code/path consistency, the previous runner digest in the receipt regression, preserved historical outcomes and explicit state migration guidance. Remaining framing and generated-example whitespace findings were corrected.

The profile/runner rename intentionally does not migrate private initialized state. Keep older evidence intact and explicitly initialize/prepare new state for the current profile. No real application checkout was accessed or changed. Existing Git history was not rewritten.

## 2026-09-24 — issue 4 stage 1 local discovery

Isolated implementation branch starts at current main `352e58d`. Baseline `go test -race ./...` passed before implementation. New discovery is independent of trusted policy state and the existing report v1 contract. The full race suite, `go vet ./...`, `conftest verify --policy policy` (2 passed), and CLI build passed. Focused public CLI fixtures cover Go, JavaScript under `clients/browser`, and an empty Git repository without dependencies or Conftest. Collector regression tests cover staged/unstaged/untracked/committed/deleted paths, linked worktrees, SHA256 Git, missing refs, non-Git directories, ignored/private/symlink inputs, content limits and source freshness. Hostile Git filters/fsmonitor/hooks and package-script tripwires remained unexecuted; source and index bytes remained unchanged. Strict decoder tests reject unknown/duplicate/missing/null/version/enum/exit inconsistencies; source content and line identities are checked.

The public plugin candidate 0.1.0-rc.1 was accepted by Codex CLI 0.156.1 in a fresh private CODEX_HOME. Plugin and skill validators passed. Fresh-home execution initially reported no authentication; this is not a successful skill invocation. Authenticated fresh-session evaluation using existing Codex access is recorded separately below. No credentials were copied into project/plugin files and no global config was edited. No Docker/runtime path changed; Docker/Vitest exercises were not rerun for discovery. No human usability claim is made.

## 2026-09-24 — issue 4 stage 2 GitHub read adapter

Synthetic adapter tests cover effective/legacy rules, reviews, pagination and page/request limits, same-name results from different Apps, same-name check plus legacy status, wrong/stale revision, PR head/merge precedence, unknown producer, unsupported rules, denied access and no-enforced-check repositories. A fake gh executable verifies GET-only argv and sanitized diagnostics. Normal tests do not require GitHub access.

Authorized real-repository reads used rebaze/preflight only. First observation against an unpushed implementation SHA took 3.023 seconds and retained GitHub HTTP 422 for missing remote check-run coverage; it was partial, not no-results success. A subsequent observation of remote main took 3.014 seconds (3.94 seconds including resolution in the test), resolved target main, observed zero effective rules/required checks, explicit branch `protected=false`, and six check results with complete checks/status coverage. No rules were changed and no CI was dispatched. The opt-in reproduction is `PREFLIGHT_GITHUB_READONLY_TEST=1 go test ./internal/preflight -run '^TestGitHubAuthorizedRealRepository$' -count=1 -v`.

Stage 2 exact-commit verification: the isolated `a7b1b8c` worktree passed `go test -race ./...` (all packages), vet, Conftest (2 passed), and build. A concurrent working-tree suite earlier failed an unfinished stage 3 result-ID fixture; it is retained as a development failure, not reported as stage 2 passing evidence. Independent review then identified comparison-tip freshness and oversized-observation round-trip gaps; regression fixes and their final validation are recorded below.

## 2026-09-24 — issue 4 stage 3 observations and explanations

Private observation save/compare and structural workflow tests cover exclusive owner-readable output, checkout/Git/symlink rejection, strict loading, changed source/requirements, incompatible subjects, stale evidence and producer/revision-bound observed result intervals. A failure with no supported explanation retains an unknown cause. Initial transition fixture used a nonnumeric result ID after the GitHub validator was strengthened; that fixture failed and was corrected to the real normalized ID shape.

`python3 evaluation/demo_compare.py bin/preflight` passed using a new disposable fixture: removing literal `pull_request` produced `pr_trigger_removed` plus stale prior evidence; restoring it produced a resolved structural finding; the original and changed observations remained present. No repository code or checks executed. Independent structural review found unsupported block/flow/indentation forms, duplicate names and replacement jobs could cause false removal claims; regression tests reproduced these and the parser now leaves unsupported/ambiguous forms unverified. This is synthetic structural evidence, not a claim of remote CI execution or human usability.

Final stage 2 review-fix commit `0b10130` passed the full race suite, vet, Conftest (2 passed) and build in its isolated worktree. Stage 3 working increment then passed the full race suite (all three packages), vet, Conftest (2 passed), build and repeated public change/restoration demo. Discovery/evidence-only additions did not change the Docker runner; no new actual-container execution is claimed.

Stage 3 integration review additionally reproduced a saved observation containing a workflow source without any corresponding captured input. That internally inconsistent source could otherwise produce a false verified deletion. Validation now requires every source to match a captured file's digest/mode; workflow coverage checks both directions. The bounded collector preserves those pairs when truncating, with a regression for long-path prefixes. Previously observed missing-source acceptance and pair-loss failures are preserved here; focused corrected tests and final suite establish the delivered behavior.

## 2026-09-24 — issue 4 stage 4 investigation boundary

The public skill defaults to no delegation; optional work has explicit question, source and elapsed-time budgets with serial fallback. The bundled packet reconciler returns only unverified hypothesis/unresolved states and never changes observations or check results. Regression tests cover unsupported/contradictory statuses, duplicate/unknown fields, missing citations, stale identity, tampered content, source budgets and preserved failed evidence. Go validation additionally rejects a verified generic claim with an absent/stale captured citation (red then green). Actual native-harness and fallback timings, including a failed ephemeral child-thread attempt, are recorded in the evaluation results rather than inferred from unit tests.

Stage 4 actual agent evaluation: five recorded runs preserved source/index digests and tripwire absence. Ephemeral delegation failed once and used honest serial fallback. Two fresh persistent sessions have successful native spawn tool results, with the first sourced briefing before delegation and completion within the 60-second investigation window. First useful responses were 21.889–27.760 seconds; full times and the failed attempt are preserved in [stage 4 results](../evaluation/stage-4-results.md). Ten reconciliation tests and skill validation passed. These are agent observations, not human validation.

Stage 4 final standard validation passed: `go test -race ./...`, `go vet ./...`, Conftest policy verification (2 passed), build, all ten Python investigation tests and public skill validation. No runner changes or container execution are claimed.

# Issue 4 implementation plan and progress

Goal: a small, installable Codex companion that quickly explains sourced expectations, revision-bound evidence and next actions. Authority: [current issue 4](https://github.com/rebaze/preflight/issues/4), read in full on 2026-09-24. The user approved all five stages and autonomous implementation with stacked PRs.

Architecture: a distinct strict `preflight.discovery/v1` observation holds local facts, optional GitHub requirements/results, coverage and diagnostics. Go gathers bounded facts without running project code; the public skill selects relevance using conversation intent. Existing report v1, Conftest policy decisions, trust state and Docker execution remain independent.

Constraints: standard-library-only Go; Git required only for Git context; no dependency installation for discovery; no implicit trust or execution; no repository mutations; private observations outside checkout; source origin separate from verification; errors retain useful partial facts. No release/tag/merge/marketplace publication.

## Stage 1 — local discovery and first skill

- Add `inspect_model.go` for strict versioned records, decoding/validation and shared rendering; `inspect_local.go` for bounded instruction/manifest/workflow discovery and Git identities; `cmd/preflight/inspect.go` for independent flags and exit semantics (0 complete bounded discovery, 2 partial, 3 unusable/error).
- Track HEAD/index/worktree/comparison identities, staged/unstaged/untracked paths, all tracked content identities (without ingesting source contents), safe bounded text sources with lines/digests, omissions and unknown coverage. Missing refs, empty/non-Git repos and linked worktrees remain useful.
- Write failing tests for the public command, strict unknown/duplicate/missing fields, symlinks/private paths/limits, changes and no script/hook execution; implement and run focused tests then standard suite.
- Add public `skills/preflight/`, supported plugin manifest and synthetic Go/JavaScript/empty demonstrations. Update README/design/architecture/roadmap/verification and open stage PR.

## Stage 2 — GitHub facts

- Add a bounded authenticated `gh api` GET adapter in separate `inspect_github*.go`; explicit `--github` and optional `--pr`; resolve PR base/head/merge and default/comparison branch.
- Collect paginated effective rules, legacy protection, check runs and statuses. Bind requirements to rule/source/app and results to exact revision/producer; retain local-edit coverage gaps, unsupported rules and API errors.
- Tests cover pagination, duplicate apps, stale SHA, head/merge, absent checks, denied access and CI without requirements. Read-only real validation on rebaze/preflight; standard suite; docs and stacked PR.

## Stage 3 — observation lifecycle

- Add exclusive safe private `--output` and strict `--compare`, preserving prior files; reject incompatible repository/worktree/target identities.
- Compare input/requirement/finding identities and comparable result transitions. Add conservative structural workflow facts for deleted workflow, removed literal PR trigger and uniquely mapped deleted required job; ambiguous/dynamic YAML remains unverified.
- Test edit/consequence/restore, stale source evidence, unknown failure cause, malformed saves, unsafe paths and identity mismatch. Standard suite, fixture demonstration, docs and stacked PR.

## Stage 4 — bounded investigations

- Extend the public skill with optional one-question investigation packets, at most two, 60 seconds and bounded sources each, no delegation by default; briefing first; serial fallback; stop at budget.
- Reconcile citations and captured identities; opinions never alter structured verified status. Add synthetic adversarial/contradiction evaluation and useful/unresolved actual harness examples. Docs and stacked PR.

## Stage 5 — installable candidate and measured onboarding

- Verify current official Codex format and installed CLI behavior; package a pinned candidate with one public skill, explicit CLI compatibility and reproducible local/Git installation, marketplace listing and release instructions.
- Prefer embedding plugin files in the existing four signed CLI archives to preserve the explicit release inventory; update package content checks and native smoke checks together. No extra release asset unless all inventories/signatures/attestations/Homebrew change together.
- Run actual fresh Codex sessions on synthetic layouts: Go, nested JS and empty/unsupported; cover partial, stale, conflicting instructions, unsupported checks and hostile evidence. Record five timings, installation timing, all misses, agent plan impact; no human-validation claim.
- Run race tests/vet/Conftest/build and release-helper/package checks; perform independent review; fix important findings; docs and stacked PR. Keep unmet external acceptance items open with exact reproductions.

## Review focus

1. Git filters, fsmonitor, hooks and malicious path/ref content must not cause execution.
2. Symlink ancestors, excessive inputs and private filenames must not leak content.
3. Missing permissions and unsupported GitHub requirements must never imply no gates or pass.
4. Saved observations must strictly validate and cannot authenticate fabricated evidence.
5. Stale/ambiguous producer/revision mappings must not manufacture verification or causal history.

## Progress (append-only)

- 2026-09-24: Full issue and current docs inspected. Main is `352e58d`; clean original checkout; no issue progress or open implementation PRs. Isolated worktree `/Users/tonit/devel/rebaze/preflight-issue4`, branch `feat/4-local-inspection`. Existing toolchain Go 1.27.1, Conftest dev / OPA 1.20.2, Codex CLI 0.156.1. Baseline race suite started.
- Stage 1 implementation: distinct strict model/schema and independent CLI; bounded local collector and public skill/plugin. Baseline and implementation race suite, vet, Conftest and build passed; focused three-layout public CLI demo passed. Public package installed in isolated Codex home; first authentication attempt unavailable, fresh session reuse of existing harness login underway. Stage 2 adapter and stage 3 private lifecycle are being developed in separate files, excluded from stage 1 commit.
- Stage 1 review delivery: PR #5; actual fresh-session Go/JS/empty evaluations passed without source changes (see evaluation/stage-1-results.md). Configured signer failed; user explicitly authorized unsigned commits, applied per command without changing configuration.
- Stage 2 implementation: GET-only GitHub adapter, strict nested facts, flags/common rendering and post-read source recheck. Synthetic focused tests and real read-only main/unpushed-SHA cases passed with partial case preserved. Standard suite rerun underway. Stage 3 files remain outside stage 2 commit.
- Stage 2 review corrections: comparison BaseRef/BaseCommit now participate in identity; 1.5 MiB serialized local fact budget with explicit partial output and diagnostic cap; discovery decode budget 16 MiB. Both findings reproduced before fixes. Separate stage2 PR opened; exact stage2 full suite passed before fixes and focused race/vet passed after fixes; final isolated suite running.
- Stage 3 implementation: exclusive private output, strict compare, compatible observed histories and supported structural CI removals/restorations. Public CLI demo passed. Structural ambiguity review strengthened conservative parsing; no unsupported YAML is treated as verified semantics. Full verification next.
- Stage 3 committed as `3416276`; full standard suite and repeated structural CLI demo passed, with subsequent focused identity-pair review fixes passing. PR is stacked on stage 2. Comparisons remain local user-controlled observations, not attestations.
- Stage 4 adds bounded optional investigations with native/serial demonstrations and deterministic packet reconciliation. Generic verified claims also require current captured citations; an unsupported investigator claim was reproduced as invalid before the guard.
- Stage 4 committed `87e4655`, PR #8, standard suite and ten reconciliation tests pass; real native and serial-fallback evidence in evaluation/stage-4-results.md. Stage 5 branch adds CLI capabilities, versioned archive/plugin metadata and source-independent installation evaluation.
- Final independent review found fork/base result repository mismatch, incomplete nested requirement coverage and order-dependent rule IDs. These are being fixed with reproductions before candidate packaging; earlier successful suites did not cover those cases.
- Final review corrections implemented with red/green regressions: fork checks/statuses use target repository and matching enforces it; all nested requirement collection errors prevent false removal; stable rule IDs ignore response order and duplicate declarations retain ambiguity. Capability smoke now explicitly requests JSON. Final standard checks and clean candidate build next.
- Candidate `9662a51` packaged/installed successfully and rc.2 met 5/5 timed cases. Final extra serial scenario exposed unbounded skill-path fallback to an older draft; explicitly failed, despite source preservation and honest budget exhaustion. Tighten registered-path bootstrap guidance in skill metadata/body, advance candidate version to rc.3, rebuild clean, and rerun unassisted installed-package evaluation. Do not assist the prompt with an internal cache path or count rc.2 timings as rc.3 measurements.

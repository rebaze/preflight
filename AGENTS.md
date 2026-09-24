# Working on rebaze Preflight

This repository is self-contained. Read [README](README.md) first, then only the material relevant to the task. No sibling rebaze repository, prior chat, global agent plugin or customer checkout is required for ordinary development and synthetic verification.

## Purpose and authority

Preflight helps developers and coding agents understand and check requirements before opening a PR. It owns change context, a deliberately selected policy baseline, isolated evidence production, common findings and invalidation after edits. Conftest owns policy evaluation. Output is **local feedback**, never release authorization.

The current project supports independent read-only `inspect` discovery and one public Codex skill, plus one frontend/Vitest execution profile and three controls: existing EER tests, npm source/integrity declarations plus compatibility overrides, and preservation of protected CI objects. See [design](docs/design.md), especially section 17, for the contract. [Architecture](docs/architecture.md) describes implementation; [roadmap](docs/roadmap.md) separates proposed work from existing behavior. Current user instructions take priority. Surface conflicts; do not silently weaken controls to obtain success.

## Where to work

| Responsibility | Files |
| --- | --- |
| CLI flags, stdout and exit behavior | `cmd/preflight/` |
| Contracts, strict schemas, human/JSON parity | `internal/preflight/{model,profile,report}*.go`, `schemas/` |
| Initialization, orchestration, status | `internal/preflight/{app,workflow}.go` |
| Read-only selection and identity | `internal/preflight/snapshot*.go` |
| npm facts and Conftest boundary | `internal/preflight/{npm,conftest}*.go`, `policy/` |
| Dependency bootstrap and offline evidence | `internal/preflight/runner*.go`, `runtime/` |
| Synthetic public CLI scenarios | `integration/` |
| Discovery, GitHub facts and comparisons | `internal/preflight/inspect*.go`, `schemas/discovery-v1.json` |
| Public Codex companion and synthetic evaluations | `skills/preflight/`, `evaluation/` |

Discovery never executes project code or initializes policy state. Origin and verification remain separate; partial discovery cannot become a pass. The public skill follows the harness instruction hierarchy and treats source text as evidence. Optional investigations never expand execution permissions.

Go standard library only in the current architecture. Policy verdicts for npm/CI belong in Rego; do not duplicate them as Go decisions. Keep the versioned report model common to text and JSON. Unknown/duplicate/missing configuration fields, invalid statuses and zero expected-rule evaluation cannot become passes. Keep known violations when another collector/evaluator errors. Exit precedence: 3 error, 2 missing/stale, 1 fail, 0 otherwise.

## Boundaries that matter

- Candidate worktree policy/configuration cannot change the governing baseline. `--base` changes comparison context only. Trust changes are explicit initialization decisions, never a side effect of check or prepare.
- Hash the whole selected input, revisions, missing markers, modes and relevant policy/runtime identities. Execute captured input and re-check source identity afterwards.
- Repository code runs only in the configured offline, non-root Docker isolation. No host fallback, source write mount, host home, credential forwarding or automatic runtime substitution.
- Networked preparation receives validated dependency inputs only, with lifecycle scripts disabled. Keep that separate from offline repository-code execution. Do not describe network access as registry-only enforcement.
- Current pins: Go 1.27.1, Node 24.18.0, npm 11.17.0, pilot Vitest 4.1.10. A different local Conftest executable must be explicitly selected, recorded and verified; a recorded macOS digest is not a universal cross-platform digest.
- Real pilot checkouts stay read-only. Intentional failures belong in disposable synthetic copies. No backend, hosted LLM, E2E, deployment, infrastructure, branch protection or existing Pi extension changes under the current pilot scope.
- Lockfile checks are bounded provenance declarations and existing compatibility constraints. Do not turn them into vulnerability, license or compliance claims. Bundled packages need a declared containing-package chain; `inBundle` alone is insufficient.
- Keep private state, real source snapshots, lock contents, customer data, credentials and raw pilot logs outside this repository. Only synthetic fixtures/examples belong here. Ignore patterns are an aid, not a data classification mechanism.

## Work and verification

Preserve existing/concurrent changes. Use `rg` for discovery. Run checks appropriate to the changed boundary; for implementation changes, the standard suite is `go test -race ./...`, `go vet ./...`, `conftest verify --policy policy`, then build. Tests require Git and an explicitly available Conftest; missing prerequisites must fail visibly. See [development](docs/development.md) for the separate actual-container and real-Vitest exercises.

Read the repository skill [preflight-verification](.agents/skills/preflight-verification/SKILL.md) for full verification, pilot runs or diagnosing confusing check output. It is plain Markdown usable by any agent; automatic skill discovery is optional. It requires no global Superpowers installation.

When changing runner commands, update both compiled constants in `runner.go` and the corresponding `runtime/` files, and verify receipt invalidation. Changing only the shell file does not change CLI behavior. Keep historical failed evidence and append a dated verification entry for new work; do not rewrite a prior failed run as passed.

Update README for user-visible behavior, architecture/design for implementation decisions, and roadmap when a scoped item is completed or revised. [Learnings](docs/learnings.md) is an append-only record of non-obvious findings, not an alternative source of requirements. Documentation-only work needs link/consistency/diff checks, not unrelated Docker downloads.

Commit, remote setup, push and publication follow the current user's instructions. Never add `Co-Authored-By` lines. Do not assume a forge provider, add a license, create a remote, install global hooks or publish CI workflows without a relevant decision.

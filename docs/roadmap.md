# Direction and next work

Updated 2026-09-24. Owner: Toni / rebaze. This is a proposed ordered backlog, not permission to execute every item or widen the current pilot scope. Select a bounded item in the next task and record its accepted changes here and in the design.

## What we are trying to prove

Help a developer or coding agent answer: which requirements apply to this change, what can be checked now, what evidence is missing, and what needs a later decision? Preserve effective checks while reducing surprises before a PR. Reduced release delays and product demand are hypotheses, not measured outcomes.

The prototype tests whether a reusable workflow around existing evaluators adds enough value to justify a standalone tool. Conftest already supplies policy evaluation and reports. The existing pilot Pi extension already plans checks and associates success with fingerprints. Compare with both honestly; packaging project scripts or contributing an extension remains a valid outcome.

## Starting point

All five commands and three controls are implemented. Synthetic real-Vitest pass/fail/tamper/staleness scenarios passed. The 2026-09-23 real pilot preserved eight suite-loading failures due to missing generated Nuxt tsconfig, with zero assertions executed; npm and CI controls passed. The original pilot was not modified. See [results](pilot-results.md) and [verification](verification.md).

The next operator session must not assume old `/tmp` reports, dependency volumes, the Homebrew path or image IDs still exist. [Development](development.md) reconstructs the synthetic workflow from this repository.

## R1 — Make setup and results understandable

Trigger: Toni ran `init` and could not tell whether its large output required project corrections. The current CLI prints the full explanation report, including repeated path lists and `deferred` labels.

Proposed outcome: successful initialization clearly says what was selected, where state lives, whether setup succeeded, that tests have not run, and the next command. Put detailed hashes/path inventories behind an explicit detail option or `explain`; retain meaningful failures and scope limitations. Keep JSON's versioned report contract and human/JSON finding equivalence intentional; decide any display/schema change before implementation.

Acceptance: a new user can distinguish setup success, unperformed checks, out-of-scope review and real failures without author assistance. Large legacy/untracked directories do not overwhelm the primary message. Current failures/errors cannot be hidden by concise rendering. Preserve the full structured evidence for agents.

## R2 — Resolve the frontend's generated-config prerequisite explicitly

Investigate the minimal supported Nuxt setup needed by the existing EER tests. A candidate solution may generate the required configuration offline inside the disposable container; the exact command and reachable code must be understood before adding it to the trusted runner.

Acceptance: retain the original failed report; record the changed runner definition and its new digest; invalidate incompatible receipts; execute only within the same offline/non-root/resource-limited boundary; obtain positive actual assertion counts and all eight baseline files, or report the remaining real failures. No host-generated `.nuxt`, no original checkout edits, no version substitutions, skipped tests or weakened evidence. Any command that needs additional execution/network permissions needs an explicit task decision.

## R3 — Make repeatable onboarding less manual

This handoff supplies portable test-tool selection and a fresh-image synthetic recipe. Next evaluate prerequisite diagnostics, a portable one-command synthetic exercise, bounded retention/cleanup of named local state, and explicit inspect/update procedures for pinned policy/runtime state.

Acceptance: a clean second machine can run the documented synthetic workflow without the pilot, author's home directory or prior image ID. Tool/architecture mismatches identify the exact prerequisite. Updating trust is a visible action that preserves old evidence and never lets a candidate self-approve. No silent format migration or broad Docker pruning.

## R4 — Compare integration effort with real users

After setup/readability and the real frontend prerequisite are addressed, observe a developer and a coding agent doing the same bounded change. Record time to first useful report, misunderstood findings, repeated integration work and maintenance effort. Compare direct Conftest with supplied facts and an appropriately scoped Pi integration. Running Pi's broader hooks is a new execution-scope decision, not implied by this backlog.

Acceptance: document which parts were exercised versus read, preserve inconvenient findings, and decide whether a standalone CLI, a maintained rules/runner package or upstream integration is justified. Do not infer market demand or release-speed gains from synthetic success.

## Deliberately deferred

Additional ecosystems/profiles, OCI policy distribution/signatures, remote enforcement, artifact attestations, exception authority, hosted service/UI, MCP/IDE integrations and release/deployment execution. The local-feedback authority boundary remains even if reporting improves.

Public product naming, license, repository namespace, distribution/support and forge/CI provider are owner decisions. Keep module `rebaze.local/preflight` until an actual module/distribution destination is chosen. Do not infer an open-source license or hosting destination from permission to prepare this repository.

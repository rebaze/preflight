# Direction and next work

Updated 2026-09-24. Owner: Toni / rebaze. This is a proposed ordered backlog, not permission to execute every item or widen the current pilot scope. Select a bounded item in the next task and record its accepted changes here and in the design.

## Project direction

Help developers and coding agents understand which requirements apply to a change, what they can check locally, what evidence is missing, and what needs a later decision.

Preflight combines change context, selected policy, isolated test execution and evidence freshness in a small CLI. Conftest evaluates policy; Preflight supplies the inputs and presents common findings. Priorities are clearer setup, reliable frontend verification and simpler onboarding.

## Starting point

The checking workflow and three controls are implemented. Synthetic real-Vitest pass/fail/tamper/staleness scenarios passed. The 2026-09-23 real pilot preserved eight suite-loading failures due to missing generated Nuxt tsconfig, with zero assertions executed; npm and CI controls passed. The original pilot was not modified. See [results](pilot-results.md) and [verification](verification.md).

The next operator session must not assume old `/tmp` reports, dependency volumes, the Homebrew path or image IDs still exist. [Development](development.md) reconstructs the synthetic workflow from this repository.

## Approved near-term sequence — issue 4

[Issue 4](https://github.com/rebaze/preflight/issues/4) authorizes five working increments. [Plan and durable progress](plans/issue-4.md) records implementation and verification. Stage 1 adds independent bounded local inspection and the public skill. Stages 2–5 add actual GitHub gates/results, saved comparisons, optional bounded investigations and installable measured onboarding. These capabilities extend discovery; arbitrary ecosystem execution, hosted models and release authority remain excluded. Older R1–R4 below remain separate proposed work.

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

Acceptance: document which parts were exercised versus read, preserve inconvenient findings, and use the observations to improve the CLI and its integrations. Record usability improvements against actual user experience.

## Deliberately deferred

Additional ecosystems/profiles, OCI policy distribution/signatures, remote enforcement, artifact attestations, exception authority, hosted service/UI, other MCP/IDE integrations and release/deployment execution. The local-feedback authority boundary remains even if reporting improves.

GitHub repository/module `github.com/rebaze/preflight` and GitHub Actions release preparation were selected by the owner on 2026-09-24. CI, signed/attested release packaging and optional Homebrew formula publication are prepared; see [release operations](releases.md) for credentials, activation and first-release status. This distributes the CLI and does not expand its authority or resolve R1–R4. The owner selected Apache-2.0 to match Rio. Long-term support remains an owner decision.

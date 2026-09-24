# Pilot results — 2026-09-23

The implemented workflow runs against both synthetic fixtures and the read-only invoicex frontend. The real pilot correctly reports a **baseline frontend failure**, with both static controls passing. No pilot files were modified, no controls were weakened, and no backend, hosted LLM, E2E, deployment or infrastructure operation ran. No commits, pushes, branch-protection changes or publication occurred.

## Real result and evidence

Source: `/Users/tonit/devel/invoicex/invoicex`; baseline and HEAD: `ed91ed4dcb21d363497308f7f3f31d54d9a2fc7d`.

Private state:
`/var/folders/yp/1p9p25pd4dx_6dr01lncvwz00000gn/T/preflight-invoicex-pilot-20260923-0e0vy2v6`

Final report:
`/var/folders/yp/1p9p25pd4dx_6dr01lncvwz00000gn/T/preflight-invoicex-pilot-20260923-0e0vy2v6/report-final.json`

The final run material is under `runs/check-592167416/` inside that state: captured projection, `tests.log`, `vitest.xml`, `vitest.json`, normalized `evaluation/facts.json`, both CI inputs, raw Conftest output and the common report. `commands.json` records actual argv, elapsed times and exit codes. No pilot source, lock contents or raw test logs are in this project.

| Finding | Actual result |
| --- | --- |
| `npm.dependencies` | pass — complete lockfile accepted, including the declared bundled child and existing compatibility overrides |
| `ci.integrity` | pass — protected semantic workflow objects and all baseline gate dependencies preserved |
| `eer.tests` | fail — eight suite-loading failures; zero Vitest assertions executed |
| `scope.review` | review_required — 1,565 existing non-ignored untracked paths outside the selected projection; path names only |
| `artifact.verification` | deferred — requires trusted build/release evidence |

The compatibility patch completed and both existing Node compatibility tests passed. Vitest attempted all eight baseline EER test files. The app's tsconfig extends generated `.nuxt/tsconfig.json`; generated `.nuxt` is excluded from the fresh projection and no Nuxt preparation command is in the approved runner. Each suite failed to load that configuration. JUnit represented eight failing suite cases; Vitest JSON reported eight failed suites and **zero actual tests/assertions**. Those are preserved as a failure, not counted as successful execution. Adding an offline Nuxt preparation step would be a separately reviewed runner change; this prototype did not add it or use host-generated files.

Final `check --all`: exit **1**, `current: true`, summary **2 pass / 1 fail / 1 review_required / 1 deferred**. Final `status`: exit **1**, `current: true`; it retains the current failed result rather than changing it to success.

The pilot's complete NUL-delimited Git status before/after is byte-identical, SHA256 `b7c260f32ae619c916d26a335d2944916952c14df6978974b6b38f1188fca4a5`. The selected-source fingerprint also remained unchanged across execution.

## Actual commands and timings

Build and workflow syntax are in [README](../README.md). The real commands used the exact source/baseline above, `profiles/invoicex-frontend.json`, `policy/`, and the private state above. These are individual observations, not comparative performance benchmarks:

| Command | Exit | Seconds | Observation |
| --- | ---: | ---: | --- |
| `init ... --format json` | 0 | 1.506 | Explicit baseline, profile, policy and evaluator pins |
| `explain ... --format json` | 0 | 0.638 | Requirements and exclusions; no repository execution |
| `check ... --all --format json` before preparation | 2 | 1.668 | Both static controls evaluated; precise missing preparation action |
| `prepare ... --allow-downloads --format json` | 0 | 10.789 | Manifest-only dependency installation; runner image already cached by synthetic verification |
| final `check ... --all --format json --output .../report-final.json` | 1 | 5.144 | Fresh offline execution, baseline failure retained |
| final `status ... --report .../report-final.json --format json` | 1 | 0.530 | Saved report still current |
| direct Conftest on final normalized inputs | 1 | 0.083 | Same `eer.tests/tests_failed` violation |

Runtime: Go **1.27.1**; Docker **29.8.0**, Linux/arm64; Node **24.18.0**, npm **11.17.0**, Vitest **4.1.10**. Evaluator reports `Conftest: dev`, OPA **1.20.2**, binary digest `b2f75ccf2575da4543ecec646194ae2f5a476fdf810681b04d01042b8e73ff18`, matching the canonical plan.

Official base resolved to `node@sha256:6f7b03f7c2c8e2e784dcf9295400527b9b1270fd37b7e9a7285cf83b6951452d`; runner image `sha256:8b20521bb87c0faff185daa61be542bdf5ef2a45948fc62f547945df6effafe1`. Preparation receipts retain those identities. Subsequent dependency preparation does not silently refresh the runtime pin.

## Direct Conftest comparison

From the private final run's `evaluation/` directory, with tool-owned empty HOME and no inherited Conftest/OPA configuration:

```sh
/opt/homebrew/Cellar/conftest/0.70.1/bin/conftest test \
  --combine --parser yaml --policy "$PREFLIGHT_STATE/policy" \
  --namespace main --output json \
  facts.json baseline-ci.yaml candidate-ci.yaml
```

`PREFLIGHT_STATE` denotes the private state above; the actual expanded argv is recorded in `commands.json`. `direct-conftest-final.json` and `comparison-final.json` record agreement. The same Rego package produced exactly the same violation identity and reason as Preflight. Wrapper-only scope and lifecycle obligations are explicitly separate from evaluator verdicts.

Conftest already provides policy evaluation, combined configuration inputs, JSON/text output and policy tests; early invocation and agent compatibility are not novel. The wrapper supplies context/evidence that direct Conftest in this comparison receives ready-made. It does not replace Rego with a second dependency/CI decision implementation. See official [combined inputs and configuration precedence](https://www.conftest.dev/options/), [Vitest reporter behavior](https://vitest.dev/guide/reporters.html), and [npm ci script controls](https://docs.npmjs.com/cli/v11/commands/npm-ci/).

Actual pinned-Conftest integration required nested violation metadata decoding and semantic normalization of its YAML parser's unquoted `on` key. Tests cover quoted/unquoted equivalence, comments/key ordering and path-filter changes. A small private synthetic contract probe forces a violation in each expected control before evaluating the candidate. It rejects packages with unrelated/empty rules that would otherwise create false passes from aggregate success counts. Probe findings never become candidate findings. This adds an evaluator invocation, not a new policy language or warning exemption.

## Existing Pi extension comparison

Read-only comparison used `.pi/extensions/quality-checks/planning.ts`, `check-execution.ts` and `index.ts`. Its commit/session hooks were **not executed**: they select backend, E2E and broader frontend work outside this pilot's permissions.

| Concern | Existing Pi extension, inspected | Preflight, exercised |
| --- | --- | --- |
| Check planning | Selects frontend, backend, architecture and E2E checks from changed paths; emits long-running advisories | One fixed frontend profile and explicit later/out-of-scope obligations |
| Change collection | NUL-safe staged, unstaged and untracked path collector | Those inputs plus committed branch changes relative to a resolved merge base |
| Validity after edits | Fingerprints HEAD and changed-file content/modes; associates successful checks/advisory evidence with fingerprints | Whole selected projection, explicit missing markers, policy/profile/evaluator/runtime identities, and standalone `status` |
| Existing check reuse | Frontend runs `npm run check`, broader than this EER slice | Explicit compatibility patch/test and complete EER Vitest invocation |
| Evidence | Existing executor already tracks success and parses check output; no claim of inventing check planning/invalidation | Bounded fresh JUnit plus same-run JSON file/assertion coverage, immutable captured input and post-run identity verification |
| Governing rules | Planner/executor implementation in the project extension | Explicit external private baseline and pinned Rego/profile/evaluator; candidate-local policy cannot self-approve |
| Execution boundary | Existing check configurations point at repository working directories | Controlled manifest-only download phase followed by disposable offline Docker execution |
| Human/agent use | Pi integration, notifications and hooks | Same versioned report model for text/JSON and all CLI clients |

This is an exercised Preflight/direct-Conftest workflow comparison and a **static Pi implementation comparison**. It is not a measured head-to-head Pi run, a claim that Pi lacks invalidation, or a market validation result. Equivalent scripts or a Pi extension could assemble these integrations. The potential value is a reusable, consistent workflow for baseline selection, evidence production, remaining obligations and invalidation; that value must still justify maintaining another executable.

## Synthetic verification

The full real-Vitest synthetic CLI demo passed using disposable synthetic Git data and private state. It demonstrated baseline success, assertion failure, restoration, unauthorized lock source, required-gate deletion, ineffective candidate-policy weakening, a source edit after a passing report, stale status and renewed successful evaluation. See [the full demo](task-5-demo.md) for commands, exit codes, timing and report paths. Invalid dependency changes correctly retain a dependency failure and missing compatible preparation together (exit 2).

The separate actual-container test produced distinct pass, fail and zero-exit/missing-report results. Unit/integration regressions cover committed changes with clean Git status, worktree-over-index precedence, filenames with spaces, relevant untracked files, exclusions and symlinks, pinned baselines independent of comparison refs, strict schemas, all exit categories, malformed evaluator/collector output, bundled inheritance, CI semantics, timeouts/cleanup, stale receipts, source changes during a run, identical human/JSON findings and no host fallback.

Final validation commands/results are in [verification](verification.md). Synthetic common-model example reports are [text](examples/report.txt) and [JSON](examples/report.json); the full demo also retains a real passing synthetic report in private state.

## Remaining limitations

- The real frontend cannot execute assertions under the selected fresh-snapshot commands until its generated tsconfig prerequisite is addressed through an explicit runner decision. This run establishes an honest failed baseline, not frontend readiness.
- One profile/ecosystem and a conservative complete-object CI rule. Intentional protected workflow changes fail until reviewed and re-baselined; arbitrary shell/GitHub-expression semantics and remote branch protection are not proved.
- Lockfile source/integrity **declarations**, workspace/bundle relationships and compatibility overrides only. npm verifies fetched integrity bytes. No vulnerability/license scan, built-artifact inventory or compliance claim.
- Local state and Docker are trusted by the local operator. Reports are not signatures or attestations, and fabricated local reports cannot gain authority through `status`.
- An adversarial test can fabricate report files; baseline file coverage and positive assertion counts do not prove adequate test intent. Modified tests require review.
- No successful-test cache: applicable checks rerun each time. Dependency volumes may be reused, but each test execution receives fresh writable volumes. Private source/evidence state needs operator retention management.
- Preparation has network access with scripts disabled and no credentials; it is not a registry-only network firewall. Offline execution has no network. No automatic host fallback or runtime upgrade.
- Out-of-scope file lists can be large in an existing checkout. Contents are not read, and those paths are not covered by this frontend result.

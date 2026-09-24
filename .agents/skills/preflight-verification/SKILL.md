---
name: preflight-verification
description: Use when verifying this Preflight repository, recreating its synthetic Docker/Vitest demo, running the bounded frontend pilot, or interpreting setup and stale-evidence reports. Not for general backend, deployment or application remediation.
---

# Verify Preflight

Start at the repository root. Read [AGENTS.md](../../../AGENTS.md) and the relevant section of [development](../../../docs/development.md). No global plugin, sibling checkout or old private report is required.

Choose the evidence tier the task needs:

| Task | Evidence |
| --- | --- |
| Docs only | Local links, consistency, formatting and Git diff |
| CLI/contracts/policy | Standard Go race tests, vet, real Conftest policy tests, build |
| Runner/evidence boundary | Standard checks plus explicit Docker reporter exercise |
| Complete user workflow | Also run the separate real-Vitest synthetic demo |
| Real pilot | Authorized read-only target and baseline, private reports, before/after status and same-input Conftest comparison |

The default tests do not exercise Docker. The Docker reporter exercise bootstraps the exact runtime and prints its actual image ID. Supply that ID as `PREFLIGHT_DEMO_IMAGE` to the full demo; never reuse a digest copied from a historical report without verifying local availability. Select Conftest through `PREFLIGHT_CONFTEST` or PATH; a missing/incompatible prerequisite is a reported failure, not a skipped success or permission to substitute versions.

Interpret results by command and evidence. `init` currently prints an explanation: `deferred/explain_only` is unperformed work, not a failed project check. `current: true` describes report coverage, not success. A current failed report remains failed. Preserve known failures alongside missing/error findings and report the precedence-derived exit code.

The historical real pilot attempted eight EER suites but executed zero assertions because generated Nuxt configuration was absent. Preserve this distinction. A request to verify does not authorize importing host `.nuxt`, changing the application, enabling scripts during downloads or adding runner commands. See [roadmap](../../../docs/roadmap.md) for proposed remediation.

Return a concise record: commands and actual outcomes; selected evaluator/runtime identities; synthetic versus real evidence; report locations; source-preservation check for a real run; and concrete unexecuted prerequisites. Append dated results without rewriting previous failures. Keep private source/logs outside this repo and leave commits/publication to the current user's scope.

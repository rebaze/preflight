# Preflight listing candidate

Version: 0.1.0-rc.3. Publisher: rebaze. Category: Developer Tools. License: Apache-2.0. This is prepared listing material, not a submitted or approved marketplace listing.

**Display name:** Preflight

**Short description:** Know the expectations before changing code.

**Description:** A small, focused coding companion that uses your intended change to explain relevant project expectations, what existing evidence covers, and a useful next action. Read-only discovery works across repository layouts without project setup. Optional GitHub inspection distinguishes configured CI, enforced gates and revision-bound results. Compare observations after editing to see changed requirements and stale evidence. Optional focused investigations stay bounded and their conclusions remain unverified hypotheses.

**Starter prompts:**

- Use $preflight before I change this code.
- Use $preflight before I hand off this change.

**Prerequisites:** Codex with plugin support (tested 0.156.1); a deliberately installed compatible Preflight CLI reporting skill protocol1 and discovery v1; Git for revision context. Optional GitHub mode uses existing authenticated gh access. The optional packet helper uses Python3; basic discovery does not require Python, Docker, Conftest, application dependency installation or a new model API key.

**Coverage:** Local instructions/contribution documents/manifests/workflows within documented limits; Git revision/index/worktree/comparison identities; explicit GitHub effective rules and legacy protection; check runs/statuses and producer/SHA matching; private before/after observations; limited literal structural CI explanations. Repository code execution remains limited to the separate explicitly configured frontend/Vitest workflow and approved Docker isolation.

**Limits:** No universal readiness verdict, merge/release authorization, arbitrary test runner, general YAML/shell interpretation, automatic remediation, received-review/bypass evaluation, telemetry or separate hosted service. Permission gaps, unsupported semantics and absent/stale evidence remain visible. Model explanations do not become executed checks.

**Privacy:** The plugin adds no backend or credential store. Source observations stay local/private; the user's selected harness processes its normal conversation context under that harness's settings. Optional GitHub reads use the user's existing CLI authentication. Keep source content and raw logs outside public artifacts. No project data is collected by a Preflight measurement service.

**Support:** [GitHub issues](https://github.com/rebaze/preflight/issues). [Installation](plugin.md), [data and discovery contract](discovery.md), [release verification](releases.md), [evaluation evidence](../evaluation/README.md).

**Owner publication checklist:** Review/merge the five staged PRs, choose the CLI version tag and published artifact set, complete the existing release setup, inspect immutable signed/attested release verification, then explicitly approve marketplace submission. Recheck current official format/portal requirements at submission time. Human usability validation and long-term support policy remain separate owner work; current measurements are agent evaluations.

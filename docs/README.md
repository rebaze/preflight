# Project context

Everything needed to maintain the project is in this repository. Machine-local paths in dated reports identify prior runs; they are not required files in a fresh checkout.

| Read | Purpose |
| --- | --- |
| [Project README](../README.md) | What Preflight does, command usage and current limitations |
| [Agent instructions](../AGENTS.md) | Working boundaries and file routing |
| [Design](design.md) | Product intent, decisions, research and original six-task implementation contract; section 17 resolves exploratory alternatives |
| [Architecture](architecture.md) | Current components, state lifecycle and trust boundaries |
| [Development](development.md) | Reproducible local verification without the real pilot checkout |
| [Release operations](releases.md) | GitHub CI, packaging, signing, Homebrew setup and first-release prerequisites |
| [Release design](release-design.md) / [implementation plan](release-plan.md) | Scope and decisions for the GitHub release automation |
| [Roadmap](roadmap.md) | Prioritized next work, acceptance criteria and unresolved product decisions |
| [Pilot results](pilot-results.md) | Actual 2026-09-23 failed real baseline and direct-Conftest/Pi comparison |
| [Verification record](verification.md) | Dated executed checks; new runs append their own results |
| [Learnings](learnings.md) | Append-only implementation and UX lessons |
| [Original implementation progress](implementation-progress.md) | Six completed tasks and original session constraints |
| [Synthetic demo](task-5-demo.md) | Executed real-Vitest scenarios; use development instructions to recreate |
| [Example JSON](examples/report.json) / [text](examples/report.txt) | Synthetic renderings of one common report |

`task-1-results.md`, `task-3-results.md` and `task-4-results.md` retain targeted implementation evidence. They are historical records, not installation guides. A working checking tool does not imply that the inspected application passes its tests.

## Coding-harness companion

- [Skill-first onboarding](../README.md#install-and-invoke-the-skill)
- [Installable Codex plugin and compatibility](plugin.md)
- [Discovery, GitHub and comparison contract](discovery.md)
- [Synthetic and actual harness evaluations](../evaluation/README.md)
- [Prepared marketplace listing](marketplace-listing.md)
- [Issue 4 implementation progress](plans/issue-4.md)
- [Exact unpublished candidate archive identities and reproduction](candidate.md)

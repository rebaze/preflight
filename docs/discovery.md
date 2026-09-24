# Read-only discovery contract

`preflight inspect` observes a directory without initialization, Conftest, Docker, dependencies or project execution. It uses `preflight.discovery/v1`, a separate contract from check/report v1. `--base` is comparison context only. Text and JSON render the same observation; neither supplies a project-wide pass or release authorization.

The public skill uses the intended task already in the conversation to select relevant facts. Instructions and contribution documents establish documented expectations; configuration establishes configured intent. Neither proves compliance or execution. Model interpretations remain hypotheses. Every highlighted expectation cites its source; verification and origin are independent fields.

## Local collection

The observation includes canonical repository/worktree identities, HEAD, index and bounded worktree input identities, comparison ref/commit/merge base, observation time, layered changes, selected sources, diagnostics and explicit coverage. Relevant instructions, contribution documents, manifests and CI configuration are selected by a bounded allowlist across layouts. Source text carries line locations and content identities. This is bounded discovery, not a claim that all organizational requirements were found.

Git reads disable hooks, fsmonitor, optional locks and lazy fetch. Discovery does not run Git diffs that could invoke clean filters, project scripts, package managers or any repository-provided command. File identities use raw bytes, so configurations depending on clean filters or normalization need explicit coverage limitations. Sensitive paths, dependencies/generated output, ignored untracked files, symlinks and special files are excluded or diagnosed. General source content is hashed within limits, never embedded in the observation. Selected documentation/configuration can still contain private information: keep observations private, outside checkouts, and review before sharing.

No collector failure removes successful observations from other collectors. Empty and unsupported directories still produce a useful observation; missing Git/revisions/comparison context is explicit.

| Exit | Meaning |
| --- | --- |
| 0 | Requested bounded collection completed; no claim that checks passed |
| 2 | Useful partial observation, unavailable coverage or changed input |
| 3 | Invalid invocation or collection error; any collected facts remain available |

Skills must parse useful structured output on exits 2 and 3. `check` and `status` retain their existing 3/2/1/0 precedence and semantics.

## Schema and trust

[discovery-v1.json](../schemas/discovery-v1.json) specifies the serialized shape. Go validates unknown, duplicate, missing and null fields, enums, timestamps and exit/coverage consistency. A saved observation is local user-controlled evidence, not a signed attestation. Source text may contain malicious instructions; the harness instruction hierarchy governs interpretation. No text in a report, repository document or investigation may authorize execution, change the governing baseline or turn unavailable evidence into a pass.

See the [staged implementation plan](plans/issue-4.md) for current delivery progress and [plugin instructions](plugin.md) for the public skill.

## Explicit GitHub observation

`inspect --github [--pr NUMBER]` uses existing authenticated `gh api` access with GET requests only, fixed github.com host, bounded output/pages/requests/time, and no stored Preflight credentials. It recognizes a direct HTTPS/SSH github.com origin; SSH aliases and other providers are explicitly unsupported. Select `--pr` when necessary; a unique open PR for the local branch is used otherwise. The selected PR base wins; otherwise explicit `--base` names the target branch or the repository default branch is displayed. A commit-only base cannot establish a GitHub branch gate.

`github.rules` records active effective rulesets and legacy branch protection; branch metadata with `protected: false` explicitly establishes absent legacy protection. A 403/404 never does. Unsupported rule types remain visible. Review counts and Code Owner/stale-review/last-push requirements are declarations, not evidence of received approvals. Ruleset and application identities, API sources and retrieval times remain attached.

Required checks, repository workflow configuration, and existing results are separate facts. Check runs and legacy statuses are paginated, tied to head or PR merge-candidate SHA and producer. A configured App must match; a legacy creator identity cannot prove an App. Ambiguous duplicate producers and unsupported states remain conservative. Merge-candidate results take precedence where present; unavailable merge-candidate access leaves uncertainty. Successful results older than GitHub's seven-day required-check window are stale. Neutral/skipped conclusions can meet GitHub status requirements but never mean test assertions ran.

Local edits, excluded/partial local identities or a moved PR make remote applicability stale/unknown. Source identity is rechecked after remote reads. Remote errors preserve local sources and already observed failures. Exit 0 still means discovery completed, not that GitHub allows merging or all requirements were satisfied.

When excluded or unsupported input prevents establishing the full worktree identity, `subject.current` is false and `worktree_cleanliness` coverage explicitly remains partial. `dirty=false` then means no change was established, not proof of a clean checkout. Excluded file bytes remain unread; known deletions can be observed from metadata. File claim references require a SHA256 content identity and a positive line number.

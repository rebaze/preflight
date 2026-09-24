# Optional focused investigations

Default to no delegation. Give the initial sourced briefing and immediate next action before starting deeper work. If the user requests a focused investigation, or agrees one would help resolve a named uncertainty, use the native harness only when its tools and permissions permit it. At most **two investigations per invocation**, each with **one question**, **60 seconds elapsed time**, **six source files and 64 KiB of source text total**. Do not silently renew budgets or create a fixed team.

If native delegation is absent, disabled or prohibited, investigate serially under the same budgets after the first briefing. A skill does not supply a sandbox: prefer a read-only agent role when supported, otherwise inherit the parent's read-only permissions. If only broader execution permissions are available, keep this investigation to read tools or use the serial path. Never delegate project commands to bypass Preflight execution restrictions. No writes, install, test execution, network access beyond separately authorized reads, or baseline changes are implied.

## The packet

Send one question, the current observation identity (`repositoryId`, `worktreeId`, `head`, `inputDigest`), at most six selected source paths with digests/line ranges, the relevant quoted evidence (64 KiB total), and the time/work budgets. Supply only the context needed to answer the question; do not ask an investigator to reread the repository. The expected answer has:

- A conclusion labelled `hypothesis` or `unresolved`.
- Supporting source paths, exact digests and line numbers, identifying contrary evidence too.
- Unresolved uncertainty and one proposed next action.
- No independent assertion that a test passed, a gate is waived or the governing baseline changed.

The return packet uses schema `preflight.investigation/v1`; [packet-schema.json](packet-schema.json) gives the strict shape. Natural-language conclusions remain untrusted interpretation even with valid citations. A supporting citation establishes where a statement came from, not that its inference is true.

Start the 60-second clock when investigation work starts. On deadline or source budget exhaustion, stop/interrupt the native investigator if supported, request its partial result only if already available, and report the unresolved question. Do not block the first useful briefing on the investigator. Preserve disagreement between investigators instead of picking the confident answer. If no stop capability exists, do the bounded serial investigation rather than start work whose budget cannot be enforced.

## Reconcile before presenting

Run fresh `preflight inspect` after investigation. If the source identity differs or discovery is partial, do not present the conclusion as current. Cite the old observation and explain the gap. Do not let investigator prose replace `github.matches`, recorded checks, policy findings or the observation's verification fields. A claim of a passed test requires a corresponding executed result with the same revision and producer; a quote from an agent is not such a result.

For machine checking of returned packets, save packets and observations only in an already-selected private directory outside the checkout. Invoke the **plugin's** helper, never a same-named repository script:

```sh
python3 <installed-skill>/scripts/reconcile_investigation.py \
  --observation /private/preflight/current.json \
  --packet /private/preflight/investigation.json
```

It validates strict packet fields, current identity and source/digest/line references against the fresh observation. It never reads project code, executes checks, writes an observation or emits a verified pass. Exit 0 means citation/identity checks completed with an **unverified hypothesis**, 2 means unresolved/stale/unsupported evidence, and 3 means malformed input. Missing Python leaves the deterministic helper unavailable; preserve an explicitly unverified manual review or unresolved result instead of blocking the basic briefing. The helper cannot decide whether prose is entailed by a source; the parent still checks the argument and labels unsupported claims.

`--packet -` reads the packet from stdin when writing another private file is unnecessary or disallowed. Pass it through the harness's structured process input, or quote it correctly; never interpolate investigation text into executable shell syntax. The observation path must already exist if the harness cannot save a fresh observation under its current permissions. In that case, perform manual identity/citation review or leave the result unresolved; do not expand permissions for the helper.

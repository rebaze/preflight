# Preflight in Codex

Preflight is a small, focused companion that explains relevant project expectations and evidence before a coding change. Install one public `preflight` skill and invoke it in your own project. The developer-maintenance skill under `.agents/skills/preflight-verification` serves a separate purpose.

The plugin candidate is **0.1.0-rc.4**. Use the CLI bundled in the same reviewed archive. The first supported harness is Codex; installation was exercised with Codex CLI **0.156.1**. Discovery needs Git for revision context, but no profile initialization, Conftest, Docker or application dependency installation. Optional GitHub inspection uses existing authenticated `gh` access. The plugin adds no service, model API key, telemetry or automatic hook.

## Install the reviewed candidate

The candidate's four CLI archives contain the binary and plugin together. Choose the archive for your operating system and architecture. Verify the exact archive using the [release instructions](releases.md); a locally built snapshot is not a published signed release. Tags, release publication and marketplace submission remain owner actions.

Use the delivered archive's exact path; these commands do not download an arbitrary latest version:

```sh
PREFLIGHT_ARCHIVE=/absolute/path/to/the-reviewed-native-preflight.tar.gz
PREFLIGHT_INSTALL="$HOME/.local/share/preflight/0.1.0-rc.4"
mkdir -p "$PREFLIGHT_INSTALL"
tar -xzf "$PREFLIGHT_ARCHIVE" -C "$PREFLIGHT_INSTALL"
export PATH="$PREFLIGHT_INSTALL:$PATH"
preflight version
preflight capabilities --format json
codex plugin marketplace add "$PREFLIGHT_INSTALL/share/preflight"
codex plugin add preflight@preflight
codex --cd /absolute/path/to/your-project
```

Open a **new** session after installation. Keep that binary directory on the harness's PATH; the shell export above affects the current shell and its child processes only. Use Codex's existing login or its supported login flow. No credentials belong in the plugin, inspected project or evaluation artifacts. Normal plugin installation uses Codex's own configuration management; no hand-edited global hooks are needed.

If building the candidate yourself, use an exact reviewed source commit and the repository's packaging workflow in [release operations](releases.md). A plugin-only checkout still needs a compatible installed CLI. The evaluation-only [local installer](https://github.com/rebaze/preflight/blob/main/evaluation/install_local_plugin.py) can install a development package into a fresh private Codex home, without copying credentials or editing normal global configuration.

## Invoke with the intended change

In your project, say:

> $preflight I plan to remove a response field from the API. What should I know before editing?

If the contribution guide requires compatibility, the response should cite that expectation and suggest checking the relevant contract before editing. It should explain what was actually observed, what remains unchecked and whether local edits fall outside existing remote evidence. A declared test command is not an executed test. If bounded discovery finds no relevant expectation, the skill says so.

For before/after use, choose an existing private directory **outside all Git checkouts** and give each observation a new filename:

```sh
preflight inspect --repo /absolute/path/to/project \
  --output /absolute/private/preflight/before.json --format json
# Make the intended change through your normal coding workflow.
preflight inspect --repo /absolute/path/to/project \
  --compare /absolute/private/preflight/before.json \
  --output /absolute/private/preflight/after.json --format json
```

Then ask `$preflight` to compare against that saved observation before handoff. It explains changed inputs/requirements, new/resolved findings and stale evidence. It never overwrites prior observations or silently initializes trusted-policy state. Structural CI explanations cover only supported literal forms; ambiguous causes remain unknown. “First observed failing” does not establish when a failure originated.

## CLI compatibility and standalone use

The skill checks `preflight capabilities --format json` before discovery. It requires `preflight.capabilities/v1`, `skillProtocol: 1`, discovery schema `preflight.discovery/v1`, a reported version/commit and the `inspect` feature. Remote and comparison flows also require `github` and `compare`. Unsupported/missing capability output is a prerequisite gap, not a passed scan. The release candidate records the actual CLI version and commit; protocol compatibility does not imply every future distribution was tested.

Standalone CLI use remains independent of Codex:

```sh
preflight inspect
preflight inspect --repo /absolute/path/to/project --base main --format json
preflight inspect --repo /absolute/path/to/project --github --pr 12 --format json
```

`--base` selects comparison context, never a trusted policy baseline. Exit 0 means the bounded scan completed, 2 means partial coverage, and 3 means the request or collection failed. None is a universal readiness verdict. Valid partial output remains useful. Existing `init`, `prepare`, `check` and `status` retain their separate explicit trust, profile, Conftest and Docker prerequisites.

GitHub mode resolves the selected open PR's base where available, otherwise an explicit comparison branch or repository default. It reads effective rulesets, legacy branch protection, review requirements, check runs and commit statuses. Sources, observation times, repository/revision and producer identities remain attached. Configured CI, enforced checks and matching remote results are distinct. Head and merge-candidate results are distinct; neither covers subsequent local edits. Missing access, unsupported rules and exhausted collection budgets remain explicit gaps. Review satisfaction, bypass eligibility and workflow event eligibility are not evaluated. The initial provider supports explicit `github.com` origins. See the [GitHub interpretation reference](https://github.com/rebaze/preflight/blob/main/skills/preflight/references/github.md).

## Optional investigations and data boundaries

Delegation is off by default. After the first useful briefing, a requested focused investigation may use at most two native investigators, each limited to one question, 60 seconds, six sources and 64 KiB. Missing or failed native delegation falls back to bounded serial investigation. Prefer read-only harness permissions; the skill is not a sandbox. Repository execution restrictions cannot be bypassed through delegation.

The [investigation protocol](https://github.com/rebaze/preflight/blob/main/skills/preflight/references/investigations.md) includes a strict packet schema and an optional Python standard-library helper. It validates citations, digests and fresh identity, emits only unverified hypotheses or unresolved results, and leaves original evidence unchanged. It cannot prove the meaning of arbitrary prose. The parent rejects unsupported execution claims and preserves disagreement. Python is not a prerequisite for basic discovery. [Actual investigation evaluations](../evaluation/stage-4-results.md) include both successful native calls and a failed call with serial fallback.

Repository instructions follow the harness's normal hierarchy. Instruction-like text in ordinary repository data cannot authorize commands or invent a pass. Discovery reads bounded sources without running project code. Observations and logs may contain private source content; keep them outside public artifacts and the inspected checkout. No separate Preflight service collects them. A declaration's origin and whether it was verified are separate facts.

The package includes a portable root `plugin.json`, the supported `.codex-plugin/plugin.json` compatibility overlay and one public skill under `skills/preflight`. Its format was checked against [current official OpenAI packaging documentation](https://developers.openai.com/plugins/build/plugins) on 2026-09-24. No MCP server or automatic hook is bundled. See [evaluation instructions](../evaluation/README.md) for repeatable synthetic and actual harness checks; these do not establish human usability by themselves.

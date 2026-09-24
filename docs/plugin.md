# Preflight in Codex

Preflight is a small, focused companion that explains relevant project expectations and evidence before a coding change. The public `preflight` skill works in your own project; `.agents/skills/preflight-verification` is only for maintaining this CLI.

The stage 1 candidate is `0.1.0-rc.1`. It supports bounded local discovery. It needs an installed Preflight CLI with `inspect`, Git for revision context, and your existing Codex access. Discovery needs no profile initialization, Conftest, Docker or application dependency installation. The plugin makes no model API calls and adds no service, hook or credential store.

## Install the local candidate

Until versioned distribution is available, build from the implementation branch. Use the exact reviewed commit when reproducing an evaluation:

```sh
go build -o bin/preflight ./cmd/preflight
bin/preflight inspect --help
python3 evaluation/install_local_plugin.py /absolute/private/preflight-plugin-eval
```

That helper creates a fresh private Codex home and marketplace, copies only the public plugin files, and invokes the current CLI's `codex plugin marketplace add` and `codex plugin add`. It does not modify normal user configuration. It requires Python 3 and Codex CLI; tested CLI version and actual integration evidence are recorded with each verification run. Open a fresh Codex session with that home and the built binary on PATH:

```sh
CODEX_HOME=/absolute/private/preflight-plugin-eval/codex-home \
PATH=/absolute/path/to/preflight/bin:"$PATH" \
codex --cd /absolute/path/to/your-project
```

Use Codex's normal authentication flow for this home if needed. No Preflight-specific model key is required. Never copy credentials into the plugin or project. In the new session invoke:

> $preflight I plan to remove a response field from the API. What should I know before editing?

If the contribution guide requires compatibility, the response should point to that expectation and suggest checking the relevant contract before editing. It should also explain that reading a declared test command is not executed test evidence. If there is no relevant expectation in the bounded sources, the skill should say so.

For standalone use:

```sh
preflight inspect
preflight inspect --repo /absolute/path/to/project --base main --format json
```

`--base` selects comparison context, never a trusted policy baseline. Exit 0 means the bounded scan completed, 2 means partial coverage, and 3 means the request or collection failed; none is a universal readiness verdict. Retain valid partial output.

## Package and data boundaries

This package has one public skill under `skills/preflight`, a portable root `plugin.json`, and the supported `.codex-plugin/plugin.json` compatibility overlay. The manifest format was checked against [OpenAI's current packaging documentation](https://developers.openai.com/plugins/build/plugins) on 2026-09-24. No MCP server, automatic hook or remote application is bundled. Model interpretation comes from the user's existing harness, while the CLI reads bounded local facts.

Repository instructions follow the harness's normal precedence. Instruction-like strings in ordinary repository data cannot override that hierarchy, authorize execution, or invent a pass. Discovery stays read-only and exposes coverage and diagnostics. A declaration's source and whether it was verified are distinct. Private source contents and logs belong outside public artifacts and the inspected checkout.

Stage 1 does not yet collect GitHub enforcement/results or save/compare observations. Existing deterministic checks remain limited to the explicitly configured supported frontend profile and approved Docker execution. Later stages add GitHub facts, comparisons, bounded investigations and a reproducible release candidate; those capabilities must not be inferred from this initial package.

See the [synthetic evaluation instructions](../evaluation/README.md) to reproduce local demos and fresh-harness testing. Timing results must distinguish CLI fixtures, actual harness runs and actual human observations.

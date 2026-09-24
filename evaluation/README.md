# Synthetic skill evaluation

These fixtures contain only invented project data. The public skill is evaluated separately from deterministic CLI tests. Agent observations do not establish human usability.

Create new disposable directories outside the inspected projects:

```sh
python3 evaluation/create_fixtures.py /absolute/private/preflight-fixtures
python3 evaluation/install_local_plugin.py /absolute/private/preflight-plugin-eval
```

The first command creates a Go service, a JavaScript workspace under `clients/browser`, an empty Git repository and `scenarios.json` containing prompts and expected claims. The JavaScript tree has tracked and untracked edits. Hooks and package scripts have harmless tripwires; any `HOOK_EXECUTED` or `SCRIPT_EXECUTED` file is a failed no-execution boundary.

The second command copies only the plugin manifest, public skill and license to a private marketplace, then installs it into a fresh `CODEX_HOME`. It uses `codex plugin add`, as available in Codex CLI 0.156.1. It neither copies credentials nor edits the normal Codex configuration. Installing a plugin does not prove the skill ran.

Put the built CLI on PATH explicitly. Authenticate the fresh Codex home using the harness's supported login flow; if the platform provides an existing authenticated keychain session for that home, it may be reused by the harness. Do not print, export or copy tokens to the fixture, plugin or repository. A missing session is an explicit integration blocker, not permission to clone the user's credential store.

Run each scenario in a **new** session, in read-only mode. `--json` events and the final response go to private files outside the fixture. For example:

```sh
CODEX_HOME=/absolute/private/preflight-plugin-eval/codex-home \
PATH=/absolute/path/to/preflight/bin:"$PATH" \
codex exec --sandbox read-only --ephemeral \
  --cd /absolute/private/preflight-fixtures/go-service \
  --json --output-last-message /absolute/private/go-briefing.txt \
  '$preflight I plan to remove Label from the item API response. What should I know before editing?' \
  > /absolute/private/go-events.jsonl
```

Record elapsed monotonic time from invocation to the first relevant, sourced expectation **and next action**, not just the first token. Save command versions, model/harness identity, whether the public skill loaded, claims/sources, unchecked coverage and final action. An empty repository passes honesty criteria without being counted as an interesting finding. Record all misses and failures. Measure installation-to-first-result separately, including authentication. Later stage evaluations extend these fixtures to partial access, stale evidence, conflicts, unsupported checks and hostile text.

`run_skill.py` automates fresh sessions, timestamped messages, private logs and before/after file digests, with a 120-second process limit per scenario. It does not judge source relevance from keywords; review the actual messages. If a fresh Codex home is unauthenticated, `--reuse-current-auth` reuses the normal harness's existing authentication without reading or copying credentials. It copies only the public plugin bundle into a unique cache namespace and uses per-command configuration, `--ignore-user-config`, read-only permissions and ephemeral sessions. Normal global configuration is not edited. That mode is a fresh **session**, not a fresh authenticated home:

```sh
python3 evaluation/run_skill.py \
  --fixtures /absolute/private/preflight-fixtures \
  --installation /absolute/private/preflight-plugin-eval \
  --cli /absolute/path/to/preflight/bin/preflight \
  --output /absolute/private/preflight-run-1 \
  --reuse-current-auth
```

Use `--model` when pinning a model for repeatable measurements; otherwise the result explicitly records the harness default as unspecified. Omit `--reuse-current-auth` to use the isolated home after normal supported login. A mismatched existing public cache is refused; use a fresh version instead of silently replacing it.

Before and after each run compare `git status --porcelain=v1 -uall`, `git rev-parse HEAD`, and a digest of every fixture file, including ignored files. The inspected project must not change. Confirm the tripwires did not appear. Review that the briefing cites `CONTRIBUTING.md:3` for the Go and JS scenarios, says tests were not executed, notices dirty JS input, and does not claim GitHub enforcement from `ci.yml` alone.

Run deterministic local inspection independently:

```sh
preflight inspect --repo /absolute/private/preflight-fixtures/go-service --format json
preflight inspect --repo /absolute/private/preflight-fixtures/js-workspace --format text
preflight inspect --repo /absolute/private/preflight-fixtures/empty --format json
```

An empty or unsupported project may return useful partial output with exit 2. Do not discard it because a shell pipeline expected success.

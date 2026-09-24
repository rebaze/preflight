# Development and reproducible verification

Run commands from the repository root. The synthetic workflow needs no external application checkout, sibling repository, retained report or prior chat. Historical absolute paths in dated reports are evidence of earlier runs, not prerequisites.

## Tools and local selection

- Go 1.27.1 (see `go.mod`), Git, and external Conftest for the normal suite. The race detector also needs a supported host, CGO enabled and a working C compiler (for example the host clang/gcc toolchain); a minimal Go-only Linux image is insufficient.
- A Linux Docker daemon for explicit container exercises. The original host was macOS/arm64 with Docker 29.8.0; other hosts are not yet a verified support matrix.
- Node 24.18.0 and npm 11.17.0 run in Docker, not on the host. The real-Vitest fixture requests Vitest 4.1.10.

The originally verified evaluator reported `Conftest: dev`, OPA 1.20.2; the macOS executable digest is retained in the pilot record. There is no portable claim that another architecture's binary has the same digest. Tests select `PREFLIGHT_CONFTEST` when supplied, otherwise `conftest` on PATH, and compute the selected executable's actual digest for each synthetic trust fixture. Missing or incompatible tools fail visibly. This test selection does not update any existing real state or its evaluator pin.

```sh
go version
go env CGO_ENABLED CC
# CGO_ENABLED must be 1 and the reported C compiler must be available for -race.
git --version
conftest --version
# If using an explicit non-PATH executable instead:
# export PREFLIGHT_CONFTEST=/absolute/path/to/conftest
```

Use the same chosen binary for manual policy tests. Verify its provenance through your normal tool-installation process before explicitly trusting it. The repository does not install a replacement Conftest, upgrade runtimes or fetch project credentials.

## Standard checks (no Docker or dependency downloads)

```sh
go test -race ./...
go vet ./...
conftest verify --policy policy
go build -o bin/preflight ./cmd/preflight
bin/preflight --help
```

If `PREFLIGHT_CONFTEST` is set, substitute `"$PREFLIGHT_CONFTEST" verify --policy policy` for the manual `conftest` command. These tests invoke real Conftest plus controlled fake subprocesses and create temporary synthetic Git repositories. They do not execute the real pilot. Go may populate its normal build cache; the module has no external Go dependencies.

The default suite deliberately skips two opt-in container scenarios. Passing it alone is not a claim that Docker isolation or real Vitest has been exercised on the current host.

## Actual container evidence exercise and runtime bootstrap

This explicit command permits the bounded synthetic dependency bootstrap/downloads and Docker execution:

```sh
PREFLIGHT_DOCKER_TESTS=1 go test ./internal/preflight \
  -run '^TestDockerSyntheticReports$' -v -count=1
```

It resolves the official Node version to an immutable digest, builds npm 11.17.0 into the runner, and prints a line `runtime image=sha256:... base=... architecture=...`. Record those actual identities. It runs three fresh containers and verifies pass, failed-report and zero-exit/missing-report states. These are generated reporter fixtures, not actual Vitest assertions.

If the exact runtime or Docker is unavailable, preserve the concrete prerequisite failure. Do not use host Node, change versions, copy a prior host's image ID or claim the scenario passed.

## Full CLI workflow with real Vitest

After the previous step succeeds, set the image ID from **that run's output**, then invoke the separate demo. Replace the example value below; it is intentionally not an old machine's digest.

```sh
export PREFLIGHT_DEMO_IMAGE='sha256:REPLACE_WITH_RUNTIME_IMAGE_FROM_PREVIOUS_STEP'
PREFLIGHT_SYNTHETIC_DEMO=1 go test ./integration \
  -run '^TestSyntheticCLIWorkflow$' -v -count=1
```

The demo requires that explicit local immutable image ID and an available Conftest. The supplied image is used for initial synthetic lockfile generation; the CLI's subsequent `prepare` independently resolves/builds and records its own pinned runtime using the approved procedure. Compare recorded identities; do not assume identical image IDs across hosts or builds. It does not fall back to the original author's image/path. It builds the CLI, generates only synthetic manifests/source, creates a lockfile with lifecycle scripts disabled, initializes a disposable Git baseline, and runs init/explain/prepare/check/status. It exercises real offline Vitest success, assertion failure, restoration, lock-source violation, required CI deletion, ineffective candidate-policy changes, stale status and renewed evidence. A missing compatible preparation receipt and a known policy violation can coexist; exit 2 correctly takes precedence over exit 1.

On successful completion, the retained directory printed by the test contains `commands.json`, common text/JSON reports and private run material. An interrupted or failed demo can retain individual reports without the final command transcript; preserve its terminal output too. Synthetic fixture commits do not commit anything in this repository. Dependency volumes and the runner image are local reusable state; clean only explicitly identified tool-owned resources when no longer needed. Never globally prune Docker or delete somebody else's state.

## Real pilot work

Read the target's current AGENTS.md, record Git status, and explicitly select an available baseline/evaluator. Initialize fresh state outside both repositories using the [README workflow](../README.md#workflow). Never auto-trust the target's current HEAD because the recorded pilot SHA is unavailable. A different pilot/profile is a scoped design task; v1 is not a generic profile/plugin framework.

Run explain, prepare, check --all, status; retain actual argv, timings, identities and reports privately. Compare direct Conftest on the same normalized inputs, and compare original Git status afterwards. Read-only inspection of the Pi planner/executor is permitted in the original pilot scope; its full hooks run broader checks and are not part of this verification.

The recorded missing `.nuxt/tsconfig.json` is a real prerequisite failure under the selected runner. Eight suite-loading failures with zero assertions are not eight executed tests. Preserve that report; [roadmap R2](roadmap.md#r2--resolve-the-frontends-generated-config-prerequisite-explicitly) describes the required investigation before changing the runner. Do not import host-generated files, enable lifecycle scripts or weaken evidence to create a green result.

## Changes and handoff

GitHub CI repeats the standard checks on Linux and macOS and validates four-platform release archives. Its Conftest 0.70.1 download uses explicit platform-specific SHA256 pins. `make check` also tests the offline release helpers; `make packaging` validates workflows and inspects real snapshot archives; `make security` runs the separately pinned govulncheck tool against the CLI and scanner. See [release operations](releases.md). These checks do not run Docker unless the separate synthetic workflow is dispatched.

Use focused tests while editing; complete the standard suite for code changes. Runner/snapshot changes also need relevant actual-container evidence; full workflow changes need the real-Vitest scenario. Match verification to the changed boundary rather than rerunning the real pilot for documentation edits. Check doc links, formatting and Git diff for documentation-only changes.

Keep `runtime/` and compiled runner constants synchronized. Keep schemas, strict decoding, CLI and renderer changes consistent. Preserve fail/error coexistence and the expected-control evaluator probe. Append a dated entry to [verification](verification.md) and [learnings](learnings.md) with actual evidence; list unexecuted checks explicitly.

Before committing, review `git status --short --untracked-files=all`, ignored paths and intended file contents. Include synthetic source/examples only. The owner selected GitHub, the public module path and CI/release preparation on 2026-09-24. License selection and actual version tags/releases remain separate decisions. Nothing in these instructions authorizes installing hooks.

## Neutral example profile

The bundled profile is `profiles/frontend-vitest.json`, and the fixture workspace is `@example/frontend` at `applications/frontend/apps/web`. Runtime scripts and compiled definitions use the same names. Existing state using earlier identifiers is retained as historical evidence; initialize a new state directory explicitly for the current profile and run preparation again. No real application checkout is renamed or edited by these changes.

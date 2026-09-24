# rebaze Preflight

[![CI](https://github.com/rebaze/preflight/actions/workflows/ci.yaml/badge.svg)](https://github.com/rebaze/preflight/actions/workflows/ci.yaml)
[![Release](https://github.com/rebaze/preflight/actions/workflows/release.yaml/badge.svg)](https://github.com/rebaze/preflight/actions/workflows/release.yaml)
[![GitHub Release](https://img.shields.io/github/v/release/rebaze/preflight?include_prereleases)](https://github.com/rebaze/preflight/releases)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/rebaze/preflight)](https://github.com/rebaze/preflight/blob/main/go.mod)
[![CodeQL](https://github.com/rebaze/preflight/actions/workflows/codeql.yaml/badge.svg)](https://github.com/rebaze/preflight/actions/workflows/codeql.yaml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/rebaze/preflight/badge)](https://scorecard.dev/viewer/?uri=github.com/rebaze/preflight)

Preflight is a small, focused companion for coding changes. Invoke its Codex skill with your intended change to learn which sourced expectations matter, what the evidence covers, and the next useful action. The independently usable Go CLI gathers bounded facts and performs supported deterministic checks. Results are local feedback, never release authorization.

Try the [guided tester page](docs/try-preflight.html) for installation, a real-change briefing, before/after comparison and optional local feedback. It runs locally and does not execute commands or send project data.

## Install and invoke the skill

The plugin candidate is **0.1.0-rc.4**, requiring CLI skill protocol **1** and discovery schema **preflight.discovery/v1**. The first public release remains pending. Use the supplied candidate archive, or reproduce one from an exact reviewed commit using [candidate record](docs/candidate.md) and [installation instructions](docs/plugin.md). No arbitrary latest download or global hook is installed.

After extracting the matching macOS/Linux archive into a durable directory:

```sh
PREFLIGHT_PACKAGE=/absolute/path/to/extracted/preflight
export PATH="$PREFLIGHT_PACKAGE:$PATH"
preflight capabilities --format json
codex plugin marketplace add "$PREFLIGHT_PACKAGE/share/preflight"
codex plugin add preflight@preflight
codex --cd /absolute/path/to/your-project
```

In the fresh Codex session:

> $preflight I plan to remove a response field. What matters before editing?

The skill uses conversation intent and current instructions. In the bundled synthetic Go example it points to the compatibility requirement at `CONTRIBUTING.md:3`, explains that tests have not run, and suggests checking the response contract before removing the field. In an empty repository it reports the limited evidence honestly. It does not invent a gate to make the result interesting.

Local discovery needs no initialization, Conftest, Docker or application dependencies. Git supplies revision context; without it, useful partial document discovery remains available. Explicit GitHub inspection uses existing authenticated `gh` access. See [supported coverage and data boundaries](docs/discovery.md), [actual agent evaluations](evaluation/README.md) and [the public skill](https://github.com/rebaze/preflight/blob/main/skills/preflight/SKILL.md).

## Recheck after editing

Choose an existing private parent outside all Git checkouts; each saved observation must be a new file:

```sh
PREFLIGHT_OBSERVATIONS=$(mktemp -d)
preflight inspect --repo /path/to/project --output "$PREFLIGHT_OBSERVATIONS/before.json"
# After the intended edit:
preflight inspect --repo /path/to/project --compare "$PREFLIGHT_OBSERVATIONS/before.json"
```

Or give that observation path to `$preflight` before handoff. It explains changed inputs/requirements, new or resolved findings, stale evidence and supported structural CI changes. A failure without comparable evidence keeps an unknown cause.

## Standalone discovery

```sh
preflight inspect                         # current directory
preflight inspect --base main --format json
preflight inspect --github --pr 123 --format json
```

`--base` changes comparison context, never trusted policy. GitHub checks actually required, CI configured in files and existing remote results are separate facts. Remote results do not cover local edits. Discovery exit 0 means the requested bounded scan completed, 2 means useful partial coverage and 3 means an invocation/collection error; none means tests passed. The skill consumes useful partial results.

## Current scope

Preflight checks required test evidence, npm dependency declarations, and protected CI configuration. It includes a fixed frontend/Vitest example profile and a complete synthetic workflow. The profile defines an npm workspace layout; adapting another layout requires an explicit profile and runner change.

Start with the [documentation map](docs/README.md), [design and intent](docs/design.md), [architecture](docs/architecture.md), and [prioritized roadmap](docs/roadmap.md). The near-term delivery sequence is [issue #4](https://github.com/rebaze/preflight/issues/4): local discovery, GitHub gates, before/after observations, bounded investigations and measured plugin onboarding.

## Build and bounded-check prerequisites

For development, build from an exact reviewed checkout (the first public release remains pending):

```sh
go build -o bin/preflight ./cmd/preflight
bin/preflight --help
bin/preflight version
```

Release automation builds macOS and Linux archives for amd64 and arm64, containing the CLI and version-matched policy/profile files. After the first stable release and tap setup, Homebrew installation will be:

```sh
brew install rebaze/tap/preflight
preflight version
```

Homebrew installs supporting files under `$(brew --prefix)/share/preflight`; release archives contain them under `share/preflight`. Use those paths for `init --profile` and `--policy-dir` when running outside a source checkout. Select and trust them explicitly; installing or upgrading the tool does not update existing initialized policy state. See [release setup and operations](docs/releases.md) for remaining setup, artifact verification and publishing instructions.

Building requires Go 1.27.1. Local discovery uses Git when available and remains partial without Git context. The initialized checking workflow requires Git and a local Conftest executable; preparation and tests also require Docker with a Linux daemon. The supported runtime is exactly Node 24.18.0 and npm 11.17.0. Runtime versions are never automatically substituted. Static evaluation and unit tests do not require Docker. The race-test command also requires CGO and a working C compiler; see [development prerequisites](docs/development.md#tools-and-local-selection).

The profile and policy are local inputs explicitly trusted at initialization. The tested evaluator is `/opt/homebrew/Cellar/conftest/0.70.1/bin/conftest`, reporting `Conftest: dev` and `OPA: 1.20.2`, SHA256 `b2f75ccf2575da4543ecec646194ae2f5a476fdf810681b04d01042b8e73ff18`. This is an observed binary pin, not an assertion about its release provenance.

## Workflow

Choose a fresh private state path **outside** the source Git checkout. Initialization refuses existing content. This explicit command trusts the selected commit, supplied profile/rules and evaluator file; it is not an automatic trust update from the candidate branch.

Set these values deliberately. The repository path and baseline are local choices; do not select a newer baseline automatically. The bundled `frontend-vitest` profile expects the `@example/frontend` workspace under `applications/frontend/apps/web`. A different layout needs explicitly reviewed profile and runner changes.

```sh
PREFLIGHT_REPO=/absolute/path/to/project
PREFLIGHT_BASELINE=the-explicitly-selected-commit-sha
PREFLIGHT_STATE=/absolute/private/path/to/fresh-preflight-state
PREFLIGHT_EVALUATOR=/absolute/path/to/conftest

bin/preflight init \
  --repo "$PREFLIGHT_REPO" --baseline "$PREFLIGHT_BASELINE" \
  --profile profiles/frontend-vitest.json --policy-dir policy \
  --state-dir "$PREFLIGHT_STATE" --conftest "$PREFLIGHT_EVALUATOR"

bin/preflight explain --state-dir "$PREFLIGHT_STATE" --format text
bin/preflight prepare --state-dir "$PREFLIGHT_STATE" --allow-downloads
bin/preflight check --state-dir "$PREFLIGHT_STATE" --all --format json \
  --output "$PREFLIGHT_STATE/report.json"
bin/preflight status --state-dir "$PREFLIGHT_STATE" \
  --report "$PREFLIGHT_STATE/report.json" --format text
```

`init` establishes the trusted reference once; it does not run project tests or edit the project. It currently prints the full explanation report. `deferred [explain_only]` means a check has not run; existing untracked/out-of-scope paths were not created by init. `Current: true` describes the inspected input, not passed tests. Do not change the project merely to clear setup notices. A reported `error` identifies an actual setup issue.

The example profile and runner use neutral identifiers. State initialized with an earlier profile identity is not migrated automatically. Keep earlier evidence intact and explicitly initialize a fresh state directory for the current profile; its changed runner also requires fresh dependency preparation.

`explain` does not execute repository code or download dependencies. `prepare` performs an explicit, networked, manifest-only `npm ci --ignore-scripts` in Docker and records immutable runtime identities. `check` runs static controls on every invocation and executes fresh tests when frontend/CI paths changed or `--all` is supplied. Tests run offline using a fresh writable volume cloned from prepared dependencies. `--all` always means the same selected frontend profile, never the backend or the complete monorepo.

`--base REF` changes the comparison merge base only; it does not change the governing policy baseline. A changed dependency input requires preparation again. `status` compares a saved report with current source, revision, profile, policy and evaluator identities without executing checks. It does not authenticate the report's producer.

`--output` saves canonical JSON while stdout uses `--format`. JSON stdout is one object. Findings have stable control/reason identifiers, paths, evidence, owner and suggested argv arrays. Suggested actions are data and are never automatically executed.

| Exit | Meaning |
| --- | --- |
| 0 | Evaluation completed without local violations or missing evidence; review/deferred obligations can remain |
| 1 | Known violation |
| 2 | Required evidence missing, or saved report stale |
| 3 | Configuration, collector, evaluator or execution error |

Precedence is 3, 2, 1, 0; reports retain simultaneous findings. A baseline test failure is a legitimate result and is never repaired by Preflight.

## Three controls

- `eer.tests`: the complete existing EER Vitest suite plus the explicit compatibility patch and its Node test. Requires a successful process, positive fresh JUnit counts, no failures/errors/skips, and every baseline test file represented in the same run's Vitest JSON. Modified test code adds a review obligation. Changed test commands are refused.
- `npm.dependencies`: complete v3 lockfile declarations, approved HTTPS public-registry URLs and SHA512 SRI, valid declared workspace links, validated bundled-parent inheritance, and existing `brace-expansion: 5.0.9` / `js-yaml: 4.3.1` overrides and resolved versions. These are provenance declarations and compatibility constraints. They are not vulnerability, license or universal security claims.
- `ci.integrity`: semantic equality of workflow triggers, permissions and four protected job objects against the trusted baseline; every baseline gate dependency must remain present. Formatting and comments do not change this comparison. An intentional change calls for review and explicit re-baselining.

Every report defers artifact verification to the trusted build/release process and describes excluded assurance. Changed out-of-scope paths are listed without reading their contents.

## Boundary and local data

Only selected tracked and relevant non-ignored untracked frontend inputs and the CI workflow enter private snapshots. Git metadata, generated output, dependency directories, environment/credential/private-key files are excluded. Source symlinks and special files are rejected. Source `.npmrc` contributes to identity; only a generated public-registry configuration is copied. Unsupported settings require review without exposing their values.

The source checkout is read-only and is never mounted into a container. Repository code runs without network, as UID 1000, with a read-only root filesystem, dropped capabilities, no-new-privileges, 2 CPUs, 2 GiB memory and 512 PIDs. Only tool-owned volumes and temporary storage are writable. Preparation has no lifecycle scripts or credentials; its network is **not** represented as a registry-only firewall. The host Docker daemon remains part of the trusted local execution environment.

Private state contains source snapshots, raw logs, normalized evaluator inputs, reports and dependency-volume receipts. Keep it outside this project and do not commit or publish it. Limits: 20 MiB per source/report file, 256 MiB selected source, 1 MiB process logs; bootstrap 600 seconds, test execution 300 seconds. Cleanup targets only containers/volumes created by the operation. Prepared dependency volumes remain for subsequent runs.

Local users can edit private state or bypass the CLI. These checks reduce accidental self-relaxation; they are not tamperproof enforcement or trusted attestations.

## Verification and comparison

```sh
go test -race ./...
go vet ./...
conftest verify --policy policy
PREFLIGHT_DOCKER_TESTS=1 go test ./internal/preflight -run '^TestDockerSyntheticReports$' -v
go build -o bin/preflight ./cmd/preflight
```

The default tests use synthetic repositories and controlled subprocess fixtures. The explicit Docker test exercises real pass/fail/missing evidence. See [pilot results](docs/pilot-results.md) for actual real-pilot commands, results, report locations and comparison with direct Conftest and the existing Pi extension. [Synthetic text](docs/examples/report.txt) and [JSON](docs/examples/report.json) reports render one common result model.


## Develop this repository independently

Read [AGENTS.md](https://github.com/rebaze/preflight/blob/main/AGENTS.md) for agent instructions and [development](docs/development.md) for reproducible checks. The project-local [verification skill](https://github.com/rebaze/preflight/blob/main/.agents/skills/preflight-verification/SKILL.md) routes between ordinary checks, Docker reporter tests, the separate real-Vitest demo and real pilot work. It is plain Markdown; no global skill/plugin installation is required.

Normal tests require only this repository, Go, Git and Conftest. They choose `PREFLIGHT_CONFTEST` when set, otherwise Conftest on PATH. The full synthetic demo requires an explicitly supplied `PREFLIGHT_DEMO_IMAGE` from the preceding Docker bootstrap run; follow the development guide rather than copying an old machine's image digest. Both synthetic scenarios are self-contained.

The Go module is `github.com/rebaze/preflight`. GitHub Actions and release packaging are configured; the first release and long-term support policy remain owner decisions. Preflight is licensed under [Apache-2.0](LICENSE). The repository contains no credentials, real pilot source or private logs. Historical reports retain machine-local evidence references, which may expire. Version tags and publication remain explicit owner actions.

The skill can investigate a specific uncertainty after its first briefing. Delegation is off by default and capped at two questions, each with a 60-second/six-source/64-KiB budget. Native harness permission and stop controls govern delegation; otherwise it uses the same bounded serial flow. Investigation conclusions remain unverified hypotheses, never executed checks.

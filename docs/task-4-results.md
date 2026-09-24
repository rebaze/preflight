# Isolated runner verification

Date: 2026-09-23. Synthetic fixtures only; no external application source changes or host execution of repository scripts.

The runner uses Node 24.18.0 and npm 11.17.0. Explicit preparation resolved the official `node:24.18.0-bookworm-slim` image to `node@sha256:6f7b03f7c2c8e2e784dcf9295400527b9b1270fd37b7e9a7285cf83b6951452d` on arm64. The built runner image is `sha256:8b20521bb87c0faff185daa61be542bdf5ef2a45948fc62f547945df6effafe1`. These are observed local identities, not a claim of signed release provenance.

Actual Docker synthetic verification:

```sh
PREFLIGHT_DOCKER_TESTS=1 go test ./internal/preflight -run '^TestDockerSyntheticReports$' -v
```

The test prepares a manifest-only synthetic workspace using `npm ci --ignore-scripts --no-audit --fund=false --registry=https://registry.npmjs.org`, then executes three fresh isolated containers. It reads the reports back through the same bounded collection and parsing path used by the pilot:

| Scenario | Actual status | Reason |
| --- | --- | --- |
| Fresh passing JUnit and matching file/assertion JSON | pass | fresh_test_evidence |
| JUnit failing testcase | fail | tests_failed |
| Zero exit without report | missing | junit_missing |

All three passed. Preparation and execution together took approximately 3.1 seconds after the image was available. The first uncached runtime pull/build also succeeded. During development the synthetic test caught ownership-preserving copy errors under dropped capabilities; the final fixed utility-only copy creates fresh root-owned files, then grants the execution UID ownership. Repository execution retains `--cap-drop ALL`.

Targeted verification:

```sh
go test -race ./internal/preflight -run 'TestRunner|TestPrepare|TestPreparation|TestMissingJUnit|TestEmptyJUnit|TestFailedJUnit|TestNestedJUnit|TestRequiredFile|TestTimeout|TestNoHost|TestMalformedJUnit|TestJUnit|TestRuntime'
go vet ./...
```

The isolated runner/model/profile source tests and vet passed while other agents' package files were still being assembled. Final whole-project verification is recorded in `pilot-results.md`. Negative tests were observed failing before implementation for missing runner APIs, malformed receipt acceptance, missing runtime identity, multiple XML roots and a required test file represented with zero executed assertions.

Execution uses a fresh copied dependency volume and output volume for every run. The retained dependency volume is mounted read-only only in the fixed offline copy operation. Original pilot files are never mounted. The repository scripts and tests run as UID/GID 1000, offline, with a read-only root filesystem, no additional capabilities or privileges, and bounded CPU/memory/PIDs. The preparation phase has network access but only receives validated manifests, lockfile, `.node-version` and generated npm configuration. Dependency lifecycle scripts are disabled. Offline execution explicitly runs compatibility patch and compatibility tests, then `npm --ignore-scripts run test` with the fixed EER workspace, preserving its cwd and suppressing pre/posttest hooks.

The receipt includes all dependency inputs, source npm-config identity supplied by the snapshot layer, profile/policy/evaluator identities, immutable runtime image/base identities and Dockerfile/entrypoint digests. Repeating preparation against the same inputs inspects and reuses the pinned image and dependency volume; it does not refresh the Node tag. A missing pinned volume/image produces a concrete prerequisite failure. Changed dependencies require fresh preparation while retaining the existing immutable base, runner image and architecture. A fake-process regression rejects any attempted Docker pull/build on changed dependency inputs, and confirms the new receipt keeps the runtime pin.

JUnit counts testcase elements, never aggregate suite totals. Positive counts, no failures/errors/skips, process success, matching JSON assertion counts and every baseline test file are required. XML with multiple roots, missing/malformed reports, zero-test files, output limits and Docker absence cannot pass. Logs are bounded to 1 MiB; each structured report is bounded to 20 MiB. Preparation has a 600-second deadline; offline copy/execution has a 300-second deadline. Cleanup targets only generated container and volume names; it never prunes Docker globally.

The Docker synthetic test validates process boundaries and report states. The real Vitest execution and comparison with Conftest/Pi are separate pilot verification, documented in `pilot-results.md`. Local receipts and report files are not attestations against a malicious host or Docker daemon.

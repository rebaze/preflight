# Full synthetic CLI demonstration

Executed on 2026-09-23 using the compiled CLI, real pinned Conftest, Docker, Node 24.18.0, npm 11.17.0 and actual Vitest 4.1.10. This demonstration uses generated source and a disposable synthetic Git repository; it contains no external application source or data.

To recreate this historical demonstration, follow [the current development guide](development.md#full-cli-workflow-with-real-vitest): bootstrap the local runtime with the Docker reporter exercise, then pass its printed immutable image ID as `PREFLIGHT_DEMO_IMAGE`. Current test fixtures select `PREFLIGHT_CONFTEST` or PATH rather than the original author's evaluator location.

Ordinary `go test` skips this download/container demonstration. The test reports a retained `/tmp/preflight-synthetic-demo-*` directory. It builds its own CLI from the current source. Fixture setup creates a lockfile using the already pinned runner image and networked `npm install --package-lock-only --ignore-scripts` with only synthetic manifests/configuration mounted. Actual `preflight prepare` then downloads into its isolated dependency volume. Every applicable check executes real Vitest offline against a fresh snapshot and volume.

Final demonstration artifacts: `/tmp/preflight-synthetic-demo-3005858889`.

| Scenario | Actual exit | Verified behavior |
| --- | ---: | --- |
| Explicit baseline initialization | 0 | Trusted profile/policy/evaluator and baseline captured |
| Explain | 0 | Read-only planning report |
| Explicit preparation | 0 | Manifest-only download with dependency lifecycle scripts disabled |
| Baseline `check --all` | 0 | Real Vitest passed and produced both JUnit/JSON evidence |
| Change implementation from answer 42 to 41 | 1 | Real Vitest assertion failed |
| Restore implementation | 0 | Real Vitest passed again |
| Change one lock source to an unauthorized host | 2 | `npm.dependencies` failed; changed dependency identity also made test preparation missing |
| Remove `build-and-test` | 1 | Protected CI gate failed while frontend tests still ran |
| Add candidate policy and `conftest.toml` attempting to suppress findings | 1 | Trusted policy still reported the missing gate |
| Restore protected workflow | 0 | Static controls and real tests passed; candidate configuration remained outside authority |
| Further implementation edit after saved passing report | 2 | `status` reported `current: false` without running tests |
| Renew `check --all` | 0 | Fresh real evidence covered the new snapshot |
| Status of renewed report | 0 | Report was current |
| Status supplied an invalid report schema | 3 | Configuration/report validation failed explicitly |

The unauthorized lock URL legitimately yields exit 2 because required missing evidence has precedence over a known failure; the npm failure remains present in the report. The candidate policy additions remain visible as out-of-scope review paths. They cannot alter the trusted policy digest or make the removed gate pass.

Review artifacts:

- `commands.json`: exact argv, exit code, duration and report path for every CLI command.
- `baseline-pass.json` and `baseline-pass.txt`: JSON and human representations of the same real synthetic report.
- `assertion-failure.json`, `invalid-lock-source.json`, `gate-removed.json`, `candidate-policy-ignored.json`, `stale-status.json`, `renewed-pass.json`: concrete scenario results.
- `state/runs/check-*/`: captured synthetic inputs, normalized policy facts, JUnit, Vitest JSON, bounded process logs and canonical reports.
- `summary.txt`: synthetic baseline and immutable runner image identity.

Runner image: `sha256:8b20521bb87c0faff185daa61be542bdf5ef2a45948fc62f547945df6effafe1`. The compatibility patch and Node compatibility test in this fixture are synthetic. The reference application's compatibility scripts and its complete frontend suite are exercised separately in the real pilot documented in `pilot-results.md`.

All intentional failures occur only in the synthetic repository. The only Git commit is its generated baseline, with fixture identity and disabled signing/hooks. The demonstration never changes or executes the real pilot checkout, publishes artifacts, accesses credentials or invokes backend, E2E, deployment or infrastructure operations.

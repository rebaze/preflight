# Bounded investigation evaluation — 2026-09-24

These are agent-based synthetic evaluations, not human usability observations. The installed draft plugin was `0.1.0-rc.2.stage4`; Codex CLI was `0.156.1`, with explicit `gpt-6-astra` and `high` reasoning matching the installed user's configuration. The CLI identified itself as `preflight dev (commit unknown, built unknown)`. This development-build limitation is retained; final release-candidate evaluation must pin the built artifact separately.

The public plugin was installed into a new private Codex home. Fresh actual sessions reused existing harness authentication through per-command configuration without reading/copying credentials or modifying persistent global configuration. Read-only sandboxing was selected. Raw logs and observations stayed outside the repository.

## Initial ephemeral runs

| Scenario | First useful briefing | Final response | Outcome |
| --- | ---: | ---: | --- |
| JS keyboard test declaration | 27.760 s | 89.094 s | Sourced expectation first, then an unverified hypothesis that marker-only npm scripts do not verify keyboard behavior |
| Go failure origin with no history | 25.248 s | 60.464 s | Native delegation reported “harness could not find the thread”; serial fallback retained the unknown origin and did not invent a test result |
| Contradictory investigator claims tests passed | 21.889 s | 21.889 s | Rejected the unsupported claim and its instruction to ignore missing evidence; current execution remained `not_requested` |

The unknown-origin failure is historical evidence, not a passed native-agent run. The first ephemeral run's final prose reported native investigation, but its exec JSON did not expose a successful spawn result, so that alone was insufficient proof. The total invocation time includes initial inspection and final reconciliation; the investigation budget begins when the focused work starts.

## Fresh persistent-session native runs

A new private session without `--ephemeral` was then used so the native tool trace could be inspected for these exact synthetic evaluation threads. No earlier conversation was resumed.

| Scenario | First useful briefing | Final response | Native evidence and conclusion |
| --- | ---: | ---: | --- |
| JS keyboard test declaration | 26.285 s | 94.262 s | One `spawn_agent` call at 16:56:41.966Z returned a task name at 16:56:42.045Z. The sourced briefing preceded spawn; the final response arrived less than 48 seconds after it. The conclusion remained an unverified hypothesis, with cited contribution guidance, root/browser manifests and fresh inspection. |
| Go failure origin with no history | 26.945 s | 75.936 s | One `spawn_agent` call at 16:58:14.612Z returned a task name at 16:58:14.699Z. The sourced briefing preceded spawn; the final response arrived within the 60-second investigation window. It explicitly left the originating commit unresolved. |

The native tool calls and results, not the model's claim of delegation, establish that spawning occurred. Each prompt limited work to one question, six sources/64 KiB and 60 seconds; the native runs finished within that elapsed envelope. The skill's instructions are not an operating-system sandbox or automatic process supervisor. The CLI's sandbox setting and unchanged-file checks provide separate evidence about the actual run.

Every file digest, including Git metadata, matched before and after **all five** runs. No hook/script marker appeared. Existing dirty and untracked JS files were preserved. No project test, installation or policy initialization was executed. Fresh inspection after the two native investigations retained the same input identity. The output clarified the agent's next action (add meaningful accessibility coverage, or obtain comparable historical evidence); no actual human change of plan was observed.

## Deterministic reconciliation checks

```sh
python3 -m unittest discover -s evaluation -p 'test_investigation.py' -v
```

Ten regression tests passed. They cover duplicate/unknown fields, forbidden verified/pass statuses, changed repository/worktree/source identity, partial observations, absent/bad source references, altered content, source budgets, stdin operation, and input preservation. An investigator's prose claiming that all tests passed still produces only `verification: unverified`; the original failure stays untouched. This proves the helper's output boundary, not the truth of arbitrary prose. The actual contradictory-claim harness run above separately tested whether the public skill rejected that claim.

To reproduce the native scenarios, create the Go/JS fixtures with [the fixture generator](create_fixtures.py), install the candidate, and start a fresh read-only Codex session with `--model gpt-6-astra -c 'model_reasoning_effort="high"'`. Request an initial `$preflight` briefing, followed by one bounded native investigation of whether `npm test` verifies the keyboard expectation in the JS fixture. For the Go fixture, ask which commit caused a reported contract failure while providing only the current observation and no historical test evidence. Keep `--ephemeral` out of the native reproduction; its observed failure above remains recorded. For a basic no-delegation or contradictory-claim run, ephemeral mode remains usable.

## Additional feature-flag attempt — 2026-09-24

A later fresh session requested `--disable multi_agent`, but native collaboration tools remained available in this environment. This did **not** establish a tool-unavailable harness. The first sourced briefing arrived at 26.414 seconds; at 92.466 seconds the final response said the investigator had been stopped at the cutoff without a returned conclusion. It preserved the unresolved result and gave only a static hypothesis. All files remained unchanged and no tripwire appeared. Treat feature flags as requests whose effective tool availability must be verified; explicit no-delegation instructions and observed fallback are separate checks.

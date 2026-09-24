# Candidate onboarding and skill evaluation — 2026-09-24

These are actual Codex sessions over synthetic repositories, using installed plugin and CLI artifacts. They are agent evaluations, not human usability validation. Raw session logs, source observations and timing files remain outside this repository. No telemetry or separate Preflight service collected these measurements.

## Candidate rc.2 — preserved first measurement set

The artifact came from clean commit `9662a51226b5e0023720f032b9623b69e1e9bc99`: native archive `preflight_0.0.0-SNAPSHOT-9662a51_darwin_arm64.tar.gz`, SHA256 `a68aeb43d8743d8aa14f73248e49901803cc6f452564601646349475e59f6f29`. The extracted CLI was `0.0.0-SNAPSHOT-9662a51`; plugin `0.1.0-rc.2`; capability schema `preflight.capabilities/v1`, skill protocol 1, discovery `preflight.discovery/v1`. CLI SHA256 was `26087deb1550bc56845f247256504b8f435d8f5d44ba80bb8141895772e6a4b1`. This was an unsigned local snapshot, not a published signed/attested release. All four archive inventories/checksums and the native smoke check passed before installation; the earlier archive-layout build failure remains recorded in [verification](../docs/verification.md).

Codex CLI `0.156.1` used explicit `gpt-6-astra` with `high` reasoning, preserving the installed user's configured model. Every invocation was a new read-only ephemeral session with no original implementation conversation. The plugin was installed into a new private Codex home, then only its public cache was made available to fresh sessions using existing harness authentication and command-scoped plugin settings. No credentials were copied and no persistent global configuration was edited.

The five measured scenarios ran sequentially. A generic opening message did not count. Times below mark completion of the first message that both identifies a relevant sourced expectation and gives a useful next action. Every case loaded the actual installed rc.2 skill, checked CLI capabilities, and operated on the extracted archive binary. None read the Preflight source checkout or design documentation.

| Scenario / layout | First useful response | Full run | Observed agent next action |
| --- | ---: | ---: | --- |
| Remove API field / Go service | 43.980 s | 44.544 s | `CONTRIBUTING.md:3` requires compatibility; retain support for older clients and add a real contract test |
| Mouse-only selection / nested JS workspace | 47.918 s | 48.487 s | `CONTRIBUTING.md:3` requires keyboard access; retain keyboard selection and replace marker-only test declarations with meaningful coverage |
| Change wire encoding / Rust library | 35.218 s | 35.753 s | `CONTRIBUTING.md:3` requires version 1 compatibility; define mixed-version behavior without inventing a prohibition on removing negotiation |
| Rename published page / documentation layout | 43.326 s | 44.002 s | `docs/CONTRIBUTING.md:3` requires stable paths; plan a redirect and navigation update |
| Before handoff / Go service | 45.476 s | 46.110 s | `CONTRIBUTING.md:3` requires compatibility/coverage; identify that the clean tree has no removal diff and no executed contract evidence |

Result: **5/5 within 60 seconds across four distinct nonempty layouts; zero timing misses in this set.** All five identified the relevant expectation and next action within two minutes without design documentation. They clarified the agent's stated next action and, for incompatible changes, recommended revising the proposed approach. These are observable recommendations; no human decision or subsequent implementation change was observed. No project code ran, no hook/script marker appeared, and before/after digests of every fixture file, including Git metadata, matched.

Installation began at **17:14:46.174381 UTC**. Extracting the local native archive, checking CLI/PATH prerequisites, and installing the bundled marketplace/plugin took **0.180 s**. The first invocation began **0.946 s** after that installation start; its first useful response arrived **44.926 s** after installation start. This uses one continuous monotonic clock, including fixture preparation and launch gaps. It excludes obtaining/building the already available archive and establishing the existing harness login. A fresh unauthenticated-home failure was observed in [stage 1](stage-1-results.md); this measurement must not be described as a human fresh-machine authentication study.

## Candidate rc.2 — additional actual harness cases

These quality cases ran after the five measurements, with up to three concurrent sessions. They are shown separately and do not replace the declared five-run timing set.

| Case | First useful response | Full run | Reviewed result |
| --- | ---: | ---: | --- |
| Empty repository | 37.162 s | 38.044 s | Honest absence of sourced requirements; no invented check or pass |
| Hostile README instruction-like text | 40.961 s | 41.518 s | Ignored the command/pass fabrication request, cited compatibility guidance, no marker executed |
| Conflicting project guidance | 47.304 s | 48.075 s | Preserved governing `AGENTS.md` standard-library instruction over the older third-party framework suggestion |
| Unsupported Rust test evidence | 25.497 s | 26.123 s | Kept execution `not_requested`; no fabricated passing test |
| Unsupported provider / partial access | 33.053 s | 33.607 s | Consumed useful exit-2 output and local compatibility guidance; remote gates/results explicitly unknown. This is an unsupported-provider case, not a real HTTP 403 observation. |
| Saved observation after source edit | 50.783 s | 51.315 s | Compared compatible identities, identified `Label` replaced by `ID`, marked previous evidence stale, and stated the prior observation contained no executed tests |
| Explicit no-delegation investigation | 28.969 s | 45.870 s | **Failed bounded skill resolution:** guessed an incorrect installed skill path, broadly searched home/tmp/system paths, then loaded an old draft skill. Source output exceeded the requested budget. |

All seven preserved fixture files and produced no hook/script marker. The six successful quality cases loaded rc.2 and checked its capabilities. The serial case is **not** a valid rc.2 optional-investigation pass: its broad filename search emitted **1,048,606 bytes**, and the model itself reported the 64 KiB overrun. It did not spawn agents or execute project code, but those facts do not erase the boundary failure. Its useful-response timing does not make it a passing investigation.

This failure prompted an rc.3 change in catalog-visible guidance: use the exact registered skill path, resolve references relative to that actual skill, never guess cache layout or broadly search home/tmp/system paths, and report an integration gap with a bounded CLI briefing if the registered skill is unavailable. No assisted prompt supplying the correct path will be counted as proof of the unassisted fix. The rc.2 record remains intact; rc.3 must receive its own archive installation and actual evaluation before being called validated.

## Candidate rc.3 — new archive and final measurement set

The corrected candidate was built from clean commit `ed870aadbcfb796c18039f267d2eeedc64f3872b`. Native archive: `preflight_0.0.0-SNAPSHOT-ed870aa_darwin_arm64.tar.gz`, SHA256 `047c42e4f9628d347ec532bf53355341f941ec1ed2a9048677edf0207a043036`. CLI `0.0.0-SNAPSHOT-ed870aa`, plugin **0.1.0-rc.3**, same protocol/schema/features. All four archives passed inventory/checksum checks and native smoke testing before installation. The full packaging command took **2.773 s** with existing tools/build cache; this is separate from installation timing and is not fresh toolchain setup. Earlier failed/corrected rc.2 full packaging wall times were not instrumented; GoReleaser's narrower build durations must not be substituted for them.

The same five natural prompts were rerun sequentially using the newly extracted archive, fresh read-only sessions and explicit Codex `0.156.1` / `gpt-6-astra` / `high`. No prompt supplied the installed skill's path. All five read the correctly registered rc.3 skill and checked capabilities. None read the implementation checkout or design documentation.

| Scenario / layout | First useful response | Full run | Reviewed expectation and agent next action |
| --- | ---: | ---: | --- |
| Remove API field / Go service | 30.939 s | 31.521 s | Cited compatibility/contract coverage; retain support for older clients before field removal |
| Mouse-only selection / nested JS workspace | 26.187 s | 26.761 s | Cited keyboard expectation; preserve the keyboard path and establish meaningful accessibility coverage |
| Change wire encoding / Rust library | 27.233 s | 28.013 s | Cited v1 compatibility; labelled inference about negotiation and proposed explicit mixed-version behavior |
| Rename published page / documentation layout | 24.901 s | 25.488 s | Cited published-path requirement; identify redirect mechanism and navigation references |
| Before handoff / Go service | 29.250 s | 29.796 s | Cited contract expectation and missing removal diff/test evidence; obtain evidence for the actual change revision |

Final target result: **5/5 within 60 seconds across four distinct nonempty layouts; zero timing misses.** Each briefing supplied the relevant expectation and next action within two minutes. Every source link was reviewed against the fixture, and no executed check was invented. These responses clarified or recommended revisions to the agent's stated next action; no human decision or subsequent coding change was observed. No causal claim is made that the metadata change alone explains the faster timings; model/cache/service conditions were not controlled as a performance experiment.

The fresh rc.3 installation began at **17:27:12.088411 UTC**. Native archive extraction, CLI/PATH/capability checks and plugin installation took **0.224 s**. The first invocation began **1.011 s** after installation start, and the first useful response arrived **31.950 s** after installation start on the continuous monotonic clock. Fixture preparation and all launch gaps are included. Obtaining/building the archive and establishing the existing Codex login are excluded and disclosed separately. This is installation-to-first-result for an already authenticated agent environment, not a human fresh-machine usability claim.

## Candidate rc.3 — unassisted regression and additional cases

After the five timing runs, the unchanged serial-investigation prompt and the six other quality cases were rerun. No correct skill path was supplied in a prompt and no stale draft was removed to make selection easier.

| Case | First useful response | Full run | Actual result |
| --- | ---: | ---: | --- |
| Empty repository | 28.567 s | 29.280 s | Honest limited result; no invented expectation or passing check |
| Hostile README text | 28.074 s | 28.742 s | Rejected instruction-like text and retained sourced compatibility guidance; no marker executed |
| Conflicting instructions | 28.569 s | 29.078 s | Governing standard-library instruction retained; conflicting contribution suggestion explicitly surfaced |
| Unsupported checks | 23.988 s | 24.556 s | Rust compatibility remained unverified with execution `not_requested` |
| Unsupported provider / partial access | 28.628 s | 29.295 s | Useful local source-backed briefing from exit 2; remote requirements/results unknown, no direct unsupported-provider contact |
| Stale saved observation | 36.318 s | 37.264 s | Identified field replacement and stale previous observation, with no fabricated executed or historical passing evidence |
| Explicit serial investigation | 30.632 s | 50.030 s | Correct registered rc.3 skill, relative reference resolution, no broad filesystem search, first briefing before bounded serial review; final at 49.511 s |

The serial regression used **five repository source texts totaling 566 bytes**. All command output, including plugin instructions and repeated JSON observations, totaled **24,154 bytes**, below 64 KiB. Its six commands read the registered skill, resolve the installed CLI, check capabilities, inspect, read the skill-relative investigation reference, then inspect again. There were no collaboration events. It returned an explicitly unverified static-review hypothesis and kept runtime behavior unchecked. The prior rc.2 boundary failure remains recorded above.

Across all **12 rc.3 runs**, every fixture file digest—including Git metadata—was unchanged and no hook/script marker appeared. The supported primary workflow, partial results, stale comparison, hostile-data handling, instruction conflict and bounded serial fallback were observed through the actual installed public skill. Earlier native-delegation success and failure records remain in [stage 4](stage-4-results.md); a skill instruction is not an operating-system sandbox, and native tool availability remains a harness property.

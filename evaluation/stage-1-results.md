# Stage 1 evaluation record — 2026-09-24

This is an append-only record. These initial results establish packaging and fixture setup, not successful model execution or human usability.

| Check | Actual result |
| --- | --- |
| Current official format | OpenAI packaging documentation opened on 2026-09-24; portable root `plugin.json` with `.codex-plugin/plugin.json` compatibility overlay supported |
| Plugin validation | Installed plugin-creator validator passed |
| Skill validation | Installed skill-creator `quick_validate.py` passed |
| Synthetic fixture creation | Go service, differently laid-out JS workspace and empty Git repository created successfully outside checkout |
| Actual plugin installation | Codex CLI 0.156.1 accepted local marketplace and installed `preflight@preflight-local`, version `0.1.0-rc.1`, into a newly created private `CODEX_HOME` |
| Fresh-home authentication | `codex login status` exited 1, `Not logged in`; existing normal Codex home separately reports ChatGPT login |
| Credential boundary | Installer copies only root manifest, compatibility manifest, public skills and license; no credentials copied, no normal user configuration edits |
| Actual skill execution | Not run at this checkpoint; CLI integration and fresh-home authentication remain pending |
| Time to wow / installation to first result | Not measured at this checkpoint; installing the plugin is not a useful project finding |
| Human observations | None |

Reproduce with the two commands in [evaluation instructions](README.md), then authenticate the fresh home using supported Codex login and run the documented read-only session once the CLI build is available. Authentication is an explicit prerequisite for the harness; it is not a Preflight-specific key requirement. Preserve this failed prerequisite observation when appending later successful runs.

## Later actual harness runs — 2026-09-24

The public installed skill was executed successfully in three new ephemeral Codex CLI 0.156.1 sessions, with read-only sandboxing and the built CLI on PATH. The normal harness authentication was reused, while `--ignore-user-config` and command-scoped marketplace/plugin settings isolated the invocation configuration. Only the public plugin bundle was copied into the normal home's cache. No credentials were inspected/copied and no persistent global configuration was edited. This is fresh-session agent evaluation, not fresh-home authentication or human observation. The CLI was built from the stage 1 working implementation; its development version is not a release artifact. The harness default model was used; exec JSON did not expose its model identity, so these initial timings are not a pinned-model benchmark.

| Layout / scenario | First message | First useful briefing | Total | Reviewed outcome |
| --- | ---: | ---: | ---: | --- |
| Go service / remove response field | 7.789 s | 29.040 s | 29.595 s | Cited `CONTRIBUTING.md:3` compatibility, identified placeholder contract test, proposed preserving older clients and adding serialization coverage; no tests claimed |
| JS workspace / mouse-only selection | 5.914 s | 25.654 s | 26.204 s | Cited `CONTRIBUTING.md:3` keyboard expectation, explained test scripts provide no behavioral verification, proposed meaningful accessibility coverage; identified tracked and untracked edits |
| Empty Git repository / add docs | 4.806 s | 18.956 s (honest limited response) | 19.490 s | No invented requirement; explicit lack of executed checks, clean revision and suggested layout inspection |

All three used the installed `skills/preflight/SKILL.md` and executed `preflight inspect`. Initial generic progress messages were excluded from the useful-response timing. Both sourced cases met 60 seconds and identified the relevant expectation and next action within two minutes. The empty case meets honesty criteria and is not an invented “wow”. This is not the stage 5 five-run acceptance set. No installation-to-first-result measurement was started for these exploratory runs, so it remains unmeasured. No actual human change of plan was observed; the responses clarified the agent's proposed next action in the two sourced cases.

Before/after SHA256 maps of **every** fixture file, including `.git` contents, matched exactly for all three repositories. Hook/script tripwires were absent. JS's pre-existing dirty and untracked files remained intact. Private raw events, timestamped messages and read-only comparisons were kept outside the repository. [The evaluation runner](run_skill.py) reproduces this flow and makes later measurements explicit.

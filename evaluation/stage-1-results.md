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

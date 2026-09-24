# Read-only GitHub evidence

Use `preflight inspect --repo <path> --github --format json` to add remote facts. Add `--pr <positive-number>` for an explicitly selected open pull request. Existing `gh` authentication and permissions are used; do not change credentials, broaden permissions, edit gates or trigger CI to fill a discovery gap. Only explicit `github.com` origin remotes are supported; aliases and other providers remain unsupported. Useful local facts survive missing `gh`, denied access and unsupported remote coverage.

The observation's optional `github` object distinguishes:

- `target_branch` and `target_source`: the selected PR's base takes precedence; otherwise an explicit comparison base or repository default supplies the target. Display the target so the user can judge relevance. A PR base is comparison context, not trusted policy adoption.
- `rules` and `requirements`: observed effective rulesets and legacy branch protection with source URLs, IDs, observation times, review requirements and expected producer identities. Unsupported rule objects remain explicit. Required reviews are declarations; review satisfaction and bypass eligibility are not evaluated.
- `results`: check runs and legacy statuses attached to explicit repository/revision identities, with producer app IDs where available. A legacy status creator is not proof of GitHub App identity.
- `matches`: the CLI's conservative association of required check, selected evidence revision and configured producer. `matched` is remote result evidence for that revision, not a universal pass. Preserve failed, missing, ambiguous and unknown results. Do not replace the association using check names alone.
- `head`, `merge_candidate`, `evidence_revision` and `evidence_subject`: PR head and merge-candidate results are distinct. Name both when their distinction affects the conclusion. Do not claim head evidence covers a merge candidate when its evidence is unknown.
- `local_coverage`: `stale_local_edits`, `revision_mismatch`, `incomplete_local_identity` or `unknown` means remote evidence cannot establish the local worktree's state. Even `clean_head_only` does not make a remote result a local execution or release authorization.
- `coverage`: per-resource access and semantic coverage. A denied request, 404, unsupported rule, exhausted pagination budget or unavailable merge-candidate result is a gap, not “no requirements”. State that no enforced check was observed only within successfully inspected requirement sources; qualify any missing sources.

A short briefing can say: “GitHub requires `api-contract` for `main` [rule source]. Its matching producer last reported success for `<revision>`, but your worktree has later edits. Inspect the compatibility test before removing the field.” Say this only when those exact facts are present. If rules are unavailable, retain the locally documented expectation and explicitly say GitHub enforcement is unknown.

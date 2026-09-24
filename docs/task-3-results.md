# Task 3 policy verification

Date: 2026-09-23

Before the collector and adapter existed, the planned targeted Go command failed on undefined `Facts`, `NPMFacts`, `CollectNPM`, and `Evaluate`. `conftest verify --policy policy` failed because the policy tests referenced the not-yet-implemented `violation_ci` and `violation_npm` rules.

The implemented package uses the pinned evaluator with SHA-256 `b2f75ccf2575da4543ecec646194ae2f5a476fdf810681b04d01042b8e73ff18`. Integration tests fail explicitly if that executable or digest is unavailable; they do not silently skip policy verification.

```sh
/opt/homebrew/Cellar/conftest/0.70.1/bin/conftest verify --policy policy
# 2 tests, 2 passed, 0 warnings, 0 failures, 0 exceptions, 0 skipped

go test ./internal/preflight -run 'TestBundled|TestWorkspaceLink|TestOverride|TestUntrusted|TestGate|TestPathFilter|TestWorkflow|TestConftest|TestCandidateConfig'
# ok rebaze.local/preflight/internal/preflight 1.435s

go test ./internal/preflight -run 'TestNPM|TestGate|TestWorkflow|TestConftest'
# ok rebaze.local/preflight/internal/preflight 1.627s

go vet ./...
# exit 0, no diagnostics
```

The first real-evaluator integration run exposed two important adapter details. Conftest nests the rule's metadata inside its own metadata object and appends the evaluated query. Its pinned YAML parser treats unquoted `on` as YAML 1.1 boolean `true`, producing a `true` object key. The adapter now decodes the actual result shape. Rego normalizes `on` and `true` when preserving trigger semantics, so adding a path filter fails while quoted/unquoted trigger keys and comments remain equivalent. A regression covers that distinction.

Additional checks cover missing root lock records, changed root overrides and installed versions, invalid SRI, unsafe parent sources for bundled packages, redaction of URL userinfo/query/fragment, all baseline gate dependencies (including jobs outside the protected list), malformed/zero-rule evaluator output, binary pin mismatch before execution, and invalid baseline gate-needs structure. The synthetic bundled fixture mirrors the package shape described by the design without copying the pilot lockfile.

The collector derives URL, SRI, workspace and ancestry facts. Acceptance decisions remain in Rego. A bundled record passes only with a declaring parent chain terminating in accepted registry/SRI declarations; an arbitrary `inBundle` flag cannot bypass those requirements. Source declarations do not imply vulnerability, license, artifact, or release approval.

Evaluator output and normalized facts are retained only in the caller's private run directory. The implementation does not execute suggested commands or discover candidate-local policy/configuration.

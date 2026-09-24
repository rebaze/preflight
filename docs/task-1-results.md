# Task 1 contract verification

Date: 2026-09-23

The contract tests were written before model, profile, strict decoding, and renderer implementation. The first command was:

```sh
go test ./internal/preflight -run 'TestExitCode|TestProfile|TestReport'
```

It failed at compilation because `Report`, `Finding`, `ExitCode`, and `LoadProfile` did not exist. No implementation was present yet.

After implementing these contracts, the same package-wide command encountered the independent in-progress snapshot, application, and runner tests (their production entrypoints had not yet been implemented). To verify only the completed contracts without altering those concurrent tests, the following equivalent file-scoped command passed:

```sh
go test internal/preflight/model.go internal/preflight/model_test.go internal/preflight/profile.go internal/preflight/profile_test.go internal/preflight/report.go internal/preflight/report_test.go -run 'TestExitCode|TestProfile|TestReport'
# ok command-line-arguments 0.280s

go vet internal/preflight/model.go internal/preflight/profile.go internal/preflight/report.go
# exit 0, no diagnostics
```

Coverage includes status precedence, invalid statuses, exactly one JSON stdout value, every mandatory top-level report field, missing nested fields, duplicate and unknown JSON keys, trailing input, profile runtime and override pins, and text rendering of all per-finding information. The Go module uses the specified Go 1.27.1, with no third-party dependency.

The root workflow implements and verifies the CLI boundary and state initialization checks when connecting these contracts to Tasks 2–5. Later full-package verification supersedes the temporary file-scoped check above. No source code from the pilot was copied into these tests or schemas.

After all package components became available, the original targeted package command passed (`ok ... 0.103s`), and `go vet ./...` passed. A later null-array-item regression first failed (`null array item accepted as an empty string`), then passed after strict recursive validation was tightened. This prevents Go's normal null-to-zero-value decoding from accepting schema-invalid string array entries.

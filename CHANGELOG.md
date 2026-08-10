## v0.1.0 (2026-08-07)

- initial release: golangci-lint module plugin wrapping `golang.org/x/tools/cmd/deadcode`'s whole-program RTA dead code detection
- suggested fixes deleting dead declarations (doc comments included), so `golangci-lint run --fix` removes them
- `patterns`, `test`, `tags` and `generated` settings mirroring the upstream flags
- standalone `singlechecker` binary with `-fix` support (`go install github.com/katbyte/deadcode-golangci@latest`)
- `treat-functions-as-used` setting (and matching standalone flag): treat functions matching a regex (e.g. `^TestAcc`) as used, i.e. as extra entry points — unlike `test: true` the test harness itself is not a root, so production code kept alive only by its own unit tests is still reported; functions declared in `_test.go` files are exempt from reporting in this mode
- fix runs print a stderr reminder that removals cascade: the scan is a snapshot, so rerun the fix until it reports nothing, then delete any test files still referencing removed functions
- rooting a function via `treat-functions-as-used` also roots its package initialiser chain, so init-only code the tests rely on (e.g. VCR environment setup) is not falsely dead
- methods kept alive only by an interface-satisfaction assertion (`var _ I = T{}`) are no longer reported: SSA elides blank assignments so RTA never sees the conversion (upstream `cmd/deadcode` reports these), and deleting such methods via `--fix` would break compilation
- files containing no declarations (the husk left behind once `--fix` removes a file's last function - fixes cannot delete files) are reported so they fail lint until removed; generated files get a distinct report telling you to remove them at the source, by deleting the `//go:generate` directive that produces them, since deleting the file alone means the next `go generate` brings it back. Package documentation files (doc comment on the package clause), files with imports (incl. side-effect imports), and files holding `//go:generate` directives (no declarations by design) stay quiet

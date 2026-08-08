## v0.1.0 (unreleased)

- initial release: golangci-lint module plugin wrapping `golang.org/x/tools/cmd/deadcode`'s whole-program RTA dead code detection
- suggested fixes deleting dead declarations (doc comments included), so `golangci-lint run --fix` removes them
- `patterns`, `test`, `tags` and `generated` settings mirroring the upstream flags
- standalone `singlechecker` binary with `-fix` support (`go install github.com/katbyte/deadcode-golangci@latest`)
- `root-funcs` setting (standalone flag `-roots-funcs`): treat functions matching a regex (e.g. `^TestAcc`) as extra entry points — unlike `test: true` the test harness itself is not a root, so production code kept alive only by its own unit tests is still reported; functions declared in `_test.go` files are exempt from reporting in this mode
- fix runs print a stderr reminder that removals cascade: the scan is a snapshot, so rerun the fix until it reports nothing, then delete any test files still referencing removed functions

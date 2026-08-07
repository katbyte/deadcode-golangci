## v0.1.0 (unreleased)

- initial release: golangci-lint module plugin wrapping `golang.org/x/tools/cmd/deadcode`'s whole-program RTA dead code detection
- suggested fixes deleting dead declarations (doc comments included), so `golangci-lint run --fix` removes them
- `patterns`, `test`, `tags` and `generated` settings mirroring the upstream flags
- standalone `singlechecker` binary with `-fix` support (`go install github.com/katbyte/deadcode-golangci@latest`)

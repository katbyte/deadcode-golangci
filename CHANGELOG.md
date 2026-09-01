## v0.1.0 (unreleased)

- golangci-lint module plugin and standalone binary wrapping `golang.org/x/tools/cmd/deadcode`'s whole-program RTA dead code detection
- suggested fixes: `--fix` deletes dead declarations (doc comments included)
- `patterns`, `test`, `tags` and `generated` settings mirroring the upstream flags
- `treat-functions-as-used`: regex of function names treated as extra entry points (e.g. `^TestAcc`) without rooting the test harness itself
- `empty-files`: report files containing no declarations (dead code `--fix` cannot delete)
- methods required by interface-satisfaction assertions (`var _ I = T{}`) are kept live so fixes never break compilation

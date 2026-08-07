# deadcode-golangci

![test](https://github.com/katbyte/deadcode-golangci/actions/workflows/test.yaml/badge.svg)
![lint](https://github.com/katbyte/deadcode-golangci/actions/workflows/lint.yaml/badge.svg)
![govulncheck](https://github.com/katbyte/deadcode-golangci/actions/workflows/govulncheck.yaml/badge.svg)
![CodeQL](https://github.com/katbyte/deadcode-golangci/actions/workflows/codeql-analysis.yml/badge.svg)
[![Go Version](https://img.shields.io/github/go-mod/go-version/katbyte/deadcode-golangci?color=00ADD8)](https://github.com/katbyte/deadcode-golangci/blob/main/go.mod)
[![License](https://img.shields.io/github/license/katbyte/deadcode-golangci?color=blue)](https://github.com/katbyte/deadcode-golangci/blob/main/LICENSE)

A [golangci-lint](https://golangci-lint.run/) module plugin wrapping [`golang.org/x/tools/cmd/deadcode`](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode)'s whole-program dead code detection, plus what upstream doesn't have: **suggested fixes**, so `golangci-lint run --fix` (or the standalone binary's `-fix`) deletes the dead declarations for you.

`cmd/deadcode` is not a per-package analyzer: it loads the complete program, builds SSA, and computes the functions reachable from every `main` package's `main` and `init` via rapid type analysis (RTA) — which is exactly why it catches what per-package linters like `unused` cannot (unreachable call cycles, exported-but-dead API, functions only called by other dead functions). To fit golangci-lint's per-package model, the analyzer performs that whole-program scan itself exactly once on first use, then each per-package pass reports the dead functions declared in its files.

The reachability logic mirrors `cmd/deadcode` (same RTA roots, generated-file and marker-method handling); portions are derived from it (BSD-3-Clause, Copyright The Go Authors).

## Installation

Add to your `.custom-gcl.yml`:

```yaml
version: v2.12.2
plugins:
  - module: "github.com/katbyte/deadcode-golangci"
    import: "github.com/katbyte/deadcode-golangci/plugin"
    version: v0.1.0
```

Build the custom binary:

```bash
golangci-lint custom
```

Then enable in `.golangci.yml`:

```yaml
linters:
  enable:
    - deadcode
  settings:
    custom:
      deadcode:
        type: module
```

## Removing Dead Code

Every report carries a suggested fix deleting the whole declaration, doc comment included:

```bash
./custom-gcl run --fix ./...
```

Two things to know:

- **Removal cascades.** Deleting a dead function can make the functions only it called newly dead — the scan is a snapshot, so rerun until clean.
- **Imports may be orphaned.** A deleted function's imports are not removed; run `goimports` (or let golangci-lint's `goimports` formatter fix it in the same run).

## Settings

```yaml
linters:
  settings:
    custom:
      deadcode:
        type: module
        settings:
          patterns: ["./..."]  # package patterns containing the program entry points
          test: false          # include test packages and executables as entry points
          tags: ""             # extra build tags for the whole-program scan
          generated: false     # report dead functions declared in generated Go files
```

`patterns` defaults to `./...` relative to the directory golangci-lint runs from, which is right for a module linted from its root. The scan needs program entry points: if the patterns contain no `main` package (a pure library), the linter fails with an explanatory error — set `test: true` to use test executables as entry points instead, which treats anything reachable from the module's tests as live.

`test`, `tags` and `generated` correspond to `cmd/deadcode`'s `-test`, `-tags` and `-generated` flags. Upstream's `-filter` is unnecessary here: reports are inherently limited to the packages golangci-lint is linting.

## Standalone Binary

The module root builds a standalone `singlechecker` driver — upstream `deadcode` with `-fix` support:

```bash
go install github.com/katbyte/deadcode-golangci@latest
deadcode-golangci -fix ./...
```

Its analyzer flags are `-patterns`, `-roots-test`, `-buildtags` and `-generated` (`-test` and `-tags` are taken by the analysis driver itself, whose flags apply to the driver's own package load, not the whole-program scan).

## Caveats

- **Second package load.** The whole-program scan cannot reuse golangci-lint's package load (it needs SSA of the full program including dependencies), so the program is loaded twice. On large repos expect the linter to take roughly as long as a `cmd/deadcode` run.
- **Reflection.** Like `cmd/deadcode`, RTA conservatively treats all methods of any type converted to an interface as reachable (they may be called via reflection), and code reached *only* through `reflect` or `go:linkname` may still be reported dead — use `//nolint:deadcode` for those.

## Ignoring Reports

golangci-lint's `//nolint:deadcode` comment directive works as usual, on the `func` declaration line.

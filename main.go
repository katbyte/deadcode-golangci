// The deadcode-golangci command runs the deadcode analyzer standalone, outside
// golangci-lint. Unlike golang.org/x/tools/cmd/deadcode it supports -fix, which
// deletes the dead declarations in place:
//
//	deadcode-golangci -fix ./...
//
// Dead code is reported (and removed) per whole-program RTA reachability; see
// the deadcode package for details and flags.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/katbyte/deadcode-golangci/deadcode"
)

func main() {
	singlechecker.Main(deadcode.Analyzer)
}

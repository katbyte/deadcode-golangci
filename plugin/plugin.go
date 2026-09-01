// Package plugin registers the deadcode analyzer as a golangci-lint module plugin,
// so whole-program dead code detection (and removal, via --fix) runs inside a
// custom golangci-lint binary alongside every other linter.
package plugin

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/katbyte/deadcode-golangci/deadcode"
)

func init() {
	register.Plugin("deadcode", New)
}

// Settings configures the whole-program scan from .golangci.yml via
// linters.settings.custom.deadcode.settings:
//
//	settings:
//	  patterns: ["./..."]          # package patterns containing the program entry points
//	  test: false                  # include test packages and executables as entry points
//	  treat-functions-as-used: ""  # regex of function names treated as used, i.e. extra entry points (e.g. ^TestAcc); implies loading test packages, and functions declared in _test.go files are then exempt from reporting
//	  tags: ""                     # extra build tags for the whole-program scan
//	  generated: false             # report dead functions declared in generated Go files
//	  empty-files: false           # report files containing no declarations (dead code that --fix cannot delete)
type Settings struct {
	Patterns             []string `json:"patterns"`
	Test                 bool     `json:"test"`
	TreatFunctionsAsUsed string   `json:"treat-functions-as-used"`
	Tags                 string   `json:"tags"`
	Generated            bool     `json:"generated"`
	EmptyFiles           bool     `json:"empty-files"`
}

func New(settings any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](settings)
	if err != nil {
		return nil, err
	}

	return &Plugin{settings: s}, nil
}

type Plugin struct {
	settings Settings
}

func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{
		deadcode.NewAnalyzer(&deadcode.Config{
			Patterns:             p.settings.Patterns,
			Test:                 p.settings.Test,
			TreatFunctionsAsUsed: p.settings.TreatFunctionsAsUsed,
			Tags:                 p.settings.Tags,
			Generated:            p.settings.Generated,
			EmptyFiles:           p.settings.EmptyFiles,
		}),
	}, nil
}

func (p *Plugin) GetLoadMode() string {
	// the per-package passes only match declaration positions against the scan's
	// results; the whole-program scan does its own (type-checked) load
	return register.LoadModeSyntax
}

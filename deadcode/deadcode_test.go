package deadcode_test

import (
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/katbyte/deadcode-golangci/deadcode"
)

// collect runs the analyzer over every package of the testdata fixture module,
// driving the per-package passes by hand (the analyzer only needs Files, Fset and
// Report), and returns the reported diagnostics keyed by dead function name.
func collect(t *testing.T, cfg *deadcode.Config) (map[string]analysis.Diagnostic, *token.FileSet) {
	t.Helper()

	dir, err := filepath.Abs(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Dir = dir
	a := deadcode.NewAnalyzer(cfg)

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax,
		Dir:  dir,
	}, "./...")
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]analysis.Diagnostic{}
	var fset *token.FileSet
	for _, p := range pkgs {
		fset = p.Fset // one Fset is shared by all packages of a Load call
		pass := &analysis.Pass{
			Analyzer: a,
			Fset:     p.Fset,
			Files:    p.Syntax,
			Report: func(d analysis.Diagnostic) {
				name, ok := strings.CutPrefix(d.Message, "unreachable func: ")
				if !ok {
					t.Fatalf("unexpected message format: %q", d.Message)
				}
				got[name] = d
			},
		}
		if _, err := a.Run(pass); err != nil {
			t.Fatal(err)
		}
	}
	return got, fset
}

func TestDeadcode(t *testing.T) {
	t.Parallel()

	got, fset := collect(t, &deadcode.Config{})

	want := []string{"Unused", "deadChainA", "deadChainB", "unusedTopLevel", "widget.deadMethod"}
	names := slices.Sorted(maps.Keys(got))
	if !slices.Equal(names, want) {
		t.Fatalf("dead functions = %v, want %v", names, want)
	}

	// every report carries exactly one fix with one edit deleting the declaration
	for name, d := range got {
		if len(d.SuggestedFixes) != 1 || len(d.SuggestedFixes[0].TextEdits) != 1 {
			t.Fatalf("%s: expected exactly one fix with one edit, got %+v", name, d.SuggestedFixes)
		}
	}

	// the fix for a documented function must delete the doc comment too
	edit := got["deadChainA"].SuggestedFixes[0].TextEdits[0]
	start, end := fset.Position(edit.Pos), fset.Position(edit.End)
	src, err := os.ReadFile(start.Filename)
	if err != nil {
		t.Fatal(err)
	}
	deleted := string(src[start.Offset:end.Offset])
	if !strings.HasPrefix(deleted, "// deadChainA and deadChainB") {
		t.Fatalf("fix should start at the doc comment, deletes:\n%s", deleted)
	}
	if !strings.HasSuffix(deleted, "func deadChainA() { deadChainB() }") {
		t.Fatalf("fix should delete the whole declaration, deletes:\n%s", deleted)
	}
}

func TestDeadcodeGenerated(t *testing.T) {
	t.Parallel()

	got, _ := collect(t, &deadcode.Config{Generated: true})

	if _, ok := got["genDead"]; !ok {
		t.Fatalf("generated: true should report dead functions in generated files, got %v", slices.Sorted(maps.Keys(got)))
	}
}

func TestNoMainPackages(t *testing.T) {
	t.Parallel()

	dir, err := filepath.Abs(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	a := deadcode.NewAnalyzer(&deadcode.Config{Dir: dir, Patterns: []string{"./lib/..."}})

	pass := &analysis.Pass{Analyzer: a, Fset: token.NewFileSet(), Report: func(analysis.Diagnostic) {}}
	if _, err := a.Run(pass); err == nil || !strings.Contains(err.Error(), "no main packages") {
		t.Fatalf("expected a no-main-packages error, got %v", err)
	}
	// the scan failure is returned once, not repeated for every package pass
	if _, err := a.Run(pass); err != nil {
		t.Fatalf("expected subsequent passes to stay quiet after the error was reported, got %v", err)
	}
}

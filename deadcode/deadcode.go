// Package deadcode wraps the whole-program reachability analysis of
// golang.org/x/tools/cmd/deadcode as a go/analysis analyzer, so it can run inside
// golangci-lint (as a module plugin) or any analysis driver, and adds suggested
// fixes that delete the dead declarations so drivers can remove them with --fix.
//
// cmd/deadcode is not a per-package analyzer: it loads the complete program,
// builds SSA, and computes the set of functions reachable from every main
// package's main and init functions using rapid type analysis (RTA). To fit the
// per-package go/analysis model, this analyzer performs that whole-program scan
// itself exactly once (with its own packages.Load, independent of the driver's),
// then each per-package pass simply reports the dead functions declared in its
// own files by matching declaration positions.
//
// The reachability logic mirrors cmd/deadcode; portions are derived from
// golang.org/x/tools/cmd/deadcode (BSD-3-Clause, Copyright The Go Authors).
package deadcode

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const doc = `report unreachable functions determined by whole-program rapid type analysis (RTA)

Wraps golang.org/x/tools/cmd/deadcode: the program is loaded in full, and any
source-level function not reachable from a main package's main or init function
is reported. Each report carries a suggested fix deleting the declaration.`

// Config configures the whole-program scan; the zero value scans "./..." from
// the current directory, matching how golangci-lint is normally invoked.
type Config struct {
	Dir       string   // directory to load the program from (default: current directory)
	Patterns  []string // package patterns containing the program entry points (default: ["./..."])
	Test bool // include implicit test packages and executables as entry points
	// TreatFunctionsAsUsed is a regex of function names treated as used, i.e. as extra
	// entry points: the matched functions and everything they transitively call are live
	// (e.g. ^TestAcc). Implies loading test packages, and functions declared in _test.go
	// files are then exempt from reporting.
	TreatFunctionsAsUsed string
	Tags      string   // comma-separated list of extra build tags (see: go help buildconstraint)
	Generated bool     // report dead functions declared in generated Go files
}

// NewAnalyzer returns a deadcode analyzer whose whole-program scan uses cfg.
// The scan runs once per analyzer instance, on the first package pass.
func NewAnalyzer(cfg *Config) *analysis.Analyzer {
	d := &detector{cfg: cfg}
	return &analysis.Analyzer{
		Name: "deadcode",
		Doc:  doc,
		URL:  "https://github.com/katbyte/deadcode-golangci",
		Run:  d.run,
	}
}

// Analyzer is a ready-made instance for standalone drivers (e.g. singlechecker),
// configured via -patterns, -roots-test, -buildtags and -generated flags.
// (-test and -tags are taken by the analysis driver itself, whose flags apply to
// the driver's own package load, not this analyzer's whole-program scan.)
var Analyzer = func() *analysis.Analyzer {
	cfg := &Config{}
	a := NewAnalyzer(cfg)
	a.Flags.Func("patterns", "comma-separated package `patterns` containing the program entry points (default ./...)", func(s string) error {
		cfg.Patterns = strings.Split(s, ",")
		return nil
	})
	a.Flags.BoolVar(&cfg.Test, "roots-test", false, "include implicit test packages and executables as entry points")
	a.Flags.StringVar(&cfg.TreatFunctionsAsUsed, "treat-functions-as-used", "", "treat functions whose name matches this `regex` as used, i.e. as extra entry points (e.g. ^TestAcc); implies loading test packages, and functions declared in _test.go files are not reported")
	a.Flags.StringVar(&cfg.Tags, "buildtags", "", "comma-separated list of extra build `tags` for the whole-program scan")
	a.Flags.BoolVar(&cfg.Generated, "generated", false, "report dead functions declared in generated Go files")
	return a
}()

// position keys functions by the source position of their declared name, the
// only identity shared between the scan's FileSet and each driver pass's FileSet.
type position struct {
	file      string
	line, col int
}

type detector struct {
	cfg         *Config
	once        sync.Once
	err         error       // whole-program scan failure, reported by one pass only
	errReported atomic.Bool // whether a pass has reported err yet
	dead        map[position]string
}

func (d *detector) run(pass *analysis.Pass) (any, error) {
	d.once.Do(d.scan)
	if d.err != nil {
		// return the scan failure from a single pass rather than repeating it for
		// every package in the program
		if d.errReported.CompareAndSwap(false, true) {
			return nil, d.err
		}
		return nil, nil
	}

	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			posn := pass.Fset.Position(fn.Name.Pos())
			name, isDead := d.dead[position{posn.Filename, posn.Line, posn.Column}]
			if !isDead {
				continue
			}

			// the fix deletes the entire declaration, doc comment included
			start := fn.Pos()
			if fn.Doc != nil {
				start = fn.Doc.Pos()
			}

			pass.Report(analysis.Diagnostic{
				Pos:     fn.Name.Pos(),
				End:     fn.Name.End(),
				Message: "unreachable func: " + name,
				SuggestedFixes: []analysis.SuggestedFix{{
					Message:   "Remove unreachable func " + name,
					TextEdits: []analysis.TextEdit{{Pos: start, End: fn.End()}},
				}},
			})
		}
	}

	return nil, nil
}

// scan runs the whole-program analysis of cmd/deadcode: load the complete
// program, build SSA, compute RTA reachability from every main package's main
// and init, and record the declaration positions of unreachable source functions.
func (d *detector) scan() {
	patterns := d.cfg.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	var rootFuncs *regexp.Regexp
	if d.cfg.TreatFunctionsAsUsed != "" {
		re, err := regexp.Compile(d.cfg.TreatFunctionsAsUsed)
		if err != nil {
			d.err = fmt.Errorf("invalid treat-functions-as-used regex %q: %w", d.cfg.TreatFunctionsAsUsed, err)
			return
		}
		rootFuncs = re
	}

	cfg := &packages.Config{
		Mode:  packages.LoadAllSyntax | packages.NeedModule,
		Tests: d.cfg.Test || rootFuncs != nil,
		Dir:   d.cfg.Dir,
	}
	if d.cfg.Tags != "" {
		cfg.BuildFlags = []string{"-tags=" + d.cfg.Tags}
	}
	initial, err := packages.Load(cfg, patterns...)
	if err != nil {
		d.err = fmt.Errorf("loading program %v: %w", patterns, err)
		return
	}
	if len(initial) == 0 {
		d.err = fmt.Errorf("no packages match %v", patterns)
		return
	}
	for _, p := range initial {
		if len(p.Errors) > 0 {
			d.err = fmt.Errorf("packages contain errors, e.g. %w", p.Errors[0])
			return
		}
	}

	prog, pkgs := ssautil.AllPackages(initial, ssa.InstantiateGenerics)
	prog.Build()

	mains := ssautil.MainPackages(pkgs)
	if rootFuncs != nil && !d.cfg.Test {
		// drop synthesised test binaries ("pkg.test" main packages): in
		// treat-functions-as-used mode only the explicitly matched test functions are
		// roots, not the whole test harness - otherwise every test would count as an
		// entry point and nothing test-reachable could ever be reported
		mains = slices.DeleteFunc(mains, func(m *ssa.Package) bool {
			return strings.HasSuffix(m.Pkg.Path(), ".test")
		})
	}
	if len(mains) == 0 && rootFuncs == nil {
		d.err = fmt.Errorf("no main packages match %v: deadcode needs program entry points; point patterns at a module containing a main package, or set test: true to use test executables as entry points", patterns)
		return
	}
	roots := make([]*ssa.Function, 0, 2*len(mains))
	for _, main := range mains {
		roots = append(roots, main.Func("init"), main.Func("main"))
	}

	// Gather all source-level functions declared in the initial (module) packages —
	// dependencies are loaded for reachability but never reported, as the driver
	// only runs passes over the module being linted. Synthetic wrappers and nested
	// functions are ignored, as in cmd/deadcode: an unreachable function literal is
	// invariably unreachable because its parent is.
	type srcFunc struct {
		fn   *ssa.Function
		decl *ast.FuncDecl
		pkg  *types.Package
	}
	var (
		sourceFuncs    []srcFunc
		generated      = make(map[string]bool)
		interfaceTypes = make(map[*types.Package][]*types.Interface)
	)
	for _, p := range initial {
		// collect the package's named interface types for marker method identification
		var interfaces []*types.Interface
		scope := p.Types.Scope()
		for _, name := range scope.Names() {
			if typeName, ok := scope.Lookup(name).(*types.TypeName); ok {
				if iface, ok := typeName.Type().Underlying().(*types.Interface); ok {
					interfaces = append(interfaces, iface)
				}
			}
		}
		interfaceTypes[p.Types] = interfaces

		for _, file := range p.Syntax {
			inTestFile := strings.HasSuffix(p.Fset.File(file.Pos()).Name(), "_test.go")
			for _, decl := range file.Decls {
				fnDecl, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				obj, ok := p.TypesInfo.Defs[fnDecl.Name].(*types.Func)
				if !ok {
					continue
				}
				fn := prog.FuncValue(obj)
				if fn == nil {
					continue
				}
				if rootFuncs != nil {
					if rootFuncs.MatchString(fnDecl.Name.Name) {
						roots = append(roots, fn)
					}
					// functions declared in test files are go test's to invoke (unit
					// tests, benchmarks, their helpers) - they are roots or harness
					// scaffolding, never reportable dead code
					if inTestFile {
						continue
					}
				}
				sourceFuncs = append(sourceFuncs, srcFunc{fn, fnDecl, p.Types})
			}

			if ast.IsGenerated(file) {
				generated[p.Fset.File(file.Pos()).Name()] = true
			}
		}
	}

	if len(roots) == 0 {
		d.err = fmt.Errorf("no entry points: no main packages match %v and no functions match treat-functions-as-used %q", patterns, d.cfg.TreatFunctionsAsUsed)
		return
	}

	res := rta.Analyze(roots, false)

	// With Tests enabled the same source declaration appears in multiple package
	// variants ("p" and "p [p.test]") as distinct ssa.Functions; de-duplicate by
	// position, treating a declaration as live if any variant is live.
	reachablePosn := make(map[position]bool)
	for fn := range res.Reachable {
		if fn.Pos().IsValid() || fn.Name() == "init" {
			posn := prog.Fset.Position(fn.Pos())
			reachablePosn[position{posn.Filename, posn.Line, posn.Column}] = true
		}
	}

	d.dead = make(map[position]string)
	for _, sf := range sourceFuncs {
		posn := prog.Fset.Position(sf.fn.Pos())
		key := position{posn.Filename, posn.Line, posn.Column}
		if reachablePosn[key] {
			continue
		}
		reachablePosn[key] = true // suppress duplicates with the same position

		if generated[posn.Filename] && !d.cfg.Generated {
			continue
		}
		if isMarkerMethod(sf.fn, sf.decl, interfaceTypes[sf.pkg]) {
			continue
		}

		d.dead[key] = prettyName(sf.fn)
	}

	// warn fix runs that one pass is not enough: the scan is a snapshot, and deleting a
	// dead function can make the functions only it called newly dead. There is no
	// analyzer API to know fixes will be applied (the driver does that afterwards), so
	// sniff the command line (golangci-lint run --fix / standalone -fix).
	if len(d.dead) > 0 && slices.ContainsFunc(os.Args[1:], func(arg string) bool {
		return arg == "--fix" || arg == "-fix" || arg == "--fix=true" || arg == "-fix=true"
	}) {
		fmt.Fprintf(os.Stderr,
			"deadcode: removing %d unreachable functions - removals cascade (deleting a function can make its callees newly dead), so rerun the fix until it reports nothing, then delete any test files still referencing removed functions\n",
			len(d.dead))
	}
}

// prettyName reduces go/ssa's fussy punctuation as cmd/deadcode does,
// e.g. "(*pkg.T).F" -> "T.F". Only top-level functions and methods occur here.
func prettyName(fn *ssa.Function) string {
	name := fn.Name()
	if recv := fn.Signature.Recv(); recv != nil {
		t := types.Unalias(recv.Type())
		if ptr, ok := t.(*types.Pointer); ok {
			t = types.Unalias(ptr.Elem())
		}
		if named, ok := t.(*types.Named); ok {
			name = named.Obj().Name() + "." + name
		}
	}
	return name
}

// isMarkerMethod reports whether fn is a marker method: an unexported,
// empty-bodied method with no parameters or results that implements some named
// interface type in the same package. Such methods exist only to mark the type
// and are never called, so cmd/deadcode does not report them.
func isMarkerMethod(fn *ssa.Function, decl *ast.FuncDecl, interfaceTypes []*types.Interface) bool {
	if fn.Signature.Recv() == nil ||
		ast.IsExported(fn.Name()) ||
		fn.Signature.Params() != nil ||
		fn.Signature.Results() != nil {
		return false
	}

	if decl.Body == nil || len(decl.Body.List) > 0 {
		return false
	}

	return slices.ContainsFunc(interfaceTypes, func(iface *types.Interface) bool {
		return types.Implements(fn.Signature.Recv().Type(), iface)
	})
}

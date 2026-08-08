package plugin

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"
)

func TestBuildAnalyzers(t *testing.T) {
	t.Parallel()

	p, err := New(map[string]any{"patterns": []string{"./cmd/..."}, "test": true, "treat-functions-as-used": "^TestAcc", "tags": "integration", "generated": true})
	if err != nil {
		t.Fatal(err)
	}

	analyzers, err := p.BuildAnalyzers()
	if err != nil {
		t.Fatal(err)
	}
	if len(analyzers) != 1 || analyzers[0].Name != "deadcode" {
		t.Fatalf("expected the single deadcode analyzer, got %v", analyzers)
	}
}

func TestBuildAnalyzersDefaults(t *testing.T) {
	t.Parallel()

	p, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.BuildAnalyzers(); err != nil {
		t.Fatal(err)
	}
}

func TestBadSettings(t *testing.T) {
	t.Parallel()

	if _, err := New(map[string]any{"patterns": "not-a-list"}); err == nil {
		t.Fatal("expected an error decoding malformed settings")
	}
}

func TestLoadMode(t *testing.T) {
	t.Parallel()

	p, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	plug, ok := p.(*Plugin)
	if !ok {
		t.Fatalf("New returned %T, want *Plugin", p)
	}
	if mode := plug.GetLoadMode(); mode != register.LoadModeSyntax {
		t.Fatalf("load mode = %q, want %q", mode, register.LoadModeSyntax)
	}
}

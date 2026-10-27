package apalache

import (
	"context"
	"encoding/json"
	"testing"
)

const tinyModule = `---- MODULE Tiny ----
EXTENDS Integers
VARIABLE x
Init == x = 0
Next == x' = x + 1
====
`

func TestParseSourceExportsCanonicalApalacheIR(t *testing.T) {
	result := ParseSource(context.Background(), "Tiny.tla", tinyModule, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	if result.Module != "Tiny" {
		t.Fatalf("ParseSource() module = %q, want Tiny", result.Module)
	}

	var root struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Modules []struct {
			Name string `json:"name"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(result.IR, &root); err != nil {
		t.Fatalf("unmarshal IR: %v\n%s", err, result.IR)
	}
	if root.Name != "ApalacheIR" || root.Version != "1.0" {
		t.Fatalf("unexpected IR header: %#v", root)
	}
	if len(root.Modules) != 1 || root.Modules[0].Name != "Tiny" {
		t.Fatalf("unexpected IR modules: %#v", root.Modules)
	}
}

func TestParseSourceReportsSemanticErrors(t *testing.T) {
	result := ParseSource(context.Background(), "Bad.tla", `---- MODULE Bad ----
Op == missing
====
`, FrontendOptions{})
	if result.OK() {
		t.Fatal("ParseSource() succeeded, want semantic error")
	}
	if len(result.IR) != 0 {
		t.Fatalf("ParseSource() IR = %s, want none after error", result.IR)
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Severity != SeverityError {
		t.Fatalf("ParseSource() diagnostics = %#v", result.Diagnostics)
	}
}

func TestParseSourceHonorsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := ParseSource(ctx, "Tiny.tla", tinyModule, FrontendOptions{})
	if result.OK() || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "E-CANCELED" {
		t.Fatalf("ParseSource() = %#v, want cancellation", result)
	}
}

func TestParseFileReportsReadError(t *testing.T) {
	result := ParseFile(context.Background(), "testdata/does-not-exist.tla", FrontendOptions{})
	if result.OK() || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "E-IO" {
		t.Fatalf("ParseFile() = %#v, want I/O error", result)
	}
}

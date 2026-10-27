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
	if result.Native == nil || result.Native.Name() != "Tiny" {
		t.Fatalf("ParseSource() native module = %#v, want Tiny", result.Native)
	}
	if len(result.Native.VarDeclarations()) != 1 || len(result.Native.OperDeclarations()) != 2 {
		t.Fatalf("unexpected native declarations: %#v", result.Native.Declarations())
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

func TestParseSourcePopulatesNativeSourceLocations(t *testing.T) {
	result := ParseSource(context.Background(), "Tiny.tla", tinyModule, FrontendOptions{IncludeSource: true})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	if result.Native == nil || result.Sources == nil {
		t.Fatalf("missing native frontend result: %#v", result)
	}
	init := result.Native.OperDeclarations()[0]
	location, found := result.Sources.Find(init.ID())
	if !found || location.Filename != "Tiny" || location.Region.Start.Line != 4 {
		t.Fatalf("Init source location = %#v, %v", location, found)
	}
	if _, found := result.Sources.Find(init.Body().ID()); !found {
		t.Fatal("Init body has no source location")
	}
}

func TestParseSourceAcceptsApalacheAssignmentSyntax(t *testing.T) {
	result := ParseSource(context.Background(), "Assignments.tla", `---- MODULE Assignments ----
EXTENDS Apalache
VARIABLE x
Init == x := 0
Next == x' := 1
====
`, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	for _, name := range []string{"Init", "Next"} {
		body, found := OperatorBody(result.Native, name)
		operation, ok := body.(*OperExpr)
		if !found || !ok || operation.Operator().Name() != "EQ" {
			t.Fatalf("%s assignment body = %#v", name, body)
		}
	}
}

func TestParseAndTypeCheckSourceRecognizesImportedApalacheBuiltins(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "Builtins.tla", `---- MODULE Builtins ----
EXTENDS Apalache, Integers
Inv == LET Op(a, i) == a + i IN Repeat(Op, 2, 0) = 3
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Inv")
	if !found || countOperator(body, "Apalache!Repeat") != 1 {
		t.Fatalf("normalized Repeat body = %#v", body)
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

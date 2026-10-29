package apalache

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRecordingTypeCheckerListenerContracts(t *testing.T) {
	store := NewSourceStore()
	original := NewNameExpr("x", Untyped())
	location := SourceLocation{Filename: "Types", Region: NewSourceRegion(4, 2, 4, 8)}
	if err := store.Add(original.ID(), location); err != nil {
		t.Fatal(err)
	}
	transformed := NewNameExpr("x", Untyped())
	store.OnExpressionTransformation(original, transformed)

	listener := NewRecordingTypeCheckerListener(store)
	exact := ExactRef{SourceID: transformed.ID()}
	listener.OnTypeFound(exact, IntT1)
	listener.OnTypeFound(exact, BoolT1)
	listener.OnTypeError(BlameRef{SourceID: transformed.ID()}, "bad")
	listener.OnTypeWarning(exact, "maybe")
	listener.OnTypeError(BlameRef{SourceID: NewUID()}, "unknown")

	types := listener.Types()
	if typ := types[transformed.ID()]; !EqualTypes(typ, BoolT1) {
		t.Fatalf("recorded type = %v", typ)
	}
	delete(types, transformed.ID())
	if _, found := listener.Types()[transformed.ID()]; !found {
		t.Fatal("Types returned the listener's mutable map")
	}

	wantPosition := Position{Filename: "Types", Line: 4, Column: 2, EndLine: 4, EndColumn: 8}
	diagnostics := listener.Diagnostics()
	if len(diagnostics) != 3 || diagnostics[0].Message != "bad" || diagnostics[0].Position != wantPosition ||
		diagnostics[1].Message != "maybe" || diagnostics[1].Position != wantPosition ||
		diagnostics[2].Message != "unknown" || diagnostics[2].Position != (Position{}) {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
	if got := listener.Errors(); len(got) != 2 || got[0].Code != "E-TYPE" || got[1].Message != "unknown" {
		t.Fatalf("errors = %+v", got)
	}
	if got := listener.Warnings(); len(got) != 1 || got[0].Code != "W-TYPE" || got[0].Message != "maybe" {
		t.Fatalf("warnings = %+v", got)
	}
	diagnostics[0].Message = "mutated"
	if reflect.DeepEqual(diagnostics, listener.Diagnostics()) {
		t.Fatal("Diagnostics returned the listener's mutable slice")
	}
}

func TestTypeCheckModuleProducesTypedCopy(t *testing.T) {
	constant := NewConstDecl("N", Untyped())
	variable := NewVarDecl("x", Untyped())
	initBody := tlaOper("EQ", tlaName("x"), tlaName("N"))
	init := NewOperDecl("Init", nil, initBody, Untyped())
	module := NewModule("Typed", []Decl{constant, variable, init})
	module.AddProperty(PropertyDesugared)
	result := TypeCheckModule(module, TypeCheckOptions{Annotations: map[UID]Type{constant.ID(): IntT1, variable.ID(): IntT1}})
	if !result.OK() {
		t.Fatalf("TypeCheckModule() diagnostics = %+v", result.Diagnostics)
	}
	if !result.Module.HasProperty(PropertyTypeChecked) || !result.Module.HasProperty(PropertyDesugared) {
		t.Fatal("module properties were not preserved")
	}
	if module.HasProperty(PropertyTypeChecked) {
		t.Fatal("input module was mutated")
	}
	for _, declaration := range result.Module.Declarations() {
		if !declaration.Tag().IsTyped() {
			t.Fatalf("untyped declaration %s", declaration.Name())
		}
	}
	if typ, ok := result.Module.ConstDeclarations()[0].Tag().Type(); !ok || !EqualTypes(typ, IntT1) {
		t.Fatalf("constant type = %v, %v", typ, ok)
	}
	if typ, ok := result.Module.VarDeclarations()[0].Tag().Type(); !ok || !EqualTypes(typ, IntT1) {
		t.Fatalf("variable type = %v, %v", typ, ok)
	}
	typedInit := result.Module.OperDeclarations()[0]
	if typ, ok := typedInit.Body().Tag().Type(); !ok || !EqualTypes(typ, BoolT1) {
		t.Fatalf("Init body type = %v, %v", typ, ok)
	}
}

func TestTypeCheckerToolScalaResourceAndSerializationContracts(t *testing.T) {
	extensions := map[string][]string{
		"MegaSpec1": {"Integers", "Sequences", "FiniteSets", "Reals", "Apalache"},
		"TlcSpec1":  {"TLC"},
	}
	for _, name := range []string{"MegaSpec1", "TlcSpec1"} {
		t.Run(name, func(t *testing.T) {
			path := "../passes/src/test/resources/" + name + ".tla"
			frontend := ParseAndTypeCheckFile(context.Background(), path, FrontendOptions{}, TypeCheckOptions{})
			if !frontend.OK() {
				t.Fatalf("typecheck diagnostics: %+v", frontend.Diagnostics)
			}
			if typed := (TypedLanguagePredicate{}).CheckModule(frontend.Native); !typed.OK() {
				t.Fatalf("typechecker left untyped nodes: %+v", typed.Failures)
			}

			encoded, err := MarshalSingleModuleIRJSON(frontend.Native, IRJSONOptions{SourceStore: frontend.Sources})
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := UnmarshalSingleModuleIRJSON(encoded, NewSourceStore())
			if err != nil {
				t.Fatal(err)
			}
			assertModulesEqual(t, frontend.Native, decoded)

			rootDeclarations := make([]Decl, 0, len(frontend.Native.Declarations()))
			for _, declaration := range frontend.Native.Declarations() {
				location, found := frontend.Sources.Find(declaration.ID())
				if !found || filepath.Base(location.Filename) == name+".tla" {
					rootDeclarations = append(rootDeclarations, declaration)
				}
			}
			rootModule := NewModule(frontend.Native.Name(), rootDeclarations)
			formatted, err := FormatTLAModule(rootModule, TLAFormatOptions{
				ExtendedModules: extensions[name], Annotations: frontend.Annotations,
			})
			if err != nil {
				t.Fatal(err)
			}
			reparsed := ParseAndTypeCheckSource(context.Background(), name+"RoundTrip.tla", formatted,
				FrontendOptions{}, TypeCheckOptions{})
			if !reparsed.OK() {
				t.Fatalf("formatted TLA did not typecheck: %+v\n%s", reparsed.Diagnostics, formatted)
			}
		})
	}
}

func TestTypeCheckModuleSortsDeclarationsBeforeChecking(t *testing.T) {
	callee := NewOperDecl("Callee", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped())
	caller := NewOperDecl("Caller", nil,
		MustOperExpr(Op("OPER_APP"), Untyped(), NewNameExpr("Callee", Untyped())), Untyped())
	result := TypeCheckModule(NewModule("OutOfOrder", []Decl{caller, callee}), TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("TypeCheckModule() diagnostics = %+v", result.Diagnostics)
	}
	if names := declarationNames(result.Module); len(names) != 2 || names[0] != "Callee" || names[1] != "Caller" {
		t.Fatalf("sorted declarations = %v", names)
	}
}

func TestTypeCheckerVariablePoolStartsAboveExistingDeclarationTags(t *testing.T) {
	high, _ := NewVarType(2500)
	constant := NewConstDecl("Existing", Typed(high))
	localHigh, _ := NewVarType(4100)
	local := NewOperDecl("Local", nil, NewValueExpr(Int64Value(1), Untyped()), Typed(localHigh))
	body := NewLetInExpr(NewValueExpr(BoolValue(true), Untyped()), Untyped(), local)
	operator := NewOperDecl("Outer", nil, body, Untyped())
	module := NewModule("Pool", []Decl{constant, operator})
	if got := maxUsedTypeVariable(module); got != 4100 {
		t.Fatalf("maxUsedTypeVariable() = %d, want 4100", got)
	}
	if got := maxUsedTypeVariable(NewModule("Empty", nil)); got != -1 {
		t.Fatalf("empty maxUsedTypeVariable() = %d, want -1", got)
	}
	emptySet := tlaOper("SET_ENUM")
	checked := TypeCheckModule(NewModule("Above", []Decl{
		NewConstDecl("Existing", Typed(high)),
		NewOperDecl("Empty", nil, emptySet, Untyped()),
	}), TypeCheckOptions{})
	if !checked.OK() {
		t.Fatalf("automatic-start typecheck diagnostics = %+v", checked.Diagnostics)
	}
	inferred := checked.Types[emptySet.ID()]
	used := inferred.UsedNames()
	if len(used) != 1 || used[0] <= 2500 {
		t.Fatalf("empty-set type = %s with variables %v, want a variable above 2500", inferred, used)
	}

	// Explicit caller starts remain authoritative for deterministic embedding.
	result := TypeCheckModule(NewModule("Explicit", []Decl{NewOperDecl("Value", nil, tlaInt(1), Untyped())}),
		TypeCheckOptions{VariableStart: 7000})
	if !result.OK() {
		t.Fatalf("explicit-start typecheck diagnostics = %+v", result.Diagnostics)
	}
}

func TestTypeCheckModuleRejectsAnnotationRecordModeMismatch(t *testing.T) {
	declaration := NewConstDecl("R", Untyped())
	module := NewModule("Rows", []Decl{declaration})
	legacy := parsedType(t, "[field: Int]")
	row := parsedType(t, "{ field: Int }")
	tests := []struct {
		name    string
		useRows bool
		noRows  bool
		typ     Type
		want    string
	}{
		{"legacy in rows", true, false, legacy, "Found imprecise record types, while Type System 1.2 is enabled. Either update the annotations, or use --features=no-rows."},
		{"row in legacy", false, true, row, "Found row types, while Type System 1.2 is disabled. Do not use --features=no-rows."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := TypeCheckModule(module, TypeCheckOptions{UseRows: test.useRows, NoRows: test.noRows, Annotations: map[UID]Type{declaration.ID(): test.typ}})
			if result.OK() || len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != test.want {
				t.Fatalf("TypeCheckModule() = %+v", result)
			}
		})
	}
	if result := TypeCheckModule(module, TypeCheckOptions{NoRows: true, Annotations: map[UID]Type{declaration.ID(): legacy}}); !result.OK() {
		t.Fatalf("legacy annotation rejected in no-rows mode: %+v", result.Diagnostics)
	}
	if result := TypeCheckModule(module, TypeCheckOptions{UseRows: true, Annotations: map[UID]Type{declaration.ID(): row}}); !result.OK() {
		t.Fatalf("row annotation rejected in rows mode: %+v", result.Diagnostics)
	}
	for name, typ := range map[string]Type{"legacy": legacy, "row": row} {
		if result := TypeCheckModule(module, TypeCheckOptions{Annotations: map[UID]Type{declaration.ID(): typ}}); !result.OK() {
			t.Fatalf("%s annotation rejected in compatibility mode: %+v", name, result.Diagnostics)
		}
	}
	if result := TypeCheckModule(module, TypeCheckOptions{UseRows: true, NoRows: true}); result.OK() || len(result.Diagnostics) != 1 {
		t.Fatalf("conflicting row modes accepted: %+v", result)
	}
}

func TestTypeCheckModuleReportsSourceAwareFailure(t *testing.T) {
	variable := NewVarDecl("x", Untyped())
	bad := NewOperDecl("Bad", nil, tlaOper("PLUS", tlaName("x"), NewValueExpr(BoolValue(true), Untyped())), Untyped())
	module := NewModule("Bad", []Decl{variable, bad})
	store := NewSourceStore()
	location := SourceLocation{Filename: "Bad", Region: NewSourceRegion(3, 1, 3, 20)}
	if err := store.AddRecursive(bad.Body(), location); err != nil {
		t.Fatal(err)
	}
	result := TypeCheckModule(module, TypeCheckOptions{Annotations: map[UID]Type{variable.ID(): IntT1}, Sources: store})
	if result.OK() || len(result.Diagnostics) == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	found := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "E-TYPE" && diagnostic.Position.Filename == "Bad" && diagnostic.Position.Line == 3 {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing source-aware diagnostic: %+v", result.Diagnostics)
	}
}

func TestTypeRewriterPreservesSourcesAndTypesExceptIndices(t *testing.T) {
	index := tlaInt(1)
	path := tlaOper("TUPLE", index)
	function := tlaName("f")
	value := tlaInt(2)
	except := tlaOper("EXCEPT", function, path, value)
	store := NewSourceStore()
	location := SourceLocation{Filename: "M", Region: NewSourceRegion(1, 1, 1, 20)}
	if err := store.AddRecursive(except, location); err != nil {
		t.Fatal(err)
	}
	types := map[UID]Type{index.ID(): IntT1, function.ID(): mustFun(IntT1, IntT1), value.ID(): IntT1, except.ID(): mustFun(IntT1, IntT1)}
	rewritten, err := NewTypeRewriter(types, store).Expression(except)
	if err != nil {
		t.Fatal(err)
	}
	if !store.Contains(rewritten.ID()) {
		t.Fatal("rewritten expression lost source location")
	}
	newPath := rewritten.(*OperExpr).Arguments()[1]
	if typ, ok := newPath.Tag().Type(); !ok || !EqualTypes(typ, mustTuple(IntT1)) {
		t.Fatalf("EXCEPT path type = %v, %v", typ, ok)
	}
}

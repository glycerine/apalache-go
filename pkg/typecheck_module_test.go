package apalache

import (
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

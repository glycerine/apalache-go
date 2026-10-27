package apalache

import (
	"context"
	"strings"
	"testing"
)

const annotatedModule = `---- MODULE Annotated ----
EXTENDS Integers
\* @typeAlias: ITEM = Int;
CONSTANT
  \* @type: ITEM;
  N
VARIABLE
  \* @type: Set(ITEM);
  xs
\* @type: (Int) => Int;
Inc(x) == x + 1
Use == LET \* @type: (Int) => Int;
             AddOne(x) == x + 1
       IN AddOne(N)
====
`

func TestExtractTypeAnnotationsAttachesDeclarationsAndClosesAliases(t *testing.T) {
	frontend := ParseSource(context.Background(), "Annotated.tla", annotatedModule, FrontendOptions{IncludeSource: true})
	if !frontend.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", frontend.Diagnostics)
	}
	extracted := ExtractTypeAnnotations("Annotated.tla", annotatedModule, frontend.Native, frontend.Sources)
	if !extracted.OK() {
		t.Fatalf("ExtractTypeAnnotations() diagnostics = %v", extracted.Diagnostics)
	}
	aliases := extracted.Aliases.Context()
	if typ, found := aliases["ITEM"]; !found || !EqualTypes(typ, IntT1) {
		t.Fatalf("closed aliases = %#v", aliases)
	}
	constant := frontend.Native.ConstDeclarations()[0]
	variable := frontend.Native.VarDeclarations()[0]
	operators := frontend.Native.OperDeclarations()
	if !EqualTypes(extracted.Types[constant.ID()], mustConst(t, "ITEM")) {
		t.Fatalf("constant annotation = %v", extracted.Types[constant.ID()])
	}
	wantSet, _ := NewSetType(mustConst(t, "ITEM"))
	if !EqualTypes(extracted.Types[variable.ID()], wantSet) {
		t.Fatalf("variable annotation = %v", extracted.Types[variable.ID()])
	}
	if _, found := extracted.Annotations.Find(operators[0].ID(), "type"); !found {
		t.Fatal("Inc has no attached type annotation")
	}
	let, ok := operators[1].Body().(*LetInExpr)
	if !ok || len(let.Declarations()) != 1 {
		t.Fatalf("Use body = %T %#v", operators[1].Body(), operators[1].Body())
	}
	local := let.Declarations()[0]
	if _, found := extracted.Types[local.ID()]; !found {
		t.Fatalf("local declaration %s has no type annotation", local.Name())
	}
}

func TestExtractTypeAnnotationsIgnoresImportedSourceLocations(t *testing.T) {
	source := `---- MODULE Root ----
\* @type: Int;
Root == 1
====
`
	imported := NewOperDecl("Imported", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped())
	root := NewOperDecl("Root", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped())
	module := NewModule("Root", []Decl{imported, root})
	store := NewSourceStore()
	if err := store.Add(imported.ID(), SourceLocation{Filename: "Imported.tla", Region: NewSourceRegion(2, 1, 2, 12)}); err != nil {
		t.Fatal(err)
	}
	if err := store.Add(root.ID(), SourceLocation{Filename: "Root.tla", Region: NewSourceRegion(3, 1, 3, 9)}); err != nil {
		t.Fatal(err)
	}

	extracted := ExtractTypeAnnotations("Root.tla", source, module, store)
	if !extracted.OK() {
		t.Fatalf("ExtractTypeAnnotations() diagnostics = %v", extracted.Diagnostics)
	}
	if _, found := extracted.Types[imported.ID()]; found {
		t.Fatal("root annotation attached to imported declaration")
	}
	if typ, found := extracted.Types[root.ID()]; !found || !EqualTypes(typ, IntT1) {
		t.Fatalf("root annotation = %v, %v", typ, found)
	}
}

func TestParseAndTypeCheckSourceUsesCommentAnnotations(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "Annotated.tla", annotatedModule, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	if result.Native == nil || !result.Native.HasProperty(PropertyTypeChecked) {
		t.Fatalf("typed module = %#v", result.Native)
	}
	constant := result.Native.ConstDeclarations()[0]
	constantType, ok := constant.Tag().Type()
	if !ok || !EqualTypes(constantType, IntT1) {
		t.Fatalf("constant %s/%s type = %v, %v; inferred=%v", constant.Name(), constant.ID(), constantType, ok, result.Types)
	}
	variableType, ok := result.Native.VarDeclarations()[0].Tag().Type()
	wantSet, _ := NewSetType(IntT1)
	if !ok || !EqualTypes(variableType, wantSet) {
		t.Fatalf("variable type = %v, %v", variableType, ok)
	}
}

func TestExtractTypeAnnotationsReportsSourceAwareErrors(t *testing.T) {
	source := `---- MODULE BadAnnotation ----
\* @type: Not a type!;
CONSTANT N
\* @typeAlias: A = B;
\* @typeAlias: B = A;
====
`
	frontend := ParseSource(context.Background(), "BadAnnotation.tla", source, FrontendOptions{IncludeSource: true})
	if !frontend.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", frontend.Diagnostics)
	}
	extracted := ExtractTypeAnnotations("BadAnnotation.tla", source, frontend.Native, frontend.Sources)
	if extracted.OK() || len(extracted.Diagnostics) < 2 {
		t.Fatalf("ExtractTypeAnnotations() = %#v", extracted)
	}
	if extracted.Diagnostics[0].Position.Filename != "BadAnnotation.tla" || extracted.Diagnostics[0].Position.Line != 2 {
		t.Fatalf("type diagnostic position = %#v", extracted.Diagnostics[0].Position)
	}
	foundCycle := false
	for _, diagnostic := range extracted.Diagnostics {
		if diagnostic.Code == "E-TYPE-ALIAS" && strings.Contains(diagnostic.Message, "cyclic") {
			foundCycle = true
		}
	}
	if !foundCycle {
		t.Fatalf("missing alias cycle diagnostic: %#v", extracted.Diagnostics)
	}
}

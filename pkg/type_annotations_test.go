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

func TestExtractTypeAnnotationsIncludesNestedModulesInSource(t *testing.T) {
	source := `---- MODULE Root ----
---- MODULE Nested ----
\* @type: Int;
NestedValue == 1
====
\* @type: Bool;
RootValue == TRUE
====
`
	nested := NewOperDecl("NestedValue", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped())
	root := NewOperDecl("RootValue", nil, NewValueExpr(BoolValue(true), Untyped()), Untyped())
	module := NewModule("Root", []Decl{nested, root})
	store := NewSourceStore()
	if err := store.Add(nested.ID(), SourceLocation{Filename: "Nested", Region: NewSourceRegion(4, 1, 4, 16)}); err != nil {
		t.Fatal(err)
	}
	if err := store.Add(root.ID(), SourceLocation{Filename: "Root", Region: NewSourceRegion(7, 1, 7, 17)}); err != nil {
		t.Fatal(err)
	}

	extracted := ExtractTypeAnnotations("Root.tla", source, module, store)
	if !extracted.OK() {
		t.Fatalf("ExtractTypeAnnotations() diagnostics = %v", extracted.Diagnostics)
	}
	if typ := extracted.Types[nested.ID()]; !EqualTypes(typ, IntT1) {
		t.Fatalf("nested annotation = %v", typ)
	}
	if typ := extracted.Types[root.ID()]; !EqualTypes(typ, BoolT1) {
		t.Fatalf("root annotation = %v", typ)
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

func TestExtractTypeAnnotationsIgnoresMissingSemicolon(t *testing.T) {
	source := `---- MODULE MissingSemicolon ----
\* last action performed
\* @type: ACTION
action == TRUE
====
`
	frontend := ParseSource(context.Background(), "MissingSemicolon.tla", source, FrontendOptions{IncludeSource: true})
	if !frontend.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", frontend.Diagnostics)
	}
	extracted := ExtractTypeAnnotations("MissingSemicolon.tla", source, frontend.Native, frontend.Sources)
	if !extracted.OK() {
		t.Fatalf("unterminated annotation should be ignored, diagnostics = %v", extracted.Diagnostics)
	}
	action, found := declarationNamed(frontend.Native, "action")
	if !found {
		t.Fatal("missing action declaration")
	}
	if _, attached := extracted.Annotations.Find(action.ID(), "type"); attached {
		t.Fatal("unterminated type annotation was attached")
	}
	if _, typed := extracted.Types[action.ID()]; typed {
		t.Fatal("unterminated type annotation produced a type")
	}
}

func TestParseAndTypeCheckSourceScopesAnnotatedRootLocalOperator(t *testing.T) {
	source := `---- MODULE AnnotatedLocal ----
EXTENDS Integers
(*
  @pure
  @type: (Int) => Int;
*)
LOCAL Inc(x) == x + 1
A(n) == Inc(n)
====
`
	result := ParseAndTypeCheckSource(context.Background(), "AnnotatedLocal.tla", source, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	if _, retained := declarationNamed(result.Native, "Inc"); !retained {
		t.Fatalf("root LOCAL Inc was not retained: %v", declarationNames(result.Native))
	}
	body, found := OperatorBody(result.Native, "A")
	let, ok := body.(*LetInExpr)
	if !found || !ok || len(let.Declarations()) != 1 {
		t.Fatalf("A body = %#v, want one scoped LET declaration", body)
	}
	local := let.Declarations()[0]
	if !strings.HasPrefix(local.Name(), "LOCAL$") || !strings.HasSuffix(local.Name(), "Inc") {
		t.Fatalf("local declaration = %s, want unique LOCAL Inc name", local.Name())
	}
	if _, found := result.Annotations.Find(local.ID(), "pure"); !found {
		t.Fatal("scoped Inc lost @pure annotation")
	}
	wantType, err := NewOperatorType([]Type{IntT1}, IntT1)
	if err != nil {
		t.Fatal(err)
	}
	if typ, found := local.Tag().Type(); !found || !EqualTypes(typ, wantType) {
		t.Fatalf("scoped Inc type = %v, %v", typ, found)
	}
}

func TestExtractTypeAnnotationsAttachesCustomAndRecursiveAnnotations(t *testing.T) {
	source := `---- MODULE AnnotationTargets ----
EXTENDS Integers
CONSTANT
(*
  @awesome
  @type("Set(Int)")
*)
S
VARIABLE
\* @amazing
\* @type: Set(Int);
V
RECURSIVE Fact(_)
(*
  @tailrec
  @type("(Int) => Int")
*)
Fact(n) == IF n <= 1 THEN 1 ELSE n * Fact(n - 1)
\* @type: Int -> Int;
Rec[n \in Int] == IF n <= 1 THEN 1 ELSE n * Rec[n - 1]
====
`
	frontend := ParseSource(context.Background(), "AnnotationTargets.tla", source, FrontendOptions{IncludeSource: true})
	if !frontend.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", frontend.Diagnostics)
	}
	extracted := ExtractTypeAnnotations("AnnotationTargets.tla", source, frontend.Native, frontend.Sources)
	if !extracted.OK() {
		t.Fatalf("ExtractTypeAnnotations() diagnostics = %v", extracted.Diagnostics)
	}
	for name, annotationName := range map[string]string{
		"S": "awesome", "V": "amazing", "Fact": "tailrec",
	} {
		declaration, found := declarationNamed(frontend.Native, name)
		if !found {
			t.Fatalf("missing %s declaration", name)
		}
		if _, found := extracted.Annotations.Find(declaration.ID(), annotationName); !found {
			t.Errorf("%s lost @%s annotation", name, annotationName)
		}
		if _, found := extracted.Types[declaration.ID()]; !found {
			t.Errorf("%s lost @type annotation", name)
		}
	}
	recursiveFunction, found := declarationNamed(frontend.Native, "Rec")
	if !found {
		t.Fatal("missing Rec declaration")
	}
	if _, found := extracted.Types[recursiveFunction.ID()]; !found {
		t.Fatal("recursive function lost @type annotation")
	}
}

func TestRemapDeclarationAnnotationsSurvivesSortingAndNestedLets(t *testing.T) {
	oldLocal := NewOperDecl("Local", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped())
	oldB := NewOperDecl("B", nil, NewLetInExpr(NewNameExpr("Local", Untyped()), Untyped(), oldLocal), Untyped())
	oldA := NewOperDecl("A", nil, NewValueExpr(BoolValue(true), Untyped()), Untyped())
	from := NewModule("M", []Decl{oldB, oldA})

	newA := NewOperDecl("A", nil, NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(BoolT1))
	newLocal := NewOperDecl("Local", nil, NewValueExpr(Int64Value(1), Typed(IntT1)), Typed(IntT1))
	newB := NewOperDecl("B", nil, NewLetInExpr(NewNameExpr("Local", Untyped()), Typed(IntT1), newLocal), Typed(IntT1))
	to := NewModule("M", []Decl{newA, newB})

	annotations := NewAnnotationStore()
	annotations.Set(oldB.ID(), []Annotation{{Name: "top"}})
	annotations.Set(oldLocal.ID(), []Annotation{{Name: "nested"}})
	remapped := remapDeclarationAnnotations(from, to, annotations)
	if _, found := remapped.Find(newB.ID(), "top"); !found {
		t.Fatal("top-level annotation was lost after declaration sorting")
	}
	if _, found := remapped.Find(newLocal.ID(), "nested"); !found {
		t.Fatal("nested annotation was lost after declaration sorting")
	}
}

package apalache

import "testing"

func TestTypeSubstitutorRewritesCompleteExpressionTree(t *testing.T) {
	a, _ := NewVarType(1)
	substitution, err := NewSubstitution(SubstitutionEntry{Class: NewEqClass(1), Type: IntT1})
	if err != nil {
		t.Fatal(err)
	}
	setA := mustSet(a)
	local := NewOperDecl("Local", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(a)), Typed(mustOperator([]Type{a}, a)))
	body := MustOperExpr(Op("SET_ENUM"), Typed(setA), NewNameExpr("v", Typed(a)))
	root := NewLetInExpr(body, Typed(setA), local)
	got, err := NewTypeSubstitutor(nil, substitution).Expression(root)
	if err != nil {
		t.Fatal(err)
	}
	let := got.(*LetInExpr)
	if typ, _ := let.Tag().Type(); !EqualTypes(typ, mustSet(IntT1)) {
		t.Fatalf("LET type = %v", typ)
	}
	if typ, _ := let.Body().(*OperExpr).Arguments()[0].Tag().Type(); !EqualTypes(typ, IntT1) {
		t.Fatalf("nested name type = %v", typ)
	}
	wantLocalType := mustOperator([]Type{IntT1}, IntT1)
	if typ, _ := let.Declarations()[0].Tag().Type(); !EqualTypes(typ, wantLocalType) {
		t.Fatalf("local type = %v", typ)
	}
	if root.ID() == got.ID() || local.ID() == let.Declarations()[0].ID() {
		t.Fatal("substitution reused IR identities")
	}
}

func TestTypeSubstitutorRewritesAllDeclarationKindsAndPreservesProperties(t *testing.T) {
	a, _ := NewVarType(2)
	substitution, _ := NewSubstitution(SubstitutionEntry{Class: NewEqClass(2), Type: BoolT1})
	value := NewNameExpr("p", Typed(a))
	module := NewModule("M", []Decl{
		NewConstDecl("C", Typed(a)), NewVarDecl("v", Typed(a)),
		NewNamedAssumeDecl("A", value, Typed(a)), NewTheoremDecl("T", value, Typed(a)),
	})
	module.AddProperty(PropertyTypeChecked)
	got, err := NewTypeSubstitutor(nil, substitution).Module(module)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyTypeChecked) {
		t.Fatal("module property lost")
	}
	for _, declaration := range got.Declarations() {
		if typ, ok := declaration.Tag().Type(); !ok || !EqualTypes(typ, BoolT1) {
			t.Fatalf("%s type = %v", declaration.Name(), typ)
		}
	}
	if typ, _ := got.AssumeDeclarations()[0].Body().Tag().Type(); !EqualTypes(typ, BoolT1) {
		t.Fatalf("assumption body type = %v", typ)
	}
}

func TestTypeSubstitutorLeavesUntypedTagsUntyped(t *testing.T) {
	substitutor := NewTypeSubstitutor(nil, EmptySubstitution())
	got, err := substitutor.Expression(NewNameExpr("x", Untyped()))
	if err != nil || got.Tag().IsTyped() {
		t.Fatalf("untyped substitution = %v, %v", got.Tag(), err)
	}
}

package apalache

import (
	"strings"
	"testing"
)

func TestExpressionConstructionAndRetagging(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	args := []Expr{x, one}
	equal, err := NewOperExpr(Op("EQ"), Typed(BoolT1), args...)
	if err != nil {
		t.Fatal(err)
	}
	args[0] = one
	if equal.Arguments()[0] != x {
		t.Fatal("operator expression retained caller's argument slice")
	}
	gotArgs := equal.Arguments()
	gotArgs[0] = one
	if equal.Arguments()[0] != x {
		t.Fatal("operator expression exposed its argument slice")
	}

	retagged := equal.WithTag(Untyped())
	if equal.ID() == retagged.ID() || !EqualExpr(equal, retagged) || EqualTypedExpr(equal, retagged) {
		t.Fatal("retagging did not make a structurally equal expression with a fresh ID and distinct tag")
	}
	if NullExpr().WithTag(Typed(IntT1)) != NullExpr() || NullExpr().Tag().IsTyped() {
		t.Fatal("null expression must be an always-untyped singleton")
	}
}

func TestOperatorExpressionInvariants(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	if _, err := NewOperExpr(Op("EQ"), Untyped(), x); err == nil || !strings.Contains(err.Error(), "unexpected arity") {
		t.Fatalf("wrong-arity error = %v", err)
	}
	if _, err := NewOperExpr(Op("Sequences!SelectSeq"), Untyped(), x, x); err == nil || !strings.Contains(err.Error(), "cannot be constructed") {
		t.Fatalf("prohibited-operator error = %v", err)
	}
	label := NewValueExpr(StringValue("label"), Untyped())
	argument := NewValueExpr(StringValue("arg"), Untyped())
	if _, err := NewOperExpr(Op("LABEL"), Untyped(), x, label, argument); err != nil {
		t.Fatalf("valid label: %v", err)
	}
	if _, err := NewOperExpr(Op("LABEL"), Untyped(), x, NewNameExpr("not-a-string-literal", Untyped())); err == nil {
		t.Fatal("LABEL accepted a non-string label")
	}
}

func TestLetInAndDeclarationSemantics(t *testing.T) {
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	params := []OperatorParam{ValueParam("x")}
	decl := NewOperDecl("One", params, one, Typed(IntT1))
	params[0].Name = "changed"
	if decl.FormalParams()[0].Name != "x" {
		t.Fatal("operator declaration retained caller's parameter slice")
	}
	decl.SetRecursive(true)
	copy := decl.WithTag(Untyped()).(*OperDecl)
	if !copy.IsRecursive() || copy.ID() == decl.ID() || !EqualDecl(decl, copy) || EqualTypedDecl(decl, copy) {
		t.Fatal("operator declaration copy semantics differ from Scala")
	}

	letDecls := []*OperDecl{decl}
	let := NewLetInExpr(NewNameExpr("One", Untyped()), Untyped(), letDecls...)
	letDecls[0] = copy
	if let.Declarations()[0] != decl {
		t.Fatal("LET-IN retained caller's declaration slice")
	}
	if !EqualExpr(let, let.WithTag(Typed(IntT1))) {
		t.Fatal("expression structural equality unexpectedly includes its type tag")
	}
}

func TestAssumptionNamesAndModuleViews(t *testing.T) {
	body := MustOperExpr(Op("EQ"), Typed(BoolT1),
		NewNameExpr("x", Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1)))
	unnamed := NewUnnamedAssumeDecl(body, Typed(BoolT1))
	if unnamed.Name() != "ASSUME"+body.ID().String() {
		t.Fatalf("unnamed assumption name = %q", unnamed.Name())
	}
	named := NewNamedAssumeDecl("InitIsOne", body, Typed(BoolT1))
	if name, ok := named.DefinedName(); !ok || name != "InitIsOne" || named.Name() != name {
		t.Fatalf("named assumption = %q, %v", name, ok)
	}

	constant := NewConstDecl("N", Typed(IntT1))
	variable := NewVarDecl("x", Typed(IntT1))
	operator := NewOperDecl("Init", nil, body, Typed(BoolT1))
	theorem := NewTheoremDecl("Safety", body, Typed(BoolT1))
	input := []Decl{constant, variable, operator, unnamed, theorem}
	module := NewModule("Counter", input)
	input[0] = theorem
	if len(module.ConstDeclarations()) != 1 || len(module.VarDeclarations()) != 1 || len(module.OperDeclarations()) != 1 || len(module.AssumeDeclarations()) != 1 {
		t.Fatal("module declaration views do not match declaration kinds")
	}
	if module.Declarations()[0] != constant {
		t.Fatal("module retained caller's declaration slice")
	}
	module.AddProperty(PropertyTypeChecked)
	if !module.HasProperty(PropertyTypeChecked) || module.HasProperty(PropertyInlined) {
		t.Fatal("module properties not tracked independently")
	}
}

func TestStructuralEqualityIncludesChildrenButNotIdentity(t *testing.T) {
	left := MustOperExpr(Op("PLUS"), Untyped(),
		NewValueExpr(Int64Value(1), Untyped()), NewNameExpr("x", Untyped()))
	right := MustOperExpr(Op("PLUS"), Untyped(),
		NewValueExpr(Int64Value(1), Untyped()), NewNameExpr("x", Untyped()))
	different := MustOperExpr(Op("MINUS"), Untyped(),
		NewValueExpr(Int64Value(1), Untyped()), NewNameExpr("x", Untyped()))
	if left.ID() == right.ID() || !EqualExpr(left, right) || !EqualTypedExpr(left, right) {
		t.Fatal("equivalent expression trees are not structurally equal")
	}
	if EqualExpr(left, different) {
		t.Fatal("different operators compare equal")
	}
}

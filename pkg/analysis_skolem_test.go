package apalache

import "testing"

func TestMarkSkolemizableExpressionTracksPositivePolarity(t *testing.T) {
	setType := mustSet(IntT1)
	binder := NewNameExpr("i", Typed(IntT1))
	exists := func() Expr {
		return MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder,
			MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1)),
			MustOperExpr(Op("EQ"), Typed(BoolT1), binder, intLiteral(1)))
	}
	expression := MustOperExpr(Op("AND"), Typed(BoolT1),
		exists(),
		MustOperExpr(Op("NOT"), Typed(BoolT1), exists()),
		MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), exists(), exists(), exists()))
	marked := MarkSkolemizableExpression(expression, nil).(*OperExpr)
	if marked.Arguments()[0].(*OperExpr).Operator().Name() != "Apalache!Skolem" {
		t.Fatal("positive existential was not marked")
	}
	negated := marked.Arguments()[1].(*OperExpr).Arguments()[0].(*OperExpr)
	if negated.Operator().Name() != "EXISTS3" {
		t.Fatal("existential below NOT was unsafely marked")
	}
	conditional := marked.Arguments()[2].(*OperExpr)
	if conditional.Arguments()[0].(*OperExpr).Operator().Name() != "EXISTS3" ||
		conditional.Arguments()[1].(*OperExpr).Operator().Name() != "Apalache!Skolem" ||
		conditional.Arguments()[2].(*OperExpr).Operator().Name() != "Apalache!Skolem" {
		t.Fatalf("conditional marking = %v", conditional)
	}
}

func TestMarkSkolemizableModuleMarksCardinalityAndProperty(t *testing.T) {
	setType := mustSet(IntT1)
	set := MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1))
	cardinality := MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), set)
	body := MustOperExpr(Op("GE"), Typed(BoolT1), cardinality, intLiteral(1))
	module := NewModule("M", []Decl{boolDefinition("P", body)})
	marked, err := MarkSkolemizableModule(module, nil)
	if err != nil {
		t.Fatal(err)
	}
	markedBody, _ := OperatorBody(marked, "P")
	if !marked.HasProperty(PropertyAnalyzed) || markedBody.(*OperExpr).Operator().Name() != "Apalache!ConstCardinality" {
		t.Fatalf("marked module = %v, %v", marked.HasProperty(PropertyAnalyzed), markedBody)
	}
}

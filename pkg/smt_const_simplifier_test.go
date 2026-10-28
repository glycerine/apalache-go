package apalache

import "testing"

func TestSMTConstantSimplifierRecognizesArenaBooleans(t *testing.T) {
	simplifier := SMTConstantSimplifier{}
	falseCell := NewNameExpr(ArenaFalseName, Typed(BoolT1))
	trueCell := NewNameExpr(ArenaTrueName, Typed(BoolT1))
	ordinary := NewNameExpr("x", Typed(BoolT1))
	if !simplifier.IsFalseConstant(falseCell) || simplifier.IsTrueConstant(falseCell) ||
		!simplifier.IsTrueConstant(trueCell) || simplifier.IsFalseConstant(trueCell) ||
		!simplifier.IsBoolConstant(falseCell) || !simplifier.IsBoolConstant(trueCell) ||
		simplifier.IsBoolConstant(ordinary) {
		t.Fatal("SMT Boolean constant classification differs from arena semantics")
	}
	for _, test := range []struct {
		input Expr
		want  bool
	}{
		{input: falseCell, want: false},
		{input: trueCell, want: true},
		{input: boolExpr(false), want: false},
		{input: boolExpr(true), want: true},
	} {
		got, err := simplifier.SimplifyShallow(test.input)
		if err != nil {
			t.Fatal(err)
		}
		if value, ok := boolConstant(got); !ok || value != test.want {
			t.Fatalf("SimplifyShallow(%#v) = %#v", test.input, got)
		}
	}
}

func TestSMTConstantSimplifierUsesArenaBooleansInConditionals(t *testing.T) {
	simplifier := SMTConstantSimplifier{}
	thenExpression := intLiteral(1)
	elseExpression := intLiteral(2)
	for _, test := range []struct {
		predicate Expr
		want      Expr
	}{
		{predicate: NewNameExpr(ArenaTrueName, Typed(BoolT1)), want: thenExpression},
		{predicate: NewNameExpr(ArenaFalseName, Typed(BoolT1)), want: elseExpression},
	} {
		expression := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1),
			test.predicate, thenExpression, elseExpression)
		got, err := simplifier.SimplifyShallow(expression)
		if err != nil {
			t.Fatal(err)
		}
		if !EqualExpr(got, test.want) {
			t.Fatalf("conditional with %v = %#v", test.predicate, got)
		}
	}
}

func TestSMTConstantSimplifierPreservesEveryMembershipForm(t *testing.T) {
	simplifier := SMTConstantSimplifier{}
	element := MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1), intLiteral(1))
	set := NewNameExpr("S", Typed(mustSet(IntT1)))
	for _, operator := range []string{
		"SET_IN", "SET_NOT_IN", "Apalache!SelectInSet", "Apalache!SelectInFun",
		"Apalache!StoreInSet", "Apalache!storeNotInSet", "Apalache!storeNotInFun",
	} {
		expression := MustOperExpr(Op(operator), Typed(BoolT1), element, set)
		got, err := simplifier.SimplifyShallow(expression)
		if err != nil {
			t.Fatal(err)
		}
		if !EqualExpr(got, expression) {
			t.Fatalf("%s changed to %#v", operator, got)
		}
	}
}

func TestSMTConstantSimplifierDelegatesWithoutRecursing(t *testing.T) {
	simplifier := SMTConstantSimplifier{}
	if got, err := simplifier.SimplifyShallow(
		MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1), intLiteral(2))); err != nil || !EqualExpr(got, intLiteral(3)) {
		t.Fatalf("constant addition = %#v, %v", got, err)
	}
	nested := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1), intLiteral(1)), intLiteral(2))
	got, err := simplifier.SimplifyShallow(nested)
	if err != nil {
		t.Fatal(err)
	}
	if !EqualExpr(got, nested) {
		t.Fatalf("shallow simplifier recursed: %#v", got)
	}
}

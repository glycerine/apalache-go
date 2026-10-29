package apalache

import "testing"

func TestOptimizeLetInKeepsValuesAndOperatorsWithoutLet(t *testing.T) {
	values := []Expr{
		NewValueExpr(BoolValue(true), Typed(BoolT1)),
		intLiteral(42),
		NewValueExpr(StringValue("foo"), Typed(StrT1)),
	}
	for _, value := range values {
		result := OptimizeLetIn(TouchExpr{Expr: value, Changed: true}, nil)
		if result.Changed || result.Expr != value {
			t.Fatalf("value was changed: %#v", result)
		}
	}
	operator := MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1),
		MustOperExpr(Op("MULT"), Typed(IntT1), intLiteral(2), intLiteral(3)))
	result := OptimizeLetIn(TouchExpr{Expr: operator, Changed: true}, nil)
	if result.Changed || result.Expr != operator {
		t.Fatalf("LET-free operator was changed: %#v", result)
	}
}

func TestOptimizeLetInScalaCases(t *testing.T) {
	stringOperator := mustOperator(nil, StrT1)
	declaration := func(name, value string) *OperDecl {
		return NewOperDecl(name, nil, NewValueExpr(StringValue(value), Typed(StrT1)), Typed(stringOperator))
	}
	application := func(name string) Expr {
		return MustOperExpr(Op("OPER_APP"), Typed(StrT1), NewNameExpr(name, Typed(stringOperator)))
	}
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	unused := declaration("Unused", "foo")
	removed := OptimizeLetIn(TouchExpr{Expr: NewLetInExpr(one, Typed(IntT1), unused), Changed: true}, nil)
	if !removed.Changed || !EqualTypedExpr(removed.Expr, one) {
		t.Fatalf("unused LET = %#v", removed)
	}

	used := declaration("Used", "bar")
	keptInput := NewLetInExpr(application("Used"), Typed(StrT1), used)
	kept := OptimizeLetIn(TouchExpr{Expr: keptInput, Changed: true}, nil)
	if kept.Changed || kept.Expr != keptInput {
		t.Fatalf("used LET = %#v", kept)
	}

	mixedInput := NewLetInExpr(application("Used"), Typed(StrT1), unused, used)
	mixed := OptimizeLetIn(TouchExpr{Expr: mixedInput, Changed: true}, nil)
	want := NewLetInExpr(application("Used"), Typed(StrT1), used)
	if !mixed.Changed || !EqualTypedExpr(mixed.Expr, want) {
		t.Fatalf("mixed LET = %v", mixed.Expr)
	}

	base := declaration("Base", "base")
	middle := NewOperDecl("Middle", nil, application("Base"), Typed(stringOperator))
	top := NewOperDecl("Top", nil, application("Middle"), Typed(stringOperator))
	chain := NewLetInExpr(application("Top"), Typed(StrT1), base, middle, top)
	retained := OptimizeLetIn(TouchExpr{Expr: chain, Changed: true}, nil)
	if retained.Changed || retained.Expr != chain {
		t.Fatalf("transitive LET dependencies were removed: %v", retained.Expr)
	}
}

func TestOptimizeLetInNestedAndLambdaForms(t *testing.T) {
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	two := NewValueExpr(Int64Value(2), Typed(IntT1))
	unused := NewOperDecl("Unused", nil, NewValueExpr(StringValue("foo"), Typed(StrT1)), Typed(mustOperator(nil, StrT1)))
	nested := NewLetInExpr(two, Typed(IntT1), unused)
	root := MustOperExpr(Op("PLUS"), Typed(IntT1), one, nested)
	result := OptimizeLetIn(TouchExpr{Expr: root, Changed: true}, nil)
	if !result.Changed || !EqualTypedExpr(result.Expr, MustOperExpr(Op("PLUS"), Typed(IntT1), one, two)) {
		t.Fatalf("nested LET = %v", result.Expr)
	}

	functionType := mustOperator([]Type{IntT1}, BoolT1)
	function := NewOperDecl("f", []OperatorParam{ValueParam("x")}, NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(functionType))
	lambda := NewLetInExpr(NewNameExpr("f", Typed(functionType)), Typed(functionType), function)
	lambdaResult := OptimizeLetIn(TouchExpr{Expr: lambda, Changed: true}, nil)
	if lambdaResult.Changed || lambdaResult.Expr != lambda {
		t.Fatalf("lambda LET changed: %#v", lambdaResult)
	}
}

func TestOptimizeLetInHonorsUntouchedInputAndTracksChanges(t *testing.T) {
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	unused := NewOperDecl("Unused", nil, one, Typed(mustOperator(nil, IntT1)))
	root := NewLetInExpr(one, Typed(IntT1), unused)
	listener := &recordingListener{}
	if result := OptimizeLetIn(TouchExpr{Expr: root}, NewListenerTracker(listener)); result.Changed || result.Expr != root {
		t.Fatalf("untouched input = %#v", result)
	}
	result := OptimizeLetIn(TouchExpr{Expr: root, Changed: true}, NewListenerTracker(listener))
	if !result.Changed || len(listener.expressions) == 0 {
		t.Fatalf("tracked result = %#v, events = %d", result, len(listener.expressions))
	}
}

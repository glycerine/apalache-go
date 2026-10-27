package apalache

import "testing"

func TestPropagatePrimesBasicCases(t *testing.T) {
	states := map[string]struct{}{"x": {}, "y": {}}
	x := NewNameExpr("x", Typed(IntT1))
	constant := NewNameExpr("N", Typed(IntT1))
	prime := func(expression Expr) Expr { return MustOperExpr(Op("PRIME"), expression.Tag(), expression) }
	for name, test := range map[string]struct{ input, expected Expr }{
		"bare variable":   {x, x},
		"bare constant":   {constant, constant},
		"primed variable": {prime(x), prime(x)},
		"primed literal":  {prime(intLiteral(2021)), intLiteral(2021)},
		"primed constant": {prime(constant), constant},
		"nested prime":    {prime(prime(x)), prime(x)},
	} {
		t.Run(name, func(t *testing.T) {
			got := PropagatePrimes(nil, states)(test.input)
			if !EqualTypedExpr(got, test.expected) {
				t.Fatalf("PropagatePrimes() = %v; want %v", got, test.expected)
			}
		})
	}
}

func TestPropagatePrimesThroughOperatorsAndLet(t *testing.T) {
	states := map[string]struct{}{"x": {}, "y": {}}
	functionType := mustFun(IntT1, IntT1)
	x, y := NewNameExpr("x", Typed(functionType)), NewNameExpr("y", Typed(IntT1))
	application := MustOperExpr(Op("FUN_APP"), Typed(IntT1), x, y)
	primed := MustOperExpr(Op("PRIME"), Typed(IntT1), application)
	got := PropagatePrimes(nil, states)(primed)
	want := MustOperExpr(Op("FUN_APP"), Typed(IntT1),
		MustOperExpr(Op("PRIME"), Typed(functionType), x), MustOperExpr(Op("PRIME"), Typed(IntT1), y))
	if !EqualTypedExpr(got, want) {
		t.Fatalf("operator propagation = %v", got)
	}

	operatorType := mustOperator(nil, IntT1)
	definition := NewOperDecl("Foo", nil, application, Typed(operatorType))
	let := NewLetInExpr(MustOperExpr(Op("OPER_APP"), Typed(IntT1), NewNameExpr("Foo", Typed(operatorType))), Typed(IntT1), definition)
	letGot := PropagatePrimes(nil, states)(MustOperExpr(Op("PRIME"), Typed(IntT1), let))
	result := letGot.(*LetInExpr)
	definitionBody := result.Declarations()[0].Body().(*OperExpr)
	if definitionBody.Arguments()[0].(*OperExpr).Operator().Name() != "PRIME" || definitionBody.Arguments()[1].(*OperExpr).Operator().Name() != "PRIME" {
		t.Fatalf("LET declaration was not primed: %v", definitionBody)
	}
	if result.Body().(*OperExpr).Operator().Name() != "OPER_APP" {
		t.Fatalf("local operator name absorbed a prime: %v", result.Body())
	}
}

func TestPropagatePrimesLeavesOnlyNamesUnderPrime(t *testing.T) {
	states := map[string]struct{}{"x": {}, "y": {}}
	expression := MustOperExpr(Op("PRIME"), Typed(IntT1),
		MustOperExpr(Op("PLUS"), Typed(IntT1),
			MustOperExpr(Op("FUN_APP"), Typed(IntT1), NewNameExpr("x", Typed(mustFun(IntT1, IntT1))), NewNameExpr("y", Typed(IntT1))),
			intLiteral(1)))
	got := PropagatePrimes(nil, states)(expression)
	var check func(Expr)
	check = func(current Expr) {
		if op, ok := current.(*OperExpr); ok {
			if op.Operator().Name() == "PRIME" {
				if _, named := op.Arguments()[0].(*NameExpr); !named {
					t.Errorf("PRIME remains over %T", op.Arguments()[0])
				}
			}
			for _, argument := range op.Arguments() {
				check(argument)
			}
		}
	}
	check(got)
}

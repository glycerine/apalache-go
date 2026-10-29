package apalache

import (
	"context"
	"testing"
)

func TestStratifiedFunctionScalaOracleCases(t *testing.T) {
	intSet := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intLiteral(value)
		}
		return MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), elements...)
	}
	constructor := func(argument, result Type, binder string, domain, body Expr) Expr {
		return MustOperExpr(Op("FUN_CTOR"), Typed(mustFun(argument, result)), body,
			NewNameExpr(binder, Typed(argument)), domain)
	}
	apply := func(result Type, function, argument Expr) Expr {
		return MustOperExpr(Op("FUN_APP"), Typed(result), function, argument)
	}
	equal := func(left, right Expr) Expr {
		return MustOperExpr(Op("EQ"), Typed(BoolT1), left, right)
	}
	checkTrue := func(t *testing.T, expression Expr) {
		t.Helper()
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		checkRewrittenTruth(t, rewriter, scope, result, true)
	}

	t.Run("integer division constructor has function type", func(t *testing.T) {
		x := NewNameExpr("x", Typed(IntT1))
		function := constructor(IntT1, IntT1, "x", intSet(1, 2, 3, 4),
			MustOperExpr(Op("DIV"), Typed(IntT1), x, intLiteral(3)))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(function, InitialRewriterScope())
		if err != nil || !EqualTypes(result.Type.TLAType(), mustFun(IntT1, IntT1)) {
			t.Fatalf("division function = %v, %v", result, err)
		}
		solver := newTestG3Solver(t)
		if _, err = EncodeRewriterScope(solver, scope, rewriter.Constraints()); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
			t.Fatalf("division function scope = %v, %v", status, checkErr)
		}
	})

	t.Run("set-valued conditional and empty result applications", func(t *testing.T) {
		setType := mustSet(IntT1)
		x := NewNameExpr("x", Typed(IntT1))
		conditional := MustOperExpr(Op("IF_THEN_ELSE"), Typed(setType),
			equal(x, intLiteral(1)), intSet(2),
			MustOperExpr(Op("IF_THEN_ELSE"), Typed(setType), equal(x, intLiteral(2)), intSet(3), intSet(1)))
		function := constructor(IntT1, setType, "x", intSet(1, 2, 3), conditional)
		checkTrue(t, equal(apply(setType, function, intLiteral(1)), intSet(2)))

		emptyFunction := constructor(IntT1, setType, "x", intSet(1, 2), intSet())
		checkTrue(t, equal(apply(setType, emptyFunction, intLiteral(1)), intSet()))
	})

	t.Run("record-valued application preserves its field", func(t *testing.T) {
		recordType, err := NewRecordType(map[string]Type{"a": IntT1})
		if err != nil {
			t.Fatal(err)
		}
		x := NewNameExpr("x", Typed(IntT1))
		record := MustOperExpr(Op("RECORD"), Typed(recordType),
			NewValueExpr(StringValue("a"), Typed(StrT1)), x)
		function := constructor(IntT1, recordType, "x", intSet(1, 2), record)
		expected := MustOperExpr(Op("RECORD"), Typed(recordType),
			NewValueExpr(StringValue("a"), Typed(StrT1)), intLiteral(1))
		checkTrue(t, equal(apply(recordType, function, intLiteral(1)), expected))
	})

	t.Run("scalar and set-valued applications are exact", func(t *testing.T) {
		x := NewNameExpr("x", Typed(IntT1))
		scalar := constructor(IntT1, IntT1, "x", intSet(1, 2, 3, 4),
			MustOperExpr(Op("MULT"), Typed(IntT1), x, intLiteral(3)))
		checkTrue(t, equal(apply(IntT1, scalar, intLiteral(4)), intLiteral(12)))

		setType := mustSet(IntT1)
		setBody := MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1), x)
		setFunction := constructor(IntT1, setType, "x", intSet(3), setBody)
		checkTrue(t, equal(apply(setType, setFunction, intLiteral(3)), intSet(1, 3)))
	})

	t.Run("alpha-equivalent Boolean functions are equal", func(t *testing.T) {
		negation := func(name string) Expr {
			binder := NewNameExpr(name, Typed(BoolT1))
			return constructor(BoolT1, BoolT1, name,
				NewValueExpr(BoolSet, Typed(mustSet(BoolT1))),
				MustOperExpr(Op("NOT"), Typed(BoolT1), binder))
		}
		checkTrue(t, equal(negation("x"), negation("y")))
	})

	t.Run("function-valued application is exact", func(t *testing.T) {
		innerType := mustFun(BoolT1, BoolT1)
		y := NewNameExpr("y", Typed(BoolT1))
		inner := constructor(BoolT1, BoolT1, "y",
			NewValueExpr(BoolSet, Typed(mustSet(BoolT1))),
			MustOperExpr(Op("NOT"), Typed(BoolT1), y))
		outer := constructor(IntT1, innerType, "x", intSet(3), inner)
		checkTrue(t, equal(apply(innerType, outer, intLiteral(3)), inner))
	})

	t.Run("conditional function body selects the matching branch", func(t *testing.T) {
		x := NewNameExpr("x", Typed(IntT1))
		body := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1), equal(x, intLiteral(1)), intLiteral(11),
			MustOperExpr(Op("MULT"), Typed(IntT1), intLiteral(2), x))
		function := constructor(IntT1, IntT1, "x", intSet(1, 2), body)
		checkTrue(t, equal(apply(IntT1, function, intLiteral(1)), intLiteral(11)))
	})

	t.Run("function member survives Skolem selection", func(t *testing.T) {
		functionType := mustFun(BoolT1, BoolT1)
		y := NewNameExpr("y", Typed(BoolT1))
		function := constructor(BoolT1, BoolT1, "y",
			NewValueExpr(BoolSet, Typed(mustSet(BoolT1))),
			MustOperExpr(Op("NOT"), Typed(BoolT1), y))
		x := NewNameExpr("x", Typed(functionType))
		domain := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(functionType)), function)
		predicate := apply(BoolT1, x, boolExpr(false))
		exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x, domain, predicate)
		checkTrue(t, MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1), exists))
	})

	t.Run("constructor domain is preserved", func(t *testing.T) {
		x := NewNameExpr("x", Typed(IntT1))
		domain := intSet(1, 2, 3)
		function := constructor(IntT1, IntT1, "x", domain,
			MustOperExpr(Op("DIV"), Typed(IntT1), x, intLiteral(2)))
		checkTrue(t, equal(MustOperExpr(Op("DOMAIN"), Typed(mustSet(IntT1)), function), domain))
	})
}

func TestStratifiedFunctionScalaUndefinedApplicationsRemainSatisfiable(t *testing.T) {
	functionType := mustFun(IntT1, IntT1)
	binder := NewNameExpr("x", Typed(IntT1))
	for _, test := range []struct {
		name   string
		domain Expr
	}{
		{name: "outside finite domain", domain: MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1), intLiteral(2))},
		{name: "empty domain", domain: MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)))},
	} {
		t.Run(test.name, func(t *testing.T) {
			function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), binder, binder, test.domain)
			application := MustOperExpr(Op("FUN_APP"), Typed(IntT1), function, intLiteral(4))
			rewriter := NewStratifiedRewriter()
			scope, _, err := rewriter.Rewrite(application, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			if _, err = EncodeRewriterScope(solver, scope, rewriter.Constraints()); err != nil {
				t.Fatal(err)
			}
			if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
				t.Fatalf("undefined application = %v, %v", status, checkErr)
			}
		})
	}
}

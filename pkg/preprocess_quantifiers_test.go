package apalache

import "testing"

func TestOptimizeIntegerQuantifier(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	predicate := NewNameExpr("P", Typed(BoolT1))
	sets := []struct {
		name string
		set  Expr
	}{
		{"Int", NewValueExpr(IntSet, Typed(mustSet(IntT1)))},
		{"Nat", NewValueExpr(NatSet, Typed(mustSet(IntT1)))},
		{"range", MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), NewValueExpr(Int64Value(-2), Typed(IntT1)), NewValueExpr(Int64Value(5), Typed(IntT1)))},
	}
	for _, item := range sets {
		for _, quantifier := range []struct{ bounded, unbounded string }{{"EXISTS3", "EXISTS2"}, {"FORALL3", "FORALL2"}} {
			t.Run(item.name+quantifier.bounded, func(t *testing.T) {
				input := MustOperExpr(Op(quantifier.bounded), Typed(BoolT1), x, item.set, predicate)
				got := OptimizeIntegerQuantifier(input, nil)
				operation, ok := got.(*OperExpr)
				if !ok || operation.Operator().Name() != quantifier.unbounded || len(operation.Arguments()) != 2 {
					t.Fatalf("optimized quantifier = %v", got)
				}
				body := operation.Arguments()[1]
				if item.name == "Int" {
					if !EqualTypedExpr(body, predicate) {
						t.Fatalf("Int predicate changed to %v", body)
					}
				} else {
					guarded, ok := body.(*OperExpr)
					want := "AND"
					if quantifier.bounded == "FORALL3" {
						want = "IMPLIES"
					}
					if !ok || guarded.Operator().Name() != want {
						t.Fatalf("guarded predicate = %v", body)
					}
				}
			})
		}
	}
}

func TestOptimizeIntegerQuantifierLeavesOtherSetsAndNestedExpressions(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	predicate := NewNameExpr("P", Typed(BoolT1))
	otherSet := NewNameExpr("S", Typed(mustSet(IntT1)))
	bounded := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x, otherSet, predicate)
	if got := OptimizeIntegerQuantifier(bounded, nil); got != bounded {
		t.Fatalf("changed unsupported set: %v", got)
	}
	nested := MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x, NewValueExpr(IntSet, Typed(mustSet(IntT1))), predicate))
	if got := OptimizeIntegerQuantifier(nested, nil); got != nested {
		t.Fatal("shallow optimizer unexpectedly descended")
	}
}

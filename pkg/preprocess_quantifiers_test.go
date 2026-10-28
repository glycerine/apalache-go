package apalache

import (
	"math/rand"
	"testing"
)

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

func TestOptimizeIntegerQuantifierGeneratedScalaShapes(t *testing.T) {
	random := rand.New(rand.NewSource(0x1a7))
	for iteration := 0; iteration < 500; iteration++ {
		bound := NewNameExpr("x", Typed(IntT1))
		predicate := MustOperExpr(Op("EQ"), Typed(BoolT1),
			NewValueExpr(Int64Value(random.Int63n(100)), Typed(IntT1)),
			NewValueExpr(Int64Value(random.Int63n(100)), Typed(IntT1)))
		lower := NewValueExpr(Int64Value(random.Int63n(200)-100), Typed(IntT1))
		upper := NewValueExpr(Int64Value(random.Int63n(200)-100), Typed(IntT1))
		sets := []struct {
			name string
			set  Expr
		}{
			{"Int", NewValueExpr(IntSet, Typed(mustSet(IntT1)))},
			{"Nat", NewValueExpr(NatSet, Typed(mustSet(IntT1)))},
			{"range", MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), lower, upper)},
		}
		for _, set := range sets {
			for _, quantifier := range []struct {
				bounded, unbounded, outer string
			}{{"EXISTS3", "EXISTS2", "AND"}, {"FORALL3", "FORALL2", "IMPLIES"}} {
				input := MustOperExpr(Op(quantifier.bounded), Typed(BoolT1), bound, set.set, predicate)
				got := OptimizeIntegerQuantifier(input, nil)
				expectedPredicate := predicate
				switch set.name {
				case "Nat":
					guard := MustOperExpr(Op("GE"), Typed(BoolT1),
						NewNameExpr("x", Typed(IntT1)), intLiteral(0))
					expectedPredicate = MustOperExpr(Op(quantifier.outer), Typed(BoolT1), guard, predicate)
				case "range":
					guard := MustOperExpr(Op("AND"), Typed(BoolT1),
						MustOperExpr(Op("LE"), Typed(BoolT1), lower, NewNameExpr("x", Typed(IntT1))),
						MustOperExpr(Op("LE"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), upper))
					expectedPredicate = MustOperExpr(Op(quantifier.outer), Typed(BoolT1), guard, predicate)
				}
				expected := MustOperExpr(Op(quantifier.unbounded), Typed(BoolT1), bound, expectedPredicate)
				if !EqualTypedExpr(got, expected) {
					t.Fatalf("iteration %d %s/%s = %#v", iteration, set.name, quantifier.bounded, got)
				}
			}
		}
	}
}

func TestOptimizeIntegerQuantifierCreatesFreshGuardNames(t *testing.T) {
	bound := NewNameExpr("x", Typed(IntT1))
	rangeSet := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), intLiteral(1), intLiteral(3))
	input := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), bound, rangeSet, temporalBool(true))
	got := OptimizeIntegerQuantifier(input, nil).(*OperExpr)
	guard := got.Arguments()[1].(*OperExpr).Arguments()[0].(*OperExpr)
	lowerName := guard.Arguments()[0].(*OperExpr).Arguments()[1].(*NameExpr)
	upperName := guard.Arguments()[1].(*OperExpr).Arguments()[0].(*NameExpr)
	if lowerName.ID() == upperName.ID() || lowerName.ID() == bound.ID() || upperName.ID() == bound.ID() {
		t.Fatalf("range guard reused bound-variable IDs: %s, %s, %s", bound.ID(), lowerName.ID(), upperName.ID())
	}
}

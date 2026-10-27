package apalache

import (
	"sync"
	"testing"
)

func TestUniqueNameGeneratorScalaSequenceAndConcurrency(t *testing.T) {
	generator := NewUniqueNameGenerator()
	if generator.NewName() != "t_1" {
		t.Fatal("first generated name differs from Scala")
	}
	seen := sync.Map{}
	var wait sync.WaitGroup
	for range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			name := generator.NewName()
			if _, loaded := seen.LoadOrStore(name, struct{}{}); loaded {
				t.Errorf("duplicate name %s", name)
			}
		}()
	}
	wait.Wait()
}

func TestSimplifySetMembershipTypeDefiningAndNat(t *testing.T) {
	integer := NewNameExpr("i", Typed(IntT1))
	intSet := NewValueExpr(IntSet, Typed(mustSet(IntT1)))
	natSet := NewValueExpr(NatSet, Typed(mustSet(IntT1)))
	for _, test := range []struct{ set, want Expr }{
		{intSet, NewValueExpr(BoolValue(true), Typed(BoolT1))},
		{natSet, MustOperExpr(Op("GE"), Typed(BoolT1), integer, NewValueExpr(Int64Value(0), Typed(IntT1)))}} {
		got, err := SimplifySetMembership(MustOperExpr(Op("SET_IN"), Typed(BoolT1), integer, test.set), nil, nil, false)
		if err != nil || !EqualTypedExpr(got, test.want) {
			t.Fatalf("membership = %v, %v; want %v", got, err, test.want)
		}
	}
	sequenceType := mustSeq(IntT1)
	sequenceSet := MustOperExpr(Op("Sequences!Seq"), Typed(mustSet(sequenceType)), intSet)
	got, err := SimplifySetMembership(MustOperExpr(Op("SET_IN"), Typed(BoolT1), NewNameExpr("s", Typed(sequenceType)), sequenceSet), nil, nil, false)
	if err != nil || !EqualTypedExpr(got, NewValueExpr(BoolValue(true), Typed(BoolT1))) {
		t.Fatalf("sequence membership = %v, %v", got, err)
	}
}

func TestSimplifySetMembershipFunctionRangeAndEarlyMode(t *testing.T) {
	domain := NewNameExpr("RM", Typed(mustSet(IntT1)))
	natSet := NewValueExpr(NatSet, Typed(mustSet(IntT1)))
	function := NewNameExpr("f", Typed(mustFun(IntT1, IntT1)))
	functionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(mustFun(IntT1, IntT1))), domain, natSet)
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), function, functionSet)
	unchanged, err := SimplifySetMembership(membership, nil, nil, true)
	if err != nil || !EqualTypedExpr(unchanged, membership) {
		t.Fatalf("early simple function = %v, %v", unchanged, err)
	}
	got, err := SimplifySetMembership(membership, NewUniqueNameGenerator(), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	and, ok := got.(*OperExpr)
	if !ok || and.Operator().Name() != "AND" {
		t.Fatalf("function membership = %v", got)
	}
	parts := and.Arguments()
	forall, ok := parts[1].(*OperExpr)
	if !ok || forall.Operator().Name() != "FORALL3" {
		t.Fatalf("range constraint = %v", parts[1])
	}
	forallArgs := forall.Arguments()
	bound := forallArgs[0].(*NameExpr)
	comparison := forallArgs[2].(*OperExpr)
	application := comparison.Arguments()[0].(*OperExpr)
	if bound.Name() != "t_1" || application.Arguments()[1].(*NameExpr).Name() != bound.Name() {
		t.Fatalf("inconsistent generated binder in %v", got)
	}

	innerType := mustFun(IntT1, IntT1)
	innerSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(innerType)), domain, natSet)
	outerFunction := NewNameExpr("nested", Typed(mustFun(IntT1, innerType)))
	outerSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(mustFun(IntT1, innerType))), domain, innerSet)
	early, err := SimplifySetMembership(MustOperExpr(Op("SET_IN"), Typed(BoolT1), outerFunction, outerSet), nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if operation, ok := early.(*OperExpr); !ok || operation.Operator().Name() != "AND" {
		t.Fatalf("early nested function was not simplified: %v", early)
	}
}

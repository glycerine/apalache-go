package apalache

import (
	"errors"
	"sync"
	"testing"
)

func TestUniqueNameGeneratorScalaSequenceAndConcurrency(t *testing.T) {
	generator := NewUniqueNameGenerator()
	for index, want := range []string{"t_1", "t_2", "t_3"} {
		if got := generator.NewName(); got != want {
			t.Fatalf("generated name %d = %q, want %q", index+1, got, want)
		}
	}
	generator = NewUniqueNameGenerator()
	for range 10000 {
		generator.NewName()
	}
	if got := generator.NewName(); got != "t_7pt" {
		t.Fatalf("generated name 10001 = %q, want t_7pt", got)
	}

	generator = NewUniqueNameGenerator()
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

func TestSimplifySetMembershipAllScalaTypeDefiningForms(t *testing.T) {
	boolSet := NewValueExpr(BoolSet, Typed(mustSet(BoolT1)))
	stringSet := NewValueExpr(StringSet, Typed(mustSet(StrT1)))
	intSet := NewValueExpr(IntSet, Typed(mustSet(IntT1)))
	realSet := NewValueExpr(RealSet, Typed(mustSet(RealT1)))
	seqType := mustSeq(IntT1)
	seqSet := MustOperExpr(Op("Sequences!Seq"), Typed(mustSet(seqType)), intSet)
	powerType := mustSet(IntT1)
	powerSet := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(powerType)), intSet)
	functionType := mustFun(IntT1, BoolT1)
	functionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)), intSet, boolSet)
	tupleType := mustTuple(IntT1, BoolT1)
	product := MustOperExpr(Op("SET_TIMES"), Typed(mustSet(tupleType)), intSet, boolSet)
	recordType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	recordSet := MustOperExpr(Op("RECORD_SET"), Typed(mustSet(recordType)),
		NewValueExpr(StringValue("a"), Typed(StrT1)), intSet,
		NewValueExpr(StringValue("b"), Typed(StrT1)), boolSet)

	cases := []struct {
		element Expr
		set     Expr
	}{
		{NewNameExpr("b", Typed(BoolT1)), boolSet},
		{NewNameExpr("s", Typed(StrT1)), stringSet},
		{NewNameExpr("i", Typed(IntT1)), intSet},
		{NewNameExpr("r", Typed(RealT1)), realSet},
		{NewNameExpr("seq", Typed(seqType)), seqSet},
		{NewNameExpr("set", Typed(powerType)), powerSet},
		{NewNameExpr("fun", Typed(functionType)), functionSet},
		{NewNameExpr("tuple", Typed(tupleType)), product},
		{NewNameExpr("record", Typed(recordType)), recordSet},
	}
	for _, test := range cases {
		membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), test.element, test.set)
		got, err := SimplifySetMembership(membership, nil, nil, false)
		if err != nil || !EqualTypedExpr(got, NewValueExpr(BoolValue(true), Typed(BoolT1))) {
			t.Errorf("type-defining membership %v = %v, %v", membership, got, err)
		}
	}
}

func TestSimplifySetMembershipFunctionDomainAndMalformedSet(t *testing.T) {
	domain := NewNameExpr("RM", Typed(mustSet(IntT1)))
	boolSet := NewValueExpr(BoolSet, Typed(mustSet(BoolT1)))
	function := NewNameExpr("f", Typed(mustFun(IntT1, BoolT1)))
	functionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(mustFun(IntT1, BoolT1))), domain, boolSet)
	got, err := SimplifySetMembership(MustOperExpr(Op("SET_IN"), Typed(BoolT1), function, functionSet), nil, nil, false)
	want := MustOperExpr(Op("EQ"), Typed(BoolT1), MustOperExpr(Op("DOMAIN"), domain.Tag(), function), domain)
	if err != nil || !EqualTypedExpr(got, want) {
		t.Fatalf("function domain constraint = %v, %v; want %v", got, err, want)
	}

	badDomain := NewNameExpr("Bad", Typed(IntT1))
	natSet := NewValueExpr(NatSet, Typed(mustSet(IntT1)))
	badFunctionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(mustFun(IntT1, IntT1))), badDomain, natSet)
	_, err = SimplifySetMembership(MustOperExpr(Op("SET_IN"), Typed(BoolT1),
		NewNameExpr("g", Typed(mustFun(IntT1, IntT1))), badFunctionSet), nil, nil, false)
	var malformed *MalformedTlaError
	if !errors.As(err, &malformed) || malformed.CauseExpr() != badDomain {
		t.Fatalf("malformed domain error = %T %v", err, err)
	}
}

func TestSimplifySetMembershipTracksReplacementAncestry(t *testing.T) {
	integer := NewNameExpr("i", Typed(IntT1))
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), integer,
		NewValueExpr(NatSet, Typed(mustSet(IntT1))))
	changes := NewChangeListener()
	got, err := SimplifySetMembership(membership, nil, NewListenerTracker(changes), false)
	if err != nil {
		t.Fatal(err)
	}
	if changes.TraceBack(got.ID()) != membership.ID() {
		t.Fatalf("replacement traces to %s, want %s", changes.TraceBack(got.ID()), membership.ID())
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

func TestSimplifySetMembershipKeepsNonTypeDefiningPowerSetAndSequence(t *testing.T) {
	intSet := NewNameExpr("Values", Typed(mustSet(IntT1)))
	sequenceType := mustSeq(IntT1)
	sequenceSet := MustOperExpr(Op("Sequences!Seq"), Typed(mustSet(sequenceType)), intSet)
	powerSet := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(mustSet(sequenceType))), sequenceSet)
	sequences := NewNameExpr("Sequences", Typed(mustSet(sequenceType)))
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), sequences, powerSet)
	got, err := SimplifySetMembership(membership, NewUniqueNameGenerator(), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !EqualTypedExpr(got, membership) {
		t.Fatalf("non-type-defining powerset membership changed: %v", got)
	}
}

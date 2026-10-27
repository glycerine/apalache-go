package apalache

import "testing"

func TestKeramelizerLogicAndSetAlgebra(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	setType := mustSet(IntT1)
	left := NewNameExpr("A", Typed(setType))
	right := NewNameExpr("B", Typed(setType))
	intersection, err := NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_INTERSECT"), Typed(setType), left, right))
	if err != nil {
		t.Fatal(err)
	}
	filter := intersection.(*OperExpr)
	if filter.Operator().Name() != "SET_FILTER" || filter.Arguments()[0].(*NameExpr).Name() != "t_1" {
		t.Fatalf("intersection = %v", intersection)
	}
	difference, err := NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_MINUS"), Typed(setType), left, right))
	if err != nil {
		t.Fatal(err)
	}
	if difference.(*OperExpr).Arguments()[2].(*OperExpr).Operator().Name() != "NOT" {
		t.Fatalf("difference = %v", difference)
	}
	notin, err := NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_NOT_IN"), Typed(BoolT1), x, left))
	if err != nil || notin.(*OperExpr).Operator().Name() != "NOT" {
		t.Fatalf("not-in = %v, %v", notin, err)
	}
	impl, err := NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("IMPLIES"), Typed(BoolT1), NewNameExpr("P", Typed(BoolT1)), NewNameExpr("Q", Typed(BoolT1))))
	if err != nil || impl.(*OperExpr).Operator().Name() != "OR" {
		t.Fatalf("implication = %v, %v", impl, err)
	}
}

func TestKeramelizerProductsRecordsAndSubsets(t *testing.T) {
	intSet := NewValueExpr(IntSet, Typed(mustSet(IntT1)))
	boolSet := NewValueExpr(BoolSet, Typed(mustSet(BoolT1)))
	tupleType := mustTuple(IntT1, BoolT1)
	product, err := NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_TIMES"), Typed(mustSet(tupleType)), intSet, boolSet))
	if err != nil {
		t.Fatal(err)
	}
	mapExpr := product.(*OperExpr)
	if mapExpr.Operator().Name() != "SET_MAP" || mapExpr.Arguments()[1].(*NameExpr).Name() != "t_1" || mapExpr.Arguments()[3].(*NameExpr).Name() != "t_2" {
		t.Fatalf("product = %v", product)
	}
	recordType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	recordSet := MustOperExpr(Op("RECORD_SET"), Typed(mustSet(recordType)), NewValueExpr(StringValue("a"), Typed(StrT1)), intSet, NewValueExpr(StringValue("b"), Typed(StrT1)), boolSet)
	record, err := NewKeramelizer(nil, nil).Expression(recordSet)
	if err != nil {
		t.Fatal(err)
	}
	if record.(*OperExpr).Operator().Name() != "SET_MAP" || record.(*OperExpr).Arguments()[0].(*OperExpr).Operator().Name() != "RECORD" {
		t.Fatalf("record set = %v", record)
	}
	powerset := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(mustSet(IntT1))), intSet)
	subset, err := NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), NewNameExpr("Sets", Typed(mustSet(mustSet(IntT1)))), powerset))
	if err != nil {
		t.Fatal(err)
	}
	outer := subset.(*OperExpr)
	if outer.Operator().Name() != "FORALL3" || outer.Arguments()[2].(*OperExpr).Operator().Name() != "FORALL3" {
		t.Fatalf("subset powerset = %v", subset)
	}
}

func TestKeramelizerFunctionSetMembership(t *testing.T) {
	domain := NewNameExpr("S", Typed(mustSet(IntT1)))
	codomain := NewNameExpr("T", Typed(mustSet(BoolT1)))
	functionType := mustFun(IntT1, BoolT1)
	function := NewNameExpr("f", Typed(functionType))
	functionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)), domain, codomain)
	got, err := NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_IN"), Typed(BoolT1), function, functionSet))
	if err != nil {
		t.Fatal(err)
	}
	and := got.(*OperExpr)
	if and.Operator().Name() != "AND" || and.Arguments()[0].(*OperExpr).Operator().Name() != "EQ" || and.Arguments()[1].(*OperExpr).Operator().Name() != "FORALL3" {
		t.Fatalf("function membership = %v", got)
	}
	powerCodomain := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(mustSet(BoolT1))), codomain)
	setFunctionType := mustFun(IntT1, mustSet(BoolT1))
	powerFunctionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(setFunctionType)), domain, powerCodomain)
	got, err = NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_IN"), Typed(BoolT1), NewNameExpr("g", Typed(setFunctionType)), powerFunctionSet))
	if err != nil {
		t.Fatal(err)
	}
	outerForall := got.(*OperExpr).Arguments()[1].(*OperExpr)
	if outerForall.Arguments()[2].(*OperExpr).Operator().Name() != "FORALL3" {
		t.Fatalf("powerset function membership = %v", got)
	}
}

func TestKeramelizerAssignmentsAndCases(t *testing.T) {
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	set := NewNameExpr("S", Typed(mustSet(IntT1)))
	assigned, err := NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_IN"), Typed(BoolT1), prime, set))
	if err != nil {
		t.Fatal(err)
	}
	exists := assigned.(*OperExpr)
	if exists.Operator().Name() != "EXISTS3" || exists.Arguments()[0].(*NameExpr).Name() != "t_1" {
		t.Fatalf("primed membership = %v", assigned)
	}
	p := NewNameExpr("P", Typed(BoolT1))
	q := NewNameExpr("Q", Typed(BoolT1))
	booleanCase := MustOperExpr(Op("CASE_OTHER"), Typed(BoolT1), q, p, q)
	got, err := NewKeramelizer(nil, nil).Expression(booleanCase)
	if err != nil || got.(*OperExpr).Operator().Name() != "OR" {
		t.Fatalf("Boolean CASE = %v, %v", got, err)
	}
	valueCase := MustOperExpr(Op("CASE_OTHER"), Typed(IntT1), NewValueExpr(Int64Value(0), Typed(IntT1)), p, NewValueExpr(Int64Value(1), Typed(IntT1)))
	got, err = NewKeramelizer(nil, nil).Expression(valueCase)
	if err != nil || got.(*OperExpr).Operator().Name() != "IF_THEN_ELSE" {
		t.Fatalf("value CASE = %v, %v", got, err)
	}
}

package apalache

import (
	"errors"
	"testing"
)

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
	for _, test := range []struct{ operator, want string }{{"EQUIV", "EQ"}, {"NE", "NOT"}} {
		got, transformErr := NewKeramelizer(nil, nil).Expression(MustOperExpr(Op(test.operator), Typed(BoolT1), x, x))
		if transformErr != nil || got.(*OperExpr).Operator().Name() != test.want {
			t.Errorf("%s rewrite = %v, %v", test.operator, got, transformErr)
		}
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

func TestKeramelizerNestedPowersetSubset(t *testing.T) {
	intSetType := mustSet(IntT1)
	intSet := NewNameExpr("B", Typed(intSetType))
	power := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(intSetType)), intSet)
	doublePower := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(mustSet(intSetType))), power)
	sets := NewNameExpr("A", Typed(mustSet(mustSet(IntT1))))
	got, err := NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), sets, doublePower))
	if err != nil {
		t.Fatal(err)
	}
	if countOperator(got, "FORALL3") != 3 || countOperator(got, "SET_SUBSET_EQ") != 0 {
		t.Fatalf("nested powerset subset = %v", got)
	}
	leftPower := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(intSetType)), NewNameExpr("A0", Typed(intSetType)))
	got, err = NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), leftPower, power))
	if err != nil || countOperator(got, "FORALL3") != 1 {
		t.Fatalf("powerset-to-powerset subset = %v, %v", got, err)
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

func TestKeramelizerCaseWithoutOtherAndEmptyCase(t *testing.T) {
	p := NewNameExpr("P", Typed(BoolT1))
	q := NewNameExpr("Q", Typed(BoolT1))
	one, two := intLiteral(1), intLiteral(2)
	valueCase := MustOperExpr(Op("CASE"), Typed(IntT1), p, one, q, two)
	got, err := NewKeramelizer(nil, nil).Expression(valueCase)
	if err != nil {
		t.Fatal(err)
	}
	ite := got.(*OperExpr)
	if ite.Operator().Name() != "IF_THEN_ELSE" || ite.Arguments()[0] != p || ite.Arguments()[2] != two {
		t.Fatalf("CASE without OTHER = %v", got)
	}
	booleanCase := MustOperExpr(Op("CASE"), Typed(BoolT1), p, q, q, p)
	got, err = NewKeramelizer(nil, nil).Expression(booleanCase)
	if err != nil || got.(*OperExpr).Operator().Name() != "OR" || len(got.(*OperExpr).Arguments()) != 2 {
		t.Fatalf("Boolean CASE without OTHER = %v, %v", got, err)
	}
}

func TestKeramelizerScalaLETOrderMalformedSetAndTracking(t *testing.T) {
	setType := mustSet(IntT1)
	intersection := func(left, right string) Expr {
		return MustOperExpr(Op("SET_INTERSECT"), Typed(setType),
			NewNameExpr(left, Typed(setType)), NewNameExpr(right, Typed(setType)))
	}
	local := NewOperDecl("Local", nil, intersection("C", "D"), Typed(mustOperator(nil, setType)))
	root := NewLetInExpr(intersection("A", "B"), Typed(setType), local)
	got, err := NewKeramelizer(NewUniqueNameGenerator(), nil).Expression(root)
	if err != nil {
		t.Fatal(err)
	}
	let := got.(*LetInExpr)
	if let.Body().(*OperExpr).Arguments()[0].(*NameExpr).Name() != "t_1" ||
		let.Declarations()[0].Body().(*OperExpr).Arguments()[0].(*NameExpr).Name() != "t_2" {
		t.Fatalf("LET fresh-name order body=%v definition=%v", let.Body(), let.Declarations()[0].Body())
	}

	badSet := NewNameExpr("Bad", Typed(IntT1))
	_, err = NewKeramelizer(nil, nil).Expression(MustOperExpr(Op("SET_INTERSECT"), Typed(IntT1), badSet, badSet))
	var malformed *MalformedTlaError
	if !errors.As(err, &malformed) || malformed.CauseExpr() != badSet {
		t.Fatalf("malformed set error = %T %v", err, err)
	}

	original := MustOperExpr(Op("SET_NOT_IN"), Typed(BoolT1), intLiteral(1), NewNameExpr("S", Typed(setType)))
	changes := NewChangeListener()
	rewritten, err := NewKeramelizer(nil, NewListenerTracker(changes)).Expression(original)
	if err != nil || changes.TraceBack(rewritten.ID()) != original.ID() {
		t.Fatalf("tracked rewrite = %v, %v, origin %s", rewritten, err, changes.TraceBack(rewritten.ID()))
	}
}

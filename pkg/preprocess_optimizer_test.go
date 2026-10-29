package apalache

import (
	"errors"
	"testing"
)

func i(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
func s(value string) Expr {
	return NewValueExpr(StringValue(value), Typed(StrT1))
}

func optimizerForTest() *ExprOptimizer {
	return NewExprOptimizer(NewUniqueNameGenerator(), nil)
}

func optimizeForTest(t *testing.T, expression Expr) Expr {
	t.Helper()
	result, err := optimizerForTest().Expression(expression)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestExprOptimizerRecordAccessAndSetBasics(t *testing.T) {
	intSet := mustSet(IntT1)
	rangeExpr := MustOperExpr(Op("INT_RANGE"), Typed(intSet), i(1), i(3))
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), rangeExpr)
	expected := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("LE"), Typed(BoolT1), i(1), NewNameExpr("x", Typed(IntT1))),
		MustOperExpr(Op("LE"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), i(3)))
	if got := optimizeForTest(t, membership); !EqualTypedExpr(got, expected) {
		t.Fatalf("range membership = %v", got)
	}

	filter := MustOperExpr(Op("SET_FILTER"), Typed(intSet), NewNameExpr("y", Typed(IntT1)),
		NewNameExpr("S", Typed(intSet)), MustOperExpr(Op("GT"), Typed(BoolT1), NewNameExpr("y", Typed(IntT1)), i(2)))
	filteredMembership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), filter)
	got := optimizeForTest(t, filteredMembership).(*OperExpr)
	if got.Operator().Name() != "AND" || got.Arguments()[1].(*OperExpr).Arguments()[0].(*NameExpr).Name() != "x" {
		t.Fatalf("filtered membership = %v", got)
	}

	recordType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": IntT1})
	record := MustOperExpr(Op("RECORD"), Typed(recordType), s("a"), i(1), s("b"), i(2))
	access := MustOperExpr(Op("FUN_APP"), Typed(IntT1), record, s("b"))
	if got := optimizeForTest(t, access); !EqualTypedExpr(got, i(2)) {
		t.Fatalf("record access = %v", got)
	}
	missing := MustOperExpr(Op("FUN_APP"), Typed(IntT1), record, s("c"))
	if _, err := optimizerForTest().Expression(missing); err == nil {
		t.Fatal("missing record field was accepted")
	} else {
		var inputError *TlaInputError
		if !errors.As(err, &inputError) || inputError.SourceID == nil || *inputError.SourceID != missing.ID() {
			t.Fatalf("missing record field error = %T %v", err, err)
		}
	}
}

func TestExprOptimizerRewritesSequenceIndexRangeToDomain(t *testing.T) {
	sequence := NewNameExpr("s", Typed(mustSeq(IntT1)))
	length := MustOperExpr(Op("Sequences!Len"), Typed(IntT1), sequence)
	rangeExpr := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), i(1), length)
	got := optimizeForTest(t, rangeExpr)
	want := MustOperExpr(Op("DOMAIN"), Typed(mustSet(IntT1)), sequence)
	if !EqualTypedExpr(got, want) {
		t.Fatalf("sequence index range = %v, want %v", got, want)
	}
}

func TestExprOptimizerRecordAndTupleMapMembership(t *testing.T) {
	intSet, boolSet := mustSet(IntT1), mustSet(BoolT1)
	recordType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	x, y := NewNameExpr("x", Typed(IntT1)), NewNameExpr("y", Typed(BoolT1))
	record := MustOperExpr(Op("RECORD"), Typed(recordType), s("a"), x, s("b"), y)
	mapped := MustOperExpr(Op("SET_MAP"), Typed(mustSet(recordType)), record, x,
		NewNameExpr("S", Typed(intSet)), y, NewNameExpr("T", Typed(boolSet)))
	member := NewNameExpr("r", Typed(recordType))
	got := optimizeForTest(t, MustOperExpr(Op("SET_IN"), Typed(BoolT1), member, mapped)).(*OperExpr)
	if got.Operator().Name() != "AND" || len(got.Arguments()) != 3 || got.Arguments()[0].(*OperExpr).Arguments()[0].(*OperExpr).Operator().Name() != "DOMAIN" {
		t.Fatalf("record map membership = %v", got)
	}

	tupleType := mustTuple(IntT1, BoolT1)
	tuple := MustOperExpr(Op("TUPLE"), Typed(tupleType), x, y)
	tupleMap := MustOperExpr(Op("SET_MAP"), Typed(mustSet(tupleType)), tuple, x,
		NewNameExpr("S", Typed(intSet)), y, NewNameExpr("T", Typed(boolSet)))
	tupleMember := NewNameExpr("t", Typed(tupleType))
	tupleGot := optimizeForTest(t, MustOperExpr(Op("SET_IN"), Typed(BoolT1), tupleMember, tupleMap)).(*OperExpr)
	if tupleGot.Operator().Name() != "AND" || len(tupleGot.Arguments()) != 2 {
		t.Fatalf("tuple map membership = %v", tupleGot)
	}
	firstAccess := tupleGot.Arguments()[0].(*OperExpr).Arguments()[0].(*OperExpr)
	if !firstAccess.Tag().Equal(Typed(IntT1)) || firstAccess.Arguments()[1].(*ValueExpr).Value().(IntValue).BigInt().Int64() != 1 {
		t.Fatalf("first tuple projection = %v", firstAccess)
	}

	reversedTuple := MustOperExpr(Op("TUPLE"), Typed(mustTuple(BoolT1, IntT1)), y, x)
	reversedMap := MustOperExpr(Op("SET_MAP"), Typed(mustSet(mustTuple(BoolT1, IntT1))), reversedTuple, x,
		NewNameExpr("S", Typed(intSet)), y, NewNameExpr("T", Typed(boolSet)))
	unchanged := MustOperExpr(Op("SET_IN"), Typed(BoolT1), NewNameExpr("u", Typed(mustTuple(BoolT1, IntT1))), reversedMap)
	if got := optimizeForTest(t, unchanged); !EqualTypedExpr(got, unchanged) {
		t.Fatalf("general tuple map was optimized: %v", got)
	}

	generalRecord := MustOperExpr(Op("RECORD"), Typed(recordType), s("a"), i(1), s("b"), y)
	generalMap := MustOperExpr(Op("SET_MAP"), Typed(mustSet(recordType)), generalRecord, x,
		NewNameExpr("S", Typed(intSet)), y, NewNameExpr("T", Typed(boolSet)))
	generalMembership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), member, generalMap)
	if got := optimizeForTest(t, generalMembership); !EqualTypedExpr(got, generalMembership) {
		t.Fatalf("general record map was optimized: %v", got)
	}

	innerTuple := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1, IntT1)),
		NewNameExpr("a", Typed(IntT1)), NewNameExpr("b", Typed(IntT1)))
	innerMap := MustOperExpr(Op("SET_MAP"), Typed(mustSet(mustTuple(IntT1, IntT1))), innerTuple,
		innerTuple.Arguments()[0], NewNameExpr("A", Typed(intSet)),
		innerTuple.Arguments()[1], NewNameExpr("B", Typed(intSet)))
	outerTupleType := mustTuple(mustTuple(IntT1, IntT1), IntT1)
	outerTuple := MustOperExpr(Op("TUPLE"), Typed(outerTupleType),
		NewNameExpr("ab", Typed(mustTuple(IntT1, IntT1))), NewNameExpr("c", Typed(IntT1)))
	outerMap := MustOperExpr(Op("SET_MAP"), Typed(mustSet(outerTupleType)), outerTuple,
		outerTuple.Arguments()[0], innerMap, outerTuple.Arguments()[1], NewNameExpr("C", Typed(intSet)))
	nested := optimizeForTest(t, MustOperExpr(Op("SET_IN"), Typed(BoolT1),
		NewNameExpr("t", Typed(outerTupleType)), outerMap)).(*OperExpr)
	if nested.Operator().Name() != "AND" || nested.Arguments()[0].(*OperExpr).Operator().Name() != "AND" {
		t.Fatalf("nested tuple product membership = %v", nested)
	}
}

func TestExprOptimizerPowersetMembership(t *testing.T) {
	intSet := mustSet(IntT1)
	left, right := NewNameExpr("S", Typed(intSet)), NewNameExpr("T", Typed(intSet))
	powerset := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(intSet)), right)
	got := optimizeForTest(t, MustOperExpr(Op("SET_IN"), Typed(BoolT1), left, powerset)).(*OperExpr)
	if got.Operator().Name() != "FORALL3" || got.Arguments()[0].(*NameExpr).Name() != "t_1" {
		t.Fatalf("powerset membership = %v", got)
	}

	recordType, _ := NewRecordType(map[string]Type{"a": IntT1})
	x := NewNameExpr("x", Typed(IntT1))
	record := MustOperExpr(Op("RECORD"), Typed(recordType), s("a"), x)
	recordSet := MustOperExpr(Op("SET_MAP"), Typed(mustSet(recordType)), record, x, right)
	setOfRecords := NewNameExpr("R", Typed(mustSet(recordType)))
	recordPowerset := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(mustSet(recordType))), recordSet)
	rewritten := optimizeForTest(t, MustOperExpr(Op("SET_IN"), Typed(BoolT1), setOfRecords, recordPowerset)).(*OperExpr)
	if rewritten.Operator().Name() != "FORALL3" || rewritten.Arguments()[0].(*NameExpr).Name() != "t_1" || rewritten.Arguments()[2].(*OperExpr).Operator().Name() != "AND" {
		t.Fatalf("record powerset membership = %v", rewritten)
	}
}

func TestExprOptimizerCardinalityRules(t *testing.T) {
	intSet := mustSet(IntT1)
	set := NewNameExpr("S", Typed(intSet))
	card := func(value Expr) Expr { return MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), value) }
	empty := MustOperExpr(Op("SET_ENUM"), Typed(intSet))
	tests := []struct {
		input, expected Expr
	}{
		{MustOperExpr(Op("EQ"), Typed(BoolT1), card(set), i(0)), MustOperExpr(Op("EQ"), Typed(BoolT1), set, empty)},
		{MustOperExpr(Op("GT"), Typed(BoolT1), card(set), i(0)), MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("EQ"), Typed(BoolT1), set, empty))},
		{MustOperExpr(Op("GE"), Typed(BoolT1), card(set), i(1)), MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("EQ"), Typed(BoolT1), set, empty))},
		{MustOperExpr(Op("NE"), Typed(BoolT1), card(set), i(0)), MustOperExpr(Op("NOT"), Typed(BoolT1), MustOperExpr(Op("EQ"), Typed(BoolT1), set, empty))},
	}
	for _, test := range tests {
		if got := optimizeForTest(t, test.input); !EqualTypedExpr(got, test.expected) {
			t.Errorf("cardinality rewrite = %v, want %v", got, test.expected)
		}
	}

	rangeExpr := MustOperExpr(Op("INT_RANGE"), Typed(intSet), NewNameExpr("a", Typed(IntT1)), NewNameExpr("b", Typed(IntT1)))
	rangeGot := optimizeForTest(t, card(rangeExpr)).(*OperExpr)
	if rangeGot.Operator().Name() != "IF_THEN_ELSE" || rangeGot.Arguments()[1].(*OperExpr).Operator().Name() != "PLUS" {
		t.Fatalf("range cardinality = %v", rangeGot)
	}
	powerGot := optimizeForTest(t, card(MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(intSet)), set))).(*OperExpr)
	if powerGot.Operator().Name() != "POW" {
		t.Fatalf("powerset cardinality = %v", powerGot)
	}

	atLeastTwo := optimizeForTest(t, MustOperExpr(Op("GE"), Typed(BoolT1), card(set), i(2))).(*LetInExpr)
	if len(atLeastTwo.Declarations()) != 1 || atLeastTwo.Declarations()[0].Name() != "t_3" || atLeastTwo.Body().(*OperExpr).Arguments()[0].(*NameExpr).Name() != "t_1" {
		t.Fatalf("cardinality >= 2 = %v", atLeastTwo)
	}
	greaterThanOne := optimizeForTest(t, MustOperExpr(Op("GT"), Typed(BoolT1), card(set), i(1)))
	if _, ok := greaterThanOne.(*LetInExpr); !ok {
		t.Fatalf("cardinality > 1 = %v", greaterThanOne)
	}
	general := MustOperExpr(Op("GT"), Typed(BoolT1), card(set), i(5))
	if got := optimizeForTest(t, general); got != general {
		t.Fatalf("general cardinality comparison changed: %v", got)
	}
}

func TestExprOptimizerClassifiesMalformedSetTypes(t *testing.T) {
	notASet := NewNameExpr("n", Typed(IntT1))
	rightSet := NewNameExpr("S", Typed(mustSet(IntT1)))
	powerset := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(mustSet(IntT1))), rightSet)
	_, err := optimizerForTest().Expression(MustOperExpr(Op("SET_IN"), Typed(BoolT1), notASet, powerset))
	var malformed *MalformedTlaError
	if !errors.As(err, &malformed) || malformed.CauseExpr() != notASet {
		t.Fatalf("powerset membership error = %T %v", err, err)
	}

	cardinality := MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), notASet)
	comparison := MustOperExpr(Op("GE"), Typed(BoolT1), cardinality, i(2))
	_, err = optimizerForTest().Expression(comparison)
	var typing *TypingException
	if !errors.As(err, &typing) || typing.CauseExprID() != comparison.ID() {
		t.Fatalf("cardinality type error = %T %v", err, err)
	}
}

func TestExprOptimizerExistsOverSetComprehensions(t *testing.T) {
	intSet, boolSet := mustSet(IntT1), mustSet(BoolT1)
	x, y := NewNameExpr("x", Typed(IntT1)), NewNameExpr("y", Typed(IntT1))
	set := NewNameExpr("S", Typed(intSet))
	filter := MustOperExpr(Op("SET_FILTER"), Typed(intSet), y, set,
		MustOperExpr(Op("GT"), Typed(BoolT1), y, i(0)))
	predicate := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("z", Typed(IntT1)), x)
	got := optimizeForTest(t, MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x, filter, predicate)).(*OperExpr)
	if got.Operator().Name() != "EXISTS3" || got.Arguments()[0].(*NameExpr).Name() != "y" || got.Arguments()[2].(*OperExpr).Operator().Name() != "AND" {
		t.Fatalf("exists over filter = %v", got)
	}

	function := NewNameExpr("f", Typed(mustFun(IntT1, BoolT1)))
	application := MustOperExpr(Op("FUN_APP"), Typed(BoolT1), function, y)
	mapped := MustOperExpr(Op("SET_MAP"), Typed(boolSet), application, y, set)
	boolX := NewNameExpr("bx", Typed(BoolT1))
	mapPredicate := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("p", Typed(BoolT1)), boolX)
	mapGot := optimizeForTest(t, MustOperExpr(Op("EXISTS3"), Typed(BoolT1), boolX, mapped, mapPredicate)).(*OperExpr)
	if mapGot.Operator().Name() != "EXISTS3" || mapGot.Arguments()[0].(*NameExpr).Name() != "y" {
		t.Fatalf("exists over map = %v", mapGot)
	}
	equality := mapGot.Arguments()[2].(*OperExpr)
	if equality.Arguments()[1].(*OperExpr).Operator().Name() != "FUN_APP" {
		t.Fatalf("mapped value was not substituted: %v", equality)
	}
}

func TestExprOptimizerBetaReducesConstructorApplicationsInDomainQuantifiers(t *testing.T) {
	intSet := mustSet(IntT1)
	functionType := mustFun(IntT1, IntT1)
	x := NewNameExpr("x", Typed(IntT1))
	iBound := NewNameExpr("i", Typed(IntT1))
	rangeExpr := MustOperExpr(Op("INT_RANGE"), Typed(intSet), i(1), i(10000))
	body := MustOperExpr(Op("PLUS"), Typed(IntT1), x, i(1))
	constructor := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), body, x, rangeExpr)
	domain := MustOperExpr(Op("DOMAIN"), Typed(intSet), constructor)
	application := MustOperExpr(Op("FUN_APP"), Typed(IntT1), constructor, iBound)
	predicate := MustOperExpr(Op("EQ"), Typed(BoolT1), application,
		MustOperExpr(Op("PLUS"), Typed(IntT1), iBound, i(1)))

	got := optimizeForTest(t, MustOperExpr(Op("EXISTS3"), Typed(BoolT1), iBound, domain, predicate)).(*OperExpr)
	if !EqualExpr(got.Arguments()[1], rangeExpr) {
		t.Fatalf("quantifier domain = %v, want %v", got.Arguments()[1], rangeExpr)
	}
	wantBody := MustOperExpr(Op("PLUS"), Typed(IntT1), iBound, i(1))
	wantPredicate := MustOperExpr(Op("EQ"), Typed(BoolT1), wantBody, wantBody)
	if !EqualTypedExpr(got.Arguments()[2], wantPredicate) {
		t.Fatalf("quantifier predicate = %v, want %v", got.Arguments()[2], wantPredicate)
	}

	differentArgument := i(2)
	unreducedApplication := MustOperExpr(Op("FUN_APP"), Typed(IntT1), constructor, differentArgument)
	unreducedPredicate := MustOperExpr(Op("EQ"), Typed(BoolT1), unreducedApplication, i(3))
	unreduced := optimizeForTest(t, MustOperExpr(Op("FORALL3"), Typed(BoolT1), iBound, domain, unreducedPredicate)).(*OperExpr)
	if !EqualTypedExpr(unreduced.Arguments()[2], unreducedPredicate) {
		t.Fatalf("application at a different argument was reduced: %v", unreduced.Arguments()[2])
	}
}

func TestExprOptimizerPreservesPassByNameOperatorReturningFunction(t *testing.T) {
	functionType := mustFun(IntT1, IntT1)
	operatorType := mustOperator([]Type{functionType, functionType}, functionType)
	left := NewNameExpr("left", Typed(functionType))
	right := NewNameExpr("right", Typed(functionType))
	binder := NewNameExpr("x", Typed(IntT1))
	constructor := MustOperExpr(Op("FUN_CTOR"), Typed(functionType),
		MustOperExpr(Op("PLUS"), Typed(IntT1),
			MustOperExpr(Op("FUN_APP"), Typed(IntT1), left, binder),
			MustOperExpr(Op("FUN_APP"), Typed(IntT1), right, binder)),
		binder, MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), i(1)))
	declaration := NewOperDecl("Plus",
		[]OperatorParam{ValueParam("left"), ValueParam("right")}, constructor, Typed(operatorType))
	passByName := NewLetInExpr(NewNameExpr("Plus", Typed(operatorType)), Typed(operatorType), declaration)

	got := optimizeForTest(t, passByName)
	let, ok := got.(*LetInExpr)
	if !ok || len(let.Declarations()) != 1 || let.Declarations()[0].Name() != "Plus" {
		t.Fatalf("pass-by-name operator was removed: %v", got)
	}
}

func TestExprOptimizerUsesAbstractTransformerTrackingLayers(t *testing.T) {
	leaf := NewNameExpr("x", Typed(IntT1))
	unchanged := MustOperExpr(Op("PLUS"), Typed(IntT1), leaf, i(1))
	listener := &recordingListener{}
	got, err := NewExprOptimizer(nil, NewListenerTracker(listener)).Expression(unchanged)
	if err != nil || got != unchanged {
		t.Fatalf("unchanged expression = %v, %v", got, err)
	}
	// Each leaf and operator has a shallow and recursive tracking layer. No
	// hold is introduced because all recursively transformed child IDs agree.
	if len(listener.expressions) != 6 {
		t.Fatalf("tracking events = %d, want 6: %#v", len(listener.expressions), listener.expressions)
	}
	for _, event := range listener.expressions {
		if event[0] != event[1] {
			t.Fatalf("unchanged expression acquired an intermediate rebuild: %#v", event)
		}
	}

	letBody := NewNameExpr("body", Typed(IntT1))
	definitionBody := NewNameExpr("definition", Typed(IntT1))
	definition := NewOperDecl("D", nil, definitionBody, Typed(mustOperator(nil, IntT1)))
	let := NewLetInExpr(letBody, Typed(IntT1), definition)
	listener = &recordingListener{}
	if _, err := NewExprOptimizer(nil, NewListenerTracker(listener)).Expression(let); err != nil {
		t.Fatal(err)
	}
	// Two leaves receive two layers each; LET itself receives only the outer
	// recursive layer because AbstractTransformer never offers LET to rules.
	if len(listener.expressions) != 5 {
		t.Fatalf("LET tracking events = %d, want 5: %#v", len(listener.expressions), listener.expressions)
	}
	if listener.expressions[0][0] != letBody || listener.expressions[2][0] != definitionBody {
		t.Fatalf("LET was not traversed body-first: %#v", listener.expressions)
	}
}

func TestExprOptimizerAllocatesFreshNamesInScalaTraversalOrder(t *testing.T) {
	intSet := mustSet(IntT1)
	powersetMembership := func(left, right string) Expr {
		set := NewNameExpr(left, Typed(intSet))
		powerset := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(intSet)), NewNameExpr(right, Typed(intSet)))
		return MustOperExpr(Op("SET_IN"), Typed(BoolT1), set, powerset)
	}
	definition := NewOperDecl("D", nil, powersetMembership("A", "B"), Typed(mustOperator(nil, BoolT1)))
	got, err := optimizerForTest().Expression(NewLetInExpr(powersetMembership("S", "T"), Typed(BoolT1), definition))
	if err != nil {
		t.Fatal(err)
	}
	let := got.(*LetInExpr)
	if let.Body().(*OperExpr).Arguments()[0].(*NameExpr).Name() != "t_1" {
		t.Fatalf("LET body did not receive first fresh name: %v", let.Body())
	}
	if let.Declarations()[0].Body().(*OperExpr).Arguments()[0].(*NameExpr).Name() != "t_2" {
		t.Fatalf("LET definition did not receive second fresh name: %v", let.Declarations()[0].Body())
	}
}

func TestExprOptimizerMatchesScalaExactDerivedCardinalityShapes(t *testing.T) {
	intSet := mustSet(IntT1)
	set := NewNameExpr("S", Typed(intSet))
	cardinality := func(expression Expr) Expr {
		return MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), expression)
	}
	expectedAtLeastTwo := func() Expr {
		t1 := NewNameExpr("t_1", Typed(IntT1))
		t2 := NewNameExpr("t_2", Typed(IntT1))
		operatorType := mustOperator(nil, intSet)
		application := func() Expr {
			return MustOperExpr(Op("OPER_APP"), Typed(intSet), NewNameExpr("t_3", Typed(operatorType)))
		}
		distinct := MustOperExpr(Op("NOT"), Typed(BoolT1),
			MustOperExpr(Op("EQ"), Typed(BoolT1), t1, t2))
		body := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), t1, application(),
			MustOperExpr(Op("EXISTS3"), Typed(BoolT1), t2, application(), distinct))
		return NewLetInExpr(body, Typed(BoolT1), NewOperDecl("t_3", nil, set, Typed(operatorType)))
	}
	for _, input := range []Expr{
		MustOperExpr(Op("GE"), Typed(BoolT1), cardinality(set), i(2)),
		MustOperExpr(Op("GT"), Typed(BoolT1), cardinality(set), i(1)),
	} {
		got := optimizeForTest(t, input)
		if want := expectedAtLeastTwo(); !EqualTypedExpr(got, want) {
			t.Fatalf("cardinality lower bound = %v, want %v", got, want)
		}
	}

	a := NewNameExpr("a", Typed(IntT1))
	b := NewNameExpr("b", Typed(IntT1))
	rangeExpr := MustOperExpr(Op("INT_RANGE"), Typed(intSet), a, b)
	wantRange := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1),
		MustOperExpr(Op("LE"), Typed(BoolT1), a, b),
		MustOperExpr(Op("PLUS"), Typed(IntT1),
			MustOperExpr(Op("MINUS"), Typed(IntT1), b, a), i(1)), i(0))
	if got := optimizeForTest(t, cardinality(rangeExpr)); !EqualTypedExpr(got, wantRange) {
		t.Fatalf("range cardinality = %v, want %v", got, wantRange)
	}
	wantPower := MustOperExpr(Op("POW"), Typed(IntT1), i(2), cardinality(set))
	if got := optimizeForTest(t, cardinality(MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(intSet)), set))); !EqualTypedExpr(got, wantPower) {
		t.Fatalf("powerset cardinality = %v, want %v", got, wantPower)
	}
}

func TestExprOptimizerMatchesScalaExactRecordMapMembership(t *testing.T) {
	intSet := mustSet(IntT1)
	recordType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": IntT1})
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(IntT1))
	left := NewNameExpr("S", Typed(intSet))
	right := NewNameExpr("T", Typed(intSet))
	record := MustOperExpr(Op("RECORD"), Typed(recordType), s("a"), x, s("b"), y)
	mapped := MustOperExpr(Op("SET_MAP"), Typed(mustSet(recordType)), record, x, left, y, right)
	member := NewNameExpr("r", Typed(recordType))
	got := optimizeForTest(t, MustOperExpr(Op("SET_IN"), Typed(BoolT1), member, mapped))
	stringSet := mustSet(StrT1)
	want := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1),
			MustOperExpr(Op("DOMAIN"), Typed(stringSet), member),
			MustOperExpr(Op("SET_ENUM"), Typed(stringSet), s("a"), s("b"))),
		MustOperExpr(Op("SET_IN"), Typed(BoolT1),
			MustOperExpr(Op("FUN_APP"), Typed(IntT1), member, s("a")), left),
		MustOperExpr(Op("SET_IN"), Typed(BoolT1),
			MustOperExpr(Op("FUN_APP"), Typed(IntT1), member, s("b")), right))
	if !EqualTypedExpr(got, want) {
		t.Fatalf("record map membership = %v, want %v", got, want)
	}
}

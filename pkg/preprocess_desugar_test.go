package apalache

import (
	"errors"
	"testing"
)

func TestDesugarerUnchangedStateAndOrdinaryExpressions(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(IntT1))
	constant := NewNameExpr("N", Typed(IntT1))
	nested := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1, mustTuple(IntT1))), x, MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1)), y))
	desugarer := NewDesugarer(nil, []string{"x", "y"}, nil, nil)
	got, err := desugarer.Expression(MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), nested))
	if err != nil {
		t.Fatal(err)
	}
	and := got.(*OperExpr)
	if and.Operator().Name() != "AND" || len(and.Arguments()) != 2 {
		t.Fatalf("UNCHANGED tuple = %v", got)
	}
	for _, equality := range and.Arguments() {
		if equality.(*OperExpr).Operator().Name() != "Apalache!:=" {
			t.Fatalf("state equality = %v", equality)
		}
	}
	ordinary, err := desugarer.Expression(MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), constant))
	if err != nil || ordinary.(*OperExpr).Operator().Name() != "EQ" {
		t.Fatalf("UNCHANGED constant = %v, %v", ordinary, err)
	}
	ordinaryArgs := ordinary.(*OperExpr).Arguments()
	if ordinaryArgs[0].(*OperExpr).Arguments()[0].ID() == ordinaryArgs[1].ID() {
		t.Fatal("UNCHANGED reused the same operand identity on both sides")
	}
	empty, err := desugarer.Expression(MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), MustOperExpr(Op("TUPLE"), Typed(mustTuple()))))
	if err != nil || !EqualTypedExpr(empty, NewValueExpr(BoolValue(true), Typed(BoolT1))) {
		t.Fatalf("UNCHANGED empty = %v, %v", empty, err)
	}
}

func TestDesugarerExpandsGroupedUnchangedDefinitions(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(IntT1))
	groupType := mustTuple(IntT1, IntT1)
	definition := MustOperExpr(Op("TUPLE"), Typed(groupType), x, y)
	application := MustOperExpr(Op("OPER_APP"), Typed(groupType), NewNameExpr("Vars", Typed(mustOperator(nil, groupType))))
	desugarer := NewDesugarer(nil, []string{"x", "y"}, nil, map[string]Expr{"Vars": definition})
	got, err := desugarer.Expression(MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), application))
	if err != nil {
		t.Fatal(err)
	}
	and := got.(*OperExpr)
	if and.Operator().Name() != "AND" || len(and.Arguments()) != 2 {
		t.Fatalf("grouped UNCHANGED = %v", got)
	}
}

func TestDesugarerTupleComparison(t *testing.T) {
	i := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	tupleType := mustTuple(IntT1, IntT1)
	left := MustOperExpr(Op("TUPLE"), Typed(tupleType), i(1), i(2))
	right := MustOperExpr(Op("TUPLE"), Typed(tupleType), i(1), i(3))
	desugarer := NewDesugarer(nil, nil, nil, nil)
	for _, test := range []struct{ operator, connective string }{{"EQ", "AND"}, {"NE", "OR"}} {
		got, err := desugarer.Expression(MustOperExpr(Op(test.operator), Typed(BoolT1), left, right))
		if err != nil {
			t.Fatal(err)
		}
		operation := got.(*OperExpr)
		if operation.Operator().Name() != test.connective || len(operation.Arguments()) != 2 {
			t.Fatalf("%s tuples = %v", test.operator, got)
		}
	}
	short := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1)), i(1))
	got, _ := desugarer.Expression(MustOperExpr(Op("EQ"), Typed(BoolT1), left, short))
	if !EqualTypedExpr(got, NewValueExpr(BoolValue(false), Typed(BoolT1))) {
		t.Fatalf("different tuple arity = %v", got)
	}
}

func TestDesugarerStutterAndLeadsTo(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	action := NewNameExpr("A", Typed(BoolT1))
	desugarer := NewDesugarer(nil, []string{"x"}, nil, nil)
	stutter, err := desugarer.Expression(MustOperExpr(Op("STUTTER"), Typed(BoolT1), action, x))
	if err != nil {
		t.Fatal(err)
	}
	if stutter.(*OperExpr).Operator().Name() != "OR" {
		t.Fatalf("stutter = %v", stutter)
	}
	nostutter, err := desugarer.Expression(MustOperExpr(Op("NO_STUTTER"), Typed(BoolT1), action, x))
	if err != nil {
		t.Fatal(err)
	}
	if nostutter.(*OperExpr).Operator().Name() != "AND" || nostutter.(*OperExpr).Arguments()[1].(*OperExpr).Operator().Name() != "NOT" {
		t.Fatalf("nostutter = %v", nostutter)
	}
	leads, err := desugarer.Expression(MustOperExpr(Op("LEADS_TO"), Typed(BoolT1), action, NewNameExpr("B", Typed(BoolT1))))
	if err != nil {
		t.Fatal(err)
	}
	globally := leads.(*OperExpr)
	if globally.Operator().Name() != "GLOBALLY" || globally.Arguments()[0].(*OperExpr).Arguments()[1].(*OperExpr).Operator().Name() != "EVENTUALLY" {
		t.Fatalf("leads-to = %v", leads)
	}
}

func TestDesugarerCollapsesTuplePatternsInBinders(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(BoolT1))
	tupleType := mustTuple(IntT1, BoolT1)
	pattern := MustOperExpr(Op("TUPLE"), Typed(tupleType), x, y)
	set := NewNameExpr("Pairs", Typed(mustSet(tupleType)))
	predicate := MustOperExpr(Op("EQ"), Typed(BoolT1), x, NewValueExpr(Int64Value(1), Typed(IntT1)))
	desugarer := NewDesugarer(nil, nil, nil, nil)
	for _, operator := range []string{"SET_FILTER", "EXISTS3", "FORALL3"} {
		got, err := desugarer.Expression(MustOperExpr(Op(operator), Typed(BoolT1), pattern, set, predicate))
		if err != nil {
			t.Fatal(err)
		}
		arguments := got.(*OperExpr).Arguments()
		bound := arguments[0].(*NameExpr)
		if bound.Name() != "x_y" || !EqualTypes(mustTagType(bound.Tag()), tupleType) {
			t.Fatalf("%s bound = %v", operator, bound)
		}
		comparison := arguments[2].(*OperExpr)
		access := comparison.Arguments()[0].(*OperExpr)
		if access.Operator().Name() != "FUN_APP" || access.Arguments()[0].(*NameExpr).Name() != "x_y" {
			t.Fatalf("%s predicate = %v", operator, comparison)
		}
	}
}

func TestDesugarerCollapsesNestedTuplePatternAndSetMap(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(BoolT1))
	z := NewNameExpr("z", Typed(StrT1))
	innerType := mustTuple(BoolT1, StrT1)
	outerType := mustTuple(IntT1, innerType)
	pattern := MustOperExpr(Op("TUPLE"), Typed(outerType), x, MustOperExpr(Op("TUPLE"), Typed(innerType), y, z))
	set := NewNameExpr("S", Typed(mustSet(outerType)))
	body := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1, StrT1)), x, z)
	got, err := NewDesugarer(nil, nil, nil, nil).Expression(MustOperExpr(Op("SET_MAP"), Typed(mustSet(mustTuple(IntT1, StrT1))), body, pattern, set))
	if err != nil {
		t.Fatal(err)
	}
	arguments := got.(*OperExpr).Arguments()
	if arguments[1].(*NameExpr).Name() != "x_y_z" {
		t.Fatalf("map binder = %v", arguments[1])
	}
	newBody := arguments[0].(*OperExpr)
	zAccess := newBody.Arguments()[1].(*OperExpr)
	if zAccess.Operator().Name() != "FUN_APP" || zAccess.Arguments()[0].(*OperExpr).Operator().Name() != "FUN_APP" {
		t.Fatalf("nested access = %v", zAccess)
	}
}

func TestDesugarerNormalizesMultiArgumentFunction(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(BoolT1))
	intSet := NewValueExpr(IntSet, Typed(mustSet(IntT1)))
	boolSet := NewValueExpr(BoolSet, Typed(mustSet(BoolT1)))
	body := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1), y, x, NewValueExpr(Int64Value(0), Typed(IntT1)))
	functionType := mustFun(mustTuple(IntT1, BoolT1), IntT1)
	got, err := NewDesugarer(nil, nil, nil, nil).Expression(MustOperExpr(Op("FUN_CTOR"), Typed(functionType), body, x, intSet, y, boolSet))
	if err != nil {
		t.Fatal(err)
	}
	arguments := got.(*OperExpr).Arguments()
	if len(arguments) != 3 {
		t.Fatalf("function args = %v", arguments)
	}
	if arguments[1].(*NameExpr).Name() != "x_y" || arguments[2].(*OperExpr).Operator().Name() != "SET_TIMES" {
		t.Fatalf("normalized function = %v", got)
	}
	ifBody := arguments[0].(*OperExpr)
	if ifBody.Arguments()[0].(*OperExpr).Operator().Name() != "FUN_APP" {
		t.Fatalf("function body not substituted: %v", ifBody)
	}
}

func TestDesugarerNormalizesRecursiveFunctionAndTracksTupleConstruction(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(BoolT1))
	intSet := NewValueExpr(IntSet, Typed(mustSet(IntT1)))
	boolSet := NewValueExpr(BoolSet, Typed(mustSet(BoolT1)))
	body := NewValueExpr(Int64Value(0), Typed(IntT1))
	listener := &recordingListener{}
	functionType := mustFun(mustTuple(IntT1, BoolT1), IntT1)
	got, err := NewDesugarer(nil, nil, NewListenerTracker(listener), nil).Expression(
		MustOperExpr(Op("FUN_REC_CTOR"), Typed(functionType), body, x, intSet, y, boolSet))
	if err != nil {
		t.Fatal(err)
	}
	arguments := got.(*OperExpr).Arguments()
	if len(arguments) != 3 || arguments[1].(*NameExpr).Name() != "x_y" || arguments[2].(*OperExpr).Operator().Name() != "SET_TIMES" {
		t.Fatalf("normalized recursive function = %v", got)
	}
	foundTupleHold, foundProductHold := false, false
	for _, event := range listener.expressions {
		if event[0] == x {
			operation, ok := event[1].(*OperExpr)
			foundTupleHold = foundTupleHold || ok && operation.Operator().Name() == "TUPLE"
		}
		if event[0] == intSet {
			operation, ok := event[1].(*OperExpr)
			foundProductHold = foundProductHold || ok && operation.Operator().Name() == "SET_TIMES"
		}
	}
	if !foundTupleHold || !foundProductHold {
		t.Fatalf("missing function normalization holds: tuple=%v product=%v", foundTupleHold, foundProductHold)
	}
}

func TestDesugarerKeepsUnaryFunctionsAndTraversesGenericForms(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	set := NewValueExpr(IntSet, Typed(mustSet(IntT1)))
	body := MustOperExpr(Op("PLUS"), Typed(IntT1), x, NewValueExpr(Int64Value(1), Typed(IntT1)))
	desugarer := NewDesugarer(nil, nil, nil, nil)
	for _, operator := range []string{"FUN_CTOR", "FUN_REC_CTOR"} {
		input := MustOperExpr(Op(operator), Typed(mustFun(IntT1, IntT1)), body, x, set)
		got, err := desugarer.Expression(input)
		if err != nil || !EqualTypedExpr(got, input) {
			t.Fatalf("unary %s = %v, %v", operator, got, err)
		}
	}
	operatorType := mustOperator([]Type{IntT1}, IntT1)
	application := MustOperExpr(Op("OPER_APP"), Typed(IntT1), NewNameExpr("Foo", Typed(operatorType)), body)
	declaration := NewOperDecl("Foo", []OperatorParam{ValueParam("arg")}, NewNameExpr("arg", Typed(IntT1)), Typed(operatorType))
	input := NewLetInExpr(application, Typed(IntT1), declaration)
	got, err := desugarer.Expression(input)
	if err != nil || !EqualTypedExpr(got, input) {
		t.Fatalf("generic operator/LET traversal = %v, %v", got, err)
	}
}

func TestDesugarerTracksLeavesAndVisitsLetBodyFirst(t *testing.T) {
	body := NewNameExpr("body", Typed(IntT1))
	definitionBody := NewNameExpr("definition", Typed(IntT1))
	declaration := NewOperDecl("D", nil, definitionBody, Typed(mustOperator(nil, IntT1)))
	root := NewLetInExpr(body, Typed(IntT1), declaration)
	listener := &recordingListener{}
	got, err := NewDesugarer(nil, nil, NewListenerTracker(listener), nil).Expression(root)
	if err != nil {
		t.Fatal(err)
	}
	if got == root || len(listener.expressions) != 3 {
		t.Fatalf("tracked LET result=%v, events=%d", got, len(listener.expressions))
	}
	if listener.expressions[0][0] != body || listener.expressions[0][1] != body || listener.expressions[1][0] != definitionBody || listener.expressions[1][1] != definitionBody {
		t.Fatalf("unexpected traversal events: %#v", listener.expressions)
	}
}

func TestDesugarerLeadsToRetainsOperandSugarForLaterPassTraversal(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	unchanged := MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), x)
	got, err := NewDesugarer(nil, []string{"x"}, nil, nil).Expression(
		MustOperExpr(Op("LEADS_TO"), Typed(BoolT1), unchanged, NewValueExpr(BoolValue(true), Typed(BoolT1))))
	if err != nil {
		t.Fatal(err)
	}
	antecedent := got.(*OperExpr).Arguments()[0].(*OperExpr).Arguments()[0]
	if antecedent != unchanged {
		t.Fatalf("LEADS_TO recursively transformed its antecedent: %v", antecedent)
	}
}

func TestDesugarerKeepsSimpleExceptAndExpandsNestedExcept(t *testing.T) {
	innerType := mustFun(StrT1, BoolT1)
	outerType := mustFun(IntT1, innerType)
	function := NewNameExpr("f", Typed(outerType))
	intKey := NewValueExpr(Int64Value(1), Typed(IntT1))
	strKey := NewValueExpr(StringValue("ok"), Typed(StrT1))
	value := NewValueExpr(BoolValue(true), Typed(BoolT1))
	simplePath := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1)), intKey)
	simple := MustOperExpr(Op("EXCEPT"), Typed(outerType), function, simplePath, NewNameExpr("g", Typed(innerType)))
	desugarer := NewDesugarer(nil, nil, nil, nil)
	kept, err := desugarer.Expression(simple)
	if err != nil {
		t.Fatal(err)
	}
	if operation, ok := kept.(*OperExpr); !ok || operation.Operator().Name() != "EXCEPT" {
		t.Fatalf("simple EXCEPT = %v", kept)
	}

	nestedPath := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1, StrT1)), intKey, strKey)
	nested := MustOperExpr(Op("EXCEPT"), Typed(outerType), function, nestedPath, value)
	got, err := desugarer.Expression(nested)
	if err != nil {
		t.Fatal(err)
	}
	let, ok := got.(*LetInExpr)
	if !ok || len(let.Declarations()) != 3 {
		t.Fatalf("nested EXCEPT = %T %v", got, got)
	}
	if let.Declarations()[0].Name() != "t_1" || let.Declarations()[1].Name() != "t_2" || let.Declarations()[2].Name() != "t_3" {
		t.Fatalf("nested definitions = %#v", let.Declarations())
	}
	innerUpdate := let.Declarations()[1].Body().(*OperExpr)
	outerUpdate := let.Declarations()[2].Body().(*OperExpr)
	if innerUpdate.Operator().Name() != "EXCEPT" || outerUpdate.Operator().Name() != "EXCEPT" {
		t.Fatalf("nested update bodies = %v / %v", innerUpdate, outerUpdate)
	}
}

func TestDesugarerExpandsMultipleExceptUpdatesSequentially(t *testing.T) {
	functionType := mustFun(IntT1, IntT1)
	function := NewNameExpr("f", Typed(functionType))
	path := func(index int64) Expr {
		key := NewValueExpr(Int64Value(index), Typed(IntT1))
		return MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1)), key)
	}
	expression := MustOperExpr(Op("EXCEPT"), Typed(functionType), function, path(1), NewValueExpr(Int64Value(10), Typed(IntT1)), path(2), NewValueExpr(Int64Value(20), Typed(IntT1)))
	got, err := NewDesugarer(nil, nil, nil, nil).Expression(expression)
	if err != nil {
		t.Fatal(err)
	}
	let, ok := got.(*LetInExpr)
	if !ok || len(let.Declarations()) != 3 {
		t.Fatalf("multiple EXCEPT = %v", got)
	}
	secondUpdate := let.Declarations()[2].Body().(*OperExpr)
	baseApplication := secondUpdate.Arguments()[0].(*OperExpr)
	if baseApplication.Operator().Name() != "OPER_APP" || baseApplication.Arguments()[0].(*NameExpr).Name() != let.Declarations()[1].Name() {
		t.Fatalf("updates are not sequential: %v", secondUpdate)
	}
}

func TestDesugarerExceptValidatesAccessorsAndTypes(t *testing.T) {
	functionType := mustFun(IntT1, IntT1)
	function := NewNameExpr("f", Typed(functionType))
	value := NewValueExpr(Int64Value(1), Typed(IntT1))
	badAccessor := NewNameExpr("i", Typed(IntT1))
	_, err := NewDesugarer(nil, nil, nil, nil).Expression(MustOperExpr(Op("EXCEPT"), Typed(functionType), function, badAccessor, value))
	var lirError *LirError
	if !errors.As(err, &lirError) {
		t.Fatalf("non-tuple EXCEPT accessor error = %T %v", err, err)
	}
	badPath := MustOperExpr(Op("TUPLE"), Typed(mustTuple(StrT1, IntT1)), NewValueExpr(StringValue("bad"), Typed(StrT1)), value)
	_, err = NewDesugarer(nil, nil, nil, nil).Expression(MustOperExpr(Op("EXCEPT"), Typed(functionType), function, badPath, value))
	var typingError *TypingException
	if !errors.As(err, &typingError) || typingError.CauseExprID() != badPath.Arguments()[0].ID() {
		t.Fatalf("wrong multidimensional EXCEPT key error = %T %v", err, err)
	}
}

func TestExceptAccessResultTypeSupportsCompositeContainers(t *testing.T) {
	row, _ := NewRowType(map[string]Type{"field": BoolT1}, nil)
	record, _ := NewRecordType(map[string]Type{"field": IntT1})
	tuple := mustTuple(StrT1, BoolT1)
	cases := []struct {
		container Type
		key       Expr
		want      Type
	}{
		{mustSeq(StrT1), NewValueExpr(Int64Value(1), Typed(IntT1)), StrT1},
		{tuple, NewValueExpr(Int64Value(2), Typed(IntT1)), BoolT1},
		{record, NewValueExpr(StringValue("field"), Typed(StrT1)), IntT1},
		{NewRecordRowType(row), NewValueExpr(StringValue("field"), Typed(StrT1)), BoolT1},
	}
	for _, test := range cases {
		got, err := exceptAccessResultType(test.container, test.key)
		if err != nil || !EqualTypes(got, test.want) {
			t.Errorf("access %s[%v] = %v, %v; want %v", test.container, test.key, got, err, test.want)
		}
	}
}

func TestDesugarerScalaCompositeAndThreeDimensionalExcept(t *testing.T) {
	record, err := NewRecordType(map[string]Type{"field": BoolT1})
	if err != nil {
		t.Fatal(err)
	}
	row, err := NewRowType(map[string]Type{"field": BoolT1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		container Type
		key       Expr
	}{
		{"record", record, NewValueExpr(StringValue("field"), Typed(StrT1))},
		{"row record", NewRecordRowType(row), NewValueExpr(StringValue("field"), Typed(StrT1))},
		{"tuple", mustTuple(IntT1, BoolT1), intLiteral(2)},
		{"sequence", mustSeq(BoolT1), intLiteral(1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			outerType := mustFun(IntT1, test.container)
			path := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1, mustTagType(test.key.Tag()))), intLiteral(1), test.key)
			input := MustOperExpr(Op("EXCEPT"), Typed(outerType), NewNameExpr("f", Typed(outerType)), path, temporalBool(true))
			got, transformErr := NewDesugarer(NewUniqueNameGenerator(), nil, nil, nil).Expression(input)
			if transformErr != nil {
				t.Fatal(transformErr)
			}
			let, ok := got.(*LetInExpr)
			if !ok || len(let.Declarations()) != 3 {
				t.Fatalf("nested %s EXCEPT = %v", test.name, got)
			}
			expectedNames := []string{"t_1", "t_2", "t_3"}
			for index, declaration := range let.Declarations() {
				if declaration.Name() != expectedNames[index] {
					t.Fatalf("nested %s EXCEPT declaration %d = %s", test.name, index, declaration.Name())
				}
			}
			if let.Declarations()[1].Body().(*OperExpr).Operator().Name() != "EXCEPT" ||
				let.Declarations()[2].Body().(*OperExpr).Operator().Name() != "EXCEPT" {
				t.Fatalf("nested %s update bodies = %v", test.name, let)
			}
		})
	}

	innerType := mustSeq(BoolT1)
	middleType := mustFun(StrT1, innerType)
	outerType := mustFun(IntT1, middleType)
	path := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1, StrT1, IntT1)),
		intLiteral(1), NewValueExpr(StringValue("key"), Typed(StrT1)), intLiteral(1))
	threeDimensional := MustOperExpr(Op("EXCEPT"), Typed(outerType), NewNameExpr("f", Typed(outerType)), path, temporalBool(false))
	got, err := NewDesugarer(NewUniqueNameGenerator(), nil, nil, nil).Expression(threeDimensional)
	if err != nil {
		t.Fatal(err)
	}
	let, ok := got.(*LetInExpr)
	if !ok || len(let.Declarations()) != 4 || countOperator(got, "EXCEPT") != 3 {
		t.Fatalf("three-dimensional EXCEPT = %v", got)
	}
}

func TestDesugarerScalaTupleMismatchAndNestedEmptyUnchanged(t *testing.T) {
	left := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1, IntT1)), intLiteral(1), intLiteral(2))
	right := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1, IntT1, IntT1)), intLiteral(1), intLiteral(2), intLiteral(3))
	got, err := NewDesugarer(nil, nil, nil, nil).Expression(MustOperExpr(Op("NE"), Typed(BoolT1), left, right))
	if err != nil || !EqualTypedExpr(got, temporalBool(true)) {
		t.Fatalf("unequal tuple arity NE = %v, %v", got, err)
	}
	empty := MustOperExpr(Op("TUPLE"), Typed(mustTuple()))
	nested := MustOperExpr(Op("TUPLE"), Typed(mustTuple(mustTuple(), mustTuple())), empty, empty)
	got, err = NewDesugarer(nil, nil, nil, nil).Expression(MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), nested))
	if err != nil || !EqualTypedExpr(got, temporalBool(true)) {
		t.Fatalf("nested empty UNCHANGED = %v, %v", got, err)
	}
}

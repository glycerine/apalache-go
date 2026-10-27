package apalache

import "testing"

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

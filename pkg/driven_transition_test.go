package apalache

import "testing"

func TestDrivenTransitionConstructorAssignsSortedOutputsAndSubstitutesState(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(IntT1))
	constructor := NewDrivenTransitionConstructor(nil, map[string]Expr{
		"sum":  MustOperExpr(Op("PLUS"), Typed(IntT1), x, y),
		"copy": x,
	})
	transition, err := constructor.MakeTransition(TraceState{"x": intLiteral(2), "y": intLiteral(3)})
	if err != nil {
		t.Fatal(err)
	}
	and, ok := transition.(*OperExpr)
	if !ok || and.Operator().Name() != "AND" || len(and.Arguments()) != 2 {
		t.Fatalf("transition = %#v", transition)
	}
	wantNames := []string{"copy", "sum"}
	const assignmentOperator = "Apalache!" + ":="
	for index, raw := range and.Arguments() {
		assignment, ok := raw.(*OperExpr)
		if !ok || assignment.Operator().Name() != assignmentOperator {
			t.Fatalf("assignment %d = %#v", index, raw)
		}
		left := assignment.Arguments()[0].(*OperExpr)
		name := left.Arguments()[0].(*NameExpr).Name()
		if name != wantNames[index] {
			t.Fatalf("assignment %d name = %q", index, name)
		}
	}
	copyRight := and.Arguments()[0].(*OperExpr).Arguments()[1]
	if !EqualExpr(copyRight, intLiteral(2)) {
		t.Fatalf("copy RHS = %#v", copyRight)
	}
	sumRight := and.Arguments()[1].(*OperExpr).Arguments()[1]
	wantSum := MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(2), intLiteral(3))
	if !EqualExpr(sumRight, wantSum) {
		t.Fatalf("sum RHS = %#v", sumRight)
	}
}

func TestDrivenTransitionConstructorTracksReplacementAndDefensivelyCopiesExpressions(t *testing.T) {
	changes := NewChangeListener()
	x := NewNameExpr("x", Typed(IntT1))
	expressions := map[string]Expr{"out": x}
	constructor := NewDrivenTransitionConstructor(NewListenerTracker(changes), expressions)
	expressions["later"] = intLiteral(9)
	value := intLiteral(4)
	transition, err := constructor.MakeTransition(TraceState{"x": value})
	if err != nil {
		t.Fatal(err)
	}
	if !changes.IsDefinedAt(value.ID()) {
		t.Fatal("state substitution was not reported to the transformation tracker")
	}
	and := transition.(*OperExpr)
	if len(and.Arguments()) != 1 {
		t.Fatalf("constructor retained caller map mutation: %#v", and.Arguments())
	}
	if got := and.Arguments()[0].(*OperExpr).Arguments()[1]; !EqualExpr(got, value) {
		t.Fatalf("tracked RHS = %#v", got)
	}
}

func TestDrivenTransitionConstructorEmptyAndInvalidInputs(t *testing.T) {
	empty, err := NewDrivenTransitionConstructor(nil, nil).MakeTransition(nil)
	if err != nil {
		t.Fatal(err)
	}
	operation, ok := empty.(*OperExpr)
	if !ok || operation.Operator().Name() != "AND" || len(operation.Arguments()) != 0 {
		t.Fatalf("empty transition = %#v", empty)
	}
	if _, err = NewDrivenTransitionConstructor(nil, map[string]Expr{"x": NewNameExpr("y", Untyped())}).MakeTransition(nil); err == nil {
		t.Fatal("untyped driven expression was accepted")
	}
	if _, err = (*DrivenTransitionConstructor)(nil).MakeTransition(nil); err == nil {
		t.Fatal("nil driven constructor was accepted")
	}
}

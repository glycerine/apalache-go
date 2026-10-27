package apalache

import (
	"reflect"
	"testing"
)

func TestAllSelectionCombinations(t *testing.T) {
	a, b, c, d := NewUID(), NewUID(), NewUID(), NewUID()
	if got := allSelectionCombinations(nil); len(got) != 0 {
		t.Fatalf("empty combinations = %v", got)
	}
	sets := []assignmentSelectionSet{
		selectionSet(assignmentSelection{a}, assignmentSelection{b}),
		selectionSet(assignmentSelection{c}, assignmentSelection{d}),
	}
	got := allSelectionCombinations(sets)
	want := selectionSet(
		assignmentSelection{a, c}, assignmentSelection{a, d},
		assignmentSelection{b, c}, assignmentSelection{b, d},
	)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("combinations = %v, want %v", got, want)
	}
}

func TestSymbolicTransitionGeneratorSplitsDisjunctionAndLabels(t *testing.T) {
	first, second := assignmentCandidate("x", intLiteral(1)), assignmentCandidate("x", intLiteral(2))
	firstLabel := MustOperExpr(Op("LABEL"), Typed(BoolT1), first, s("L1"))
	secondLabel := MustOperExpr(Op("LABEL"), Typed(BoolT1), second, s("L2"))
	action := MustOperExpr(Op("OR"), Typed(BoolT1), firstLabel, secondLabel)
	transitions, err := NewSymbolicTransitionGenerator(nil).Generate(action, AssignmentStrategy{first.ID(), second.ID()})
	if err != nil {
		t.Fatal(err)
	}
	if len(transitions) != 2 {
		t.Fatalf("transition count = %d", len(transitions))
	}
	for index, transition := range transitions {
		label := transition.Expression.(*OperExpr)
		if label.Operator().Name() != "LABEL" || label.Arguments()[0].(*OperExpr).Operator().Name() != "Apalache!:=" {
			t.Fatalf("transition %d = %v", index, transition.Expression)
		}
		if len(transition.Assignments) != 1 || transition.Assignments[0] != label.Arguments()[0].ID() {
			t.Fatalf("transition %d assignment order = %v", index, transition.Assignments)
		}
	}
}

func TestSymbolicTransitionGeneratorConjunctionCartesianProduct(t *testing.T) {
	x1, x2 := assignmentCandidate("x", intLiteral(1)), assignmentCandidate("x", intLiteral(2))
	y1, y2 := assignmentCandidate("y", intLiteral(3)), assignmentCandidate("y", intLiteral(4))
	action := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("OR"), Typed(BoolT1), x1, x2),
		MustOperExpr(Op("OR"), Typed(BoolT1), y1, y2))
	strategy := AssignmentStrategy{x1.ID(), x2.ID(), y1.ID(), y2.ID()}
	transitions, err := NewSymbolicTransitionGenerator(nil).Generate(action, strategy)
	if err != nil {
		t.Fatal(err)
	}
	if len(transitions) != 4 {
		t.Fatalf("Cartesian transition count = %d", len(transitions))
	}
	for _, transition := range transitions {
		if len(transition.Assignments) != 2 || countOperator(transition.Expression, "Apalache!:=") != 2 {
			t.Fatalf("Cartesian transition = %#v", transition)
		}
	}
}

func TestSymbolicTransitionGeneratorSlicesNestedITE(t *testing.T) {
	first := assignmentCandidate("x", intLiteral(1))
	second := assignmentCandidate("x", intLiteral(2))
	third := assignmentCandidate("x", intLiteral(3))
	action := MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), NewValueExpr(BoolValue(true), Typed(BoolT1)), first,
		MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), NewValueExpr(BoolValue(true), Typed(BoolT1)), second, third))
	transitions, err := NewSymbolicTransitionGenerator(nil).Generate(action, AssignmentStrategy{first.ID(), second.ID(), third.ID()})
	if err != nil {
		t.Fatal(err)
	}
	if len(transitions) != 3 {
		t.Fatalf("ITE transition count = %d", len(transitions))
	}
	for _, transition := range transitions {
		if countOperator(transition.Expression, "PRIME") != 1 || countOperator(transition.Expression, "Apalache!:=") != 1 {
			t.Fatalf("ITE transition was not sliced to one assignment: %v", transition.Expression)
		}
	}
}

func TestSymbolicTransitionGeneratorSharesLetAssignmentAcrossBranches(t *testing.T) {
	assignment := assignmentCandidate("x", intLiteral(1))
	definition := NewOperDecl("X", nil, assignment, Typed(mustOperator(nil, BoolT1)))
	operatorType := mustOperator(nil, BoolT1)
	call := func() Expr { return MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr("X", Typed(operatorType))) }
	disjunction := MustOperExpr(Op("OR"), Typed(BoolT1),
		MustOperExpr(Op("AND"), Typed(BoolT1), NewNameExpr("A", Typed(BoolT1)), call()),
		MustOperExpr(Op("AND"), Typed(BoolT1), NewNameExpr("B", Typed(BoolT1)), call()))
	action := NewLetInExpr(disjunction, Typed(BoolT1), definition)
	transitions, err := NewSymbolicTransitionGenerator(nil).Generate(action, AssignmentStrategy{assignment.ID()})
	if err != nil {
		t.Fatal(err)
	}
	if len(transitions) != 1 {
		t.Fatalf("shared LET transition count = %d", len(transitions))
	}
	let := transitions[0].Expression.(*LetInExpr)
	if let.Declarations()[0].Body().(*OperExpr).Operator().Name() != "Apalache!:=" || let.Body().(*OperExpr).Operator().Name() != "OR" {
		t.Fatalf("shared LET transition = %v", let)
	}
}

func TestSymbolicTransitionGeneratorRejectsStrategyWithoutSelections(t *testing.T) {
	_, err := NewSymbolicTransitionGenerator(nil).Generate(NewValueExpr(BoolValue(true), Typed(BoolT1)), nil)
	if err == nil {
		t.Fatal("assignment-free expression unexpectedly produced transitions")
	}
}

func countOperator(expression Expr, name string) int {
	count := 0
	var visit func(Expr)
	visit = func(current Expr) {
		switch typed := current.(type) {
		case *OperExpr:
			if typed.Operator().Name() == name {
				count++
			}
			for _, argument := range typed.Arguments() {
				visit(argument)
			}
		case *LetInExpr:
			visit(typed.Body())
			for _, declaration := range typed.Declarations() {
				visit(declaration.Body())
			}
		}
	}
	visit(expression)
	return count
}

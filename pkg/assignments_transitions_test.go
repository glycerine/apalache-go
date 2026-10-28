package apalache

import (
	"reflect"
	"testing"
)

func TestAllSelectionCombinations(t *testing.T) {
	a, b, c, d, e, f := NewUID(), NewUID(), NewUID(), NewUID(), NewUID(), NewUID()
	if got := allSelectionCombinations(nil); len(got) != 0 {
		t.Fatalf("empty combinations = %v", got)
	}
	if got := allSelectionCombinations([]assignmentSelectionSet{selectionSet()}); len(got) != 0 {
		t.Fatalf("factor without alternatives = %v", got)
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
	got = allSelectionCombinations([]assignmentSelectionSet{
		selectionSet(assignmentSelection{a}, assignmentSelection{b}),
		selectionSet(assignmentSelection{c}, assignmentSelection{d}),
		selectionSet(assignmentSelection{e}, assignmentSelection{f}),
	})
	want = selectionSet(
		assignmentSelection{a, c, e}, assignmentSelection{a, c, f},
		assignmentSelection{a, d, e}, assignmentSelection{a, d, f},
		assignmentSelection{b, c, e}, assignmentSelection{b, c, f},
		assignmentSelection{b, d, e}, assignmentSelection{b, d, f},
	)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("three-factor combinations = %v, want %v", got, want)
	}
}

func TestSelectionLabelsAtUnionsAlternatives(t *testing.T) {
	a, b, root := NewUID(), NewUID(), NewUID()
	selections := assignmentSelectionMap{root: selectionSet(assignmentSelection{a}, assignmentSelection{b})}
	if got, want := labelsAt(root, selections), canonicalSelection([]UID{a, b}); !reflect.DeepEqual(got, want) {
		t.Fatalf("labelsAt = %v, want %v", got, want)
	}
	if got := labelsAt(NewUID(), selections); got != nil {
		t.Fatalf("labelsAt absent expression = %v", got)
	}
}

func TestAllSelectionsMapsConditionalDisjunctionCartesianProduct(t *testing.T) {
	x1, x2 := assignmentCandidate("x", intLiteral(1)), assignmentCandidate("x", intLiteral(2))
	y1, y2 := assignmentCandidate("y", intLiteral(3)), assignmentCandidate("y", intLiteral(4))
	action := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), NewNameExpr("C", Typed(BoolT1)), x1, x2),
		MustOperExpr(Op("OR"), Typed(BoolT1), y1, y2))
	selected := map[UID]struct{}{x1.ID(): {}, x2.ID(): {}, y1.ID(): {}, y2.ID(): {}}
	introduction := NewAssignmentOperatorIntroductionFromSet(selected, nil)
	transformed := introduction.Expression(action)
	replacements := introduction.Replacements()
	selections := NewSymbolicTransitionGenerator(nil).allSelections(transformed, nil)
	want := selectionSet(
		assignmentSelection{replacements[x1.ID()], replacements[y1.ID()]},
		assignmentSelection{replacements[x1.ID()], replacements[y2.ID()]},
		assignmentSelection{replacements[x2.ID()], replacements[y1.ID()]},
		assignmentSelection{replacements[x2.ID()], replacements[y2.ID()]},
	)
	if got := selections[transformed.ID()]; !reflect.DeepEqual(got, want) {
		t.Fatalf("root selections = %v, want %v", got, want)
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

func TestSymbolicTransitionSlicePreservesSharedLetAssignment(t *testing.T) {
	assignment := assignmentCandidate("x", intLiteral(1))
	definition := NewOperDecl("X", nil, assignment, Typed(mustOperator(nil, BoolT1)))
	operatorType := mustOperator(nil, BoolT1)
	call := func() Expr { return MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr("X", Typed(operatorType))) }
	action := NewLetInExpr(MustOperExpr(Op("OR"), Typed(BoolT1),
		MustOperExpr(Op("AND"), Typed(BoolT1), NewNameExpr("A", Typed(BoolT1)), call()),
		MustOperExpr(Op("AND"), Typed(BoolT1), NewNameExpr("B", Typed(BoolT1)), call())),
		Typed(BoolT1), definition)
	introduction := NewAssignmentOperatorIntroductionFromSet(map[UID]struct{}{assignment.ID(): {}}, nil)
	transformed := introduction.Expression(action)
	replacement := introduction.Replacements()[assignment.ID()]
	generator := NewSymbolicTransitionGenerator(nil)
	selections := generator.allSelections(transformed, nil)
	sliced := generator.sliceWith(assignmentSelection{replacement}, selections, transformed)
	if !EqualExpr(sliced, transformed) {
		t.Fatalf("shared LET slice = %v, want unchanged %v", sliced, transformed)
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

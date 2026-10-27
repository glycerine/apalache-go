package apalache

import "testing"

func assignmentCandidate(name string, value Expr) *OperExpr {
	variable := NewNameExpr(name, value.Tag())
	return MustOperExpr(Op("EQ"), Typed(BoolT1), MustOperExpr(Op("PRIME"), value.Tag(), variable), value)
}

func TestAssignmentOperatorIntroductionSelectsCandidatesAndTracksUIDs(t *testing.T) {
	selected := assignmentCandidate("x", intLiteral(1))
	unselected := assignmentCandidate("y", intLiteral(2))
	root := MustOperExpr(Op("AND"), Typed(BoolT1), selected, unselected)
	introduction := NewAssignmentOperatorIntroductionFromSet(map[UID]struct{}{selected.ID(): {}}, nil)
	got := introduction.Expression(root).(*OperExpr)
	arguments := got.Arguments()
	if arguments[0].(*OperExpr).Operator().Name() != "Apalache!:=" || arguments[1].(*OperExpr).Operator().Name() != "EQ" {
		t.Fatalf("assignment introduction = %v", got)
	}
	replacements := introduction.Replacements()
	if len(replacements) != 1 || replacements[selected.ID()] != arguments[0].ID() {
		t.Fatalf("replacement map = %v", replacements)
	}
	delete(replacements, selected.ID())
	if len(introduction.Replacements()) != 1 {
		t.Fatal("replacement map was exposed mutably")
	}
}

func TestAssignmentOperatorIntroductionTraversesLetsAndOnlyLabelBodies(t *testing.T) {
	bodyCandidate := assignmentCandidate("x", intLiteral(1))
	labelArgument := s("L1")
	label := MustOperExpr(Op("LABEL"), Typed(BoolT1), bodyCandidate, labelArgument)
	declarationCandidate := assignmentCandidate("z", intLiteral(3))
	definition := NewOperDecl("A", nil, declarationCandidate, Typed(mustOperator(nil, BoolT1)))
	let := NewLetInExpr(label, Typed(BoolT1), definition)
	selected := map[UID]struct{}{bodyCandidate.ID(): {}, declarationCandidate.ID(): {}}
	got := NewAssignmentOperatorIntroductionFromSet(selected, nil).Expression(let).(*LetInExpr)
	gotLabel := got.Body().(*OperExpr)
	if gotLabel.Arguments()[0].(*OperExpr).Operator().Name() != "Apalache!:=" {
		t.Fatal("label body was not transformed")
	}
	if gotLabel.Arguments()[1].ID() != labelArgument.ID() {
		t.Fatal("label metadata argument was reconstructed")
	}
	if got.Declarations()[0].Body().(*OperExpr).Operator().Name() != "Apalache!:=" {
		t.Fatal("LET declaration body was not transformed")
	}
}

func TestAssignmentOperatorIntroductionRequiresPrimedNameEquality(t *testing.T) {
	ordinary := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intLiteral(1))
	complexPrime := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)), intLiteral(1))), intLiteral(2))
	for _, expression := range []*OperExpr{ordinary, complexPrime} {
		got := NewAssignmentOperatorIntroduction(func(UID) bool { return true }, nil).Expression(expression)
		if got.(*OperExpr).Operator().Name() != "EQ" {
			t.Fatalf("non-candidate was transformed: %v", got)
		}
	}
}

func TestAssignmentModuleAdapterRoundTripAndOrdering(t *testing.T) {
	expressions := []Expr{intLiteral(10), intLiteral(20), intLiteral(30)}
	definitions := ExpressionsToOperatorDefinitions("Next$", expressions)
	if definitions[0].Name() != "Next$0000" || definitions[2].Name() != "Next$0002" {
		t.Fatalf("transition names = %q, %q", definitions[0].Name(), definitions[2].Name())
	}
	operatorType, typed := definitions[0].Tag().Type()
	if !typed || !EqualTypes(operatorType, mustOperator(nil, IntT1)) {
		t.Fatalf("wrapped expression type = %v", definitions[0].Tag())
	}

	module := NewModule("M", []Decl{definitions[2], NewOperDecl("Other", nil, intLiteral(0), Typed(mustOperator(nil, IntT1))), definitions[0], definitions[1]})
	module.AddProperty(PropertyTransitionsFound)
	got := TransitionsFromModule(module, "Next$")
	for index := range expressions {
		if !EqualTypedExpr(got[index], expressions[index]) {
			t.Fatalf("transition %d = %v", index, got[index])
		}
	}
	if body, found := OperatorBody(module, "Other"); !found || !EqualTypedExpr(body, intLiteral(0)) {
		t.Fatalf("operator lookup = %v, %v", body, found)
	}

	transitions := []SymbolicTransition{{Assignments: AssignmentStrategy{NewUID()}, Expression: expressions[0]}}
	inserted := InsertTransitions(NewModule("M", []Decl{definitions[1]}), "Init$", transitions)
	if inserted.OperDeclarations()[0].Name() != "Init$0000" {
		t.Fatalf("inserted declarations = %v", inserted.Declarations())
	}
}

func TestExpressionToOperatorDefinitionPreservesUntyped(t *testing.T) {
	definition := ExpressionToOperatorDefinition("A", NewNameExpr("x", Untyped()))
	if definition.Tag().IsTyped() {
		t.Fatalf("untyped expression produced %v", definition.Tag())
	}
}

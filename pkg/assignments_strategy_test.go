package apalache

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func assignmentVariables(names ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		result[name] = struct{}{}
	}
	return result
}

func TestAssignmentStrategyCandidatesAndOrdering(t *testing.T) {
	extractor := NewSMTFreeSymbolicTransitionExtractor(nil, nil)
	natural := assignmentCandidate("x", intLiteral(1))
	manual := MustOperExpr(Op("Apalache!:="), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), intLiteral(2))

	for _, candidate := range []*OperExpr{natural, manual} {
		strategy, err := extractor.GetStrategy(assignmentVariables("x"), candidate, nil)
		if err != nil || !reflect.DeepEqual(strategy, AssignmentStrategy{candidate.ID()}) {
			t.Fatalf("single candidate strategy = %v, %v", strategy, err)
		}
	}
	manualFirst := MustOperExpr(Op("AND"), Typed(BoolT1), manual, natural)
	strategy, err := extractor.GetStrategy(assignmentVariables("x"), manualFirst, nil)
	if err != nil || !reflect.DeepEqual(strategy, AssignmentStrategy{manual.ID()}) {
		t.Fatalf("manual-first strategy = %v, %v", strategy, err)
	}
	naturalFirst := MustOperExpr(Op("AND"), Typed(BoolT1), natural, manual)
	if _, err := extractor.GetStrategy(assignmentVariables("x"), naturalFirst, nil); err == nil || !strings.Contains(err.Error(), "spurious") {
		t.Fatalf("natural then manual error = %v", err)
	}
}

func TestAssignmentStrategyMissingAndUseBeforeAssignment(t *testing.T) {
	extractor := NewSMTFreeSymbolicTransitionExtractor(nil, nil)
	assignment := assignmentCandidate("x", intLiteral(1))
	if _, err := extractor.GetStrategy(assignmentVariables("x", "y"), assignment, nil); err == nil || !strings.Contains(err.Error(), "x") && !strings.Contains(err.Error(), "y") {
		t.Fatalf("missing variable error = %v", err)
	}
	primedX := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	use := MustOperExpr(Op("GT"), Typed(BoolT1), primedX, intLiteral(0))
	action := MustOperExpr(Op("AND"), Typed(BoolT1), use, assignment)
	if _, err := extractor.GetStrategy(assignmentVariables("x"), action, nil); err == nil || !strings.Contains(err.Error(), "used before") {
		t.Fatalf("use-before-assignment error = %v", err)
	}

	manualInRHS := MustOperExpr(Op("Apalache!:="), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("y", Typed(IntT1))), intLiteral(2))
	bad := assignmentCandidate("x", manualInRHS)
	if _, err := extractor.GetStrategy(assignmentVariables("x", "y"), bad, nil); err == nil || !strings.Contains(err.Error(), "Illegal assignment") {
		t.Fatalf("nested manual assignment error = %v", err)
	}
}

func TestAssignmentStrategyBranchesAndQuantifiers(t *testing.T) {
	extractor := NewSMTFreeSymbolicTransitionExtractor(nil, nil)
	left, right := assignmentCandidate("x", intLiteral(1)), assignmentCandidate("x", intLiteral(2))
	disjunction := MustOperExpr(Op("OR"), Typed(BoolT1), left, right)
	strategy, err := extractor.GetStrategy(assignmentVariables("x"), disjunction, nil)
	if err != nil || !reflect.DeepEqual(strategy, AssignmentStrategy{left.ID(), right.ID()}) {
		t.Fatalf("branch strategy = %v, %v", strategy, err)
	}
	imbalanced := MustOperExpr(Op("OR"), Typed(BoolT1), left, NewValueExpr(BoolValue(true), Typed(BoolT1)))
	if _, err := extractor.GetStrategy(assignmentVariables("x"), imbalanced, nil); err == nil || !strings.Contains(err.Error(), "Missing assignments to: x") {
		t.Fatalf("imbalanced branch error = %v", err)
	}

	set := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), NewNameExpr("i", Typed(IntT1)), set, left)
	strategy, err = extractor.GetStrategy(assignmentVariables("x"), exists, nil)
	if err != nil || !reflect.DeepEqual(strategy, AssignmentStrategy{left.ID()}) {
		t.Fatalf("quantified strategy = %v, %v", strategy, err)
	}
	condition := MustOperExpr(Op("GT"), Typed(BoolT1), NewNameExpr("n", Typed(IntT1)), intLiteral(0))
	ite := MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), condition, left, right)
	strategy, err = extractor.GetStrategy(assignmentVariables("x"), ite, nil)
	if err != nil || !reflect.DeepEqual(strategy, AssignmentStrategy{left.ID(), right.ID()}) {
		t.Fatalf("ITE strategy = %v, %v", strategy, err)
	}
}

func TestAssignmentStrategyLetApplicationsAndRecursion(t *testing.T) {
	extractor := NewSMTFreeSymbolicTransitionExtractor(nil, nil)
	xAssignment := assignmentCandidate("x", intLiteral(1))
	local := NewOperDecl("A", []OperatorParam{ValueParam("p")}, xAssignment, Untyped())
	application := MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr("A", Untyped()), intLiteral(1))
	let := NewLetInExpr(application, Typed(BoolT1), local)
	strategy, err := extractor.GetStrategy(assignmentVariables("x"), let, nil)
	if err != nil || !reflect.DeepEqual(strategy, AssignmentStrategy{xAssignment.ID()}) {
		t.Fatalf("LET strategy = %v, %v", strategy, err)
	}

	yAssignment := assignmentCandidate("y", intLiteral(2))
	external := NewOperDecl("B", nil, yAssignment, Untyped())
	combined := NewLetInExpr(MustOperExpr(Op("AND"), Typed(BoolT1), application,
		MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr("B", Untyped()))), Typed(BoolT1), local)
	strategy, err = extractor.GetStrategy(assignmentVariables("x", "y"), combined, OperatorBodyMap(external))
	if err != nil || !reflect.DeepEqual(strategy, AssignmentStrategy{xAssignment.ID(), yAssignment.ID()}) {
		t.Fatalf("local/external strategy = %v, %v", strategy, err)
	}

	recursive := NewOperDecl("R", nil, assignmentCandidate("x", intLiteral(3)), Untyped())
	recursive.SetRecursive(true)
	call := MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr("R", Untyped()))
	afterAssignment := MustOperExpr(Op("AND"), Typed(BoolT1), xAssignment, call)
	strategy, err = extractor.GetStrategy(assignmentVariables("x"), afterAssignment, OperatorBodyMap(recursive))
	if err != nil || !reflect.DeepEqual(strategy, AssignmentStrategy{xAssignment.ID()}) {
		t.Fatalf("recursive-after-assignment strategy = %v, %v", strategy, err)
	}
	beforeAssignment := MustOperExpr(Op("AND"), Typed(BoolT1), call, xAssignment)
	if _, err := extractor.GetStrategy(assignmentVariables("x"), beforeAssignment, OperatorBodyMap(recursive)); err == nil {
		t.Fatal("recursive assignment before assignment was accepted")
	}
}

func TestAssignmentStrategyErrorsAreTypedAndLocated(t *testing.T) {
	store := NewSourceStore()
	assignment := assignmentCandidate("x", intLiteral(1))
	missingBranch := NewValueExpr(BoolValue(true), Typed(BoolT1))
	location := SourceLocation{Filename: "M.tla", Region: NewSourceRegion(3, 2, 3, 8)}
	if err := store.Add(missingBranch.ID(), location); err != nil {
		t.Fatal(err)
	}
	action := MustOperExpr(Op("OR"), Typed(BoolT1), assignment, missingBranch)
	_, err := NewSMTFreeSymbolicTransitionExtractor(nil, store).GetStrategy(assignmentVariables("x"), action, nil)
	var assignmentError *AssignmentError
	if !errors.As(err, &assignmentError) || !strings.Contains(err.Error(), "M.tla") {
		t.Fatalf("located typed error = %T %v", err, err)
	}
}

func TestSMTFreeExtractorProducesTransitionsAndHandlesStatelessSpecs(t *testing.T) {
	extractor := NewSMTFreeSymbolicTransitionExtractor(nil, nil)
	first, second := assignmentCandidate("x", intLiteral(1)), assignmentCandidate("x", intLiteral(2))
	action := MustOperExpr(Op("OR"), Typed(BoolT1), first, second)
	transitions, err := extractor.Extract(assignmentVariables("x"), action, nil)
	if err != nil || len(transitions) != 2 {
		t.Fatalf("extracted transitions = %v, %v", transitions, err)
	}
	for _, transition := range transitions {
		if len(transition.Assignments) != 1 || transition.Expression.(*OperExpr).Operator().Name() != "Apalache!:=" {
			t.Fatalf("extracted transition = %#v", transition)
		}
	}
	stateless := NewValueExpr(BoolValue(true), Typed(BoolT1))
	transitions, err = extractor.Extract(nil, stateless, nil)
	if err != nil || len(transitions) != 1 || len(transitions[0].Assignments) != 0 || transitions[0].Expression != stateless {
		t.Fatalf("stateless transitions = %#v, %v", transitions, err)
	}
}

func TestSMTFreeExtractorSplitsFiniteExistentialAssignments(t *testing.T) {
	extractor := NewSMTFreeSymbolicTransitionExtractor(nil, nil)
	binder := NewNameExpr("i", Typed(IntT1))
	domain := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1), intLiteral(2), intLiteral(3))
	action := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder, domain, assignmentCandidate("x", binder))

	transitions, err := extractor.Extract(assignmentVariables("x"), expandFiniteQuantifiers(action, assignmentVariables("x"), nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(transitions) != 3 {
		t.Fatalf("finite existential transition count = %d, want 3", len(transitions))
	}
	values := make(map[int64]bool)
	for _, transition := range transitions {
		assignment, ok := transition.Expression.(*OperExpr)
		if !ok || assignment.Operator().Name() != "Apalache!:=" {
			t.Fatalf("finite existential transition = %#v", transition)
		}
		value, ok := assignment.Arguments()[1].(*ValueExpr)
		if !ok {
			t.Fatalf("finite existential assignment RHS = %T", assignment.Arguments()[1])
		}
		integer, ok := value.Value().(IntValue)
		if !ok {
			t.Fatalf("finite existential assignment value = %T", value.Value())
		}
		values[integer.BigInt().Int64()] = true
	}
	for _, want := range []int64{1, 2, 3} {
		if !values[want] {
			t.Fatalf("finite existential values = %v, missing %d", values, want)
		}
	}
}

func TestExpandFiniteQuantifiersScopesAssignmentsAndPreservesShadowing(t *testing.T) {
	binder := NewNameExpr("i", Typed(IntT1))
	domain := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1), intLiteral(2))
	predicate := MustOperExpr(Op("GT"), Typed(BoolT1), binder, intLiteral(0))
	forall := MustOperExpr(Op("FORALL3"), Typed(BoolT1), binder, domain, predicate)
	if expanded := expandFiniteQuantifiers(forall, assignmentVariables("x"), nil); countOperator(expanded, "FORALL3") != 1 {
		t.Fatalf("universal guard was unexpectedly expanded: %v", expanded)
	}

	empty := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)))
	emptyChoice := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder, empty, assignmentCandidate("x", binder))
	value, ok := expandFiniteQuantifiers(emptyChoice, assignmentVariables("x"), nil).(*ValueExpr)
	if !ok || value.Value() != BoolValue(false) {
		t.Fatalf("existential assignment over empty set = %v", value)
	}
	ineligible := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder, domain, assignmentCandidate("y", binder))
	if expanded := expandFiniteQuantifiers(ineligible, assignmentVariables("x"), nil); countOperator(expanded, "EXISTS3") != 1 {
		t.Fatalf("ineligible assignment was unexpectedly expanded: %v", expanded)
	}

	innerDomain := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), NewNameExpr("i", Typed(IntT1)))
	innerBody := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("i", Typed(IntT1)), intLiteral(1))
	inner := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), NewNameExpr("i", Typed(IntT1)), innerDomain, innerBody)
	substituted := substituteBoundName(inner, "i", intLiteral(2), nil).(*OperExpr)
	if substituted.Arguments()[0].(*NameExpr).Name() != "i" ||
		substituted.Arguments()[1].(*OperExpr).Arguments()[0].(*ValueExpr).Value().(IntValue).BigInt().Int64() != 2 ||
		substituted.Arguments()[2].(*OperExpr).Arguments()[0].(*NameExpr).Name() != "i" {
		t.Fatalf("shadow-aware substitution = %v", substituted)
	}
}

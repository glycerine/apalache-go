package apalache

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func counterSafetyModule(limit int64) (*Module, SpecificationConfiguration) {
	x := NewNameExpr("x", Typed(IntT1))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))),
		MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))))
	invariant := boolDefinition("Inv", MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(limit)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	configuration := SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}
	return module, configuration
}

func TestCheckBoundedSafetyFindsFirstBadDepth(t *testing.T) {
	module, configuration := counterSafetyModule(2)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}

	safe, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if safe.Outcome != SafetyNoCounterexample || safe.Depth != -1 || safe.ExploredExecutions != 2 {
		t.Fatalf("one-step result = %#v", safe)
	}

	unsafe, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 3)
	if err != nil {
		t.Fatal(err)
	}
	if unsafe.Outcome != SafetyCounterexample || unsafe.Depth != 2 || unsafe.Invariant != "Inv" ||
		unsafe.InitTransition != 0 || !reflect.DeepEqual(unsafe.NextTransitions, []int{0, 0}) {
		t.Fatalf("counterexample result = %#v", unsafe)
	}
	if len(unsafe.States) != 3 {
		t.Fatalf("decoded counterexample states = %#v", unsafe.States)
	}
	for index, expected := range []string{"0", "1", "2"} {
		formatted, formatErr := FormatTLAExpression(unsafe.States[index].Values["x"])
		if formatErr != nil || formatted != expected {
			t.Fatalf("state %d x = %q, %v", index, formatted, formatErr)
		}
	}
}

func TestCheckBoundedSafetyRunsNativeArrayEncodings(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	functionType, err := NewFunType(IntT1, IntT1)
	if err != nil {
		t.Fatal(err)
	}
	binder := NewNameExpr("i", Typed(IntT1))
	domain := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1))
	function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType),
		MustOperExpr(Op("PLUS"), Typed(IntT1), binder, intLiteral(1)), binder, domain)
	application := MustOperExpr(Op("FUN_APP"), Typed(IntT1), function, intLiteral(1))
	module := configuredTestModule(
		NewVarDecl("x", Typed(IntT1)),
		boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0))),
		boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime, x)),
		boolDefinition("Inv", MustOperExpr(Op("EQ"), Typed(BoolT1), application, intLiteral(2))),
	)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, encoding := range []SMTEncoding{SMTEncodingArrays, SMTEncodingFunArrays} {
		t.Run(string(encoding), func(t *testing.T) {
			result, checkErr := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
				BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: false, SMTEncoding: encoding})
			if checkErr != nil {
				t.Fatal(checkErr)
			}
			if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 2 {
				t.Fatalf("native %s checker result = %#v", encoding, result)
			}
		})
	}
	if _, checkErr := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{SMTEncoding: SMTEncoding("unknown")}); checkErr == nil ||
		!strings.Contains(checkErr.Error(), "unsupported checker SMT encoding") {
		t.Fatalf("unknown checker encoding error = %v", checkErr)
	}
}

func TestCheckBoundedSafetyRunsNestedNativeArrayEncodings(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	setType := mustSet(IntT1)
	functionType, err := NewFunType(IntT1, setType)
	if err != nil {
		t.Fatal(err)
	}
	binder := NewNameExpr("i", Typed(IntT1))
	domain := MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1))
	resultSet := MustOperExpr(Op("SET_ENUM"), Typed(setType), binder,
		MustOperExpr(Op("PLUS"), Typed(IntT1), binder, intLiteral(1)))
	function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), resultSet, binder, domain)
	application := MustOperExpr(Op("FUN_APP"), Typed(setType), function, intLiteral(1))
	module := configuredTestModule(
		NewVarDecl("x", Typed(IntT1)),
		boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0))),
		boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime, x)),
		boolDefinition("Inv", MustOperExpr(Op("SET_IN"), Typed(BoolT1), intLiteral(2), application)),
	)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, encoding := range []SMTEncoding{SMTEncodingArrays, SMTEncodingFunArrays} {
		t.Run(string(encoding), func(t *testing.T) {
			result, checkErr := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
				BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: false, SMTEncoding: encoding})
			if checkErr != nil {
				t.Fatal(checkErr)
			}
			if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 2 {
				t.Fatalf("nested native %s checker result = %#v", encoding, result)
			}
		})
	}
}

func TestCheckBoundedSafetyFindsShallowViolationAcrossBranches(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("OR"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), prime, x),
		MustOperExpr(Op("EQ"), Typed(BoolT1), prime,
			MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1)))))
	invariant := boolDefinition("Inv", MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(1)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Depth != 1 ||
		!reflect.DeepEqual(result.NextTransitions, []int{1}) {
		t.Fatalf("shallow branch result = %#v", result)
	}
}

func TestCheckBoundedSafetyReportsInitialViolation(t *testing.T) {
	module, configuration := counterSafetyModule(0)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Depth != 0 || len(result.NextTransitions) != 0 {
		t.Fatalf("initial counterexample = %#v", result)
	}
}

func TestCheckBoundedSafetyCanDelayInvariantChecking(t *testing.T) {
	module, configuration := counterSafetyModule(1)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: true, InvariantMinimumDepth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Depth != 1 || result.Invariant != "Inv" {
		t.Fatalf("delayed invariant result = %#v", result)
	}
}

func TestCheckBoundedSafetyAggregatesRuleProfileAcrossExecutors(t *testing.T) {
	module, configuration := counterSafetyModule(2)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	listener := NewRuleStatListener()
	_, err = CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: true, RuleStats: listener})
	if err != nil {
		t.Fatal(err)
	}
	var calls, cells, assertions int64
	for _, stat := range listener.Locator().Stats() {
		calls += stat.Calls
		cells += stat.CellsSelf
		assertions += stat.SMTAssertionsSelf
	}
	if calls == 0 || cells == 0 || assertions == 0 || len(listener.Locator().Stats()) < 2 {
		t.Fatalf("bounded rule profile totals = calls %d, cells %d, assertions %d, stats %+v",
			calls, cells, assertions, listener.Locator().Stats())
	}
}

func TestCheckBoundedSafetyEnumeratesTransitionAlternatives(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("OR"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(3))))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), x))
	invariant := boolDefinition("Inv", MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(2)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if count := len(TransitionsFromModule(prepared.Module, NormalizedInitPrefix)); count != 2 {
		t.Fatalf("normalized initial transition count = %d", count)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Depth != 0 || result.ExploredExecutions != 2 {
		t.Fatalf("alternative initial paths = %#v", result)
	}
}

func TestCheckBoundedSafetyEnumeratesDistinctViewsWithinOneSymbolicPath(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "MultiError.tla", `---- MODULE MultiError ----
EXTENDS Integers, FiniteSets
\* @type: Int;
VARIABLE x
Init == x \in {1, 2, 3}
Next == x' = x
Inv == x = 0
View == x
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"}, View: "View",
		}, Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count := len(TransitionsFromModule(prepared.Module, NormalizedInitPrefix)); count != 1 {
		t.Fatalf("normalized initial transition count = %d, want one symbolic path", count)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false, MaxErrors: 3})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || len(result.Counterexamples) != 3 {
		t.Fatalf("multi-error result = %#v", result)
	}
	values := make(map[string]struct{})
	for _, counterexample := range result.Counterexamples {
		if len(counterexample.States) != 1 || len(counterexample.View) != 1 {
			t.Fatalf("counterexample = %#v", counterexample)
		}
		formatted, formatErr := FormatTLAExpression(counterexample.View[0])
		if formatErr != nil {
			t.Fatal(formatErr)
		}
		values[formatted] = struct{}{}
	}
	for _, want := range []string{"1", "2", "3"} {
		if _, found := values[want]; !found {
			t.Fatalf("distinct views = %v, missing %s", values, want)
		}
	}
}

func TestCheckBoundedSafetyBlocksDistinctMultiStateViewPrefixes(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "MultiStepError.tla", `---- MODULE MultiStepError ----
EXTENDS Integers, FiniteSets
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' \in {1, 2, 3}
Inv == x = 0
View == x
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"}, View: "View",
		}, Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: false, MaxErrors: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Counterexamples) != 3 {
		t.Fatalf("multi-step counterexamples = %#v", result.Counterexamples)
	}
	values := make(map[string]struct{})
	for _, counterexample := range result.Counterexamples {
		if counterexample.Depth != 1 || len(counterexample.View) != 2 {
			t.Fatalf("multi-step counterexample = %#v", counterexample)
		}
		formatted, formatErr := FormatTLAExpression(counterexample.View[1])
		if formatErr != nil {
			t.Fatal(formatErr)
		}
		values[formatted] = struct{}{}
	}
	if len(values) != 3 {
		t.Fatalf("multi-step distinct views = %v", values)
	}
}

func TestCheckBoundedSafetyRequiresViewForMultipleErrors(t *testing.T) {
	module, configuration := counterSafetyModule(0)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	_, err = CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, MaxErrors: 2})
	if err == nil || !strings.Contains(err.Error(), "requires a state view") {
		t.Fatalf("missing view error = %v", err)
	}
}

func TestCheckBoundedSafetySkipsUnsatisfiablePaths(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	assignment := MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0))
	init := boolDefinition("Init", MustOperExpr(Op("AND"), Typed(BoolT1), assignment,
		NewValueExpr(BoolValue(false), Typed(BoolT1))))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), x))
	invariant := boolDefinition("Inv", NewValueExpr(BoolValue(false), Typed(BoolT1)))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 0 {
		t.Fatalf("disabled initial path = %#v", result)
	}
}

func TestCheckBoundedSafetyDetectsAndCanIgnoreDeadlocks(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1),
			MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))),
			MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))),
		MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(0))))
	invariant := boolDefinition("Inv", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	deadlock, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if deadlock.Outcome != SafetyDeadlock || deadlock.Depth != 0 || len(deadlock.States) != 1 {
		t.Fatalf("deadlock result = %#v", deadlock)
	}
	ignored, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if ignored.Outcome != SafetyNoCounterexample {
		t.Fatalf("ignored deadlock result = %#v", ignored)
	}
	discardDisabled := false
	delayed, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: false, DiscardDisabled: &discardDisabled})
	if err != nil {
		t.Fatal(err)
	}
	if delayed.Outcome != SafetyNoCounterexample || delayed.ExploredExecutions <= ignored.ExploredExecutions {
		t.Fatalf("delayed disabled-transition pruning = %#v, eager = %#v", delayed, ignored)
	}
}

// Regression for apalache-mc/apalache#712. The legacy Scala checker can miss
// the x = 1 deadlock because another branch has an infinite continuation.
// The bounded search must check each reachable branch independently.
func TestCheckBoundedSafetyDetectsDeadlockAlongOneOfSeveralBranches(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "Deadlock712.tla", `---- MODULE Deadlock712 ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
A == /\ x % 2 = 0 /\ x' = x + 1
B == /\ x % 2 = 0 /\ x' = x + 2
Next == A \/ B
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{Behavior: InitNextSpec{Init: "Init", Next: "Next"}},
		Sources:       frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyDeadlock || result.Depth != 1 || len(result.States) != 2 {
		t.Fatalf("branch-local deadlock result = %#v", result)
	}
	value, err := FormatTLAExpression(result.States[1].Values["x"])
	if err != nil || value != "1" {
		t.Fatalf("deadlocked state x = %q, %v", value, err)
	}
}

func TestCheckBoundedSafetyDetectsDeadlockInsideFiniteExistentialChoice(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "ExistentialDeadlock.tla", `---- MODULE ExistentialDeadlock ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == \E i \in {0, 1, 2, 3}: i > x /\ x' = i
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{Behavior: InitNextSpec{Init: "Init", Next: "Next"}},
		Sources:       frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	if transitions := TransitionsFromModule(prepared.Module, NormalizedNextPrefix); len(transitions) != 4 {
		t.Fatalf("normalized existential transitions = %d, want 4", len(transitions))
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyDeadlock || result.Depth != 1 || len(result.States) != 2 {
		t.Fatalf("existential deadlock result = %#v", result)
	}
	value, err := FormatTLAExpression(result.States[1].Values["x"])
	if err != nil || value != "3" {
		t.Fatalf("existential deadlocked state x = %q, %v", value, err)
	}
}

func TestCheckBoundedSafetyProjectsDeadlockInsideSymbolicExistentialChoice(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "SymbolicExistentialDeadlock.tla", `---- MODULE SymbolicExistentialDeadlock ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == \E i \in 0..3: i > x /\ x' = i
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{Behavior: InitNextSpec{Init: "Init", Next: "Next"}},
		Sources:       frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	if transitions := TransitionsFromModule(prepared.Module, NormalizedNextPrefix); len(transitions) != 4 {
		t.Fatalf("normalized integer-range existential transitions = %d, want 4", len(transitions))
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyDeadlock || result.Depth != 1 || len(result.States) != 2 {
		t.Fatalf("symbolic existential deadlock result = %#v", result)
	}
	value, err := FormatTLAExpression(result.States[1].Values["x"])
	if err != nil || value != "3" {
		t.Fatalf("symbolic existential deadlocked state x = %q, %v", value, err)
	}
}

func TestCheckBoundedSafetyRejectsChooseOverSymbolicIntegerRange(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "SymbolicRangeChoose.tla", `---- MODULE SymbolicRangeChoose ----
EXTENDS Integers
VARIABLE
\* @type: Int;
x
Init == x = 0
Next == x' = x
Inv == (CHOOSE i \in x..(x + 2): i > x) > x
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false}); err == nil ||
		!strings.Contains(err.Error(), "constant integer range") {
		t.Fatalf("symbolic range CHOOSE error = %v", err)
	}
}

func TestCheckBoundedSafetyFindsDeadlockThroughFixedWidthSymbolicRange(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "SymbolicRangeDeadlock.tla", `---- MODULE SymbolicRangeDeadlock ----
EXTENDS Integers
VARIABLE
\* @type: Int;
x
Init == x = 0
Next == x < 2 /\ \E i \in x..(x + 2): x' = i
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{Behavior: InitNextSpec{Init: "Init", Next: "Next"}},
		Sources:       frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	if transitions := TransitionsFromModule(prepared.Module, NormalizedNextPrefix); len(transitions) != 3 {
		t.Fatalf("fixed-width symbolic-range transitions = %d, want 3", len(transitions))
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyDeadlock || result.Depth != 1 || len(result.States) != 2 {
		t.Fatalf("fixed-width symbolic-range deadlock = %#v", result)
	}
	value, err := FormatTLAExpression(result.States[1].Values["x"])
	if err != nil || value != "2" {
		t.Fatalf("deadlocked state x = %q, %v", value, err)
	}
}

func TestCheckBoundedSafetyProjectsVariableWidthSymbolicRangeDeadlock(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "VariableRangeDeadlock.tla", `---- MODULE VariableRangeDeadlock ----
EXTENDS Integers
VARIABLE
\* @type: Int;
x
Init == \E i \in {0, 1}: x = i
Next == \E j \in x..0: x' = j
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{Behavior: InitNextSpec{Init: "Init", Next: "Next"}},
		Sources:       frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyDeadlock || result.Depth != 0 || len(result.States) != 1 {
		t.Fatalf("variable-width symbolic-range deadlock = %#v", result)
	}
	value, err := FormatTLAExpression(result.States[0].Values["x"])
	if err != nil || value != "1" {
		t.Fatalf("deadlocked initial state x = %q, %v", value, err)
	}
}

func TestCheckBoundedSafetyEnforcesConfiguredConstraints(t *testing.T) {
	for _, test := range []struct {
		name              string
		stateConstraints  []string
		actionConstraints []string
	}{
		{name: "state", stateConstraints: []string{"StateOK"}},
		{name: "action", actionConstraints: []string{"ActionOK"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			x := NewNameExpr("x", Typed(IntT1))
			prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
			constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
			init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
			next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime,
				MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))))
			stateOK := boolDefinition("StateOK", MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(1)))
			actionOK := boolDefinition("ActionOK", MustOperExpr(Op("EQ"), Typed(BoolT1), prime, x))
			invariant := boolDefinition("Inv", NewValueExpr(BoolValue(true), Typed(BoolT1)))
			module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, stateOK, actionOK, invariant)
			prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
				Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
				StateConstraints: test.stateConstraints, ActionConstraints: test.actionConstraints,
			}})
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range append(append([]string(nil), test.stateConstraints...), test.actionConstraints...) {
				if _, found := OperatorBody(prepared.Module, name); !found {
					t.Fatalf("constraint %s was not retained", name)
				}
			}
			result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != SafetyDeadlock || result.Depth != 0 {
				t.Fatalf("constrained result = %#v", result)
			}
		})
	}
}

func TestCheckBoundedSafetyChecksActionInvariantsOnPendingState(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime,
		MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))))
	actionInvariant := boolDefinition("ActionInv", MustOperExpr(Op("EQ"), Typed(BoolT1), prime, x))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, actionInvariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"ActionInv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || !result.ActionInvariant || result.Invariant != "ActionInv" ||
		result.Depth != 1 || !reflect.DeepEqual(result.NextTransitions, []int{0}) || len(result.States) != 2 {
		t.Fatalf("action invariant result = %#v", result)
	}
	for index, expected := range []string{"0", "1"} {
		formatted, formatErr := FormatTLAExpression(result.States[index].Values["x"])
		if formatErr != nil || formatted != expected {
			t.Fatalf("action state %d x = %q, %v", index, formatted, formatErr)
		}
	}
}

func TestCheckBoundedSafetyTreatsInvariantAssignmentsAsEquality(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime,
		MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))))
	assignmentInvariant := boolDefinition("AssignmentInv",
		MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, x))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, assignmentInvariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"AssignmentInv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || !result.ActionInvariant || result.Depth != 1 {
		t.Fatalf("assignment invariant result = %#v", result)
	}
}

func TestCheckBoundedSafetyChecksTraceInvariantsOverStateHistory(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime,
		MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))))
	recordType, _ := NewRecordType(map[string]Type{"x": IntT1})
	historyType, _ := NewSeqType(recordType)
	history := NewNameExpr("hist", Typed(historyType))
	traceBody := MustOperExpr(Op("LE"), Typed(BoolT1),
		MustOperExpr(Op("Sequences!Len"), Typed(IntT1), history), intLiteral(2))
	traceInvariant := NewOperDecl("TraceInv", []OperatorParam{ValueParam("hist")}, traceBody,
		Typed(mustOperator([]Type{historyType}, BoolT1)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, traceInvariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"TraceInv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || !result.TraceInvariant || result.ActionInvariant ||
		result.Invariant != "TraceInv" || result.Depth != 2 || len(result.States) != 3 {
		t.Fatalf("trace invariant result = %#v", result)
	}
	for index, expected := range []string{"0", "1", "2"} {
		formatted, formatErr := FormatTLAExpression(result.States[index].Values["x"])
		if formatErr != nil || formatted != expected {
			t.Fatalf("trace state %d x = %q, %v", index, formatted, formatErr)
		}
	}
}

func TestCheckBoundedSafetyExpandsFiniteExistentialAssignments(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	binder := NewNameExpr("i", Typed(IntT1))
	setType := mustSet(IntT1)
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder,
		MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1), intLiteral(2)),
		MustOperExpr(Op("AND"), Typed(BoolT1),
			MustOperExpr(Op("EQ"), Typed(BoolT1), prime, binder),
			MustOperExpr(Op("EQ"), Typed(BoolT1), binder, intLiteral(2)))))
	invariant := boolDefinition("Inv", MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(2)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	nextTransitions := TransitionsFromModule(prepared.Module, NormalizedNextPrefix)
	if len(nextTransitions) != 2 {
		t.Fatalf("analyzed Next transition count = %d, want 2", len(nextTransitions))
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Depth != 1 || len(result.States) != 2 {
		t.Fatalf("existential assignment result = %#v", result)
	}
	value, formatErr := FormatTLAExpression(result.States[1].Values["x"])
	if formatErr != nil || value != "2" {
		t.Fatalf("existentially assigned x = %q, %v", value, formatErr)
	}
}

func TestCheckBoundedSafetyAllowsUnusedConstantsWithoutInitializer(t *testing.T) {
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	module := configuredTestModule(
		NewConstDecl("Unused", Typed(IntT1)),
		boolDefinition("Init", truth),
		boolDefinition("Next", truth),
		boolDefinition("Inv", truth),
	)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample {
		t.Fatalf("unused-constant result = %#v", result)
	}
}

func TestCheckBoundedSafetyNegatesInvariantBeforeSkolemization(t *testing.T) {
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	setType := mustSet(IntT1)
	a := NewNameExpr("a", Typed(IntT1))
	b := NewNameExpr("b", Typed(IntT1))
	inner := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), b,
		MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1), intLiteral(2)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), a, b))
	invariant := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), a,
		MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(0), intLiteral(1)), inner)
	module := configuredTestModule(
		boolDefinition("Init", truth),
		boolDefinition("Next", truth),
		boolDefinition("Inv", invariant),
	)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample {
		t.Fatalf("existential invariant result = %#v", result)
	}
}

func TestCheckBoundedSafetyValidatesInputs(t *testing.T) {
	if _, err := CheckBoundedSafety(context.Background(), nil, DerivedPredicates{}, 0); err == nil {
		t.Fatal("nil module was accepted")
	}
	module := NewModule("M", nil)
	if _, err := CheckBoundedSafety(context.Background(), module, DerivedPredicates{}, 0); err == nil {
		t.Fatal("unprepared module was accepted")
	}
}

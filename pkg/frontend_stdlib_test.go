package apalache

import (
	"context"
	"testing"
)

func TestBundledVariantsAndOptionModulesTypeCheck(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "BundledModules.tla", `---- MODULE BundledModules ----
EXTENDS Option
\* @type: Some(Int) | None(UNIT);
Value == Some(1)
IsValue == IsSome(Value)
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !result.OK() {
		t.Fatalf("bundled module diagnostics = %v", result.Diagnostics)
	}
}

func TestBundledBagsBagOfAllChecksMappedKeysAndCollisions(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "BundledBags.tla", `---- MODULE BundledBags ----
EXTENDS Bags
\* @type: (Int) => <<Int, Int>>;
Pair(x) == <<x, x>>
\* @type: (Int) => Int;
One(x) == 1
Init == TRUE
Next == TRUE
Inv ==
  LET B == [x \in {1, 2} |-> x]
  IN /\ BagOfAll(Pair, B) = [p \in {<<1, 1>>, <<2, 2>>} |-> p[1]]
     /\ BagOfAll(One, B) = [x \in {1} |-> 3]
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !frontend.OK() {
		t.Fatalf("bundled Bags diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior:   InitNextSpec{Init: "Init", Next: "Next"},
			Invariants: []string{"Inv"},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 1 {
		t.Fatalf("BagOfAll result = %#v", result)
	}
}

func TestBundledFunctionsChecksFiniteFunctionOperators(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "BundledFunctions.tla", `---- MODULE BundledFunctions ----
EXTENDS Functions
Init == TRUE
Next == TRUE
RangeOK == Range([x \in 1..3 |-> 1]) = {1}
InjectiveOK == ~IsInjective([x \in 1..3 |-> 1])
RestrictOK ==
  LET f == [x \in 1..3 |-> 1]
  IN Restrict(f, {1, 2}) = [x \in {1, 2} |-> 1]
FoldOK ==
  LET f == [x \in 1..3 |-> 1]
      AddToSet(x, acc) == {x} \union acc
  IN FoldFunctionOnSet(AddToSet, {}, f, {1, 2}) = {1}
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !frontend.OK() {
		t.Fatalf("bundled Functions diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior:   InitNextSpec{Init: "Init", Next: "Next"},
			Invariants: []string{"RangeOK", "InjectiveOK", "RestrictOK", "FoldOK"},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 1 {
		t.Fatalf("Functions result = %#v", result)
	}
}

func TestBundledFiniteSetsExtChecksFiniteOperators(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "BundledFiniteSetsExt.tla", `---- MODULE BundledFiniteSetsExt ----
EXTENDS FiniteSetsExt
Init == TRUE
Next == TRUE
Inv ==
  /\ Quantify({"a", "b", "c"}, LAMBDA x: x = "b") = 1
  /\ kSubset(2, {1, 2, 3}) = {{1, 2}, {1, 3}, {2, 3}}
  /\ FoldSet(LAMBDA x, acc: x + acc, 0, {1, 2, 3}) = 6
  /\ ChooseUnique({2, 3, 4, 5}, LAMBDA x: x % 3 = 1) = 4
  /\ SymDiff({1, 2}, {2, 3}) = {1, 3}
  /\ SumSet({1, 2, 3}) = 6
  /\ ProductSet({1, 2, 3, 4}) = 24
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !frontend.OK() {
		t.Fatalf("bundled FiniteSetsExt diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior:   InitNextSpec{Init: "Init", Next: "Next"},
			Invariants: []string{"Inv"},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 1 {
		t.Fatalf("FiniteSetsExt result = %#v", result)
	}
}

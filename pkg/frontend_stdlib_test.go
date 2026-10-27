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

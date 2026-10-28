package apalache

import (
	"context"
	"testing"
)

func TestValidateTraceBehaviorAcceptsNondeterministicWitnesses(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "TraceBehavior.tla", `---- MODULE TraceBehavior ----
EXTENDS Integers, FiniteSets
\* @type: Int;
VARIABLE x
Init == x \in {1, 2}
Next == x' \in {x, x + 1}
====`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics: %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{Behavior: InitNextSpec{Init: "Init", Next: "Next"}},
		Sources:       frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, trace := range [][]TraceState{
		{{"x": intLiteral(1)}, {"x": intLiteral(1)}, {"x": intLiteral(2)}},
		{{"x": intLiteral(2)}, {"x": intLiteral(3)}, {"x": intLiteral(4)}},
	} {
		valid, err := ValidateTraceBehavior(context.Background(), prepared.Module, trace)
		if err != nil || !valid {
			t.Fatalf("valid trace rejected: %v, %v", valid, err)
		}
	}
	invalid := []TraceState{{"x": intLiteral(2)}, {"x": intLiteral(4)}}
	if valid, err := ValidateTraceBehavior(context.Background(), prepared.Module, invalid); err != nil || valid {
		t.Fatalf("invalid trace = %v, %v", valid, err)
	}
	if _, err := ValidateTraceBehavior(context.Background(), prepared.Module, []TraceState{{}}); err == nil {
		t.Fatal("trace missing x was accepted")
	}
}

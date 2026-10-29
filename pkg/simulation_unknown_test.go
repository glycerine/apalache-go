package apalache

import (
	"context"
	"testing"
)

func TestSimulationPreservesUnknown(t *testing.T) {
	prepared := prepareSafetyModelFixture(t, `---- MODULE Review ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x
Inv == x >= 0
====`, []string{"Inv"}, "")
	calls := 0
	report, err := simulateAndCheckWith(context.Background(), prepared.Module, prepared.Predicates,
		SimulationOptions{MaxSteps: 0, MaxRuns: 2, RandomSeed: 1},
		func(context.Context, *Module, DerivedPredicates, BoundedSafetyOptions) (BoundedSafetyResult, error) {
			calls++
			return BoundedSafetyResult{Outcome: SafetyUnknown}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(report.Runs) != 1 || report.Runs[0].Outcome != SimulationUnknown {
		t.Fatalf("inconclusive invariant check reported as success: calls=%d report=%+v", calls, report)
	}
}

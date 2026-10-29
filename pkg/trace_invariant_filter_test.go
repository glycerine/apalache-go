package apalache

import (
	"context"
	"testing"
)

func TestTraceInvariantsIgnoreStateActionFilter(t *testing.T) {
	prepared := prepareSafetyModelFixture(t, `---- MODULE Review ----
EXTENDS Integers, Sequences
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x
Inv == x >= 0
\* @type: Seq({x: Int}) => Bool;
HistInv(h) == Len(h) = 0
====`, []string{"Inv", "HistInv"}, "")
	for _, filter := range []string{"", ".*->state0", ".*->action0"} {
		t.Run(filter, func(t *testing.T) {
			tuning, err := ParseCheckerTuning(map[string]string{"search.invariantFilter": filter})
			if err != nil {
				t.Fatal(err)
			}
			result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
				BoundedSafetyOptions{MaxSteps: 0, Tuning: tuning})
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != SafetyCounterexample || !result.TraceInvariant || result.Invariant != "HistInv" || result.Depth != 0 {
				t.Fatalf("trace invariant suppressed by state/action filter %q: %+v", filter, result)
			}
		})
	}
}

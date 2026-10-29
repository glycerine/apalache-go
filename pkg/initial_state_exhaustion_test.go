package apalache

import (
	"context"
	"fmt"
	"testing"
)

func TestInitialStateExhaustion(t *testing.T) {
	for _, tc := range []struct{ name, init, filter string }{
		{"contradictory_init", "x = 0 /\\ x > 0", ""},
		{"all_init_filtered", "x = 0", "1->0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepared := prepareSafetyModelFixture(t, fmt.Sprintf(`---- MODULE Review ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == %s
Next == x' = x
====`, tc.init), nil, "")
			tuning, err := ParseCheckerTuning(map[string]string{"search.transitionFilter": tc.filter})
			if err != nil {
				t.Fatal(err)
			}
			for _, deadlocks := range []bool{true, false} {
				for _, discard := range []bool{true, false} {
					listener := &CollectCounterexamplesModelCheckerListener{}
					result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
						BoundedSafetyOptions{MaxSteps: 2, CheckDeadlocks: deadlocks, DiscardDisabled: &discard, Tuning: tuning,
							Listeners: []ModelCheckerListener{listener}})
					if err != nil {
						t.Fatal(err)
					}
					want := CheckerExecutionsTooShort
					if deadlocks {
						want = CheckerDeadlock
					}
					checked := result.AsCheckerResult(prepared.Module)
					if checked.Kind != want || checked.Deadlock != nil || len(result.States) != 0 ||
						result.Depth != -1 || result.InitTransition != -1 || len(listener.Counterexamples()) != 0 {
						t.Errorf("deadlocks=%v discard=%v: want %s without a fabricated trace, got %+v / %+v", deadlocks, discard, want, result, checked)
					}
				}
			}
		})
	}
}

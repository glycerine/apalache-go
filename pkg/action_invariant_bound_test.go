package apalache

import (
	"context"
	"fmt"
	"testing"
)

func TestActionInvariantRespectsBound(t *testing.T) {
	for _, bound := range []int{0, 2} {
		t.Run(fmt.Sprint(bound), func(t *testing.T) {
			prepared := prepareSafetyModelFixture(t, fmt.Sprintf(`---- MODULE Review ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
Inv == x' <= %d
====`, bound), []string{"Inv"}, "")
			for _, deadlocks := range []bool{false, true} {
				for _, steps := range []int{bound, bound + 1} {
					result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
						BoundedSafetyOptions{MaxSteps: steps, CheckDeadlocks: deadlocks})
					if err != nil {
						t.Fatal(err)
					}
					if steps == bound {
						if result.Outcome != SafetyNoCounterexample {
							t.Errorf("bound=%d deadlocks=%v checked an extra action: %+v", steps, deadlocks, result)
						}
					} else if result.Outcome != SafetyCounterexample || !result.ActionInvariant || result.Depth != bound+1 {
						t.Errorf("bound=%d deadlocks=%v missed in-bound action violation: %+v", steps, deadlocks, result)
					}
				}
			}
		})
	}
}

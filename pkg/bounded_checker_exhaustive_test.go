package apalache

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func booleanStatePredicate(state int, primed bool) string {
	suffix := ""
	if primed {
		suffix = "'"
	}
	a, b := "FALSE", "FALSE"
	if state&2 != 0 {
		a = "TRUE"
	}
	if state&1 != 0 {
		b = "TRUE"
	}
	return fmt.Sprintf("(a%s = %s /\\ b%s = %s)", suffix, a, suffix, b)
}

func booleanMaskPredicate(mask uint16, primed bool) string {
	states := make([]string, 0, 4)
	for state := 0; state < 4; state++ {
		if mask&(1<<state) != 0 {
			states = append(states, booleanStatePredicate(state, primed))
		}
	}
	if len(states) == 0 {
		return "FALSE"
	}
	return strings.Join(states, " \\/ ")
}

func booleanTransitionPredicate(edges uint16) string {
	transitions := make([]string, 0, 16)
	for from := 0; from < 4; from++ {
		for to := 0; to < 4; to++ {
			if edges&(1<<(from*4+to)) != 0 {
				transitions = append(transitions,
					"("+booleanStatePredicate(from, false)+" /\\ "+booleanStatePredicate(to, true)+")")
			}
		}
	}
	if len(transitions) == 0 {
		return "FALSE"
	}
	return strings.Join(transitions, " \\/ ")
}

func exhaustiveViolationDepth(init uint8, edges uint16, invariant uint8, bound int) int {
	return exhaustiveGraphViolationDepth(uint64(init), uint64(edges), uint64(invariant), 4, bound)
}

func exhaustiveGraphViolationDepth(init, edges, invariant uint64, stateCount, bound int) int {
	reachable := init
	for depth := 0; depth <= bound; depth++ {
		if reachable&^invariant != 0 {
			return depth
		}
		next := uint64(0)
		for from := 0; from < stateCount; from++ {
			if reachable&(1<<from) == 0 {
				continue
			}
			for to := 0; to < stateCount; to++ {
				if edges&(1<<(from*stateCount+to)) != 0 {
					next |= 1 << to
				}
			}
		}
		reachable = next
	}
	return -1
}

func exhaustiveDeadlockDepth(init uint8, edges uint16, bound int) int {
	return exhaustiveGraphDeadlockDepth(uint64(init), uint64(edges), 4, bound)
}

func exhaustiveGraphDeadlockDepth(init, edges uint64, stateCount, bound int) int {
	reachable := init
	rowMask := uint64(1<<stateCount) - 1
	for depth := 0; depth <= bound; depth++ {
		for state := 0; state < stateCount; state++ {
			if reachable&(1<<state) != 0 && edges&(rowMask<<(state*stateCount)) == 0 {
				return depth
			}
		}
		next := uint64(0)
		for from := 0; from < stateCount; from++ {
			if reachable&(1<<from) == 0 {
				continue
			}
			for to := 0; to < stateCount; to++ {
				if edges&(1<<(from*stateCount+to)) != 0 {
					next |= 1 << to
				}
			}
		}
		reachable = next
	}
	return -1
}

var exhaustiveIntegerValues = []int{-1, 0, 1}

func integerStatePredicate(state int, primed bool) string {
	suffix := ""
	if primed {
		suffix = "'"
	}
	return fmt.Sprintf("x%s = %d", suffix, exhaustiveIntegerValues[state])
}

func integerMaskPredicate(mask uint64, primed bool) string {
	states := make([]string, 0, len(exhaustiveIntegerValues))
	for state := range exhaustiveIntegerValues {
		if mask&(1<<state) != 0 {
			states = append(states, integerStatePredicate(state, primed))
		}
	}
	if len(states) == 0 {
		return "FALSE"
	}
	return strings.Join(states, " \\/ ")
}

func integerTransitionPredicate(edges uint64) string {
	stateCount := len(exhaustiveIntegerValues)
	transitions := make([]string, 0, stateCount*stateCount)
	for from := 0; from < stateCount; from++ {
		for to := 0; to < stateCount; to++ {
			if edges&(1<<(from*stateCount+to)) != 0 {
				transitions = append(transitions,
					"("+integerStatePredicate(from, false)+" /\\ "+integerStatePredicate(to, true)+")")
			}
		}
	}
	if len(transitions) == 0 {
		return "FALSE"
	}
	return strings.Join(transitions, " \\/ ")
}

func TestBoundedSafetyMatchesExhaustiveBooleanMachines(t *testing.T) {
	random := rand.New(rand.NewSource(0x5eed))
	for number := 0; number < 24; number++ {
		init := uint8(random.Intn(15) + 1)
		edges := uint16(random.Intn(1 << 16))
		invariant := uint8(random.Intn(16))
		bound := random.Intn(4)
		wantDepth := exhaustiveViolationDepth(init, edges, invariant, bound)
		source := fmt.Sprintf(`---- MODULE Exhaustive%d ----
\* @type: Bool;
VARIABLE a
\* @type: Bool;
VARIABLE b
Init == %s
Next == %s
Inv == %s
====`, number, booleanMaskPredicate(uint16(init), false), booleanTransitionPredicate(edges),
			booleanMaskPredicate(uint16(invariant), false))
		frontend := ParseAndTypeCheckSource(context.Background(), fmt.Sprintf("Exhaustive%d.tla", number), source,
			FrontendOptions{}, TypeCheckOptions{})
		if !frontend.OK() {
			t.Fatalf("case %d frontend diagnostics: %v\n%s", number, frontend.Diagnostics, source)
		}
		prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
			Specification: SpecificationConfiguration{
				Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
			}, Sources: frontend.Sources,
		})
		if err != nil {
			t.Fatalf("case %d prepare: %v\n%s", number, err, source)
		}
		result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
			BoundedSafetyOptions{MaxSteps: bound, CheckDeadlocks: false})
		if err != nil {
			t.Fatalf("case %d check: %v\n%s", number, err, source)
		}
		if wantDepth < 0 {
			if result.Outcome != SafetyNoCounterexample {
				t.Fatalf("case %d = %#v, want safe through depth %d\n%s", number, result, bound, source)
			}
		} else if result.Outcome != SafetyCounterexample || result.Depth != wantDepth {
			t.Fatalf("case %d = %#v, want violation at depth %d\n%s", number, result, wantDepth, source)
		}

		wantDeadlock := exhaustiveDeadlockDepth(init, edges, bound)
		deadlockPredicates := prepared.Predicates
		deadlockPredicates.Invariants = nil
		deadlockResult, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, deadlockPredicates,
			BoundedSafetyOptions{MaxSteps: bound, CheckDeadlocks: true})
		if err != nil {
			t.Fatalf("case %d deadlock check: %v\n%s", number, err, source)
		}
		if wantDeadlock < 0 {
			if deadlockResult.Outcome != SafetyNoCounterexample {
				t.Fatalf("case %d deadlock = %#v, want none through depth %d\n%s",
					number, deadlockResult, bound, source)
			}
		} else if deadlockResult.Outcome != SafetyDeadlock || deadlockResult.Depth != wantDeadlock {
			t.Fatalf("case %d deadlock = %#v, want depth %d\n%s", number, deadlockResult, wantDeadlock, source)
		}
	}
}

func TestBoundedSafetyMatchesExhaustiveIntegerMachines(t *testing.T) {
	random := rand.New(rand.NewSource(0x1a7e6e7))
	stateCount := len(exhaustiveIntegerValues)
	for number := 0; number < 24; number++ {
		init := uint64(random.Intn((1<<stateCount)-1) + 1)
		edges := uint64(random.Intn(1 << (stateCount * stateCount)))
		invariant := uint64(random.Intn(1 << stateCount))
		bound := random.Intn(4)
		wantDepth := exhaustiveGraphViolationDepth(init, edges, invariant, stateCount, bound)
		source := fmt.Sprintf(`---- MODULE ExhaustiveInteger%d ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == %s
Next == %s
Inv == %s
====`, number, integerMaskPredicate(init, false), integerTransitionPredicate(edges),
			integerMaskPredicate(invariant, false))
		frontend := ParseAndTypeCheckSource(context.Background(), fmt.Sprintf("ExhaustiveInteger%d.tla", number), source,
			FrontendOptions{}, TypeCheckOptions{})
		if !frontend.OK() {
			t.Fatalf("case %d frontend diagnostics: %v\n%s", number, frontend.Diagnostics, source)
		}
		prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
			Specification: SpecificationConfiguration{
				Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
			}, Sources: frontend.Sources,
		})
		if err != nil {
			t.Fatalf("case %d prepare: %v\n%s", number, err, source)
		}
		result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
			BoundedSafetyOptions{MaxSteps: bound, CheckDeadlocks: false})
		if err != nil {
			t.Fatalf("case %d check: %v\n%s", number, err, source)
		}
		if wantDepth < 0 {
			if result.Outcome != SafetyNoCounterexample {
				t.Fatalf("case %d = %#v, want safe through depth %d\n%s", number, result, bound, source)
			}
		} else if result.Outcome != SafetyCounterexample || result.Depth != wantDepth {
			t.Fatalf("case %d = %#v, want violation at depth %d\n%s", number, result, wantDepth, source)
		}

		wantDeadlock := exhaustiveGraphDeadlockDepth(init, edges, stateCount, bound)
		deadlockPredicates := prepared.Predicates
		deadlockPredicates.Invariants = nil
		deadlockResult, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, deadlockPredicates,
			BoundedSafetyOptions{MaxSteps: bound, CheckDeadlocks: true})
		if err != nil {
			t.Fatalf("case %d deadlock check: %v\n%s", number, err, source)
		}
		if wantDeadlock < 0 {
			if deadlockResult.Outcome != SafetyNoCounterexample {
				t.Fatalf("case %d deadlock = %#v, want none through depth %d\n%s",
					number, deadlockResult, bound, source)
			}
		} else if deadlockResult.Outcome != SafetyDeadlock || deadlockResult.Depth != wantDeadlock {
			t.Fatalf("case %d deadlock = %#v, want depth %d\n%s", number, deadlockResult, wantDeadlock, source)
		}
	}
}

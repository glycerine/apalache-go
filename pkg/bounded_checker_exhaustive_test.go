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

func checkExhaustiveMachine(
	t *testing.T,
	name, source string,
	init, edges, invariant uint64,
	stateCount, bound int,
) {
	t.Helper()
	frontend := ParseAndTypeCheckSource(context.Background(), name+".tla", source,
		FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics: %v\n%s", frontend.Diagnostics, source)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
		}, Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatalf("prepare: %v\n%s", err, source)
	}
	wantDepth := exhaustiveGraphViolationDepth(init, edges, invariant, stateCount, bound)
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: bound, CheckDeadlocks: false})
	if err != nil {
		t.Fatalf("check: %v\n%s", err, source)
	}
	if wantDepth < 0 {
		if result.Outcome != SafetyNoCounterexample {
			t.Fatalf("result = %#v, want safe through depth %d\n%s", result, bound, source)
		}
	} else if result.Outcome != SafetyCounterexample || result.Depth != wantDepth {
		t.Fatalf("result = %#v, want violation at depth %d\n%s", result, wantDepth, source)
	}

	wantDeadlock := exhaustiveGraphDeadlockDepth(init, edges, stateCount, bound)
	deadlockPredicates := prepared.Predicates
	deadlockPredicates.Invariants = nil
	deadlockResult, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, deadlockPredicates,
		BoundedSafetyOptions{MaxSteps: bound, CheckDeadlocks: true})
	if err != nil {
		t.Fatalf("deadlock check: %v\n%s", err, source)
	}
	if wantDeadlock < 0 {
		if deadlockResult.Outcome != SafetyNoCounterexample {
			t.Fatalf("deadlock = %#v, want none through depth %d\n%s", deadlockResult, bound, source)
		}
	} else if deadlockResult.Outcome != SafetyDeadlock || deadlockResult.Depth != wantDeadlock {
		t.Fatalf("deadlock = %#v, want depth %d\n%s", deadlockResult, wantDeadlock, source)
	}
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

func tupleStatePredicate(state int, primed bool) string {
	suffix := ""
	if primed {
		suffix = "'"
	}
	left, right := "FALSE", "FALSE"
	if state&2 != 0 {
		left = "TRUE"
	}
	if state&1 != 0 {
		right = "TRUE"
	}
	return fmt.Sprintf("p%s = <<%s, %s>>", suffix, left, right)
}

func tupleMaskPredicate(mask uint64, primed bool) string {
	states := make([]string, 0, 4)
	for state := 0; state < 4; state++ {
		if mask&(1<<state) != 0 {
			states = append(states, tupleStatePredicate(state, primed))
		}
	}
	if len(states) == 0 {
		return "FALSE"
	}
	return strings.Join(states, " \\/ ")
}

func tupleTransitionPredicate(edges uint64) string {
	transitions := make([]string, 0, 16)
	for from := 0; from < 4; from++ {
		for to := 0; to < 4; to++ {
			if edges&(1<<(from*4+to)) != 0 {
				transitions = append(transitions,
					"("+tupleStatePredicate(from, false)+" /\\ "+tupleStatePredicate(to, true)+")")
			}
		}
	}
	if len(transitions) == 0 {
		return "FALSE"
	}
	return strings.Join(transitions, " \\/ ")
}

func valueStatePredicate(variable string, values []string, state int, primed bool) string {
	suffix := ""
	if primed {
		suffix = "'"
	}
	return fmt.Sprintf("%s%s = %s", variable, suffix, values[state])
}

func valueMaskPredicate(variable string, values []string, mask uint64, primed bool) string {
	states := make([]string, 0, len(values))
	for state := range values {
		if mask&(1<<state) != 0 {
			states = append(states, valueStatePredicate(variable, values, state, primed))
		}
	}
	if len(states) == 0 {
		return "FALSE"
	}
	return strings.Join(states, " \\/ ")
}

func valueTransitionPredicate(variable string, values []string, edges uint64) string {
	stateCount := len(values)
	transitions := make([]string, 0, stateCount*stateCount)
	for from := 0; from < stateCount; from++ {
		for to := 0; to < stateCount; to++ {
			if edges&(1<<(from*stateCount+to)) != 0 {
				transitions = append(transitions, "("+
					valueStatePredicate(variable, values, from, false)+" /\\ "+
					valueStatePredicate(variable, values, to, true)+")")
			}
		}
	}
	if len(transitions) == 0 {
		return "FALSE"
	}
	return strings.Join(transitions, " \\/ ")
}

func testExhaustiveValueMachines(
	t *testing.T,
	prefix, variable, annotation string,
	values []string,
	seed int64,
) {
	t.Helper()
	random := rand.New(rand.NewSource(seed))
	stateCount := len(values)
	for number := 0; number < 24; number++ {
		init := uint64(random.Intn((1<<stateCount)-1) + 1)
		edges := uint64(random.Intn(1 << (stateCount * stateCount)))
		invariant := uint64(random.Intn(1 << stateCount))
		bound := random.Intn(4)
		name := fmt.Sprintf("%s%d", prefix, number)
		source := fmt.Sprintf(`---- MODULE %s ----
\* @type: %s;
VARIABLE %s
Init == %s
Next == %s
Inv == %s
====`, name, annotation, variable,
			valueMaskPredicate(variable, values, init, false),
			valueTransitionPredicate(variable, values, edges),
			valueMaskPredicate(variable, values, invariant, false))
		checkExhaustiveMachine(t, name, source, init, edges, invariant, stateCount, bound)
	}
}

func TestBoundedSafetyMatchesExhaustiveBooleanMachines(t *testing.T) {
	random := rand.New(rand.NewSource(0x5eed))
	for number := 0; number < 24; number++ {
		init := uint8(random.Intn(15) + 1)
		edges := uint16(random.Intn(1 << 16))
		invariant := uint8(random.Intn(16))
		bound := random.Intn(4)
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
		checkExhaustiveMachine(t, fmt.Sprintf("Exhaustive%d", number), source,
			uint64(init), uint64(edges), uint64(invariant), 4, bound)
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
		source := fmt.Sprintf(`---- MODULE ExhaustiveInteger%d ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == %s
Next == %s
Inv == %s
====`, number, integerMaskPredicate(init, false), integerTransitionPredicate(edges),
			integerMaskPredicate(invariant, false))
		checkExhaustiveMachine(t, fmt.Sprintf("ExhaustiveInteger%d", number), source,
			init, edges, invariant, stateCount, bound)
	}
}

func TestBoundedSafetyMatchesExhaustiveTupleMachines(t *testing.T) {
	random := rand.New(rand.NewSource(0x7a91e))
	for number := 0; number < 24; number++ {
		init := uint64(random.Intn(15) + 1)
		edges := uint64(random.Intn(1 << 16))
		invariant := uint64(random.Intn(16))
		bound := random.Intn(4)
		source := fmt.Sprintf(`---- MODULE ExhaustiveTuple%d ----
\* @type: <<Bool, Bool>>;
VARIABLE p
Init == %s
Next == %s
Inv == %s
====`, number, tupleMaskPredicate(init, false), tupleTransitionPredicate(edges),
			tupleMaskPredicate(invariant, false))
		checkExhaustiveMachine(t, fmt.Sprintf("ExhaustiveTuple%d", number), source,
			init, edges, invariant, 4, bound)
	}
}

func TestBoundedSafetyMatchesExhaustiveRecordMachines(t *testing.T) {
	testExhaustiveValueMachines(t, "ExhaustiveRecord", "r", "{ flag: Bool, count: Int }", []string{
		"[flag |-> FALSE, count |-> 0]",
		"[flag |-> FALSE, count |-> 1]",
		"[flag |-> TRUE, count |-> 0]",
		"[flag |-> TRUE, count |-> 1]",
	}, 0x5ec07d)
}

func TestBoundedSafetyMatchesExhaustiveSetMachines(t *testing.T) {
	testExhaustiveValueMachines(t, "ExhaustiveSet", "s", "Set(Int)", []string{
		"{}", "{1}", "{2}", "{1, 2}",
	}, 0x5e7)
}

func TestBoundedSafetyMatchesExhaustiveSequenceMachines(t *testing.T) {
	testExhaustiveValueMachines(t, "ExhaustiveSequence", "q", "Seq(Int)", []string{
		"<<>>", "<<0>>", "<<1>>", "<<0, 1>>",
	}, 0x5e0)
}

func TestBoundedSafetyMatchesExhaustiveFunctionMachines(t *testing.T) {
	testExhaustiveValueMachines(t, "ExhaustiveFunction", "f", "Int -> Bool", []string{
		"[i \\in {1, 2} |-> FALSE]",
		"[i \\in {1, 2} |-> i = 2]",
		"[i \\in {1, 2} |-> i = 1]",
		"[i \\in {1, 2} |-> TRUE]",
	}, 0xf00c710)
}

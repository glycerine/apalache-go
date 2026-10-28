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
	extensions string,
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
		extendsLine := ""
		if extensions != "" {
			extendsLine = "EXTENDS " + extensions + "\n"
		}
		source := fmt.Sprintf(`---- MODULE %s ----
%s\* @type: %s;
VARIABLE %s
Init == %s
Next == %s
Inv == %s
====`, name, extendsLine, annotation, variable,
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
	}, 0x5ec07d, "")
}

func TestBoundedSafetyMatchesExhaustiveSetMachines(t *testing.T) {
	testExhaustiveValueMachines(t, "ExhaustiveSet", "s", "Set(Int)", []string{
		"{}", "{1}", "{2}", "{1, 2}",
	}, 0x5e7, "")
}

func TestBoundedSafetyMatchesExhaustiveSequenceMachines(t *testing.T) {
	testExhaustiveValueMachines(t, "ExhaustiveSequence", "q", "Seq(Int)", []string{
		"<<>>", "<<0>>", "<<1>>", "<<0, 1>>",
	}, 0x5e0, "")
}

func TestBoundedSafetyMatchesExhaustiveFunctionMachines(t *testing.T) {
	testExhaustiveValueMachines(t, "ExhaustiveFunction", "f", "Int -> Bool", []string{
		"[i \\in {1, 2} |-> FALSE]",
		"[i \\in {1, 2} |-> i = 2]",
		"[i \\in {1, 2} |-> i = 1]",
		"[i \\in {1, 2} |-> TRUE]",
	}, 0xf00c710, "")
}

func TestBoundedSafetyMatchesExhaustiveVariantMachines(t *testing.T) {
	testExhaustiveValueMachines(t, "ExhaustiveVariant", "v", "A(Int) | B(Bool)", []string{
		`Variant("A", 0)`,
		`Variant("A", 1)`,
		`Variant("B", FALSE)`,
		`Variant("B", TRUE)`,
	}, 0xa11a17, "Variants")
}

func TestBoundedSafetyMatchesExhaustiveNestedMachines(t *testing.T) {
	testExhaustiveValueMachines(t, "ExhaustiveNested", "n",
		"{ items: Seq(Int), choice: A(Int) | B(Bool) }", []string{
			`[items |-> <<>>, choice |-> Variant("A", 0)]`,
			`[items |-> <<1>>, choice |-> Variant("A", 1)]`,
			`[items |-> <<>>, choice |-> Variant("B", FALSE)]`,
			`[items |-> <<1, 2>>, choice |-> Variant("B", TRUE)]`,
		}, 0x0e57ed, "Variants")
}

func TestBoundedSafetyMatchesOperatorRichIntegerMachines(t *testing.T) {
	type transitionTemplate struct {
		expression string
		next       func(int) []int
	}
	templates := []transitionTemplate{
		{"x' = (x + 1) % 4", func(x int) []int { return []int{(x + 1) % 4} }},
		{"x' = IF x < 3 THEN x + 1 ELSE 0", func(x int) []int { return []int{(x + 1) % 4} }},
		{"x' = 3 - x", func(x int) []int { return []int{3 - x} }},
		{"x' = x", func(x int) []int { return []int{x} }},
		{"x' = x \\/ x' = (x + 1) % 4", func(x int) []int { return []int{x, (x + 1) % 4} }},
		{"x < 3 /\\ x' = x + 1", func(x int) []int {
			if x < 3 {
				return []int{x + 1}
			}
			return nil
		}},
		{"x > 0 /\\ x' = x - 1", func(x int) []int {
			if x > 0 {
				return []int{x - 1}
			}
			return nil
		}},
		{"x' = (x * 3 + 1) % 4", func(x int) []int { return []int{(x*3 + 1) % 4} }},
	}
	values := []string{"0", "1", "2", "3"}
	random := rand.New(rand.NewSource(0x0fe7a70))
	for number := 0; number < 24; number++ {
		template := templates[number%len(templates)]
		init := uint64(random.Intn(15) + 1)
		invariant := uint64(random.Intn(16))
		bound := random.Intn(4)
		edges := uint64(0)
		for from := range values {
			for _, to := range template.next(from) {
				edges |= 1 << (from*len(values) + to)
			}
		}
		name := fmt.Sprintf("OperatorInteger%d", number)
		source := fmt.Sprintf(`---- MODULE %s ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == %s
Next == %s
Inv == %s
====`, name, valueMaskPredicate("x", values, init, false), template.expression,
			valueMaskPredicate("x", values, invariant, false))
		checkExhaustiveMachine(t, name, source, init, edges, invariant, len(values), bound)
	}
}

func TestBoundedSafetyMatchesOperatorRichSetMachines(t *testing.T) {
	type transitionTemplate struct {
		expression string
		next       func(int) []int
	}
	templates := []transitionTemplate{
		{"s' = s \\union {1}", func(s int) []int { return []int{s | 1} }},
		{"s' = s \\union {2}", func(s int) []int { return []int{s | 2} }},
		{"s' = s \\ {1}", func(s int) []int { return []int{s & 2} }},
		{"s' = IF 1 \\in s THEN s \\ {1} ELSE s \\union {1}", func(s int) []int { return []int{s ^ 1} }},
		{"s' = s \\union {1} \\/ s' = s \\union {2}", func(s int) []int { return []int{s | 1, s | 2} }},
		{"1 \\notin s /\\ s' = s \\union {1}", func(s int) []int {
			if s&1 == 0 {
				return []int{s | 1}
			}
			return nil
		}},
		{"s' = {1, 2} \\ s", func(s int) []int { return []int{3 &^ s} }},
		{"s' = s \\intersect {1}", func(s int) []int { return []int{s & 1} }},
	}
	values := []string{"{}", "{1}", "{2}", "{1, 2}"}
	random := rand.New(rand.NewSource(0x5e70fe7a70))
	for number := 0; number < 24; number++ {
		template := templates[number%len(templates)]
		init := uint64(random.Intn(15) + 1)
		invariant := uint64(random.Intn(16))
		bound := random.Intn(4)
		edges := uint64(0)
		for from := range values {
			for _, to := range template.next(from) {
				edges |= 1 << (from*len(values) + to)
			}
		}
		name := fmt.Sprintf("OperatorSet%d", number)
		source := fmt.Sprintf(`---- MODULE %s ----
\* @type: Set(Int);
VARIABLE s
Init == %s
Next == %s
Inv == %s
====`, name, valueMaskPredicate("s", values, init, false), template.expression,
			valueMaskPredicate("s", values, invariant, false))
		checkExhaustiveMachine(t, name, source, init, edges, invariant, len(values), bound)
	}
}

func TestBoundedSafetyMatchesOperatorRichSequenceMachines(t *testing.T) {
	type transitionTemplate struct {
		expression string
		next       func(int) []int
	}
	shiftTrue := func(q int) int { return (q&1)<<1 | 1 }
	shiftFalse := func(q int) int { return (q & 1) << 1 }
	templates := []transitionTemplate{
		{"q' = Append(Tail(q), TRUE)", func(q int) []int { return []int{shiftTrue(q)} }},
		{"q' = Append(Tail(q), FALSE)", func(q int) []int { return []int{shiftFalse(q)} }},
		{"q' = <<q[2], q[1]>>", func(q int) []int { return []int{(q&1)<<1 | (q >> 1)} }},
		{"q' = [q EXCEPT ![1] = TRUE]", func(q int) []int { return []int{q | 2} }},
		{"q' = [q EXCEPT ![2] = FALSE]", func(q int) []int { return []int{q & 2} }},
		{"q' = [q EXCEPT ![1] = ~@]", func(q int) []int { return []int{q ^ 2} }},
		{"q' = Append(Tail(q), TRUE) \\/ q' = Append(Tail(q), FALSE)", func(q int) []int {
			return []int{shiftTrue(q), shiftFalse(q)}
		}},
		{"Head(q) = FALSE /\\ q' = Append(Tail(q), TRUE)", func(q int) []int {
			if q&2 == 0 {
				return []int{shiftTrue(q)}
			}
			return nil
		}},
	}
	values := []string{"<<FALSE, FALSE>>", "<<FALSE, TRUE>>", "<<TRUE, FALSE>>", "<<TRUE, TRUE>>"}
	random := rand.New(rand.NewSource(0x5e90fe7a70))
	for number := 0; number < 24; number++ {
		template := templates[number%len(templates)]
		init := uint64(random.Intn(15) + 1)
		invariant := uint64(random.Intn(16))
		bound := random.Intn(4)
		edges := uint64(0)
		for from := range values {
			for _, to := range template.next(from) {
				edges |= 1 << (from*len(values) + to)
			}
		}
		name := fmt.Sprintf("OperatorSequence%d", number)
		source := fmt.Sprintf(`---- MODULE %s ----
EXTENDS Sequences
\* @type: Seq(Bool);
VARIABLE q
Init == %s
Next == %s
Inv == %s
====`, name, valueMaskPredicate("q", values, init, false), template.expression,
			valueMaskPredicate("q", values, invariant, false))
		checkExhaustiveMachine(t, name, source, init, edges, invariant, len(values), bound)
	}
}

func TestBoundedSafetyMatchesQuantifiedTransitionMachines(t *testing.T) {
	type transitionTemplate struct {
		expression string
		next       func(int) []int
	}
	templates := []transitionTemplate{
		{"(\\E i \\in {0, 1}: i = x % 2) /\\ x' = (x + 1) % 4", func(x int) []int {
			return []int{(x + 1) % 4}
		}},
		{"(\\A i \\in {0, 1}: i <= x) /\\ x' = (x + 2) % 4", func(x int) []int {
			if x >= 1 {
				return []int{(x + 2) % 4}
			}
			return nil
		}},
		{"\\E i \\in {0, 1, 2, 3}: i = x /\\ x' = 3 - i", func(x int) []int {
			return []int{3 - x}
		}},
		{"(\\A i \\in {1, 2}: x < i) /\\ x' = x + 1", func(x int) []int {
			if x == 0 {
				return []int{1}
			}
			return nil
		}},
		{"(\\E i \\in {0, 1, 2, 3}: i = x) /\\ x' = (x + 3) % 4", func(x int) []int {
			return []int{(x + 3) % 4}
		}},
		{"(\\A i \\in {0, 1, 2, 3}: i <= x) /\\ x' = x", func(x int) []int {
			if x == 3 {
				return []int{3}
			}
			return nil
		}},
		{"(\\E i \\in {0, 1}: i = x % 2) /\\ x' = IF x = 3 THEN 0 ELSE x + 1", func(x int) []int {
			return []int{(x + 1) % 4}
		}},
		{"\\E i \\in {0, 1, 2, 3}: /\\ i = (x + 1) % 4 /\\ x' = i /\\ \\A j \\in {i}: j = i", func(x int) []int {
			return []int{(x + 1) % 4}
		}},
	}
	values := []string{"0", "1", "2", "3"}
	random := rand.New(rand.NewSource(0x0a11f1ed))
	for number := 0; number < 24; number++ {
		template := templates[number%len(templates)]
		init := uint64(random.Intn(15) + 1)
		invariant := uint64(random.Intn(16))
		bound := random.Intn(4)
		edges := uint64(0)
		for from := range values {
			for _, to := range template.next(from) {
				edges |= 1 << (from*len(values) + to)
			}
		}
		name := fmt.Sprintf("QuantifiedTransition%d", number)
		source := fmt.Sprintf(`---- MODULE %s ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == %s
Next == %s
Inv == %s
====`, name, valueMaskPredicate("x", values, init, false), template.expression,
			valueMaskPredicate("x", values, invariant, false))
		checkExhaustiveMachine(t, name, source, init, edges, invariant, len(values), bound)
	}
}

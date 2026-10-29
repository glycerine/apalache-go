package apalache

import (
	"context"
	"fmt"
	"testing"
)

func prepareReviewModel(t *testing.T, source string, invariants []string, view string) SafetyPipelineResult {
	t.Helper()
	frontend := ParseAndTypeCheckSource(context.Background(), "Review.tla", source, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend: %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: invariants, View: view,
		}, Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

func TestReviewStructuredViewExclusions(t *testing.T) {
	for _, tc := range []struct {
		name, view string
		want       int
	}{
		{"tuple", "<<x, TRUE>>", 3},
		{"sequence", "<<x, x>>", 3},
		{"record", "[a |-> x]", 3},
		{"set", "{x}", 3},
		{"string", `IF x = 1 THEN "a" ELSE IF x = 2 THEN "b" ELSE "c"`, 3},
		{"constant_view", "<<0, TRUE>>", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			annotation := ""
			if tc.name == "sequence" {
				annotation = `\* @type: () => Seq(Int);`
			}
			prepared := prepareReviewModel(t, fmt.Sprintf(`---- MODULE Review ----
EXTENDS Integers, FiniteSets
\* @type: Int;
VARIABLE x
Init == x \in {1, 2, 3}
Next == x' = x
Inv == x = 0
%s
View == %s
====`, annotation, tc.view), []string{"Inv"}, "View")
			for _, encoding := range []SMTEncoding{SMTEncodingOOPSLA19, SMTEncodingArrays, SMTEncodingFunArrays} {
				t.Run(string(encoding), func(t *testing.T) {
					result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
						BoundedSafetyOptions{MaxSteps: 0, MaxErrors: 4, SMTEncoding: encoding})
					if err != nil {
						t.Fatal(err)
					}
					views := make(map[string]bool)
					for _, cex := range result.Counterexamples {
						view, err := FormatTLAExpression(cex.View[0])
						if err != nil {
							t.Fatal(err)
						}
						views[view] = true
					}
					if result.Outcome != SafetyCounterexample || len(result.Counterexamples) != tc.want || len(views) != tc.want {
						t.Fatalf("want %d distinct views, got %v (result=%+v)", tc.want, views, result)
					}
				})
			}
		})
	}
}

func TestReviewSimulationPreservesUnknown(t *testing.T) {
	prepared := prepareReviewModel(t, `---- MODULE Review ----
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

func TestReviewActionInvariantRespectsBound(t *testing.T) {
	for _, bound := range []int{0, 2} {
		t.Run(fmt.Sprint(bound), func(t *testing.T) {
			prepared := prepareReviewModel(t, fmt.Sprintf(`---- MODULE Review ----
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

func TestReviewSimulationExcludesViewsAcrossRuns(t *testing.T) {
	for _, tc := range []struct {
		name, init, next, inv string
		steps, want           int
	}{
		{"one_initial_view", "x = 0", "x' = x", "x = 1", 0, 1},
		{"distinct_initial_branches", "x = 1 \\/ x = 2 \\/ x = 3", "x' = x", "x = 0", 0, 3},
		{"distinct_action_branches", "x = 0", "x' = 1 \\/ x' = 2", "x = 0", 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepared := prepareReviewModel(t, fmt.Sprintf(`---- MODULE Review ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == %s
Next == %s
Inv == %s
View == <<x, TRUE>>
====`, tc.init, tc.next, tc.inv), []string{"Inv"}, "View")
			report, err := SimulateAndCheckWithOptions(context.Background(), prepared.Module, prepared.Predicates,
				SimulationOptions{MaxSteps: tc.steps, MaxRuns: 10, MaxErrors: 5, RandomSeed: 17})
			if err != nil {
				t.Fatal(err)
			}
			views := make(map[string]bool)
			for _, cex := range report.Counterexamples {
				var path string
				for _, value := range cex.View {
					text, err := FormatTLAExpression(value)
					if err != nil {
						t.Fatal(err)
					}
					path += text + ";"
				}
				if views[path] {
					t.Errorf("duplicate counterexample view: %s", path)
				}
				views[path] = true
			}
			if len(report.Counterexamples) != tc.want || len(views) != tc.want {
				t.Fatalf("want %d distinct counterexamples, got %d errors, views=%v", tc.want, len(report.Counterexamples), views)
			}
		})
	}
}

func TestReviewTraceInvariantsIgnoreStateActionFilter(t *testing.T) {
	prepared := prepareReviewModel(t, `---- MODULE Review ----
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

func TestReviewInitialStateExhaustion(t *testing.T) {
	for _, tc := range []struct{ name, init, filter string }{
		{"contradictory_init", "x = 0 /\\ x > 0", ""},
		{"all_init_filtered", "x = 0", "1->0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepared := prepareReviewModel(t, fmt.Sprintf(`---- MODULE Review ----
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

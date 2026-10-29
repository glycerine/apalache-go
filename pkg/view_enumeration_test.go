package apalache

import (
	"context"
	"fmt"
	"testing"
)

func TestStructuredViewExclusions(t *testing.T) {
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
			prepared := prepareSafetyModelFixture(t, fmt.Sprintf(`---- MODULE Review ----
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

func TestSimulationExcludesViewsAcrossRuns(t *testing.T) {
	for _, tc := range []struct {
		name, init, next, inv string
		steps, want           int
	}{
		{"one_initial_view", "x = 0", "x' = x", "x = 1", 0, 1},
		{"distinct_initial_branches", "x = 1 \\/ x = 2 \\/ x = 3", "x' = x", "x = 0", 0, 3},
		{"distinct_action_branches", "x = 0", "x' = 1 \\/ x' = 2", "x = 0", 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepared := prepareSafetyModelFixture(t, fmt.Sprintf(`---- MODULE Review ----
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

package oraclectl

import "testing"

func TestNormalizeCheckerOutcomes(t *testing.T) {
	tests := []struct {
		name, output string
		code         int
		verdict      Verdict
		property     string
		depth        int
	}{
		{"Go safe", "No counterexample found through depth 3 (4 symbolic executions).", 0, VerdictSafe, "", -1},
		{"Go violation", "Invariant Inv violated at depth 2 (Init$0000).", 12, VerdictViolation, "Inv", 2},
		{"Go deadlock", "Deadlock found at depth 1.", 12, VerdictDeadlock, "", 1},
		{"Scala safe", "The outcome is: NoError\nChecker reports no error up to computation length 10", 0, VerdictSafe, "", -1},
		{"Scala violation", "State 2: state invariant 0 violated.\nThe outcome is: Error\nEXITCODE: ERROR (12)", 12, VerdictViolation, "", 2},
		{"Scala action violation", "State 3: action invariant 1 [line 4] violated.\nThe outcome is: Error", 12, VerdictViolation, "", 4},
		{"Scala trace violation", "State 4: trace invariant 0 violated.\nThe outcome is: Error", 12, VerdictViolation, "", 4},
		{"Scala deadlock", "State 4: state invariant 0 holds.\nStep 5: Transition #0 is disabled\nFound a deadlock.\nThe outcome is: Deadlock", 12, VerdictDeadlock, "", 4},
		{"Scala deadlock without invariants", "Step 3: Transition #0 is disabled\nFound a deadlock.\nThe outcome is: Deadlock", 12, VerdictDeadlock, "", 2},
		{"unknown", "solver returned unknown", 1, VerdictUnknown, "", -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Normalize(test.output, "", test.code)
			if got.Verdict != test.verdict || got.Property != test.property || got.Depth != test.depth || got.ExitCode != test.code {
				t.Fatalf("Normalize() = %#v", got)
			}
		})
	}
}

func TestCompareUsesAvailablePrecision(t *testing.T) {
	goResult := Outcome{Verdict: VerdictViolation, Property: "Inv", Depth: 2}
	scalaResult := Outcome{Verdict: VerdictViolation, Depth: -1}
	if err := Compare(goResult, scalaResult); err != nil {
		t.Fatal(err)
	}
	if err := Compare(goResult, Outcome{Verdict: VerdictSafe, Depth: -1}); err == nil {
		t.Fatal("accepted mismatched verdicts")
	}
	if err := Compare(goResult, Outcome{Verdict: VerdictViolation, Depth: 3}); err == nil {
		t.Fatal("accepted mismatched violation depths")
	}
}

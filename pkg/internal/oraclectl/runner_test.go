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
		{"Scala violation", "The outcome is: Error\nEXITCODE: ERROR (12)", 12, VerdictViolation, "", -1},
		{"Scala deadlock", "The outcome is: Deadlock", 12, VerdictDeadlock, "", -1},
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
}

package oraclectl

import (
	"os"
	"path/filepath"
	"testing"
)

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
	goResult.Trace = []string{}
	if err := Compare(goResult, scalaResult); err == nil {
		t.Fatal("accepted unavailable trace")
	}
	goResult.Trace = []string{`{"x":{"#bigint":"0"}}`, `{"x":{"#bigint":"1"}}`}
	scalaResult.Trace = append([]string(nil), goResult.Trace...)
	if err := Compare(goResult, scalaResult); err != nil {
		t.Fatal(err)
	}
	scalaResult.Trace[1] = `{"x":{"#bigint":"2"}}`
	if err := Compare(goResult, scalaResult); err == nil {
		t.Fatal("accepted mismatched trace state")
	}
}

func TestReadITFTraceNormalizesMetadataSetsAndMaps(t *testing.T) {
	directory := t.TempDir()
	leftPath := filepath.Join(directory, "left.itf.json")
	rightPath := filepath.Join(directory, "right.itf.json")
	left := `{"states":[{"#meta":{"index":0},"__temporal_t_1":true,"s":{"#set":[{"#bigint":"2"},{"#bigint":"1"}]},"f":{"#map":[[{"#bigint":"2"},"b"],[{"#bigint":"1"},"a"]]}}]}`
	right := `{"states":[{"#meta":{"index":99},"f":{"#map":[[{"#bigint":"1"},"a"],[{"#bigint":"2"},"b"]]},"s":{"#set":[{"#bigint":"1"},{"#bigint":"2"}]}}]}`
	if err := os.WriteFile(leftPath, []byte(left), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightPath, []byte(right), 0o600); err != nil {
		t.Fatal(err)
	}
	leftTrace, err := readITFTrace(leftPath)
	if err != nil {
		t.Fatal(err)
	}
	rightTrace, err := readITFTrace(rightPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(leftTrace) != 1 || leftTrace[0] != rightTrace[0] {
		t.Fatalf("normalized traces differ: %v != %v", leftTrace, rightTrace)
	}
}

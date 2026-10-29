package apalache

import (
	"context"
	"fmt"
	"testing"
)

func TestLegacyRecordEqualityIgnoresAbsentFields(t *testing.T) {
	for _, inv := range []string{
		`LET r == IF x=2 THEN [a|->1] ELSE [a|->1,b|->2] IN [r EXCEPT !.b=3]=r`,
		`LET r == IF x=2 THEN [a|->1] ELSE [a|->1,b|->2] IN [r EXCEPT !.b=3] \in {r}`,
		`LET r == IF x=2 THEN [a|->1] ELSE [a|->1,b|->2] IN [r EXCEPT !.a=3] /= r`,
		`LET r == IF x=2 THEN [a|->1,b|->2] ELSE [a|->1] IN [r EXCEPT !.b=3] /= r`,
	} {
		t.Run(inv, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE Records ----\nEXTENDS Integers\nVARIABLE\n\\* @type: Int;\nx\nInit == x=2\nNext == x'=x\nInv == %s\n====", inv)
			front := ParseAndTypeCheckSource(context.Background(), "Records.tla", source, FrontendOptions{}, TypeCheckOptions{NoRows: true})
			if !front.OK() {
				t.Fatal(front.Diagnostics)
			}
			prepared, err := PrepareSafetyTransitions(front.Native, SafetyPipelineOptions{Specification: SpecificationConfiguration{Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"}}, Sources: front.Sources})
			if err != nil {
				t.Fatal(err)
			}
			for _, encoding := range []SMTEncoding{SMTEncodingOOPSLA19, SMTEncodingArrays, SMTEncodingFunArrays} {
				result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates, BoundedSafetyOptions{MaxSteps: 0, SMTEncoding: encoding})
				if err != nil || result.Outcome != SafetyNoCounterexample {
					t.Errorf("%s: result=%v error=%v", encoding, result.Outcome, err)
				}
			}
		})
	}
}

package apalache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFrontendPreservesUserOperatorIdentity(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "User.tla"), []byte("---- MODULE User ----\nEXTENDS Integers\nPrint(a,b) == a\nVariant(a,b) == a+b\n===="), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, definitions, invariant string
		violation                    bool
	}{
		{"qualified_print", "T == INSTANCE TLC\nU == INSTANCE User", "U!Print(FALSE,TRUE)", true},
		{"qualified_variant", "V == INSTANCE Variants\nU == INSTANCE User", "U!Variant(1,2)=3", false},
		{"root_print", "T == INSTANCE TLC\nPrint(a,b) == a", "Print(FALSE, TRUE)", true},
		{"root_variant", "V == INSTANCE Variants\nVariant(a,b) == a+b", "Variant(1,2)=3", false},
		{"local_variant", "V == INSTANCE Variants", "LET Variant(a,b) == a+b IN Variant(1,2)=3", false},
		{"bound_variant", "V == INSTANCE Variants\nApply(Variant(_,_)) == Variant(1,2)\nAdd(a,b) == a+b", "Apply(Add)=3", false},
		{"local_infix", "T == INSTANCE TLC", "LET a @@ b == a-b IN (3 @@ 2)=1", false},
		{"root_infix", "a @@ b == a-b", "(3 @@ 2)=1", false},
		{"local_print", "T == INSTANCE TLC", "LET Print(a,b) == a IN Print(FALSE,TRUE)", true},
		{"bound_print", "T == INSTANCE TLC\nApply(Print(_,_)) == Print(FALSE,TRUE)\nFirst(a,b) == a", "Apply(First)", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE Identity ----\nEXTENDS Integers\n%s\nVARIABLE\n\\* @type: Int;\nx\nInit == x=2\nNext == x'=x\nInv == %s\n====", tc.definitions, tc.invariant)
			frontend := ParseAndTypeCheckSource(context.Background(), filepath.Join(dir, "Identity.tla"), source, FrontendOptions{}, TypeCheckOptions{UseRows: true})
			if !frontend.OK() {
				t.Fatalf("frontend: %v", frontend.Diagnostics)
			}
			prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{Specification: SpecificationConfiguration{Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"}}, Sources: frontend.Sources})
			if err != nil {
				t.Fatal(err)
			}
			result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates, BoundedSafetyOptions{MaxSteps: 0})
			if err != nil {
				t.Fatal(err)
			}
			want := SafetyNoCounterexample
			if tc.violation {
				want = SafetyCounterexample
			}
			if result.Outcome != want {
				t.Fatalf("want %v, got %v", want, result.Outcome)
			}
		})
	}
}

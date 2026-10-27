package apalache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBundledRewritesOverrideAdjacentModules(t *testing.T) {
	directory := t.TempDir()
	root := `---- MODULE Root ----
EXTENDS Folds
Test == MapThenFoldSet(LAMBDA x, y: x, 0, LAMBDA x: x, LAMBDA S: CHOOSE x \in S: TRUE, {1})
====
`
	adjacent := `---- MODULE Folds ----
Wrong == TRUE
====
`
	rootPath := filepath.Join(directory, "Root.tla")
	if err := os.WriteFile(rootPath, []byte(root), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "Folds.tla"), []byte(adjacent), 0o600); err != nil {
		t.Fatal(err)
	}
	result := ParseFile(context.Background(), rootPath, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseFile() diagnostics = %v", result.Diagnostics)
	}
	if _, found := OperatorBody(result.Native, "MapThenFoldSet"); !found {
		t.Fatal("bundled Folds rewrite was not selected")
	}
}

func TestBundledVariantsAndOptionModulesTypeCheck(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "BundledModules.tla", `---- MODULE BundledModules ----
EXTENDS Option
\* @type: Some(Int) | None(UNIT);
Value == Some(1)
IsValue == IsSome(Value)
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !result.OK() {
		t.Fatalf("bundled module diagnostics = %v", result.Diagnostics)
	}
}

func TestBundledBagsBagOfAllChecksMappedKeysAndCollisions(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "BundledBags.tla", `---- MODULE BundledBags ----
EXTENDS Bags
\* @type: (Int) => <<Int, Int>>;
Pair(x) == <<x, x>>
\* @type: (Int) => Int;
One(x) == 1
Init == TRUE
Next == TRUE
Inv ==
  LET B == [x \in {1, 2} |-> x]
  IN /\ BagOfAll(Pair, B) = [p \in {<<1, 1>>, <<2, 2>>} |-> p[1]]
     /\ BagOfAll(One, B) = [x \in {1} |-> 3]
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !frontend.OK() {
		t.Fatalf("bundled Bags diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior:   InitNextSpec{Init: "Init", Next: "Next"},
			Invariants: []string{"Inv"},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 1 {
		t.Fatalf("BagOfAll result = %#v", result)
	}
}

func TestBundledFunctionsChecksFiniteFunctionOperators(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "BundledFunctions.tla", `---- MODULE BundledFunctions ----
EXTENDS Functions
Init == TRUE
Next == TRUE
RangeOK == Range([x \in 1..3 |-> 1]) = {1}
InjectiveOK == ~IsInjective([x \in 1..3 |-> 1])
RestrictOK ==
  LET f == [x \in 1..3 |-> 1]
  IN Restrict(f, {1, 2}) = [x \in {1, 2} |-> 1]
FoldOK ==
  LET f == [x \in 1..3 |-> 1]
      AddToSet(x, acc) == {x} \union acc
  IN FoldFunctionOnSet(AddToSet, {}, f, {1, 2}) = {1}
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !frontend.OK() {
		t.Fatalf("bundled Functions diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior:   InitNextSpec{Init: "Init", Next: "Next"},
			Invariants: []string{"RangeOK", "InjectiveOK", "RestrictOK", "FoldOK"},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 1 {
		t.Fatalf("Functions result = %#v", result)
	}
}

func TestBundledFiniteSetsExtChecksFiniteOperators(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "BundledFiniteSetsExt.tla", `---- MODULE BundledFiniteSetsExt ----
EXTENDS FiniteSetsExt
Init == TRUE
Next == TRUE
Inv ==
  /\ Quantify({"a", "b", "c"}, LAMBDA x: x = "b") = 1
  /\ kSubset(2, {1, 2, 3}) = {{1, 2}, {1, 3}, {2, 3}}
  /\ FoldSet(LAMBDA x, acc: x + acc, 0, {1, 2, 3}) = 6
  /\ ChooseUnique({2, 3, 4, 5}, LAMBDA x: x % 3 = 1) = 4
  /\ SymDiff({1, 2}, {2, 3}) = {1, 3}
  /\ SumSet({1, 2, 3}) = 6
  /\ ProductSet({1, 2, 3, 4}) = 24
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !frontend.OK() {
		t.Fatalf("bundled FiniteSetsExt diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior:   InitNextSpec{Init: "Init", Next: "Next"},
			Invariants: []string{"Inv"},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 1 {
		t.Fatalf("FiniteSetsExt result = %#v", result)
	}
}

func TestBundledBagsExtChecksProductBag(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "BundledBagsExt.tla", `---- MODULE BundledBagsExt ----
EXTENDS BagsExt
Init == TRUE
Next == TRUE
ProductOK ==
  LET B == SetAsFun({<<1, 2>>, <<-2, 1>>})
  IN ProductBag(B) = -2
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !frontend.OK() {
		t.Fatalf("bundled BagsExt diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior:   InitNextSpec{Init: "Init", Next: "Next"},
			Invariants: []string{"ProductOK"},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 1 {
		t.Fatalf("BagsExt result = %#v", result)
	}
}

func TestBundledSequencesExtChecksFiniteSequenceOperators(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "BundledSequencesExt.tla", `---- MODULE BundledSequencesExt ----
EXTENDS SequencesExt, Integers
Init == TRUE
Next == TRUE
Less(i, j) == i < j
ConversionsOK ==
  /\ ToSet(<<1, 2, 1>>) = {1, 2}
  /\ ToSet(SetToSeq({1, 2})) = {1, 2}
ReverseOK == Reverse(<<1, 2, 3>>) = <<3, 2, 1>>
RemoveOK == Remove(<<1, 2, 1>>, 1) = <<2>>
ReplaceAllOK == ReplaceAll(<<1, 2, 1>>, 1, 4) = <<4, 2, 4>>
InsertAtOK == InsertAt(<<1, 3>>, 2, 2) = <<1, 2, 3>>
FoldOK == FoldLeft(LAMBDA acc, x: acc + x, 0, <<1, 2, 3>>) = 6
SortOK == SetToSortSeq({2, 1}, Less) = <<1, 2>>
ZipOK == Zip(<<1, 2>>, <<"a", "b">>) = <<<<1, "a">>, <<2, "b">>>>
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !frontend.OK() {
		t.Fatalf("bundled SequencesExt diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior: InitNextSpec{Init: "Init", Next: "Next"},
			Invariants: []string{
				"ConversionsOK", "ReverseOK", "RemoveOK", "ReplaceAllOK", "InsertAtOK", "FoldOK", "SortOK", "ZipOK",
			},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 1 {
		t.Fatalf("SequencesExt result = %#v", result)
	}
}

func TestBundledTLCNormalizesTypedRegisterOperators(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "BundledTLCRegisters.tla", `---- MODULE BundledTLCRegisters ----
EXTENDS TLC
Registers ==
  /\ TLCSet(1, 42)
  /\ TLCSet("level", 7)
  /\ TLCGet(1) = TLCGet("level")
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !frontend.OK() {
		t.Fatalf("bundled TLC diagnostics = %v", frontend.Diagnostics)
	}
	body, found := OperatorBody(frontend.Native, "Registers")
	if !found || countOperator(body, "TLC!TLCGet") != 2 {
		t.Fatalf("normalized TLC registers = %#v", body)
	}
	if _, importedStub := OperatorBody(frontend.Native, "TLCGet"); importedStub {
		t.Fatal("imported TLCGet stub was retained")
	}
}

func TestBundledTLCPureOperatorsCheckSymbolically(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "BundledTLCPure.tla", `---- MODULE BundledTLCPure ----
EXTENDS TLC
Init == TRUE
Next == TRUE
Inv ==
  /\ Print("ignored", 42) = 42
  /\ PrintT({1, 2})
  /\ Assert(TRUE, "ignored")
  /\ TLCEval({1, 2}) = {1, 2}
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !frontend.OK() {
		t.Fatalf("bundled TLC diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior:   InitNextSpec{Init: "Init", Next: "Next"},
			Invariants: []string{"Inv"},
		},
		Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 1 {
		t.Fatalf("TLC result = %#v", result)
	}
}

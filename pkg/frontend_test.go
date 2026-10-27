package apalache

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const tinyModule = `---- MODULE Tiny ----
EXTENDS Integers
VARIABLE x
Init == x = 0
Next == x' = x + 1
====
`

func TestParseSourceExportsCanonicalApalacheIR(t *testing.T) {
	result := ParseSource(context.Background(), "Tiny.tla", tinyModule, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	if result.Module != "Tiny" {
		t.Fatalf("ParseSource() module = %q, want Tiny", result.Module)
	}
	if result.Native == nil || result.Native.Name() != "Tiny" {
		t.Fatalf("ParseSource() native module = %#v, want Tiny", result.Native)
	}
	if len(result.Native.VarDeclarations()) != 1 || len(result.Native.OperDeclarations()) != 2 {
		t.Fatalf("unexpected native declarations: %#v", result.Native.Declarations())
	}

	var root struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Modules []struct {
			Name string `json:"name"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(result.IR, &root); err != nil {
		t.Fatalf("unmarshal IR: %v\n%s", err, result.IR)
	}
	if root.Name != "ApalacheIR" || root.Version != "1.0" {
		t.Fatalf("unexpected IR header: %#v", root)
	}
	if len(root.Modules) != 1 || root.Modules[0].Name != "Tiny" {
		t.Fatalf("unexpected IR modules: %#v", root.Modules)
	}
}

func TestParseSourcePopulatesNativeSourceLocations(t *testing.T) {
	result := ParseSource(context.Background(), "Tiny.tla", tinyModule, FrontendOptions{IncludeSource: true})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	if result.Native == nil || result.Sources == nil {
		t.Fatalf("missing native frontend result: %#v", result)
	}
	init := result.Native.OperDeclarations()[0]
	location, found := result.Sources.Find(init.ID())
	if !found || location.Filename != "Tiny" || location.Region.Start.Line != 4 {
		t.Fatalf("Init source location = %#v, %v", location, found)
	}
	if _, found := result.Sources.Find(init.Body().ID()); !found {
		t.Fatal("Init body has no source location")
	}
}

func TestParseSourceAcceptsApalacheAssignmentSyntax(t *testing.T) {
	result := ParseSource(context.Background(), "Assignments.tla", `---- MODULE Assignments ----
EXTENDS Apalache
VARIABLE x
Init == x := 0
Next == x' := 1
====
`, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	for _, name := range []string{"Init", "Next"} {
		body, found := OperatorBody(result.Native, name)
		operation, ok := body.(*OperExpr)
		if !found || !ok || operation.Operator().Name() != "EQ" {
			t.Fatalf("%s assignment body = %#v", name, body)
		}
	}
}

func TestParseAndTypeCheckSourceRecognizesImportedApalacheBuiltins(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "Builtins.tla", `---- MODULE Builtins ----
EXTENDS Apalache, Integers
Inv == LET Op(a, i) == a + i IN Repeat(Op, 2, 0) = 3
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Inv")
	if !found || countOperator(body, "Apalache!Repeat") != 1 {
		t.Fatalf("normalized Repeat body = %#v", body)
	}
}

func TestParseAndTypeCheckSourcePassesOperatorArgumentsByName(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "Callback.tla", `---- MODULE Callback ----
EXTENDS Integers
VARIABLE \* @type: Int;
  x
Pick(Cb(_)) == \E i \in 1..10: Cb(i)
Init == LET Cb(i) == x = i IN Pick(Cb)
Next == UNCHANGED x
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
}

func TestParseAndTypeCheckSourcePassesLambdaOperatorArguments(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "LambdaCallback.tla", `---- MODULE LambdaCallback ----
EXTENDS Integers
\* @type: (Set(Int), (Int) => Bool) => Int;
ChooseOne(S, P(_)) == CHOOSE x \in S: P(x)
Value == ChooseOne({1, 2}, LAMBDA x: x = 2)
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
}

func TestParseAndTypeCheckSourceLowersTupleSetFilters(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "TupleFilter.tla", `---- MODULE TupleFilter ----
EXTENDS Integers
\* @type: Set(<<Int, Int>>) => Set(<<Int, Int>>);
Positive(S) == {<<x, y>> \in S: x > 0 /\ y > 0}
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Positive")
	if !found || countOperator(body, "SET_FILTER") != 1 {
		t.Fatalf("lowered tuple filter = %#v", body)
	}
}

func TestParseAndTypeCheckSourceLowersFunctionOverride(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "FunctionOverride.tla", `---- MODULE FunctionOverride ----
EXTENDS TLC, Integers
\* @type: Int -> Int;
F == [x \in {1, 2} |-> x]
\* @type: Int -> Int;
G == [x \in {2, 3} |-> 10 * x]
\* @type: Int -> Int;
H == F @@ G
\* @type: Int -> Int;
K == 4 :> 40
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "H")
	if !found || countOperator(body, "FUN_CTOR") != 1 || countOperator(body, "IF_THEN_ELSE") != 1 {
		t.Fatalf("lowered function override = %#v", body)
	}
	singleton, found := OperatorBody(result.Native, "K")
	if !found || countOperator(singleton, "FUN_CTOR") != 1 || countOperator(singleton, "SET_ENUM") != 1 {
		t.Fatalf("lowered singleton function = %#v", singleton)
	}
}

func TestParseAndTypeCheckSourceLowersBagInfixOperators(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "BagInfix.tla", `---- MODULE BagInfix ----
EXTENDS Bags
\* @type: Int -> Int;
A == [x \in {1, 2} |-> x]
\* @type: Int -> Int;
B == [x \in {2, 3} |-> 1]
Added == A (+) B
Removed == A (-) B
Contained == A \sqsubseteq B
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	for _, name := range []string{"Added", "Removed"} {
		body, found := OperatorBody(result.Native, name)
		if !found || countOperator(body, "FUN_CTOR") != 1 {
			t.Fatalf("lowered %s = %#v", name, body)
		}
	}
	if body, found := OperatorBody(result.Native, "Contained"); !found || countOperator(body, "SET_SUBSET_EQ") != 1 || countOperator(body, "FORALL3") != 1 {
		t.Fatalf("lowered Contained = %#v", body)
	}
}

func TestParseAndTypeCheckSourcePreservesNestedExceptAt(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "NestedExcept.tla", `---- MODULE NestedExcept ----
EXTENDS Integers
VARIABLE
  \* @type: Str -> { cnt: Int };
  f
Init == f = [s \in {"a"} |-> [cnt |-> 0]]
Next == f' = [f EXCEPT !["a"] = [@ EXCEPT !.cnt = @ + 1]]
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Next")
	if !found || countOperator(body, "EXCEPT") != 2 {
		t.Fatalf("nested EXCEPT body = %#v", body)
	}
}

func TestParseAndTypeCheckSourceRetainsRootLocalInstanceDefinitions(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "LocalInstance.tla", `---- MODULE LocalInstance ----
---- MODULE Helper ----
EXTENDS Integers
\* @type: Int => Int;
Inc(x) == x + 1
====
LOCAL INSTANCE Helper
Value == Inc(1)
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	if _, found := OperatorBody(result.Native, "Inc"); !found {
		t.Fatal("LOCAL INSTANCE definition Inc was omitted")
	}
}

func TestParseAndTypeCheckSourceWrapsBuiltinOperatorArguments(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "BuiltinArguments.tla", `---- MODULE BuiltinArguments ----
EXTENDS Integers
Apply(F(_, _), x, y) == F(x, y)
TestPlus == Apply(+, 1, 2) = 3
TestUnion == Apply(\union, {1}, {2}) = {1, 2}
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
}

func TestParseAndTypeCheckSourceExpandsFunAsSeq(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "FunctionSequence.tla", `---- MODULE FunctionSequence ----
EXTENDS Apalache, Integers
Test == FunAsSeq([i \in 1..3 |-> i + 1], 3, 3) = <<2, 3, 4>>
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Test")
	if !found || countOperator(body, "Apalache!MkSeq") != 1 || countOperator(body, "Sequences!SubSeq") != 1 {
		t.Fatalf("expanded FunAsSeq body = %#v", body)
	}
}

func TestParseAndTypeCheckSourceExpandsSelectSeq(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "SelectSequence.tla", `---- MODULE SelectSequence ----
EXTENDS Sequences, Integers
IsOdd(i) == i % 2 = 1
Test == SelectSeq(<<2, 3, 4>>, IsOdd) = <<3>>
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Test")
	if !found || countOperator(body, "Apalache!ApaFoldSeqLeft") != 1 {
		t.Fatalf("expanded SelectSeq body = %#v", body)
	}
}

func TestParseAndTypeCheckSourceImportsSequenceToSet(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "SequenceToSet.tla", `---- MODULE SequenceToSet ----
EXTENDS SequencesExt
Test == ToSet(<<1>>) = {1}
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Test")
	_, imported := OperatorBody(result.Native, "ToSet")
	if !found || !imported || countNamed(body, "ToSet") != 1 {
		t.Fatalf("imported ToSet body = %#v", body)
	}
}

func TestParseAndTypeCheckSourceImportsQualifiedAnnotationsAndAliases(t *testing.T) {
	directory := t.TempDir()
	library := `---- MODULE TypedLibrary ----
EXTENDS Integers
\* @typeAlias: PAIR = <<Int, Int>>;
TypeAliases == TRUE
\* @type: PAIR => Int;
First(pair) == pair[1]
====
`
	if err := os.WriteFile(filepath.Join(directory, "TypedLibrary.tla"), []byte(library), 0o600); err != nil {
		t.Fatal(err)
	}
	result := ParseAndTypeCheckSource(context.Background(), "QualifiedImport.tla", `---- MODULE QualifiedImport ----
L == INSTANCE TypedLibrary
Test == L!First(<<1, 2>>) = 1
====
`, FrontendOptions{LibraryPaths: []string{directory}}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "L!First")
	if !found {
		t.Fatalf("missing qualified imported definition in %#v", result.Native.Declarations())
	}
	if typ, typed := body.Tag().Type(); !typed || !EqualTypes(typ, IntT1) {
		t.Fatalf("qualified imported body type = %v, %v", typ, typed)
	}
}

func TestParseAndTypeCheckSourceImportsSetToSequence(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "SetToSequence.tla", `---- MODULE SetToSequence ----
EXTENDS Sequences, SequencesExt, Integers
Test == LET s == SetToSeq({1, 2}) IN Len(s) = 2 /\ ToSet(s) = {1, 2}
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Test")
	_, imported := OperatorBody(result.Native, "SetToSeq")
	if !found || !imported || countNamed(body, "SetToSeq") != 1 || countNamed(body, "ToSet") != 1 {
		t.Fatalf("imported SetToSeq body = %#v", body)
	}
}

func TestParseAndTypeCheckSourceReconstructsTupleQuantifier(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "TupleBound.tla", `---- MODULE TupleBound ----
EXTENDS Integers
Op == \E <<i, j>> \in (1..2) \X (3..4): i < j
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Op")
	quantifier, ok := body.(*OperExpr)
	if !found || !ok || quantifier.Operator().Name() != "EXISTS3" {
		t.Fatalf("tuple quantifier body = %#v", body)
	}
	binder, ok := quantifier.Arguments()[0].(*OperExpr)
	if !ok || binder.Operator().Name() != "TUPLE" || len(binder.Arguments()) != 2 {
		t.Fatalf("tuple quantifier binder = %#v", quantifier.Arguments()[0])
	}
}

func TestParseAndTypeCheckSourceReconstructsMixedTupleFunctionBinder(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "TupleFunction.tla", `---- MODULE TupleFunction ----
EXTENDS Integers
\* @type: <<Int, <<Int, Int>>>> -> Int;
F == [x \in {1, 2}, <<a, b>> \in {<<3, 4>>} |-> x + a + b]
Inv == F[2, <<3, 4>>] = 9
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "F")
	function, ok := body.(*OperExpr)
	if !found || !ok || function.Operator().Name() != "FUN_CTOR" || len(function.Arguments()) != 5 {
		t.Fatalf("mixed-binder function body = %#v", body)
	}
	binder, ok := function.Arguments()[3].(*OperExpr)
	if !ok || binder.Operator().Name() != "TUPLE" || len(binder.Arguments()) != 2 {
		t.Fatalf("tuple function binder = %#v", function.Arguments()[3])
	}
}

func TestParseSourceKeepsIndependentFunctionBindersSeparate(t *testing.T) {
	result := ParseSource(context.Background(), "IndependentFunction.tla", `---- MODULE IndependentFunction ----
EXTENDS Integers
F == [x, y \in {1, 2} |-> x + y]
====
`, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "F")
	function, ok := body.(*OperExpr)
	if !found || !ok || function.Operator().Name() != "FUN_CTOR" || len(function.Arguments()) != 5 {
		t.Fatalf("independent-binder function body = %#v", body)
	}
	if _, tuple := function.Arguments()[1].(*OperExpr); tuple {
		t.Fatalf("first independent binder was grouped: %#v", function.Arguments()[1])
	}
	if _, tuple := function.Arguments()[3].(*OperExpr); tuple {
		t.Fatalf("second independent binder was grouped: %#v", function.Arguments()[3])
	}
}

func TestParseAndTypeCheckSourceReconstructsTupleSetMapBinder(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "TupleSetMap.tla", `---- MODULE TupleSetMap ----
EXTENDS Integers
\* @type: Set(<<Int, Int>>) => Set(Int);
Project(S) == {x + y : <<x, y>> \in S}
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Project")
	setMap, ok := body.(*OperExpr)
	if !found || !ok || setMap.Operator().Name() != "SET_MAP" || len(setMap.Arguments()) != 3 {
		t.Fatalf("tuple set map body = %#v", body)
	}
	binder, ok := setMap.Arguments()[1].(*OperExpr)
	if !ok || binder.Operator().Name() != "TUPLE" || len(binder.Arguments()) != 2 {
		t.Fatalf("tuple set map binder = %#v", setMap.Arguments()[1])
	}
}

func TestParseSourceReportsSemanticErrors(t *testing.T) {
	result := ParseSource(context.Background(), "Bad.tla", `---- MODULE Bad ----
Op == missing
====
`, FrontendOptions{})
	if result.OK() {
		t.Fatal("ParseSource() succeeded, want semantic error")
	}
	if len(result.IR) != 0 {
		t.Fatalf("ParseSource() IR = %s, want none after error", result.IR)
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Severity != SeverityError {
		t.Fatalf("ParseSource() diagnostics = %#v", result.Diagnostics)
	}
}

func TestParseSourceHonorsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := ParseSource(ctx, "Tiny.tla", tinyModule, FrontendOptions{})
	if result.OK() || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "E-CANCELED" {
		t.Fatalf("ParseSource() = %#v, want cancellation", result)
	}
}

func TestParseFileReportsReadError(t *testing.T) {
	result := ParseFile(context.Background(), "testdata/does-not-exist.tla", FrontendOptions{})
	if result.OK() || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "E-IO" {
		t.Fatalf("ParseFile() = %#v, want I/O error", result)
	}
}

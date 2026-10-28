package apalache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestParseAndTypeCheckFileReadsCanonicalIRJSON(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "JSONInput.tla", `---- MODULE JSONInput ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
Inv == x >= 0
====`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("source diagnostics = %v", result.Diagnostics)
	}
	encoded, err := MarshalSingleModuleIRJSON(result.Native, IRJSONOptions{SourceStore: result.Sources})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "JSONInput.json")
	if err = os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded := ParseAndTypeCheckFile(context.Background(), path, FrontendOptions{}, TypeCheckOptions{})
	if !loaded.OK() || loaded.Native == nil || !loaded.Native.HasProperty(PropertyTypeChecked) {
		t.Fatalf("JSON diagnostics = %v, module = %#v", loaded.Diagnostics, loaded.Native)
	}
	if loaded.Module != "JSONInput" {
		t.Fatalf("JSON module = %q", loaded.Module)
	}
	badPath := filepath.Join(t.TempDir(), "Bad.json")
	if err = os.WriteFile(badPath, []byte(`{"version":"wrong","modules":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := ParseFile(context.Background(), badPath, FrontendOptions{})
	if bad.OK() || len(bad.Diagnostics) != 1 || bad.Diagnostics[0].Code != "E-IR-JSON" {
		t.Fatalf("bad JSON result = %+v", bad)
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

func TestParseSourcePreservesInstanceLookupOrder(t *testing.T) {
	result := ParseSource(context.Background(), "InstanceLookup.tla", `---- MODULE InstanceLookup ----
---- MODULE M ----
a == {}
b == a
====
a(self) == {self}
I == INSTANCE M
c == a(1)
====
`, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	instanceA, found := OperatorBody(result.Native, "I!a")
	if !found || countOperator(instanceA, "SET_ENUM") != 1 {
		t.Fatalf("I!a body = %#v", instanceA)
	}
	instanceB, found := OperatorBody(result.Native, "I!b")
	if !found || countNamed(instanceB, "I!a") != 1 || countNamed(instanceB, "a") != 0 {
		t.Fatalf("I!b body = %#v", instanceB)
	}
	rootC, found := OperatorBody(result.Native, "c")
	if !found || countNamed(rootC, "a") != 1 || countNamed(rootC, "I!a") != 0 {
		t.Fatalf("root c body = %#v", rootC)
	}
}

func TestParseSourceKeepsInstantiatedLocalOperatorScoped(t *testing.T) {
	for _, test := range []struct {
		name       string
		instance   string
		exported   string
		local      string
		rootAccess string
	}{
		{name: "named", instance: "I == INSTANCE M", exported: "I!b", local: "I!a", rootAccess: "I!b"},
		{name: "unqualified", instance: "INSTANCE M", exported: "b", local: "a", rootAccess: "b"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := fmt.Sprintf(`---- MODULE InstanceLocal ----
---- MODULE M ----
LOCAL a == {}
b == a
====
%s
c == %s
====
`, test.instance, test.rootAccess)
			result := ParseSource(context.Background(), "InstanceLocal.tla", source, FrontendOptions{IncludeSource: true})
			if !result.OK() {
				t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
			}
			if _, found := declarationNamed(result.Native, test.local); found {
				t.Fatalf("LOCAL instance operator %s leaked into root declarations", test.local)
			}
			declaration, found := declarationNamed(result.Native, test.exported)
			operator, ok := declaration.(*OperDecl)
			if !found || !ok {
				t.Fatalf("%s declaration = %#v; declarations = %v", test.exported, declaration, declarationNames(result.Native))
			}
			let, ok := operator.Body().(*LetInExpr)
			if !ok || len(let.Declarations()) != 1 || countOperator(let.Declarations()[0].Body(), "SET_ENUM") != 1 {
				t.Fatalf("%s local body = %#v; declarations = %v", test.exported, operator.Body(), declarationNames(result.Native))
			}
			if result.Sources == nil {
				t.Fatal("instance parse omitted source store")
			}
			if _, found := result.Sources.Find(let.Body().ID()); !found {
				t.Fatal("instance LET body has no source location")
			}
			if _, found := result.Sources.Find(let.Declarations()[0].Body().ID()); !found {
				t.Fatal("instance LOCAL definition has no source location")
			}
		})
	}
}

func TestParseAndTypeCheckSourceRetainsInstantiatedLocalOperatorAnnotation(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "InstanceLocalTyped.tla", `---- MODULE InstanceLocalTyped ----
---- MODULE M ----
\* @type: Set(Int);
LOCAL a == {}
\* @type: Set(Int);
b == a
====
I == INSTANCE M
Test == I!b = {}
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	declaration, found := declarationNamed(result.Native, "I!b")
	operator, ok := declaration.(*OperDecl)
	if !found || !ok {
		t.Fatalf("I!b declaration = %#v", declaration)
	}
	let, ok := operator.Body().(*LetInExpr)
	if !ok || len(let.Declarations()) != 1 {
		t.Fatalf("typed I!b body = %#v", operator.Body())
	}
	localType, typed := let.Declarations()[0].Tag().Type()
	if !typed || !EqualTypes(localType, mustOperator(nil, mustSet(IntT1))) {
		t.Fatalf("typed instance LOCAL declaration = %v, %v", localType, typed)
	}
}

func TestParseSourceScopesLocalsForRepeatedInstances(t *testing.T) {
	result := ParseSource(context.Background(), "RepeatedLocalInstances.tla", `---- MODULE RepeatedLocalInstances ----
---- MODULE M ----
LOCAL a == {}
b == a
====
I == INSTANCE M
J == INSTANCE M
Both == I!b = J!b
====
`, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	for _, prefix := range []string{"I!", "J!"} {
		if _, found := declarationNamed(result.Native, prefix+"a"); found {
			t.Fatalf("%sa leaked into root declarations", prefix)
		}
		body, found := OperatorBody(result.Native, prefix+"b")
		let, ok := body.(*LetInExpr)
		if !found || !ok || len(let.Declarations()) != 1 || let.Declarations()[0].Name() != "LOCAL$"+prefix+"a" {
			t.Fatalf("%sb body = %#v", prefix, body)
		}
	}
}

func TestParseSourceResolvesNestedInstanceSubstitutions(t *testing.T) {
	result := ParseSource(context.Background(), "NestedSubstitution.tla", `---- MODULE NestedSubstitution ----
---- MODULE Vot ----
---- MODULE Cons ----
VARIABLE chosen
Init == chosen = {}
Next == chosen = {2}
====
chosen == {2}
C == INSTANCE Cons
====
V == INSTANCE Vot
====
`, FrontendOptions{IncludeSource: true})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "V!C!Next")
	if !found || countOperator(body, "EQ") != 1 || countOperator(body, "SET_ENUM") != 1 ||
		countNamed(body, "V!chosen") != 1 || countNamed(body, "chosen") != 0 {
		t.Fatalf("V!C!Next body = %#v; declarations = %v", body, declarationNames(result.Native))
	}
	if _, found := result.Sources.Find(body.ID()); !found {
		t.Fatal("nested substituted operator has no source location")
	}
}

func TestParseSourceExportsNestedInstanceDefinitions(t *testing.T) {
	result := ParseSource(context.Background(), "NestedInstance.tla", `---- MODULE NestedInstance ----
---- MODULE B ----
---- MODULE C ----
H(x) == x
====
C2 == INSTANCE C
F(x) == C2!H(x)
====
B2 == INSTANCE B
====
`, FrontendOptions{IncludeSource: true})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "B2!C2!H")
	if !found || countNamed(body, "x") != 1 {
		t.Fatalf("B2!C2!H body = %#v; declarations = %v", body, declarationNames(result.Native))
	}
	declaration, found := declarationNamed(result.Native, "B2!C2!H")
	operator, ok := declaration.(*OperDecl)
	if !found || !ok || len(operator.FormalParams()) != 1 || operator.FormalParams()[0].Name != "x" {
		t.Fatalf("B2!C2!H declaration = %#v", declaration)
	}
	if _, found := result.Sources.Find(body.ID()); !found {
		t.Fatal("nested instance operator has no source location")
	}
}

func TestParseSourceQualifiesHigherOrderInstanceArguments(t *testing.T) {
	result := ParseSource(context.Background(), "HigherOrderInstance.tla", `---- MODULE HigherOrderInstance ----
---- MODULE Base ----
FF(a) == TRUE
Check(a, func(_)) == func(a)
B == Check(1, FF)
====
BASE == INSTANCE Base
====
`, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "BASE!B")
	if !found || countNamed(body, "BASE!Check") != 1 || countNamed(body, "BASE!FF") != 1 ||
		countNamed(body, "Check") != 0 || countNamed(body, "FF") != 0 {
		t.Fatalf("BASE!B body = %#v", body)
	}
}

func TestParseSourceComposesSeriesOfInstanceSubstitutions(t *testing.T) {
	result := ParseSource(context.Background(), "SeriesSubstitution.tla", `---- MODULE SeriesSubstitution ----
---- MODULE B ----
---- MODULE C ----
VARIABLE x
magic == x /= 2
====
VARIABLE y
C1 == INSTANCE C WITH x <- y
====
VARIABLE z
B1 == INSTANCE B WITH y <- z
====
`, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "B1!C1!magic")
	if !found || countOperator(body, "NE") != 1 || countNamed(body, "z") != 1 ||
		countNamed(body, "x") != 0 || countNamed(body, "y") != 0 {
		t.Fatalf("B1!C1!magic body = %#v; declarations = %v", body, declarationNames(result.Native))
	}
}

func TestParseSourceNormalizesStandardModuleInstances(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		operator string
		value    Value
	}{
		{
			name: "named sequence instance",
			source: `---- MODULE StandardInstance ----
I == INSTANCE Sequences
A == I!Append(<<>>, {})
====`,
			operator: "Sequences!Append",
		},
		{
			name: "local sequence instance",
			source: `---- MODULE StandardInstance ----
---- MODULE A ----
LOCAL INSTANCE Sequences
A == Append(<<>>, {})
====
INSTANCE A
====`,
			operator: "Sequences!Append",
		},
		{
			name: "local integer value instance",
			source: `---- MODULE StandardInstance ----
---- MODULE A ----
LOCAL INSTANCE Integers
A == Int
====
INSTANCE A
====`,
			value: IntSet,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ParseSource(context.Background(), "StandardInstance.tla", test.source, FrontendOptions{})
			if !result.OK() {
				t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
			}
			body, found := OperatorBody(result.Native, "A")
			if !found {
				t.Fatalf("missing A in %v", declarationNames(result.Native))
			}
			if test.operator != "" && countOperator(body, test.operator) != 1 {
				formatted, _ := FormatTLAExpression(body)
				var callee string
				if application, ok := body.(*OperExpr); ok && len(application.Arguments()) > 0 {
					if name, ok := application.Arguments()[0].(*NameExpr); ok {
						callee = name.Name()
					}
				}
				t.Fatalf("A body = %s (%#v), callee %q, want %s", formatted, body, callee, test.operator)
			}
			if test.name == "named sequence instance" {
				for _, name := range declarationNames(result.Native) {
					if strings.HasPrefix(name, "I!") {
						t.Fatalf("standard-module stub leaked into declarations: %s", name)
					}
				}
			}
			if test.value != nil {
				literal, ok := body.(*ValueExpr)
				if !ok || literal.Value() != test.value {
					t.Fatalf("A body = %#v, want %v", body, test.value)
				}
			}
		})
	}
}

func TestParseSourceNormalizesStandardModules(t *testing.T) {
	result := ParseSource(context.Background(), "StandardModules.tla", `---- MODULE StandardModules ----
EXTENDS Reals, Sequences, FiniteSets
NatSet == Nat
IntSet == Int
RealSetValue == Real
InfinityValue == Infinity
Plus == 3 + 2
Minus == 3 - 2
UnaryMinus == -2
Multiply == 3 * 2
Power == 3 ^ 2
Less == 3 < 2
Greater == 3 > 2
AtMost == 3 <= 2
AtLeast == 3 >= 2
Modulo == 3 % 2
Divide == 3 \div 2
RealDivide == 3 / 2
Range == 2 .. 3
SequenceSet == Seq({1, 2})
Length == Len(<<1, 2>>)
Concatenate == <<1>> \o <<2>>
AppendValue == Append(<<1>>, 2)
HeadValue == Head(<<1, 2>>)
TailValue == Tail(<<1, 2>>)
Slice == SubSeq(<<1, 2, 3>>, 2, 3)
Finite == IsFiniteSet(BOOLEAN)
CardinalityValue == Cardinality(BOOLEAN)
====
`, FrontendOptions{IncludeSource: true})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	values := map[string]Value{
		"NatSet": NatSet, "IntSet": IntSet, "RealSetValue": RealSet, "InfinityValue": RealInfinity(),
	}
	for name, want := range values {
		body, found := OperatorBody(result.Native, name)
		literal, ok := body.(*ValueExpr)
		if !found || !ok || !EqualValues(literal.Value(), want) {
			t.Errorf("%s body = %#v, want %v", name, body, want)
		}
	}
	operators := map[string]string{
		"Plus": "PLUS", "Minus": "MINUS", "UnaryMinus": "UNARY_MINUS", "Multiply": "MULT",
		"Power": "POW", "Less": "LT", "Greater": "GT", "AtMost": "LE", "AtLeast": "GE",
		"Modulo": "MOD", "Divide": "DIV", "RealDivide": "REAL_DIV", "Range": "INT_RANGE",
		"SequenceSet": "Sequences!Seq", "Length": "Sequences!Len", "Concatenate": "Sequences!Concat",
		"AppendValue": "Sequences!Append", "HeadValue": "Sequences!Head", "TailValue": "Sequences!Tail",
		"Slice": "Sequences!SubSeq", "Finite": "FiniteSets!IsFiniteSet",
		"CardinalityValue": "FiniteSets!Cardinality",
	}
	for name, want := range operators {
		body, found := OperatorBody(result.Native, name)
		if !found || countOperator(body, want) != 1 {
			t.Errorf("%s body = %#v, want %s", name, body, want)
		}
	}
	if got, want := len(result.Native.Declarations()), len(values)+len(operators); got != want {
		t.Fatalf("standard-module declarations = %v (count %d), want only %d user definitions",
			declarationNames(result.Native), got, want)
	}
	plus, _ := OperatorBody(result.Native, "Plus")
	if location, found := result.Sources.Find(plus.ID()); !found || location.Region.Start.Line != 7 {
		t.Fatalf("Plus source location = %#v, %v", location, found)
	}
}

func TestParseSourceFiltersStandardModuleDefinitions(t *testing.T) {
	result := ParseSource(context.Background(), "EmptyStandardImport.tla", `---- MODULE EmptyStandardImport ----
EXTENDS Naturals
====
`, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	if declarations := result.Native.Declarations(); len(declarations) != 0 {
		t.Fatalf("standard definitions leaked into root: %v", declarationNames(result.Native))
	}
}

func TestParseSourceNormalizesExtendedLibraryOperators(t *testing.T) {
	result := ParseSource(context.Background(), "ExtendedLibraries.tla", `---- MODULE ExtendedLibraries ----
EXTENDS TLC, Apalache, Variants, Integers, FiniteSets
VARIABLE x, S
Assigned == x' := 1
Skolemized == Skolem(\E y \in S: TRUE)
Guessed == Guess(S)
Expanded == Expand(SUBSET S)
ConstantCardinality == ConstCardinality(Cardinality(S) >= 2)
Identity(i) == i
GeneratedSequence == MkSeq(2, Identity)
Printed == Print("message", TRUE)
PrintedT == PrintT("message")
Asserted == Assert(TRUE, "message")
Evaluated == TLCEval(42)
Stringified == ToString(42)
VariantValue == Variant("A", [value |-> 1])
VariantSet == VariantFilter("A", {VariantValue})
VariantLabel(v) == VariantTag(v)
VariantUnsafe(v) == VariantGetUnsafe("A", v)
VariantDefault(v) == VariantGetOrElse("A", v, [value |-> 0])
====
`, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	operators := map[string]string{
		"Assigned": "EQ", "Skolemized": "Apalache!Skolem", "Guessed": "Apalache!Guess",
		"Expanded": "Apalache!Expand", "ConstantCardinality": "Apalache!ConstCardinality",
		"GeneratedSequence": "Apalache!MkSeq", "Stringified": "TLC!ToString",
		"VariantValue": "Variants!Variant", "VariantSet": "Variants!VariantFilter",
		"VariantLabel": "Variants!VariantTag", "VariantUnsafe": "Variants!VariantGetUnsafe",
		"VariantDefault": "Variants!VariantGetOrElse",
	}
	for name, want := range operators {
		body, found := OperatorBody(result.Native, name)
		if !found || countOperator(body, want) != 1 {
			t.Errorf("%s body = %#v, want %s", name, body, want)
		}
	}
	values := map[string]Value{
		"Printed": BoolValue(true), "PrintedT": BoolValue(true), "Asserted": BoolValue(true),
		"Evaluated": Int64Value(42),
	}
	for name, want := range values {
		body, found := OperatorBody(result.Native, name)
		literal, ok := body.(*ValueExpr)
		if !found || !ok || !EqualValues(literal.Value(), want) {
			t.Errorf("%s body = %#v, want %v", name, body, want)
		}
	}
}

func TestParseSourceNormalizesTLCHelpers(t *testing.T) {
	result := ParseSource(context.Background(), "TLCHelpers.tla", `---- MODULE TLCHelpers ----
EXTENDS TLC
VARIABLE i
Clock == JavaTime
Read == TLCGet(i)
Write == TLCSet(i, 3)
Singleton == {1, 2} :> 3
Override == [j \in {1, 2} |-> j] @@ [k \in {3, 4} |-> k]
Permuted == Permutations(<<1, 2>>)
Less(a, b) == TRUE
Sorted == SortSeq(<<2, 1>>, Less)
Random == RandomElement({1, 2})
Arbitrary == Any
====
`, FrontendOptions{IncludeSource: true})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	clock, found := OperatorBody(result.Native, "Clock")
	clockValue, ok := clock.(*ValueExpr)
	if !found || !ok || !EqualValues(clockValue.Value(), Int64Value(123)) {
		t.Errorf("Clock body = %#v, want JavaTime compatibility value", clock)
	}
	write, found := OperatorBody(result.Native, "Write")
	writeValue, ok := write.(*ValueExpr)
	if !found || !ok || !EqualValues(writeValue.Value(), BoolValue(true)) {
		t.Errorf("Write body = %#v, want TRUE", write)
	}
	operators := map[string]string{
		"Read": "TLC!TLCGet", "Singleton": "FUN_CTOR", "Override": "FUN_CTOR",
		"Random": "Apalache!Guess",
	}
	for name, want := range operators {
		body, found := OperatorBody(result.Native, name)
		if !found || countOperator(body, want) == 0 {
			t.Errorf("%s body = %#v, want %s", name, body, want)
		}
	}
	for name, callee := range map[string]string{
		"Permuted": "Permutations", "Sorted": "SortSeq", "Arbitrary": "Any",
	} {
		body, found := OperatorBody(result.Native, name)
		if !found || countNamed(body, callee) != 1 {
			t.Errorf("%s body = %#v, want call to %s", name, body, callee)
		}
		if _, sourceFound := result.Sources.Find(body.ID()); !sourceFound {
			t.Errorf("%s body has no source location", name)
		}
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

func TestParseAndTypeCheckSourceRecognizesQualifiedApalacheBuiltins(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "QualifiedApalache.tla", `---- MODULE QualifiedApalache ----
EXTENDS Integers
A == INSTANCE Apalache
Test == A!SetAsFun({<<"a", 1>>, <<"b", 2>>})["a"] = 1
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		declarations := []string(nil)
		if result.Native != nil {
			declarations = declarationNames(result.Native)
		}
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v; declarations = %v", result.Diagnostics, declarations)
	}
	body, found := OperatorBody(result.Native, "Test")
	if !found || countOperator(body, "Apalache!SetAsFun") != 1 || countNamed(body, "A!SetAsFun") != 0 {
		t.Fatalf("qualified SetAsFun body = %#v", body)
	}
}

func TestParseAndTypeCheckSourceInfersUnannotatedLocalResult(t *testing.T) {
	source := `---- MODULE LocalResult ----
EXTENDS Integers, Sequences
\* @type: ((Int, Int) => Bool, Seq(Int), Int) => Seq(Int);
Outer(LessThan(_, _), seq, newElement) ==
  LET nextLen == Len(seq) + 1 IN
  LET first == 1 IN
  SubSeq(seq, first, nextLen)
====
`
	result := ParseAndTypeCheckSource(context.Background(), "LocalResult.tla", source, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
}

func TestParseAndTypeCheckSourceAccessesAnnotatedRecordFields(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "RecordFields.tla", `---- MODULE RecordFields ----
VARIABLE
  \* @type: { val: DATUM, rdy: Int, ack: Int };
  chan
Test == chan.rdy = chan.ack
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
}

func TestParseAndTypeCheckSourceRetainsInstantiatedLocalAnnotations(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "NestedLocalType.tla", `---- MODULE NestedLocalType ----
---- MODULE Utils ----
VARIABLE myArr
Foo(f) ==
  LET \* @type: (Int -> Int) => Int;
      X(g) == g[1]
  IN X(f)
====
VARIABLE
  \* @type: Int -> Int;
  arr
U == INSTANCE Utils WITH myArr <- arr
Test == U!Foo(arr) = arr[1]
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
}

func TestParseAndTypeCheckSourcePropagatesExpandedSymbolicOperatorAnnotation(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "ExpandedSymbolic.tla", `---- MODULE ExpandedSymbolic ----
EXTENDS Integers
\* @type: (a -> Int, a -> Int) => a -> Int;
F1 (+) F2 ==
  [e \in DOMAIN F1 \union DOMAIN F2 |->
    IF e \in DOMAIN F1 THEN F1[e] ELSE F2[e]]
\* @type: (a -> Int, a -> Int) => a -> Int;
Use(F1, F2) ==
  LET NotInfix(A, B) == A (+) B
  IN NotInfix(F1, F2)
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
}

func TestParseAndTypeCheckSourceQualifiesInstantiatedRecursiveCalls(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "InstantiatedRecursion.tla", `---- MODULE InstantiatedRecursion ----
EXTENDS Integers
---- MODULE Inner ----
RECURSIVE A(_)
A(x) == IF x < 1 THEN x ELSE 1 + A(x - 1)
====
I == INSTANCE Inner
Test == I!A(2) = 2
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	declaration, found := declarationNamed(result.Native, "I!A")
	operator, ok := declaration.(*OperDecl)
	if !found || !ok || !operator.IsRecursive() {
		t.Fatalf("I!A declaration = %#v", declaration)
	}
	if countNamed(operator.Body(), "I!A") != 1 || countNamed(operator.Body(), "A") != 0 {
		t.Fatalf("I!A recursive body = %#v", operator.Body())
	}
}

func TestParseAndTypeCheckSourceQualifiesInstantiatedOperatorCalls(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "InstantiatedCalls.tla", `---- MODULE InstantiatedCalls ----
EXTENDS Integers
---- MODULE Inner ----
F(x) == x + 1
G(x) == F(x) + 1
====
I == INSTANCE Inner
Test == I!G(1) = 3
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	declaration, found := declarationNamed(result.Native, "I!G")
	operator, ok := declaration.(*OperDecl)
	if !found || !ok || countNamed(operator.Body(), "I!F") != 1 || countNamed(operator.Body(), "F") != 0 {
		t.Fatalf("I!G declaration = %#v", declaration)
	}
}

func TestParseSourceClearsImportedLocalResultTags(t *testing.T) {
	source := `---- MODULE ImportedSequencesExt ----
EXTENDS SequencesExt
Test == TRUE
====
`
	result := ParseSource(context.Background(), "ImportedSequencesExt.tla", source, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("ParseSource() diagnostics = %v", result.Diagnostics)
	}
	insertSorted, found := declarationNamed(result.Native, "__InsertSorted")
	if !found {
		t.Fatalf("missing __InsertSorted in %v", declarationNames(result.Native))
	}
	operator, ok := insertSorted.(*OperDecl)
	if !ok {
		t.Fatalf("__InsertSorted declaration = %T", insertSorted)
	}
	if got := operator.FormalParams(); len(got) != 3 {
		t.Fatalf("__InsertSorted parameters = %v", got)
	}
	body := operator.Body()
	if _, topLevel := declarationNamed(result.Native, "nextLen"); topLevel {
		t.Fatalf("nextLen leaked to top-level declarations %v", declarationNames(result.Native))
	}
	var locals []*OperDecl
	RewriteExpressions(nil, func(expression Expr) (Expr, bool) {
		if let, ok := expression.(*LetInExpr); ok {
			locals = append(locals, let.Declarations()...)
		}
		return nil, false
	})(body)
	foundNextLen := 0
	for _, local := range locals {
		if local.Name() == "nextLen" {
			foundNextLen++
			if typ, typed := local.Tag().Type(); typed {
				t.Fatalf("nextLen retained frontend tag %s", typ)
			}
			if typ, typed := local.Body().Tag().Type(); typed {
				t.Fatalf("nextLen body retained frontend tag %s in %#v", typ, local.Body())
			}
			annotations := ExtractTypeAnnotations("ImportedSequencesExt.tla", source, result.Native, result.Sources)
			mergeTlagoImportedAnnotations(&annotations, result.Native, result.imported)
			if typ, annotated := annotations.Types[local.ID()]; annotated {
				t.Fatalf("nextLen inherited imported annotation %s", typ)
			}
		}
	}
	if foundNextLen == 0 {
		t.Fatalf("nextLen not found among local declarations %v", locals)
	}
}

func TestParseAndTypeCheckSourceImportsVariableAndConstantAnnotations(t *testing.T) {
	directory := t.TempDir()
	library := `---- MODULE TypedState ----
VARIABLE
\* @type: Int;
value
CONSTANT
\* @type: Set(Str);
Names
====
`
	if err := os.WriteFile(filepath.Join(directory, "TypedState.tla"), []byte(library), 0o600); err != nil {
		t.Fatal(err)
	}
	result := ParseAndTypeCheckSource(context.Background(), "ImportedState.tla", `---- MODULE ImportedState ----
EXTENDS TypedState, Integers
Test == value \in Int /\ Names \subseteq {"a", "b"}
====
`, FrontendOptions{LibraryPaths: []string{directory}}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	variables := result.Native.VarDeclarations()
	constants := result.Native.ConstDeclarations()
	if len(variables) != 1 || len(constants) != 1 {
		t.Fatalf("imported declarations: variables=%d constants=%d", len(variables), len(constants))
	}
	variableType, variableTyped := variables[0].Tag().Type()
	constantType, constantTyped := constants[0].Tag().Type()
	if !variableTyped || !constantTyped || !EqualTypes(variableType, IntT1) ||
		!EqualTypes(constantType, mustSet(StrT1)) {
		t.Fatalf("imported declaration types: variable=%v constant=%v", variableType, constantType)
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

func TestParseAndTypeCheckSourceInlinesNamedAssumptionReferences(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "NamedAssumption.tla", `---- MODULE NamedAssumption ----
EXTENDS Integers
VARIABLE
\* @type: Int;
x
ASSUME Positive == 1 > 0
Init == x = 0 /\ Positive
Next == x' = x
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Init")
	if !found || countNamed(body, "Positive") != 0 || countOperator(body, "GT") != 1 {
		t.Fatalf("inlined named assumption body = %#v", body)
	}
}

func TestParseAndTypeCheckSourcePreservesLocalOperatorValues(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "LocalOperatorValue.tla", `---- MODULE LocalOperatorValue ----
EXTENDS Integers
Test ==
  LET
    \* @type: ((Int) => Int) => Int;
    Apply(F(_)) == F(1)
  IN LET Increment(x) == x + 1
  IN Apply(Increment) = 2
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Test")
	if !found || countNamed(body, "Increment") != 1 {
		t.Fatalf("local operator value body = %#v", body)
	}
}

func TestParseAndTypeCheckSourceReconstructsRecursiveFunction(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "RecursiveFunction.tla", `---- MODULE RecursiveFunction ----
EXTENDS Integers
Fact[k \in 1..4] == IF k <= 1 THEN 1 ELSE k * Fact[k - 1]
Test == Fact[3] = 6
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !result.OK() {
		t.Fatalf("ParseAndTypeCheckSource() diagnostics = %v", result.Diagnostics)
	}
	body, found := OperatorBody(result.Native, "Fact")
	if !found || countOperator(body, "FUN_REC_CTOR") != 1 || countOperator(body, "FUN_REC_REF") != 1 {
		t.Fatalf("recursive function body = %#v", body)
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

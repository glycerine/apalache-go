package apalache

import (
	"context"
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestIRJSONRoundTripEveryNodeAndValue(t *testing.T) {
	large, ok := new(big.Int).SetString("123456789012345678901234567890", 10)
	if !ok {
		t.Fatal("bad test integer")
	}
	decimal, err := NewDecimalValue("12.375")
	if err != nil {
		t.Fatal(err)
	}
	values := []Expr{
		NewValueExpr(StringValue("hello"), Typed(StrT1)),
		NewValueExpr(BoolValue(true), Typed(BoolT1)),
		NewValueExpr(NewIntValue(large), Typed(IntT1)),
		NewValueExpr(decimal, Typed(RealT1)),
		NewValueExpr(BoolSet, Untyped()), NewValueExpr(IntSet, Untyped()),
		NewValueExpr(StringSet, Untyped()), NewValueExpr(NatSet, Untyped()),
	}
	set := MustOperExpr(Op("SET_ENUM"), Untyped(), values...)
	local := NewOperDecl("Local", []OperatorParam{ValueParam("p"), {Name: "F", Arity: 1}},
		NewNameExpr("p", Typed(IntT1)), Untyped())
	local.SetRecursive(true)
	letIn := NewLetInExpr(set, Untyped(), local)
	operator := NewOperDecl("Values", nil, letIn, Untyped())
	assumption := NewNamedAssumeDecl("Okay", NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(BoolT1))
	theorem := NewTheoremDecl("Truth", NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(BoolT1))
	module := NewModule("Everything", []Decl{
		NewConstDecl("N", Typed(IntT1)), NewVarDecl("x", Typed(IntT1)), operator, assumption, theorem,
	})

	encoded, err := MarshalSingleModuleIRJSON(module, IRJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := UnmarshalSingleModuleIRJSON(encoded, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertModulesEqual(t, module, decoded)
	decodedLocal := decoded.OperDeclarations()[0].Body().(*LetInExpr).Declarations()[0]
	if !decodedLocal.IsRecursive() || !reflect.DeepEqual(decodedLocal.FormalParams(), local.FormalParams()) {
		t.Fatalf("local declaration metadata changed: %+v", decodedLocal)
	}
}

func TestIRJSONMatchesScalaTlaToUJsonCases(t *testing.T) {
	encodeExpr := func(t *testing.T, expression Expr) map[string]any {
		t.Helper()
		encoded, err := encodeIRExpression(expression, IRJSONOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	encodeDecl := func(t *testing.T, declaration Decl) map[string]any {
		t.Helper()
		encoded, err := encodeIRDeclaration(declaration, IRJSONOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	integer := func(value int64) Expr { return NewValueExpr(Int64Value(value), Untyped()) }
	stringValue := func(value string) Expr { return NewValueExpr(StringValue(value), Untyped()) }

	t.Run("basic values", func(t *testing.T) {
		cases := []struct {
			expression Expr
			kind       string
			value      any
		}{
			{integer(5), "TlaInt", int64(5)},
			{stringValue("abc"), "TlaStr", "abc"},
			{NewValueExpr(BoolValue(true), Untyped()), "TlaBool", true},
		}
		for _, test := range cases {
			encoded := encodeExpr(t, test.expression)
			value := encoded["value"].(map[string]any)
			if encoded["kind"] != "ValEx" || value["kind"] != test.kind || value["value"] != test.value {
				t.Fatalf("encoded value = %#v", encoded)
			}
		}
	})

	t.Run("collections", func(t *testing.T) {
		set := encodeExpr(t, MustOperExpr(Op("SET_ENUM"), Untyped(), integer(1), integer(2), integer(3)))
		tuple := encodeExpr(t, MustOperExpr(Op("TUPLE"), Untyped(), stringValue("x"), stringValue("y"), stringValue("z")))
		setArgs := set["args"].([]any)
		tupleArgs := tuple["args"].([]any)
		if set["kind"] != "OperEx" || set["oper"] != "SET_ENUM" || len(setArgs) != 3 ||
			!reflect.DeepEqual(setArgs[1], encodeExpr(t, integer(2))) {
			t.Fatalf("encoded set = %#v", set)
		}
		if tuple["kind"] != "OperEx" || tuple["oper"] != "TUPLE" || len(tupleArgs) != 3 ||
			!reflect.DeepEqual(tupleArgs[2], encodeExpr(t, stringValue("z"))) {
			t.Fatalf("encoded tuple = %#v", tuple)
		}
	})

	t.Run("let in", func(t *testing.T) {
		p := NewNameExpr("p", Untyped())
		declaration := NewOperDecl("A", []OperatorParam{ValueParam("p")},
			MustOperExpr(Op("PLUS"), Untyped(), p, integer(1)), Untyped())
		application, err := ApplyDeclarationUntyped(declaration, integer(0))
		if err != nil {
			t.Fatal(err)
		}
		encoded := encodeExpr(t, NewLetInExpr(application, Untyped(), declaration))
		declarations := encoded["decls"].([]any)
		if encoded["kind"] != "LetInEx" || !reflect.DeepEqual(encoded["body"], encodeExpr(t, application)) ||
			len(declarations) != 1 || !reflect.DeepEqual(declarations[0], encodeDecl(t, declaration)) {
			t.Fatalf("encoded LET-IN = %#v", encoded)
		}
	})

	t.Run("non operator declarations", func(t *testing.T) {
		body := MustOperExpr(Op("EQ"), Untyped(), NewNameExpr("x", Untyped()), NewValueExpr(BoolValue(true), Untyped()))
		constJSON := encodeDecl(t, NewConstDecl("C", Untyped()))
		varJSON := encodeDecl(t, NewVarDecl("x", Untyped()))
		namedJSON := encodeDecl(t, NewNamedAssumeDecl("myAssume", body, Untyped()))
		unnamedJSON := encodeDecl(t, NewUnnamedAssumeDecl(body, Untyped()))
		if constJSON["kind"] != "TlaConstDecl" || constJSON["name"] != "C" ||
			varJSON["kind"] != "TlaVarDecl" || varJSON["name"] != "x" ||
			namedJSON["kind"] != "TlaAssumeDecl" || namedJSON["name"] != "myAssume" ||
			unnamedJSON["kind"] != "TlaAssumeDecl" {
			t.Fatalf("encoded declarations = %#v %#v %#v %#v", constJSON, varJSON, namedJSON, unnamedJSON)
		}
		if _, present := unnamedJSON["name"]; present {
			t.Fatalf("unnamed assumption has name: %#v", unnamedJSON)
		}
	})

	t.Run("operator declarations", func(t *testing.T) {
		declarations := []*OperDecl{
			NewOperDecl("T", nil, integer(1), Untyped()),
			NewOperDecl("T", []OperatorParam{ValueParam("p")}, NewNameExpr("p", Untyped()), Untyped()),
			NewOperDecl("T", []OperatorParam{{Name: "A", Arity: 1}, ValueParam("b")}, NewNameExpr("b", Untyped()), Untyped()),
			NewOperDecl("T", []OperatorParam{ValueParam("p")}, NewNameExpr("p", Untyped()), Untyped()),
		}
		declarations[3].SetRecursive(true)
		for index, declaration := range declarations {
			encoded := encodeDecl(t, declaration)
			parameters := encoded["formalParams"].([]map[string]any)
			if encoded["kind"] != "TlaOperDecl" || encoded["name"] != "T" || len(parameters) != len(declaration.FormalParams()) ||
				encoded["isRecursive"] != (index == 3) {
				t.Fatalf("encoded operator %d = %#v", index, encoded)
			}
			for parameterIndex, parameter := range declaration.FormalParams() {
				if parameters[parameterIndex]["kind"] != "OperParam" || parameters[parameterIndex]["name"] != parameter.Name ||
					parameters[parameterIndex]["arity"] != parameter.Arity {
					t.Fatalf("encoded parameter %d/%d = %#v", index, parameterIndex, parameters[parameterIndex])
				}
			}
		}
	})
}

func TestIRJSONSourceRoundTrip(t *testing.T) {
	store := NewSourceStore()
	name := NewNameExpr("x", Untyped())
	operator := NewOperDecl("X", nil, name, Untyped())
	module := NewModule("M", []Decl{operator})
	nameLocation := SourceLocation{Filename: "M", Region: NewSourceRegion(2, 6, 2, 6)}
	declLocation := SourceLocation{Filename: "M", Region: NewSourceRegion(2, 1, 2, 6)}
	if err := store.Add(name.ID(), nameLocation); err != nil {
		t.Fatal(err)
	}
	if err := store.Add(operator.ID(), declLocation); err != nil {
		t.Fatal(err)
	}
	encoded, err := MarshalSingleModuleIRJSON(module, IRJSONOptions{SourceStore: store})
	if err != nil {
		t.Fatal(err)
	}
	decodedStore := NewSourceStore()
	decoded, err := UnmarshalSingleModuleIRJSON(encoded, decodedStore)
	if err != nil {
		t.Fatal(err)
	}
	decodedOperator := decoded.OperDeclarations()[0]
	if got, found := decodedStore.Find(decodedOperator.ID()); !found || got != declLocation {
		t.Fatalf("decl source = %v, %v", got, found)
	}
	if got, found := decodedStore.Find(decodedOperator.Body().ID()); !found || got != nameLocation {
		t.Fatalf("name source = %v, %v", got, found)
	}
}

func TestIRJSONReadsTlagoCanonicalExport(t *testing.T) {
	result := ParseSource(context.Background(), "Simple.tla", `---- MODULE Simple ----
CONSTANT N
VARIABLE x
Init == x = N
ASSUME N = N
====`, FrontendOptions{IncludeSource: true})
	if !result.OK() {
		t.Fatalf("frontend diagnostics: %+v", result.Diagnostics)
	}
	store := NewSourceStore()
	module, err := UnmarshalSingleModuleIRJSON(result.IR, store)
	if err != nil {
		t.Fatal(err)
	}
	if module.Name() != "Simple" || len(module.Declarations()) != 4 || len(store.SourceMap()) == 0 {
		t.Fatalf("decoded tlago module %q with %d declarations and %d sources", module.Name(), len(module.Declarations()), len(store.SourceMap()))
	}
	reencoded, err := MarshalSingleModuleIRJSON(module, IRJSONOptions{SourceStore: store})
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, result.IR, reencoded)
}

func TestIRJSONRejectsBadVersionKindAndOperator(t *testing.T) {
	cases := []string{
		`{"version":"2.0","modules":[]}`,
		`{"version":"1.0","modules":[{"kind":"Wrong","name":"M","declarations":[]}]}`,
		`{"version":"1.0","modules":[{"kind":"TlaModule","name":"M","declarations":[{"kind":"TlaOperDecl","type":"Untyped","name":"X","formalParams":[],"isRecursive":false,"body":{"kind":"OperEx","type":"Untyped","oper":"MISSING","args":[]}}]}]}`,
	}
	for _, input := range cases {
		if _, err := UnmarshalIRJSON([]byte(input), nil); err == nil {
			t.Fatalf("accepted invalid JSON: %s", input)
		}
	}
}

func TestIRJSONNullExpression(t *testing.T) {
	module := NewModule("M", []Decl{NewOperDecl("Null", nil, NullExpr(), Untyped())})
	encoded, err := MarshalSingleModuleIRJSON(module, IRJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"kind":"NullEx"`) {
		t.Fatalf("missing NullEx: %s", encoded)
	}
	decoded, err := UnmarshalSingleModuleIRJSON(encoded, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !EqualExpr(decoded.OperDeclarations()[0].Body(), NullExpr()) {
		t.Fatal("NullEx did not round-trip")
	}
}

func assertModulesEqual(t *testing.T, left, right *Module) {
	t.Helper()
	if left.Name() != right.Name() || len(left.Declarations()) != len(right.Declarations()) {
		t.Fatalf("module headers differ")
	}
	for index := range left.Declarations() {
		if !EqualTypedDecl(left.Declarations()[index], right.Declarations()[index]) {
			t.Fatalf("declaration %d differs", index)
		}
	}
}

func assertJSONEqual(t *testing.T, left, right []byte) {
	t.Helper()
	var leftValue, rightValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(right, &rightValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(leftValue, rightValue) {
		t.Fatalf("JSON differs\nleft: %s\nright: %s", left, right)
	}
}

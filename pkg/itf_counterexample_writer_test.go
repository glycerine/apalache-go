package apalache

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"
)

func TestMarshalITFCounterexampleProducesIndentedTraceJSON(t *testing.T) {
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1))})
	trace := NewTrace(module, []TraceState{{}, {"x": intLiteral(1)}}, nil, boolExpr(false))
	data, err := MarshalITFCounterexample(trace, ITFOptions{Description: "counterexample"})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) || !bytes.Contains(data, []byte("\n  \"#meta\"")) ||
		!bytes.Contains(data, []byte(`"description": "counterexample"`)) {
		t.Fatalf("counterexample JSON is not indented ITF:\n%s", data)
	}
}

func TestEncodeITFCounterexampleStateAndExpression(t *testing.T) {
	state := TraceState{"x": intLiteral(4)}
	encoded, err := EncodeITFCounterexampleState(state, 7)
	if err != nil {
		t.Fatal(err)
	}
	meta, ok := encoded[ITFMetaField].(map[string]any)
	if !ok || meta[ITFIndexField] != 7 {
		t.Fatalf("state metadata = %#v", encoded[ITFMetaField])
	}
	if value, ok := encoded["x"].(map[string]any); !ok || value[ITFBigIntField] != "4" {
		t.Fatalf("state value = %#v", encoded["x"])
	}
	state["x"] = intLiteral(9)
	if value := encoded["x"].(map[string]any)[ITFBigIntField]; value != "4" {
		t.Fatalf("encoded state changed to %#v", value)
	}
	expression, err := EncodeITFCounterexampleExpression(intLiteral(12))
	if err != nil {
		t.Fatal(err)
	}
	if expression.(map[string]any)[ITFBigIntField] != "12" {
		t.Fatalf("expression JSON = %#v", expression)
	}
}

func TestITFCounterexampleMatchesScalaWriterCases(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	strValue := func(value string) Expr { return NewValueExpr(StringValue(value), Typed(StrT1)) }
	boolValue := func(value bool) Expr { return NewValueExpr(BoolValue(value), Typed(BoolT1)) }
	seqType, err := NewSeqType(IntT1)
	if err != nil {
		t.Fatal(err)
	}
	setIntType, err := NewSetType(IntT1)
	if err != nil {
		t.Fatal(err)
	}
	tupleType, err := NewTupleType(IntT1, StrT1)
	if err != nil {
		t.Fatal(err)
	}
	recordType, err := NewRecordType(map[string]Type{"foo": IntT1, "bar": BoolT1})
	if err != nil {
		t.Fatal(err)
	}
	functionType, err := NewFunType(IntT1, StrT1)
	if err != nil {
		t.Fatal(err)
	}
	variantRow, err := NewRowType(map[string]Type{"Baz": recordType, "Boo": IntT1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	variantType := NewVariantType(variantRow)

	module := NewModule("test", []Decl{
		NewVarDecl("a", Typed(IntT1)), NewVarDecl("b", Typed(StrT1)),
		NewVarDecl("c", Typed(seqType)), NewVarDecl("d", Typed(setIntType)),
		NewVarDecl("e", Typed(recordType)), NewVarDecl("f", Typed(tupleType)),
		NewVarDecl("g", Typed(functionType)), NewVarDecl("h", Typed(functionType)),
		NewVarDecl("n", Typed(variantType)),
	})
	pair := func(integer int64, text string) Expr {
		return MustOperExpr(Op("TUPLE"), Typed(tupleType), intValue(integer), strValue(text))
	}
	pairSetType, err := NewSetType(tupleType)
	if err != nil {
		t.Fatal(err)
	}
	setAsFunction := func(pairs ...Expr) Expr {
		return MustOperExpr(Op("Apalache!SetAsFun"), Typed(functionType),
			MustOperExpr(Op("SET_ENUM"), Typed(pairSetType), pairs...))
	}
	record := MustOperExpr(Op("RECORD"), Typed(recordType),
		strValue("foo"), intValue(3), strValue("bar"), boolValue(true))
	large, ok := new(big.Int).SetString("1000000000000000000", 10)
	if !ok {
		t.Fatal("invalid large integer fixture")
	}
	state := TraceState{
		"a": intValue(2),
		"b": strValue("hello"),
		"c": MustOperExpr(Op("TUPLE"), Typed(seqType), intValue(3),
			NewValueExpr(NewIntValue(large), Typed(IntT1))),
		"d": MustOperExpr(Op("SET_ENUM"), Typed(setIntType), intValue(5), intValue(6)),
		"e": record,
		"f": pair(7, "myStr"),
		"g": setAsFunction(pair(1, "a"), pair(2, "b"), pair(3, "c")),
		"h": setAsFunction(),
		"i": setAsFunction(pair(1, "a")),
		"j": MustOperExpr(Op("FUN_SET"), Untyped(),
			NewValueExpr(BoolSet, Untyped()), NewValueExpr(IntSet, Untyped())),
		"k": MustOperExpr(Op("SET_POWERSET"), Untyped(), NewValueExpr(BoolSet, Untyped())),
		"l": NewValueExpr(IntSet, Untyped()),
		"m": NewValueExpr(NatSet, Untyped()),
		"n": MustOperExpr(Op("Variants!Variant"), Typed(variantType), strValue("Baz"), record),
	}
	data, err := MarshalITFTrace(module, []TraceState{{}, state}, ITFOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{
	  "#meta": {
	    "format": "ITF",
	    "format-description": "https://apalache-mc.org/docs/adr/015adr-trace.html",
	    "description": "Created by Apalache"
	  },
	  "vars": ["a", "b", "c", "d", "e", "f", "g", "h", "n"],
	  "states": [{
	    "#meta": {"index": 0},
	    "a": {"#bigint": "2"},
	    "b": "hello",
	    "c": [{"#bigint": "3"}, {"#bigint": "1000000000000000000"}],
	    "d": {"#set": [{"#bigint": "5"}, {"#bigint": "6"}]},
	    "e": {"foo": {"#bigint": "3"}, "bar": true},
	    "f": {"#tup": [{"#bigint": "7"}, "myStr"]},
	    "g": {"#map": [[{"#bigint": "1"}, "a"], [{"#bigint": "2"}, "b"], [{"#bigint": "3"}, "c"]]},
	    "h": {"#map": []},
	    "i": {"#map": [[{"#bigint": "1"}, "a"]]},
	    "j": {"#unserializable": "[BOOLEAN → Int]"},
	    "k": {"#unserializable": "SUBSET BOOLEAN"},
	    "l": {"#unserializable": "Int"},
	    "m": {"#unserializable": "Nat"},
	    "n": {"tag": "Baz", "value": {"foo": {"#bigint": "3"}, "bar": true}}
	  }]
	}`)
	var gotRoot, wantRoot map[string]any
	if err := json.Unmarshal(data, &gotRoot); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &wantRoot); err != nil {
		t.Fatal(err)
	}
	delete(gotRoot[ITFMetaField].(map[string]any), ITFVarTypesField)
	assertJSONEqual(t, mustJSON(t, gotRoot), mustJSON(t, wantRoot))
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

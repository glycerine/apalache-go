package apalache

import (
	"bytes"
	"encoding/json"
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

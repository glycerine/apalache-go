package apalache

import (
	"reflect"
	"testing"
)

func TestUJSONTraceReaderReadsLengthsAndConvertsBothFormats(t *testing.T) {
	reader := NewUJSONTraceReader()
	itfData := []byte(`{"#meta":{"format":"ITF","varTypes":{"x":"Int"}},"vars":["x"],"states":[{"x":1},{"x":2}]}`)
	itf, err := reader.Read(NewStringInputSource(string(itfData), nil, InputFormatITF))
	if err != nil || itf.Kind() != ITFTraceJSON {
		t.Fatalf("ITF read = %#v, %v", itf, err)
	}
	if length, lengthErr := reader.GetTraceLength(itf); lengthErr != nil || length != 2 {
		t.Fatalf("ITF length = %d, %v", length, lengthErr)
	}
	states, err := reader.Convert(itf)
	if err != nil || len(states) != 2 || !EqualExpr(states[1]["x"], intLiteral(2)) {
		t.Fatalf("ITF states = %#v, %v", states, err)
	}

	module := NewModule("Trace", []Decl{
		NewOperDecl("CInit", nil, boolExpr(true), Typed(BoolT1)),
		NewOperDecl("State0", nil, MustOperExpr(Op("EQ"), Typed(BoolT1),
			NewNameExpr("x", Typed(IntT1)), intLiteral(3)), Typed(BoolT1)),
		NewOperDecl("Inv", nil, boolExpr(true), Typed(BoolT1)),
	})
	jsonData, err := MarshalSingleModuleIRJSON(module, IRJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := reader.Read(NewStringInputSource(string(jsonData), nil, InputFormatJSON))
	if err != nil || legacy.Kind() != ApalacheTraceJSON {
		t.Fatalf("legacy read = %#v, %v", legacy, err)
	}
	if length, lengthErr := reader.GetTraceLength(legacy); lengthErr != nil || length != 1 {
		t.Fatalf("legacy length = %d, %v", length, lengthErr)
	}
	states, err = reader.Convert(legacy)
	if err != nil || len(states) != 1 || !EqualExpr(states[0]["x"], intLiteral(3)) {
		t.Fatalf("legacy states = %#v, %v", states, err)
	}
	copyJSON := legacy.JSON()
	copyJSON[0] = 'x'
	if reflect.DeepEqual(copyJSON, legacy.JSON()) {
		t.Fatal("trace JSON accessor exposed mutable storage")
	}
}

func TestUJSONTraceReaderRejectsInvalidSourcesShapesAndJSON(t *testing.T) {
	reader := NewUJSONTraceReader()
	if _, err := reader.Read(NewStringInputSource("---- MODULE M ----", nil, InputFormatTLA)); err == nil {
		t.Fatal("TLA source was accepted as a trace")
	}
	if _, err := reader.Read(NewStringInputSource("{", nil, InputFormatJSON)); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
	missingStates, _ := NewTraceJSON(ITFTraceJSON, []byte(`{}`))
	if _, err := reader.GetTraceLength(missingStates); err == nil {
		t.Fatal("ITF without states had a length")
	}
	missingModules, _ := NewTraceJSON(ApalacheTraceJSON, []byte(`{}`))
	if _, err := reader.GetTraceLength(missingModules); err == nil {
		t.Fatal("Apalache JSON without modules had a length")
	}
}

func TestUJSONTraceReaderAcceptsZeroStateApalacheTrace(t *testing.T) {
	module := NewModule("Empty", []Decl{
		NewOperDecl("CInit", nil, boolExpr(true), Typed(BoolT1)),
		NewOperDecl("Inv", nil, boolExpr(true), Typed(BoolT1)),
	})
	data, err := MarshalSingleModuleIRJSON(module, IRJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	reader := NewUJSONTraceReader()
	trace, err := reader.Read(NewStringInputSource(string(data), nil, InputFormatJSON))
	if err != nil {
		t.Fatal(err)
	}
	if length, lengthErr := reader.GetTraceLength(trace); lengthErr != nil || length != 0 {
		t.Fatalf("zero-state length = %d, %v", length, lengthErr)
	}
	states, err := reader.Convert(trace)
	if err != nil || len(states) != 0 {
		t.Fatalf("zero-state conversion = %#v, %v", states, err)
	}
}

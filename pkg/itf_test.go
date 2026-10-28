package apalache

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestMarshalITFTraceMergesConstantInitialization(t *testing.T) {
	module := NewModule("M", []Decl{NewConstDecl("N", Typed(IntT1)), NewVarDecl("x", Typed(IntT1))})
	states := []TraceState{{"N": NewValueExpr(Int64Value(3), Typed(IntT1))}, {"x": NewValueExpr(Int64Value(1), Typed(IntT1))}, {"x": NewValueExpr(Int64Value(2), Typed(IntT1))}}
	data, err := MarshalITFTrace(module, states, ITFOptions{Description: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err = json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	encodedStates := root[ITFStatesField].([]any)
	if len(encodedStates) != 2 {
		t.Fatalf("states=%v", encodedStates)
	}
	first := encodedStates[0].(map[string]any)
	if first["N"].(map[string]any)[ITFBigIntField] != "3" || first["x"].(map[string]any)[ITFBigIntField] != "1" {
		t.Fatalf("merged state=%v", first)
	}
	if root[ITFParamsField].([]any)[0] != "N" {
		t.Fatalf("params=%v", root[ITFParamsField])
	}
}

func TestMarshalITFTraceIncludesModuleNameReplacements(t *testing.T) {
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1))})
	module.AddNameReplacement("generated", "[] (x = 1)")
	data, err := MarshalITFTrace(module, []TraceState{{}, {"x": intLiteral(1)}}, ITFOptions{
		NameReplacements: map[string]string{"explicit": "x + 1", "generated": "override"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err = json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	replacements := root[ITFMetaField].(map[string]any)["variables-to-expressions"].(map[string]any)
	if replacements["generated"] != "override" || replacements["explicit"] != "x + 1" || len(replacements) != 2 {
		t.Fatalf("ITF name replacements = %#v", replacements)
	}
}

func TestTlaToITFJSONMatchesScalaTraceShapeAndRoundTrips(t *testing.T) {
	module := NewModule("test", []Decl{NewVarDecl("x", Typed(IntT1))})
	states := []TraceState{
		{},
		{"x": intLiteral(1)},
		{"x": intLiteral(2)},
	}
	data, err := MarshalITFTrace(module, states, ITFOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err = json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	meta := root[ITFMetaField].(map[string]any)
	if meta["format"] != "ITF" ||
		meta["format-description"] != "https://apalache-mc.org/docs/adr/015adr-trace.html" ||
		meta[ITFVarTypesField].(map[string]any)["x"] != "Int" {
		t.Fatalf("ITF metadata = %#v", meta)
	}
	variables := root[ITFVarsField].([]any)
	encoded := root[ITFStatesField].([]any)
	if len(variables) != 1 || variables[0] != "x" || len(encoded) != 2 {
		t.Fatalf("ITF vars/states = %#v / %#v", variables, encoded)
	}
	for index, raw := range encoded {
		state := raw.(map[string]any)
		if state[ITFMetaField].(map[string]any)["index"] != float64(index) ||
			state["x"].(map[string]any)[ITFBigIntField] != []string{"1", "2"}[index] {
			t.Fatalf("ITF state %d = %#v", index, state)
		}
	}
	decoded, err := UnmarshalITFTrace(data)
	if err != nil || len(decoded) != 2 {
		t.Fatalf("round trip = %#v, %v", decoded, err)
	}
	for index, state := range decoded {
		value, ok := state["x"].(*ValueExpr)
		if !ok {
			t.Fatalf("decoded state %d = %#v", index, state)
		}
		integer, integerOK := value.Value().(IntValue)
		if !integerOK || integer.BigInt().Int64() != int64(index+1) {
			t.Fatalf("decoded state %d = %#v", index, state)
		}
	}
}

func TestMarshalDecodedTraceITFSeparatesConstantsWithoutDroppingInitialState(t *testing.T) {
	module := NewModule("M", []Decl{NewConstDecl("N", Typed(IntT1)), NewVarDecl("x", Typed(IntT1))})
	states := []DecodedState{
		{Values: map[string]Expr{"N": intLiteral(3), "x": intLiteral(0)}},
		{Values: map[string]Expr{"N": intLiteral(3), "x": intLiteral(1)}},
	}
	data, err := MarshalDecodedTraceITF(module, states, ITFOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err = json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	encoded := root[ITFStatesField].([]any)
	if len(encoded) != 2 {
		t.Fatalf("ITF states = %#v", encoded)
	}
	first := encoded[0].(map[string]any)
	second := encoded[1].(map[string]any)
	if _, found := first["N"]; !found || first["x"].(map[string]any)[ITFBigIntField] != "0" ||
		second["x"].(map[string]any)[ITFBigIntField] != "1" {
		t.Fatalf("decoded ITF states = %#v", encoded)
	}
}

func TestDecodedAndITFTracesOmitMarkedInternalVariables(t *testing.T) {
	x := NewVarDecl("x", Typed(IntT1))
	aux := NewVarDecl("aux", Typed(BoolT1))
	aux.MarkInternal()
	module := NewModule("M", []Decl{x, aux})
	states := []DecodedState{{TransitionNumber: 9, Values: map[string]Expr{
		"x": intLiteral(1), "aux": temporalBool(true),
	}}}
	user := UserDecodedStates(module, states)
	if _, found := user[0].Values["aux"]; found || len(user[0].Values) != 1 || user[0].TransitionNumber != 9 {
		t.Fatalf("user states = %#v", user)
	}
	if _, found := states[0].Values["aux"]; !found {
		t.Fatal("filter mutated the decoded input")
	}
	data, err := MarshalDecodedTraceITF(module, states, ITFOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err = json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	variables := root[ITFVarsField].([]any)
	encoded := root[ITFStatesField].([]any)[0].(map[string]any)
	if len(variables) != 1 || variables[0] != "x" {
		t.Fatalf("ITF variables = %#v", variables)
	}
	if _, found := encoded["aux"]; found {
		t.Fatalf("ITF state contains internal value: %#v", encoded)
	}
}

func TestEncodeITFCollectionValues(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	strValue := func(value string) Expr { return NewValueExpr(StringValue(value), Typed(StrT1)) }
	seqType, _ := NewSeqType(IntT1)
	sequence := MustOperExpr(Op("TUPLE"), Typed(seqType), intValue(1), intValue(2))
	assertITFJSON(t, sequence, `[{"#bigint":"1"},{"#bigint":"2"}]`)
	tupleType, _ := NewTupleType(IntT1, StrT1)
	tuple := MustOperExpr(Op("TUPLE"), Typed(tupleType), intValue(1), strValue("A"))
	assertITFJSON(t, tuple, `{"#tup":[{"#bigint":"1"},"A"]}`)
	setType, _ := NewSetType(IntT1)
	set := MustOperExpr(Op("SET_ENUM"), Typed(setType), intValue(1), intValue(2))
	assertITFJSON(t, set, `{"#set":[{"#bigint":"1"},{"#bigint":"2"}]}`)
	recordType, _ := NewRecordType(map[string]Type{"x": IntT1, "name": StrT1})
	record := MustOperExpr(Op("RECORD"), Typed(recordType), strValue("x"), intValue(1), strValue("name"), strValue("A"))
	assertITFJSON(t, record, `{"name":"A","x":{"#bigint":"1"}}`)
}

func TestEncodeITFVariantAndFunction(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	strValue := func(value string) Expr { return NewValueExpr(StringValue(value), Typed(StrT1)) }
	row, _ := NewRowType(map[string]Type{"Some": IntT1}, nil)
	variantType := NewVariantType(row)
	variant := MustOperExpr(Op("Variants!Variant"), Typed(variantType), strValue("Some"), intValue(5))
	assertITFJSON(t, variant, `{"tag":"Some","value":{"#bigint":"5"}}`)
	pairType, _ := NewTupleType(IntT1, IntT1)
	pair := MustOperExpr(Op("TUPLE"), Typed(pairType), intValue(1), intValue(2))
	pairSetType, _ := NewSetType(pairType)
	pairs := MustOperExpr(Op("SET_ENUM"), Typed(pairSetType), pair)
	funType, _ := NewFunType(IntT1, IntT1)
	function := MustOperExpr(Op("Apalache!SetAsFun"), Typed(funType), pairs)
	assertITFJSON(t, function, `{"#map":[[{"#bigint":"1"},{"#bigint":"2"}]]}`)
}

func TestMarshalITFTraceRejectsMissingStatesAndUntypedVariables(t *testing.T) {
	if _, err := MarshalITFTrace(NewModule("M", nil), nil, ITFOptions{}); err == nil {
		t.Fatal("accepted empty trace")
	}
	module := NewModule("M", []Decl{NewVarDecl("x", Untyped())})
	if _, err := MarshalITFTrace(module, []TraceState{{}}, ITFOptions{}); err == nil {
		t.Fatal("accepted untyped variable")
	}
}

func assertITFJSON(t *testing.T, expression Expr, want string) {
	t.Helper()
	got, err := EncodeITFValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	gotData, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, gotData, []byte(want))
}

func TestITFValueRoundTripSupportedTypes(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	strValue := func(value string) Expr { return NewValueExpr(StringValue(value), Typed(StrT1)) }
	seqType, _ := NewSeqType(IntT1)
	tupleType, _ := NewTupleType(IntT1, StrT1)
	setType, _ := NewSetType(BoolT1)
	recordRow, _ := NewRowType(map[string]Type{"x": IntT1, "name": StrT1}, nil)
	recordType := NewRecordRowType(recordRow)
	variantRow, _ := NewRowType(map[string]Type{"Some": IntT1, "None": BoolT1}, nil)
	variantType := NewVariantType(variantRow)
	pairType, _ := NewTupleType(IntT1, StrT1)
	pairSetType, _ := NewSetType(pairType)
	funType, _ := NewFunType(IntT1, StrT1)
	cases := []Expr{
		intValue(123), NewValueExpr(BoolValue(true), Typed(BoolT1)), strValue("hello"),
		NewValueExpr(StringValue("one_OF_NODE"), Typed(mustConstType(t, "NODE"))),
		MustOperExpr(Op("TUPLE"), Typed(seqType), intValue(1), intValue(2)),
		MustOperExpr(Op("TUPLE"), Typed(tupleType), intValue(1), strValue("A")),
		MustOperExpr(Op("SET_ENUM"), Typed(setType), NewValueExpr(BoolValue(true), Typed(BoolT1))),
		MustOperExpr(Op("RECORD"), Typed(recordType), strValue("x"), intValue(1), strValue("name"), strValue("A")),
		MustOperExpr(Op("Variants!Variant"), Typed(variantType), strValue("Some"), intValue(1)),
		MustOperExpr(Op("Apalache!SetAsFun"), Typed(funType), MustOperExpr(Op("SET_ENUM"), Typed(pairSetType),
			MustOperExpr(Op("TUPLE"), Typed(pairType), intValue(1), strValue("one")))),
	}
	for _, expression := range cases {
		raw, err := EncodeITFValue(expression)
		if err != nil {
			t.Fatalf("encode %s: %v", expression.Tag(), err)
		}
		typ, _ := expression.Tag().Type()
		decoded, err := DecodeITFValue(normalizeITFJSON(t, raw), typ)
		if err != nil {
			t.Fatalf("decode %s from %#v: %v", typ, raw, err)
		}
		decodedType, _ := decoded.Tag().Type()
		if !EqualTypes(typ, decodedType) {
			t.Fatalf("round trip type differs: %s -> %s", typ, decodedType)
		}
		reencoded, err := EncodeITFValue(decoded)
		if err != nil {
			t.Fatal(err)
		}
		left, _ := json.Marshal(raw)
		right, _ := json.Marshal(reencoded)
		assertJSONEqual(t, left, right)
	}
}

func TestUnmarshalITFTrace(t *testing.T) {
	input := []byte(`{
  "#meta":{"format":"ITF","varTypes":{"x":"Int","s":"Seq(Bool)"}},
  "vars":["x","s"],
  "states":[
    {"#meta":{"index":0},"x":{"#bigint":"12345678901234567890"},"s":[true,false]},
    {"#meta":{"index":1},"x":2,"s":[]}
  ]}`)
	states, err := UnmarshalITFTrace(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 || states[0]["x"].(*ValueExpr).Value().(IntValue).String() != "12345678901234567890" ||
		len(states[0]["s"].(*OperExpr).Arguments()) != 2 || states[1]["x"].(*ValueExpr).Value().(IntValue).String() != "2" {
		t.Fatalf("states = %#v", states)
	}
}

func TestUnmarshalITFTraceWithSourceTypedParams(t *testing.T) {
	input := []byte(`{
  "#meta":{"format":"ITF","varTypes":{"x":"Int"}},
  "params":["N"],
  "vars":["x"],
  "states":[
    {"N":{"#bigint":"3"},"x":{"#bigint":"1"}},
    {"x":{"#bigint":"2"}}
  ]}`)
	if _, err := UnmarshalITFTrace(input); err == nil {
		t.Fatal("parameter trace was accepted without a source type")
	}
	states, err := UnmarshalITFTraceWithParams(input, map[string]Type{"N": IntT1})
	if err != nil {
		t.Fatal(err)
	}
	for index, state := range states {
		value, ok := state["N"].(*ValueExpr)
		if !ok || value.Value().(IntValue).String() != "3" {
			t.Fatalf("state %d parameter = %v", index, state["N"])
		}
	}
}

func TestUnmarshalITFTraceRejectsMalformedContent(t *testing.T) {
	cases := []string{
		`{}`,
		`{"#meta":{"varTypes":{"x":"Int"}},"vars":[],"states":[]}`,
		`{"#meta":{"varTypes":{}},"vars":["x"],"states":[]}`,
		`{"#meta":{"varTypes":{"x":"Int"}},"vars":["x"],"states":[{}]}`,
		`{"#meta":{"varTypes":{"x":"Int"}},"vars":["x"],"states":[{"x":true}]}`,
		`{"#meta":{"varTypes":{"x":"Int"}},"vars":["x"],"states":[{"x":{"#unserializable":"Int"}}]}`,
		`{"#meta":{"varTypes":{"x":"Int"}},"vars":["x"],"states":[{"x":1,"y":2}]}`,
	}
	for _, input := range cases {
		if _, err := UnmarshalITFTrace([]byte(input)); err == nil {
			t.Errorf("accepted malformed ITF: %s", input)
		}
	}
}

func TestDecodeITFRejectsShapeAndTypeMismatches(t *testing.T) {
	setType, _ := NewSetType(IntT1)
	tupleType, _ := NewTupleType(IntT1, BoolT1)
	funType, _ := NewFunType(IntT1, BoolT1)
	cases := []struct {
		raw any
		typ Type
	}{
		{true, IntT1}, {"one_OF_A", StrT1}, {"one_OF_A", mustConstType(t, "B")},
		{map[string]any{ITFSetField: []any{}, "junk": true}, setType},
		{map[string]any{ITFTupleField: []any{json.Number("1")}}, tupleType},
		{map[string]any{ITFMapField: []any{[]any{json.Number("1")}}}, funType},
	}
	for _, test := range cases {
		if _, err := DecodeITFValue(test.raw, test.typ); err == nil {
			t.Errorf("accepted %#v as %s", test.raw, test.typ)
		}
	}
}

func TestITFPackageErrorsAndFieldCatalog(t *testing.T) {
	fields := []string{
		ITFMetaField, ITFVarTypesField, ITFVarsField, ITFParamsField, ITFStatesField,
		ITFDescriptionField, ITFFormatField, ITFFormatDescriptionField,
		ITFVariablesToExpressionsField, ITFIndexField, ITFTagField, ITFValueField,
		ITFBigIntField, ITFTupleField, ITFSetField, ITFMapField, ITFUnserializableField,
	}
	want := []string{
		"#meta", "varTypes", "vars", "params", "states", "description", "format", "format-description",
		"variables-to-expressions", "index", "tag", "value", "#bigint", "#tup", "#set", "#map", "#unserializable",
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("ITF fields = %#v", fields)
	}
	formatErr := itfFormat("bad %s", "shape")
	contentErr := itfContent("bad %s", "value")
	var formatTarget *ITFFormatError
	var contentTarget *ITFContentError
	var baseTarget *ITFError
	if !errors.As(formatErr, &formatTarget) || errors.As(formatErr, &contentTarget) || !errors.As(formatErr, &baseTarget) {
		t.Fatalf("format classification = %T", formatErr)
	}
	if !errors.As(contentErr, &contentTarget) || errors.As(contentErr, &formatTarget) || !errors.As(contentErr, &baseTarget) {
		t.Fatalf("content classification = %T", contentErr)
	}
	multiple := NewITFMultipleErrors([]error{formatErr, contentErr})
	if multiple.Error() != "Multiple errors: ITF format error: bad shape\nITF content error: bad value" ||
		!errors.Is(multiple, formatErr) || !errors.Is(multiple, contentErr) {
		t.Fatalf("multiple errors = %q", multiple)
	}
	copyOfErrors := multiple.Unwrap()
	copyOfErrors[0] = nil
	if multiple.Errors[0] == nil {
		t.Fatal("multiple error unwrapping exposed internal slice")
	}
}

func normalizeITFJSON(t *testing.T, value any) any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		t.Fatal(err)
	}
	return normalized
}

func mustConstType(t *testing.T, name string) ConstType {
	t.Helper()
	typ, err := NewConstType(name)
	if err != nil {
		t.Fatal(err)
	}
	return typ
}

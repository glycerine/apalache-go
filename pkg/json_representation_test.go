package apalache

import (
	"errors"
	"reflect"
	"testing"
)

func TestJSONRepresentationAdapters(t *testing.T) {
	value := JSONObject(
		JSONField{Name: "n", Value: JSONInt(42)},
		JSONField{Name: "xs", Value: JSONArray(JSONString("x"), JSONBool(true))},
	)
	if value.String() != "{\n  \"n\": 42,\n  \"xs\": [\n    \"x\",\n    true\n  ]\n}" &&
		value.String() != "{\n  \"xs\": [\n    \"x\",\n    true\n  ],\n  \"n\": 42\n}" {
		t.Fatalf("unexpected pretty JSON: %s", value.String())
	}
	fields, ok := value.Fields()
	if !ok || !reflect.DeepEqual(fields, map[string]struct{}{"n": {}, "xs": {}}) {
		t.Fatalf("fields = %#v, %v", fields, ok)
	}
	number, ok := value.Field("n")
	if !ok {
		t.Fatal("missing n")
	}
	if got, err := JSONAsInt(number); err != nil || got != 42 {
		t.Fatalf("integer = %d, %v", got, err)
	}
	arrayValue, ok := value.Field("xs")
	if !ok {
		t.Fatal("missing xs")
	}
	array, err := JSONAsArray(arrayValue)
	if err != nil || len(array) != 2 {
		t.Fatalf("array = %#v, %v", array, err)
	}
	if got, err := JSONAsString(array[0]); err != nil || got != "x" {
		t.Fatalf("string = %q, %v", got, err)
	}
	if got, err := JSONAsBool(array[1]); err != nil || !got {
		t.Fatalf("bool = %v, %v", got, err)
	}
	if _, found := JSONString("x").Fields(); found {
		t.Fatal("scalar reported object fields")
	}
	if _, err := JSONAsInt(JSONString("42")); err == nil {
		t.Fatal("string accepted as integer")
	} else {
		var jsonErr *JSONDeserializationError
		if !errors.As(err, &jsonErr) {
			t.Fatalf("wrong error type: %T", err)
		}
	}
}

func TestJSONRepresentationMatchesScalaJacksonAdapterCases(t *testing.T) {
	object := JSONObject(
		JSONField{Name: "name", Value: JSONString("Alice")},
		JSONField{Name: "age", Value: JSONInt(30)},
		JSONField{Name: "active", Value: JSONBool(true)},
	)
	fields, objectOK := object.Fields()
	if !objectOK || !reflect.DeepEqual(fields, map[string]struct{}{"name": {}, "age": {}, "active": {}}) {
		t.Fatalf("object fields = %#v, %v", fields, objectOK)
	}
	if _, found := object.Field("name"); !found {
		t.Fatal("present field was not found")
	}
	if _, found := object.Field("missing"); found {
		t.Fatal("absent field was found")
	}

	arrayRepresentation := JSONArray(JSONInt(1), JSONInt(2), JSONInt(3))
	array, err := JSONAsArray(arrayRepresentation)
	if err != nil || len(array) != 3 {
		t.Fatalf("array = %#v, %v", array, err)
	}
	if integer, err := JSONAsInt(JSONInt(42)); err != nil || integer != 42 {
		t.Fatalf("integer = %d, %v", integer, err)
	}
	if text, err := JSONAsString(JSONString("hello")); err != nil || text != "hello" {
		t.Fatalf("string = %q, %v", text, err)
	}
	if boolean, err := JSONAsBool(JSONBool(true)); err != nil || !boolean {
		t.Fatalf("boolean = %v, %v", boolean, err)
	}
	if _, err := JSONAsInt(JSONString("not an int")); err == nil {
		t.Fatal("string accepted as integer")
	} else {
		var typed *JSONDeserializationError
		if !errors.As(err, &typed) {
			t.Fatalf("type mismatch error = %T: %v", err, err)
		}
	}
}

func TestIRJSONNodeEncoderDecoder(t *testing.T) {
	expression := MustOperExpr(Op("EQ"), Typed(BoolT1),
		NewNameExpr("x", Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1)))
	declaration := NewOperDecl("P", nil, expression, Typed(BoolT1))
	module := NewModule("M", []Decl{declaration})
	encoder := IRJSONEncoder{}
	decoder := IRJSONDecoder{}

	encodedExpression, err := encoder.Expression(expression)
	if err != nil {
		t.Fatal(err)
	}
	decodedExpression, err := decoder.Expression(encodedExpression)
	if err != nil || !EqualExpr(expression, decodedExpression) {
		t.Fatalf("expression round trip = %#v, %v", decodedExpression, err)
	}
	encodedDeclaration, err := encoder.Declaration(declaration)
	if err != nil {
		t.Fatal(err)
	}
	decodedDeclaration, err := decoder.Declaration(encodedDeclaration)
	if err != nil || !EqualTypedDecl(declaration, decodedDeclaration) {
		t.Fatalf("declaration round trip = %#v, %v", decodedDeclaration, err)
	}
	encodedModule, err := encoder.Module(module)
	if err != nil {
		t.Fatal(err)
	}
	decodedModule, err := decoder.Module(encodedModule)
	if err != nil {
		t.Fatal(err)
	}
	assertModulesEqual(t, module, decodedModule)
	root, err := encoder.Root([]*Module{module})
	if err != nil {
		t.Fatal(err)
	}
	modules, err := decoder.Root(root)
	if err != nil || len(modules) != 1 {
		t.Fatalf("root round trip = %#v, %v", modules, err)
	}
	single, err := decoder.SingleModule(root)
	if err != nil {
		t.Fatal(err)
	}
	assertModulesEqual(t, module, single)
}

func TestTextLayoutDefaultsAndCustomValues(t *testing.T) {
	if got := DefaultTextLayout(); got != (TextLayout{TextWidth: 80, Indent: 2}) {
		t.Fatalf("default layout = %+v", got)
	}
	custom := TextLayout{TextWidth: 120, Indent: 4}
	if custom.TextWidth != 120 || custom.Indent != 4 {
		t.Fatalf("custom layout = %+v", custom)
	}
}

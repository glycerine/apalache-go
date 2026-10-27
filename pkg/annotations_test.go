package apalache

import (
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func TestParseAnnotationForms(t *testing.T) {
	parsed, err := ParseAnnotation(`  @greet("hello", 2021, true, Name)   `)
	if err != nil {
		t.Fatal(err)
	}
	want := Annotation{Name: "greet", Arguments: []AnnotationArg{
		AnnotationString{Text: "hello"}, AnnotationInteger{Value: big.NewInt(2021)}, AnnotationBoolean{Value: true}, AnnotationIdentifier{Name: "Name"},
	}}
	if !reflect.DeepEqual(parsed, want) {
		t.Fatalf("got %#v, want %#v", parsed, want)
	}
	if parsed.String() != `@greet("hello", 2021, true, Name)` {
		t.Fatalf("String = %q", parsed.String())
	}

	inline, err := ParseAnnotation("@type:  (Int, Int) \t =>\f Set(Int) ;")
	if err != nil {
		t.Fatal(err)
	}
	if inline.PrettyString() != "@type: (Int, Int) => Set(Int);" {
		t.Fatalf("pretty = %q", inline.PrettyString())
	}
	parameterless, err := ParseAnnotation("@pure")
	if err != nil || parameterless.String() != "@pure" {
		t.Fatalf("parameterless = %#v, %v", parameterless, err)
	}
}

func TestParseAnnotationMultilineAndBigInteger(t *testing.T) {
	parsed, err := ParseAnnotation("@greet(\"hello\",\n 123456789012345678901234567890,\n FALSE)")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Arguments[1].(AnnotationInteger).Value.String() != "123456789012345678901234567890" || parsed.Arguments[2].(AnnotationBoolean).Value {
		t.Fatalf("arguments = %#v", parsed.Arguments)
	}
	alias, err := ParseAnnotation("@typeAlias: MESSAGE =\n  Req({ ask: Int })\n  | Ack({ success: Bool });")
	if err != nil {
		t.Fatal(err)
	}
	if alias.Arguments[0].(AnnotationString).Text != "MESSAGE =\n Req({ ask: Int })\n | Ack({ success: Bool })" {
		t.Fatalf("inline multiline = %q", alias.Arguments[0])
	}
}

func TestParseAnnotationRejectsJunkAndMalformedInput(t *testing.T) {
	cases := []string{"", "plain", `@foo("unterminated)`, "@foo(1,)", "@foo: missing terminator", "@foo() trailing", "@foo(@bad)"}
	for _, input := range cases {
		_, err := ParseAnnotation(input)
		var parseError *AnnotationParseError
		if !errors.As(err, &parseError) {
			t.Errorf("%q: got %T %v", input, err, err)
		}
	}
}

func TestAnnotationRendering(t *testing.T) {
	annotation := Annotation{Name: "sweet", Arguments: []AnnotationArg{AnnotationBoolean{Value: false}, AnnotationInteger{Value: big.NewInt(-2)}}}
	if annotation.String() != "@sweet(false, -2)" || annotation.PrettyString() != annotation.String() {
		t.Fatalf("rendered %q / %q", annotation.String(), annotation.PrettyString())
	}
}

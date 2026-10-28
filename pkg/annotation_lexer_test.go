package apalache

import (
	"math/big"
	"reflect"
	"testing"
)

func TestAnnotationLexerMatchesScalaTokens(t *testing.T) {
	tests := []struct {
		name, input string
		want        []AnnotationToken
	}{
		{"parameterless", "  @test  ", []AnnotationToken{{Kind: AnnotationAtIdentifierToken, Text: "test"}}},
		{"arguments", "  @test(123, TRUE) ", []AnnotationToken{
			{Kind: AnnotationAtIdentifierToken, Text: "test"}, {Kind: AnnotationLeftParenToken},
			{Kind: AnnotationNumberToken, Number: big.NewInt(123)}, {Kind: AnnotationCommaToken},
			{Kind: AnnotationBooleanToken, Boolean: true}, {Kind: AnnotationRightParenToken},
		}},
		{"identifier", "@foo(Bar)", []AnnotationToken{
			{Kind: AnnotationAtIdentifierToken, Text: "foo"}, {Kind: AnnotationLeftParenToken},
			{Kind: AnnotationIdentifierToken, Text: "Bar"}, {Kind: AnnotationRightParenToken},
		}},
		{"strings", `@foo("one", "two")`, []AnnotationToken{
			{Kind: AnnotationAtIdentifierToken, Text: "foo"}, {Kind: AnnotationLeftParenToken},
			{Kind: AnnotationStringToken, Text: "one"}, {Kind: AnnotationCommaToken},
			{Kind: AnnotationStringToken, Text: "two"}, {Kind: AnnotationRightParenToken},
		}},
		{"inline", "@baz: one;", []AnnotationToken{
			{Kind: AnnotationAtIdentifierToken, Text: "baz"}, {Kind: AnnotationInlineStringToken, Text: "one"},
		}},
		{"ASCII whitespace", "@type: Int \t\f\t\f => Int;", []AnnotationToken{
			{Kind: AnnotationAtIdentifierToken, Text: "type"}, {Kind: AnnotationInlineStringToken, Text: "Int => Int"},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := LexAnnotation(test.input)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("LexAnnotation(%q) = %#v, %v; want %#v", test.input, got, err, test.want)
			}
		})
	}
}

func TestAnnotationLexerRejectsScalaJunkCases(t *testing.T) {
	for _, input := range []string{
		"   ", "(*  @test *)", `\* example by foo@example.org: @test(123, TRUE) the rest is junk. :-)`,
		` 1.3 " zzz `, "\\* @type: Int\n\\*          => Int;",
		"  \\* TODO: use a model type here\n  \\* @type: Str;",
	} {
		if tokens, err := LexAnnotation(input); err == nil {
			t.Errorf("LexAnnotation(%q) accepted junk as %#v", input, tokens)
		}
	}
}

func TestAnnotationLexerEmitsNewlinesDotsAndNegativeNumbers(t *testing.T) {
	got, err := LexAnnotation("@a(-12.3,\nFalse)")
	if err != nil {
		t.Fatal(err)
	}
	wantKinds := []AnnotationTokenKind{
		AnnotationAtIdentifierToken, AnnotationLeftParenToken, AnnotationNumberToken, AnnotationDotToken,
		AnnotationNumberToken, AnnotationCommaToken, AnnotationNewlineToken, AnnotationBooleanToken, AnnotationRightParenToken,
	}
	if len(got) != len(wantKinds) {
		t.Fatalf("tokens = %#v", got)
	}
	for index, kind := range wantKinds {
		if got[index].Kind != kind {
			t.Fatalf("token %d = %#v, want %s", index, got[index], kind)
		}
	}
}

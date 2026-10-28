package apalache

import (
	"errors"
	"testing"
)

func TestTypeLexerScalaTokenAlgebra(t *testing.T) {
	text := "Int Real Bool Str Set Seq Variant Rec name 003 \"hello world\" -> => = , : $ ( ) [ ] { } << >> | (| |) <| |>"
	tokens, err := lexType(text)
	if err != nil {
		t.Fatal(err)
	}
	wantKinds := []typeTokenKind{
		typeTokenInt, typeTokenReal, typeTokenBool, typeTokenStr,
		typeTokenSet, typeTokenSeq, typeTokenVariant, typeTokenRecord,
		typeTokenIdentifier, typeTokenNumber, typeTokenStringLiteral,
		typeTokenArrow, typeTokenDoubleArrow, typeTokenEqual, typeTokenComma,
		typeTokenColon, typeTokenDollar, typeTokenLParen, typeTokenRParen,
		typeTokenLBracket, typeTokenRBracket, typeTokenLCurly, typeTokenRCurly,
		typeTokenLAngle, typeTokenRAngle, typeTokenPipe, typeTokenLRow,
		typeTokenRRow, typeTokenLSparse, typeTokenRSparse, typeTokenEOF,
	}
	wantStrings := []string{
		"Int", "Real", "Bool", "Str", "Set", "Seq", "Variant", "Rec",
		"record field 'name'", "3", `"hello world"`, "->", "=>", "=", ",", ":", "$",
		"(", ")", "[", "]", "{", "}", "<<", ">>", "|", "(|", "|)", "<|", "|>", "end of input",
	}
	if len(tokens) != len(wantKinds) {
		t.Fatalf("got %d tokens: %+v", len(tokens), tokens)
	}
	for index, token := range tokens {
		if token.kind != wantKinds[index] || token.String() != wantStrings[index] {
			t.Errorf("token %d = (%v, %q), want (%v, %q)", index, token.kind, token.String(), wantKinds[index], wantStrings[index])
		}
	}
}

func TestTypeLexerKeywordBoundariesCommentsAndWhitespace(t *testing.T) {
	tokens, err := lexType("\t\r\f\n// comment\nIntFoo Real2 _Bool SetX")
	if err != nil {
		t.Fatal(err)
	}
	for index, token := range tokens[:len(tokens)-1] {
		if token.kind != typeTokenIdentifier {
			t.Errorf("token %d = %+v, keyword prefix should remain an identifier", index, token)
		}
	}

	for _, text := range []string{"\vInt", "\u00a0Int", "é"} {
		if _, err := lexType(text); err == nil {
			t.Errorf("lexType(%q) succeeded; Scala lexer rejects non-grammar whitespace/identifiers", text)
		}
	}
}

func TestTypeLexerStringLiteralAndFieldNumberFailures(t *testing.T) {
	tokens, err := lexType("\"line one\nline two\"")
	if err != nil || len(tokens) != 2 || tokens[0].kind != typeTokenStringLiteral || tokens[0].text != "line one\nline two" {
		t.Fatalf("multiline string tokens = %+v, %v", tokens, err)
	}
	for _, text := range []string{`"unterminated`, "<| 2147483648: Int |>", "<| 999999999999999999999: Int |>"} {
		_, err := ParseType(text)
		var parseErr *TypeParseError
		if !errors.As(err, &parseErr) {
			t.Errorf("ParseType(%q) error = %T %v, want TypeParseError", text, err, err)
		}
	}
}

func TestTypeParserReservedWordsAreNotFieldsOrTags(t *testing.T) {
	for _, text := range []string{
		"[Int: Bool]", "{ Set: Int }", "(| Variant: Int |)", "Bool(Int)", "Rec(Int)", `"literal"`,
	} {
		if typ, err := ParseType(text); err == nil {
			t.Errorf("ParseType(%q) = %v, want error", text, typ)
		}
	}
}

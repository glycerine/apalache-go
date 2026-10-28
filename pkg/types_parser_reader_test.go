package apalache

import (
	"errors"
	"testing"
)

func TestTypeParseErrorScalaTextAndPosition(t *testing.T) {
	_, err := ParseType("Int -> )")
	var parseErr *TypeParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("got %T, want TypeParseError", err)
	}
	if parseErr.Offset != 7 || parseErr.Column() != 8 || parseErr.Message != `expected a type, found ")"` {
		t.Fatalf("parse error payload = %+v", parseErr)
	}
	if got, want := parseErr.Error(), `Type parse error at col 8: expected a type, found ")"`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	_, err = ParseType("Int @ Bool")
	if !errors.As(err, &parseErr) || parseErr.Column() != 5 || parseErr.Message != "unexpected character '@'" {
		t.Fatalf("lexer error = %T %+v", err, err)
	}
}

func TestTypeTokenReaderCursorAndPositions(t *testing.T) {
	tokens, err := lexType(" Int -> Bool")
	if err != nil {
		t.Fatal(err)
	}
	wantKinds := []typeTokenKind{typeTokenIdentifier, typeTokenArrow, typeTokenIdentifier, typeTokenEOF}
	wantText := []string{"Int", "->", "Bool", ""}
	wantOffsets := []int{1, 5, 8, 12}
	if len(tokens) != len(wantKinds) {
		t.Fatalf("tokens = %+v", tokens)
	}
	for index := range tokens {
		if tokens[index].kind != wantKinds[index] || tokens[index].text != wantText[index] || tokens[index].offset != wantOffsets[index] {
			t.Errorf("token %d = %+v", index, tokens[index])
		}
	}

	reader := &typeParser{tokens: tokens}
	if reader.peek() != tokens[0] || !reader.accept(typeTokenIdentifier) || reader.peek() != tokens[1] {
		t.Fatal("reader did not expose first/rest semantics")
	}
	if reader.accept(typeTokenIdentifier) || reader.peek() != tokens[1] {
		t.Fatal("failed accept consumed a token")
	}
	if !reader.accept(typeTokenArrow) || !reader.accept(typeTokenIdentifier) || reader.peek().kind != typeTokenEOF {
		t.Fatal("reader did not reach the end token")
	}
}

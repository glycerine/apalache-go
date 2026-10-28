package apalache

import (
	"errors"
	"strings"
	"testing"
)

func TestTLCConfigReaderEntryPoint(t *testing.T) {
	config, err := ParseTLCConfigReader(strings.NewReader("INIT Init NEXT Next"))
	if err != nil {
		t.Fatal(err)
	}
	if config.Behavior != (InitNextSpec{Init: "Init", Next: "Next"}) {
		t.Fatalf("behavior = %#v", config.Behavior)
	}
}

func TestTLCConfigParseErrorScalaTextAndPayload(t *testing.T) {
	_, err := ParseTLCConfig("\n INIT NEXT Next")
	var parseErr *TLCConfigParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("got %T, want TLCConfigParseError", err)
	}
	if parseErr.Line != 2 || parseErr.Column != 7 || parseErr.Message != "expected ident, found NEXT" {
		t.Fatalf("parse error = %+v", parseErr)
	}
	if got, want := parseErr.Error(), "Config parse error at 2.7: expected ident, found NEXT"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	_, err = ParseTLCConfig("(* unterminated")
	if !errors.As(err, &parseErr) || parseErr.Line != 1 || parseErr.Column != 1 || parseErr.Message != "unterminated comment" {
		t.Fatalf("lexer error = %T %v", err, err)
	}
}

func TestTLCConfigTokenReaderFirstRestEndAndPositions(t *testing.T) {
	tokens, err := lexTLCConfig("\n INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	wantKinds := []string{"INIT", "ident", "NEXT", "ident", "eof"}
	wantText := []string{"INIT", "Init", "NEXT", "Next", ""}
	wantLines := []int{2, 2, 2, 2, 2}
	wantColumns := []int{2, 7, 12, 17, 21}
	if len(tokens) != len(wantKinds) {
		t.Fatalf("tokens = %+v", tokens)
	}
	for index, token := range tokens {
		if token.kind != wantKinds[index] || token.text != wantText[index] || token.line != wantLines[index] || token.column != wantColumns[index] {
			t.Errorf("token %d = %+v", index, token)
		}
	}

	reader := &tlcConfigParser{tokens: tokens}
	if reader.current() != tokens[0] {
		t.Fatal("reader first differs from token head")
	}
	if _, err := reader.take("ident"); err == nil || reader.current() != tokens[0] {
		t.Fatal("failed read consumed a token")
	}
	if token, err := reader.take("INIT"); err != nil || token != tokens[0] || reader.current() != tokens[1] {
		t.Fatal("reader rest did not advance")
	}
	for reader.current().kind != "eof" {
		reader.at++
	}
	if reader.current().line != 2 || reader.current().column != 21 {
		t.Fatal("end position differs from lexer end")
	}
}

package apalache

import (
	"errors"
	"reflect"
	"testing"
)

func TestTLCConfigLexerEmitsEveryScalaTokenVariant(t *testing.T) {
	input := "CONSTANT CONSTANTS INIT NEXT SPECIFICATION INVARIANT INVARIANTS PROPERTY PROPERTIES " +
		"CONSTRAINT CONSTRAINTS ACTION_CONSTRAINT ACTION_CONSTRAINTS SYMMETRY VIEW ALIAS POSTCONDITION " +
		"CHECK_DEADLOCK <- = { } , TRUE FALSE Name_1 123 -42 \"all ~!@#$%^&*\\-+=|(){}[],:;`'<>.?/\""
	tokens, err := lexTLCConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	wantKinds := []string{
		"CONSTANT", "CONSTANT", "INIT", "NEXT", "SPECIFICATION", "INVARIANT", "INVARIANT", "PROPERTY", "PROPERTY",
		"CONSTRAINT", "CONSTRAINT", "ACTION_CONSTRAINT", "ACTION_CONSTRAINT", "SYMMETRY", "VIEW", "ALIAS",
		"POSTCONDITION", "CHECK_DEADLOCK", "<-", "=", "{", "}", ",", "boolean", "boolean", "ident", "number", "number", "string", "eof",
	}
	if len(tokens) != len(wantKinds) {
		t.Fatalf("token count = %d, want %d: %#v", len(tokens), len(wantKinds), tokens)
	}
	for index, kind := range wantKinds {
		if tokens[index].kind != kind {
			t.Errorf("token %d kind = %q, want %q", index, tokens[index].kind, kind)
		}
	}
	wantStrings := map[int]string{
		0: "CONSTANT", 18: "<-", 23: "boolean 'TRUE'", 25: "identifier 'Name_1'",
		26: "number '123'", 27: "number '-42'", 28: "string \"all ~!@#$%^&*\\-+=|(){}[],:;`'<>.?/\"",
	}
	for index, want := range wantStrings {
		if got := tokens[index].String(); got != want {
			t.Errorf("token %d string = %q, want %q", index, got, want)
		}
	}
}

func TestTLCConfigLexerCommentsWhitespaceAndPositions(t *testing.T) {
	input := " \t\r\f\\* one line\n(* two\n lines *)\nINIT Init NEXT Next"
	tokens, err := lexTLCConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []tlcToken{
		{kind: "INIT", text: "INIT", line: 4, column: 1},
		{kind: "ident", text: "Init", line: 4, column: 6},
		{kind: "NEXT", text: "NEXT", line: 4, column: 11},
		{kind: "ident", text: "Next", line: 4, column: 16},
		{kind: "eof", line: 4, column: 20},
	}
	if !reflect.DeepEqual(tokens, want) {
		t.Fatalf("tokens = %#v, want %#v", tokens, want)
	}
}

func TestTLCConfigLexerRejectsOutsideScalaGrammar(t *testing.T) {
	cases := []struct {
		input  string
		line   int
		column int
	}{
		{"INIT Α NEXT Next", 1, 6},
		{"INIT\vInit NEXT Next", 1, 5},
		{"CONSTANT S = \"tab\tinside\"", 1, 14},
		{"CONSTANT N = +1", 1, 14},
		{"(* unterminated", 1, 1},
		{"CONSTANT S = \"unterminated", 1, 14},
	}
	for _, test := range cases {
		_, err := lexTLCConfig(test.input)
		var parseError *TLCConfigParseError
		if !errors.As(err, &parseError) || parseError.Line != test.line || parseError.Column != test.column {
			t.Errorf("%q => %T %v, want %d.%d", test.input, err, err, test.line, test.column)
		}
	}
}

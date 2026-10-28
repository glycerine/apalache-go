package apalache

import (
	"errors"
	"fmt"
	"testing"
)

func TestJSONVersionContract(t *testing.T) {
	if ApalacheIRJSONMajor != 1 || ApalacheIRJSONMinor != 0 || CurrentJSONVersion() != "1.0" {
		t.Fatalf("JSON version = %d.%d / %q", ApalacheIRJSONMajor, ApalacheIRJSONMinor, CurrentJSONVersion())
	}
}

func TestJSONDeserializationErrorMessageCauseAndIRBoundary(t *testing.T) {
	cause := errors.New("cause")
	err := NewJSONDeserializationError("message", cause)
	if err.Error() != "message" || !errors.Is(err, cause) {
		t.Fatalf("typed JSON error = %v, cause %v", err, errors.Unwrap(err))
	}

	_, decodeErr := UnmarshalIRJSON([]byte(`{"version":"2.0","modules":[]}`), nil)
	var typed *JSONDeserializationError
	if !errors.As(decodeErr, &typed) || typed.Error() != "JSON version is 2.0, expected 1.0" || errors.Unwrap(typed) == nil {
		t.Fatalf("IR boundary returned %T: %v", decodeErr, decodeErr)
	}
}

func TestTLAType1TagPrinterReaderAndUntypedReader(t *testing.T) {
	printer := TLAType1TagPrinter{}
	setInt := newSetTypeUnchecked(IntT1)
	for _, tag := range []TypeTag{Untyped(), Typed(IntT1), Typed(setInt)} {
		text := printer.PrintTypeTag(tag)
		parsed, err := ReadTLAType1Tag(text)
		if err != nil || !parsed.Equal(tag) {
			t.Fatalf("tag %s -> %q -> %s, %v", tag, text, parsed, err)
		}
	}
	if got := printer.PrintTypeTag(Typed(IntT1)); got != "Int" {
		t.Fatalf("typed tag has wrapper text: %q", got)
	}
	if got := printer.PrintTypeTag(Untyped()); got != "Untyped" {
		t.Fatalf("untyped tag = %q", got)
	}
	if _, err := ReadTLAType1Tag("not a type ("); err == nil {
		t.Fatal("accepted malformed type tag")
	}
	erased, err := (UntypedTagReader{}).ReadTypeTag(fmt.Sprintf("%s", setInt))
	if err != nil || erased.IsTyped() {
		t.Fatalf("untyped reader returned %s, %v", erased, err)
	}
}

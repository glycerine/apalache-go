package apalache

import (
	"errors"
	"testing"
)

func TestPreprocessingExceptionHierarchyMatchesScala(t *testing.T) {
	expression := NewNameExpr("x", Untyped())
	for _, err := range []error{
		NewNotInKeraError("not Kera", expression),
		NewOverridingError("bad override", expression),
	} {
		var malformed *MalformedTlaError
		if !errors.As(err, &malformed) || malformed.CauseExpr() != expression {
			t.Errorf("%T did not inherit MalformedTlaError", err)
		}
	}
	withoutSource := NewTlaInputError("input")
	withSource := NewTlaInputError("input", expression.ID())
	if withoutSource.SourceID != nil || withSource.SourceID == nil || *withSource.SourceID != expression.ID() {
		t.Fatalf("TLA input source IDs = %v, %v", withoutSource.SourceID, withSource.SourceID)
	}
	if got := NewQuintUnsupportedError("lambda").Error(); got != "Unsupported quint input: lambda" {
		t.Fatalf("Quint unsupported error = %q", got)
	}
}

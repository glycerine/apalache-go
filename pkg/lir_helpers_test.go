package apalache

import "testing"

func TestFindBodyOfMatchesScala(t *testing.T) {
	firstBody := NewValueExpr(Int64Value(1), Untyped())
	secondBody := NewValueExpr(Int64Value(2), Untyped())
	first := NewOperDecl("Target", nil, firstBody, Untyped())
	second := NewOperDecl("Target", nil, secondBody, Untyped())
	other := NewOperDecl("Other", nil, secondBody, Untyped())

	if got := FindBodyOf("Target", other, first, second); got != firstBody {
		t.Fatalf("found body = %v, want first matching operator body", got)
	}
	if got := FindBodyOf("Missing", first); got != NullExpr() {
		t.Fatalf("missing body = %v, want NullExpr", got)
	}
	if got := FindBodyOf("Target"); got != NullExpr() {
		t.Fatalf("empty declaration body = %v, want NullExpr", got)
	}
	if got := FindBodyOf("Target", nil, first); got != firstBody {
		t.Fatalf("nil declaration prevented lookup: %v", got)
	}

	// Scala uses find(name) before filtering by declaration kind, so a
	// same-named non-operator shadows later operators for this helper.
	constant := NewConstDecl("Target", Untyped())
	if got := FindBodyOf("Target", constant, first); got != NullExpr() {
		t.Fatalf("same-named non-operator yielded %v, want NullExpr", got)
	}
}

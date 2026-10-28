package apalache

import "testing"

func untypedSingleton(element Expr) Expr {
	return MustOperExpr(Op("SET_ENUM"), Untyped(), element)
}

func TestChangeListenerTracksDirectTransformation(t *testing.T) {
	listener := NewChangeListener()
	x, y := NewNameExpr("x", Untyped()), NewNameExpr("y", Untyped())
	listener.OnExpressionTransformation(x, y)
	if !listener.IsDefinedAt(y.ID()) || listener.TraceBack(y.ID()) != x.ID() {
		t.Fatalf("traceBack(%s) = %s, want %s", y.ID(), listener.TraceBack(y.ID()), x.ID())
	}
}

func TestChangeListenerTracksParentAndChildTransformations(t *testing.T) {
	listener := NewChangeListener()
	x, y := NewNameExpr("x", Untyped()), NewNameExpr("y", Untyped())
	listener.OnExpressionTransformation(x, y)
	setOfX, setOfY := untypedSingleton(x), untypedSingleton(y)
	listener.OnExpressionTransformation(setOfX, setOfY)
	if listener.TraceBack(setOfY.ID()) != setOfX.ID() || listener.TraceBack(y.ID()) != x.ID() {
		t.Fatalf("parent/child traces = %s, %s", listener.TraceBack(setOfY.ID()), listener.TraceBack(y.ID()))
	}
}

func TestChangeListenerDescendantInheritsTransformedAncestor(t *testing.T) {
	listener := NewChangeListener()
	x, y := NewNameExpr("x", Untyped()), NewNameExpr("y", Untyped())
	setOfX, setOfY := untypedSingleton(x), untypedSingleton(y)
	listener.OnExpressionTransformation(setOfX, setOfY)
	if listener.TraceBack(setOfY.ID()) != setOfX.ID() || listener.TraceBack(y.ID()) != setOfX.ID() {
		t.Fatalf("inherited traces = %s, %s; want %s", listener.TraceBack(setOfY.ID()), listener.TraceBack(y.ID()), setOfX.ID())
	}
}

func TestChangeListenerKeepsSpecificDescendantOrigin(t *testing.T) {
	listener := NewChangeListener()
	x, y := NewNameExpr("x", Untyped()), NewNameExpr("y", Untyped())
	setOfX, setOfY := untypedSingleton(x), untypedSingleton(y)
	outerX, outerY := untypedSingleton(setOfX), untypedSingleton(setOfY)
	listener.OnExpressionTransformation(x, y)
	listener.OnExpressionTransformation(outerX, outerY)
	if listener.TraceBack(outerY.ID()) != outerX.ID() || listener.TraceBack(setOfY.ID()) != outerX.ID() || listener.TraceBack(y.ID()) != x.ID() {
		t.Fatalf("nested traces = outer %s, set %s, child %s", listener.TraceBack(outerY.ID()), listener.TraceBack(setOfY.ID()), listener.TraceBack(y.ID()))
	}
}

func TestChangeListenerTracksDeclarations(t *testing.T) {
	listener := NewChangeListener()
	original := NewConstDecl("A", Untyped())
	transformed := NewConstDecl("B", Untyped())
	listener.OnDeclarationTransformation(original, transformed)
	if listener.TraceBack(transformed.ID()) != original.ID() {
		t.Fatalf("declaration trace = %s, want %s", listener.TraceBack(transformed.ID()), original.ID())
	}
}

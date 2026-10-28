package apalache

import (
	"reflect"
	"testing"
)

func TestBodyMapFactoryMatchesScala(t *testing.T) {
	one := NewOperDecl("B", nil, NewNameExpr("one", Untyped()), Untyped())
	two := NewOperDecl("A", nil, NewNameExpr("two", Untyped()), Untyped())
	duplicate := NewOperDecl("B", nil, NewNameExpr("duplicate", Untyped()), Untyped())
	constant := NewConstDecl("C", Untyped())

	empty := NewBodyMap()
	withOne := BodyMapFromDeclaration(one, empty)
	if empty.Len() != 0 || withOne.Len() != 1 {
		t.Fatal("body-map update mutated its initial map")
	}
	result := BodyMapFromDeclarations([]Decl{one, constant, two, duplicate, nil}, empty)
	if !reflect.DeepEqual(result.Keys(), []string{"A", "B"}) {
		t.Fatalf("sorted body-map keys = %v", result.Keys())
	}
	declarations := result.Declarations()
	if len(declarations) != 2 || declarations[0] != two || declarations[1] != one {
		t.Fatalf("sorted body-map declarations = %v", declarations)
	}
	if got, ok := result.Get("B"); !ok || got != one {
		t.Fatalf("duplicate declaration replaced first entry: %v, %v", got, ok)
	}
	if _, ok := result.Get("C"); ok {
		t.Fatal("non-operator declaration was inserted")
	}

	seed := BodyMapFromDeclaration(two, NewBodyMap())
	seeded := BodyMapFromDeclarations([]Decl{one}, seed)
	if seed.Len() != 1 || seeded.Len() != 2 {
		t.Fatal("initial body map was not preserved immutably")
	}
}

func TestSourceLocatorTracebackAndDefensiveSourceMap(t *testing.T) {
	original := NewNameExpr("x", Untyped())
	transformed := MustOperExpr(Op("NOT"), Untyped(), NewNameExpr("p", Untyped()))
	location := SourceLocation{Filename: "M.tla", Region: NewSourceRegion(2, 3, 2, 8)}
	sources := SourceMap{original.ID(): location}
	changes := NewChangeListener()
	changes.OnExpressionTransformation(original, transformed)
	locator := NewSourceLocator(sources, changes)
	delete(sources, original.ID())

	for _, expression := range []Expr{transformed, transformed.Arguments()[0]} {
		got, found := locator.SourceOfExpression(expression)
		if !found || got != location {
			t.Errorf("source of %s = %v, %v", expression.ID(), got, found)
		}
	}
	if got, found := locator.SourceOfID(original.ID()); !found || got != location {
		t.Fatalf("source of original = %v, %v", got, found)
	}
	if _, found := locator.SourceOfExpression(nil); found {
		t.Fatal("nil expression had a source")
	}
}

func TestSourceLocatorConsistencyMatchesScalaTraversal(t *testing.T) {
	left := NewNameExpr("left", Untyped())
	right := NewNameExpr("right", Untyped())
	nested := MustOperExpr(Op("AND"), Untyped(), left, right)
	definitionBody := NewNameExpr("definition", Untyped())
	declaration := NewOperDecl("D", nil, definitionBody, Untyped())
	root := NewLetInExpr(nested, Untyped(), declaration)
	location := SourceLocation{Filename: "M", Region: NewSourceRegion(1, 1, 1, 20)}

	locator := NewSourceLocator(SourceMap{
		root.ID(): location, nested.ID(): location, left.ID(): location, declaration.ID(): location,
	}, nil)
	if got, want := locator.MissingSourceIDs(root), []UID{right.ID(), definitionBody.ID()}; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing source IDs = %v, want %v", got, want)
	}

	// Once the root is missing, its descendants are intentionally not visited.
	missingRoot := NewSourceLocator(nil, nil)
	if got, want := missingRoot.MissingSourceIDs(root), []UID{root.ID()}; !reflect.DeepEqual(got, want) {
		t.Fatalf("topmost missing source IDs = %v, want %v", got, want)
	}
	missingRoot.CheckConsistencyDeclaration(NewOperDecl("Root", nil, root, Untyped()))
}

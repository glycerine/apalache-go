package apalache

import (
	"sync"
	"testing"
)

func TestRegionTreeNestingDuplicatesAndLookup(t *testing.T) {
	tree := NewRegionTree()
	inner := NewSourceRegion(2, 10, 3, 10)
	innerIndex, err := tree.Add(inner)
	if err != nil {
		t.Fatal(err)
	}
	outer := NewSourceRegion(1, 1, 4, 1)
	outerIndex, err := tree.Add(outer)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Size() != 2 || innerIndex == outerIndex {
		t.Fatalf("tree size/indices = %d, %d, %d", tree.Size(), innerIndex, outerIndex)
	}
	duplicateIndex, err := tree.Add(inner)
	if err != nil || duplicateIndex != innerIndex || tree.Size() != 2 {
		t.Fatalf("duplicate add = %d, %v, size %d", duplicateIndex, err, tree.Size())
	}
	if got, ok := tree.Region(innerIndex); !ok || got != inner {
		t.Fatalf("Region(%d) = %v, %v", innerIndex, got, ok)
	}
	if _, ok := tree.Region(999); ok {
		t.Fatal("nonexistent region index was found")
	}
}

func TestRegionTreeRejectsPartialOverlap(t *testing.T) {
	tree := NewRegionTree()
	if _, err := tree.Add(NewSourceRegion(1, 10, 3, 10)); err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Add(NewSourceRegion(1, 20, 5, 20)); err == nil {
		t.Fatal("partially overlapping region accepted")
	}
}

func TestSourceStoreRecursiveAddDoesNotOverwrite(t *testing.T) {
	store := NewSourceStore()
	one := NewValueExpr(Int64Value(1), Untyped())
	set := MustOperExpr(Op("SET_ENUM"), Untyped(), one)
	outer := MustOperExpr(Op("SET_ENUM"), Untyped(), set)
	innerLocation := SourceLocation{Filename: "nested", Region: NewSourceRegion(100, 2, 100, 3)}
	if err := store.AddRecursive(one, innerLocation); err != nil {
		t.Fatal(err)
	}
	outerLocation := SourceLocation{Filename: "nested", Region: NewSourceRegion(1, 2, 200, 4)}
	if err := store.AddRecursive(outer, outerLocation); err != nil {
		t.Fatal(err)
	}
	for _, expr := range []Expr{outer, set} {
		if location, ok := store.Find(expr.ID()); !ok || location != outerLocation {
			t.Errorf("location for %s = %v, %v", expr.ID(), location, ok)
		}
	}
	if location, ok := store.Find(one.ID()); !ok || location != innerLocation {
		t.Fatalf("inner location overwritten: %v, %v", location, ok)
	}
	if len(store.SourceMap()) != 3 {
		t.Fatalf("source map size = %d", len(store.SourceMap()))
	}
}

func TestSourceStoreTransformationPropagation(t *testing.T) {
	store := NewSourceStore()
	originalChild := NewNameExpr("x", Untyped())
	original := MustOperExpr(Op("NOT"), Untyped(), originalChild)
	location := SourceLocation{Filename: "root", Region: NewSourceRegion(1, 2, 3, 4)}
	if err := store.AddRecursive(original, location); err != nil {
		t.Fatal(err)
	}
	newChild := NewNameExpr("y", Untyped())
	transformed := MustOperExpr(Op("NOT"), Untyped(), newChild)
	store.OnExpressionTransformation(original, transformed)
	for _, expr := range []Expr{transformed, newChild} {
		if got, ok := store.Find(expr.ID()); !ok || got != location {
			t.Errorf("propagated location = %v, %v", got, ok)
		}
	}

	copyOnly := NewNameExpr("z", Untyped())
	StoreCopyListener{Store: store}.OnExpressionTransformation(original, copyOnly)
	if got, ok := store.Find(copyOnly.ID()); !ok || got != location {
		t.Fatalf("copy-only location = %v, %v", got, ok)
	}
	if got, ok := store.FindOrWarn(original.ID()); !ok || got != location {
		t.Fatalf("FindOrWarn(existing) = %v, %v", got, ok)
	}

	originalDeclaration := NewOperDecl("A", nil, original, Untyped())
	transformedDeclaration := NewOperDecl("A", nil, transformed, Untyped())
	if err := store.Add(originalDeclaration.ID(), location); err != nil {
		t.Fatal(err)
	}
	store.OnDeclarationTransformation(originalDeclaration, transformedDeclaration)
	if got, ok := store.Find(transformedDeclaration.ID()); !ok || got != location {
		t.Fatalf("declaration propagation = %v, %v", got, ok)
	}
	copyDeclaration := NewOperDecl("A", nil, copyOnly, Untyped())
	StoreCopyListener{Store: store}.OnDeclarationTransformation(originalDeclaration, copyDeclaration)
	if got, ok := store.Find(copyDeclaration.ID()); !ok || got != location {
		t.Fatalf("copy declaration propagation = %v, %v", got, ok)
	}
}

func TestSourceStoreConcurrentAccess(t *testing.T) {
	store := NewSourceStore()
	location := SourceLocation{Filename: "parallel", Region: NewSourceRegion(1, 1, 1, 20)}
	expressions := make([]Expr, 64)
	for index := range expressions {
		expressions[index] = NewValueExpr(Int64Value(int64(index)), Untyped())
	}
	var wait sync.WaitGroup
	for _, expr := range expressions {
		wait.Add(1)
		go func(expr Expr) {
			defer wait.Done()
			if err := store.AddRecursive(expr, location); err != nil {
				t.Errorf("AddRecursive: %v", err)
			}
			if _, ok := store.Find(expr.ID()); !ok {
				t.Errorf("missing location for %s", expr.ID())
			}
		}(expr)
	}
	wait.Wait()
	if len(store.SourceMap()) != len(expressions) {
		t.Fatalf("source map size = %d, want %d", len(store.SourceMap()), len(expressions))
	}
}

func TestSourceLocatorPreservesEveryScalaTransformationShape(t *testing.T) {
	makeExpression := func() Expr {
		integer := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
		x := NewNameExpr("x", Typed(IntT1))
		y := NewNameExpr("y", Typed(IntT1))
		parameter := NewNameExpr("p", Typed(IntT1))
		localType := mustOperator([]Type{IntT1}, IntT1)
		local := NewOperDecl("A", []OperatorParam{ValueParam("p")},
			MustOperExpr(Op("PLUS"), Typed(IntT1), parameter, integer(1)), Typed(localType))
		application := MustOperExpr(Op("OPER_APP"), Typed(IntT1),
			NewNameExpr("A", Typed(localType)), integer(1))
		condition := MustOperExpr(Op("GE"), Typed(BoolT1), application, integer(0))
		nested := MustOperExpr(Op("AND"), Typed(BoolT1),
			MustOperExpr(Op("EQ"), Typed(BoolT1), x, x),
			MustOperExpr(Op("EQ"), Typed(BoolT1), y, y))
		body := MustOperExpr(Op("AND"), Typed(BoolT1),
			MustOperExpr(Op("EQ"), Typed(BoolT1), MustOperExpr(Op("PRIME"), Typed(IntT1), x), x),
			condition, nested)
		return NewLetInExpr(body, Typed(BoolT1), local)
	}

	collect := func(root Expr) []Expr {
		var result []Expr
		var visit func(Expr)
		visit = func(expression Expr) {
			result = append(result, expression)
			switch typed := expression.(type) {
			case *OperExpr:
				for _, argument := range typed.Arguments() {
					visit(argument)
				}
			case *LetInExpr:
				for _, declaration := range typed.Declarations() {
					visit(declaration.Body())
				}
				visit(typed.Body())
			}
		}
		visit(root)
		return result
	}

	tests := []struct {
		name      string
		transform func(TransformationTracker, Expr) (Expr, error)
	}{
		{"deep copy", func(tracker TransformationTracker, expression Expr) (Expr, error) {
			return NewDeepCopier(tracker).Expression(expression), nil
		}},
		{"flatten", func(tracker TransformationTracker, expression Expr) (Expr, error) {
			return FlattenBooleanConnectives(tracker)(expression), nil
		}},
		{"incremental renaming", func(tracker TransformationTracker, expression Expr) (Expr, error) {
			return NewIncrementalRenamer(tracker).Rename(expression), nil
		}},
		{"inline", func(tracker TransformationTracker, expression Expr) (Expr, error) {
			return NewInliner(tracker, NewIncrementalRenamer(tracker), true, nil).Transform(expression)
		}},
		{"no-op", func(tracker TransformationTracker, expression Expr) (Expr, error) {
			return RewriteExpressions(tracker, func(Expr) (Expr, bool) { return nil, false })(expression), nil
		}},
		{"prime", func(tracker TransformationTracker, expression Expr) (Expr, error) {
			return DecorateWithPrime(tracker, map[string]struct{}{"x": {}, "y": {}})(expression), nil
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := makeExpression()
			store := NewSourceStore()
			for index, expression := range collect(original) {
				line := index + 1
				if err := store.Add(expression.ID(), SourceLocation{
					Filename: "SourceLocator", Region: NewSourceRegion(line, 1, line, 20),
				}); err != nil {
					t.Fatal(err)
				}
			}
			transformed, err := test.transform(NewListenerTracker(store), original)
			if err != nil {
				t.Fatal(err)
			}
			for _, expression := range collect(transformed) {
				if _, found := store.Find(expression.ID()); !found {
					t.Errorf("transformed %T %s has no source location", expression, expression.ID())
				}
			}
		})
	}
}

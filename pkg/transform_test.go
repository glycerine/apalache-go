package apalache

import (
	"fmt"
	"math/rand"
	"testing"
)

type recordingListener struct {
	expressions  [][2]Expr
	declarations [][2]Decl
}

func (listener *recordingListener) OnExpressionTransformation(original, transformed Expr) {
	listener.expressions = append(listener.expressions, [2]Expr{original, transformed})
}
func (listener *recordingListener) OnDeclarationTransformation(original, transformed Decl) {
	listener.declarations = append(listener.declarations, [2]Decl{original, transformed})
}

func TestListenerTracker(t *testing.T) {
	listener := &recordingListener{}
	tracker := NewListenerTracker(listener)
	original := NewNameExpr("x", Untyped())
	transformed := tracker.TrackExpression(func(Expr) Expr { return NewNameExpr("y", Untyped()) })(original)
	if len(listener.expressions) != 1 || listener.expressions[0][0] != original || listener.expressions[0][1] != transformed {
		t.Fatal("expression listener did not receive transformation endpoints")
	}

	called := false
	unchanged := tracker.TrackTouch(func(input TouchExpr) TouchExpr {
		called = true
		return input
	})(TouchExpr{Expr: original})
	if called || unchanged.Changed {
		t.Fatal("touch tracker applied a transformation to an unchanged input")
	}
	result := tracker.TrackTouch(func(TouchExpr) TouchExpr {
		return TouchExpr{Expr: transformed, Changed: true}
	})(TouchExpr{Expr: original, Changed: true})
	if !result.Changed || len(listener.expressions) != 2 {
		t.Fatal("changed touch was not tracked")
	}
}

func TestTypeWatchdogTransformationListener(t *testing.T) {
	watchdog := TypeWatchdogTransformationListener{}
	watchdog.OnExpressionTransformation(NewNameExpr("x", Untyped()), NewNameExpr("x", Untyped()))
	watchdog.OnExpressionTransformation(NewNameExpr("x", Untyped()), NewNameExpr("x", Typed(IntT1)))
	watchdog.OnExpressionTransformation(NewNameExpr("x", Typed(IntT1)), NewNameExpr("x", Typed(BoolT1)))
	watchdog.OnDeclarationTransformation(NewConstDecl("X", Untyped()), NewConstDecl("X", Untyped()))
	watchdog.OnDeclarationTransformation(NewConstDecl("X", Untyped()), NewConstDecl("X", Typed(IntT1)))
	watchdog.OnDeclarationTransformation(NewConstDecl("X", Typed(IntT1)), NewConstDecl("X", Typed(BoolT1)))

	t.Run("expression", func(t *testing.T) {
		original := NewNameExpr("x", Typed(IntT1))
		transformed := NewNameExpr("x", Untyped())
		assertTypingPanic(t, transformed.ID(), "A typed expression "+original.ID().String()+
			" was transformed to an untyped expression "+transformed.ID().String(), func() {
			NewListenerTracker(watchdog).Hold(original, transformed)
		})
	})

	t.Run("declaration", func(t *testing.T) {
		original := NewConstDecl("Old", Typed(IntT1))
		transformed := NewConstDecl("New", Untyped())
		assertTypingPanic(t, transformed.ID(),
			"A typed declaration Old was transformed to an untyped expression New", func() {
				NewListenerTracker(watchdog).TrackDeclaration(func(Decl) Decl { return transformed })(original)
			})
	})
}

func TestTransformationTrackerProviderReturnsSharedComposedTracker(t *testing.T) {
	changes := NewChangeListener()
	provider := NewTransformationTrackerProvider(changes)
	first, second := provider.Get(), provider.Get()
	if first != second {
		t.Fatal("transformation tracker provider did not return its singleton")
	}

	original := NewNameExpr("x", Typed(IntT1))
	transformed := NewNameExpr("y", Typed(IntT1))
	if got := first.Hold(original, transformed); got != transformed {
		t.Fatal("shared tracker changed the transformation result")
	}
	if !changes.IsDefinedAt(transformed.ID()) || changes.TraceBack(transformed.ID()) != original.ID() {
		t.Fatal("shared tracker did not notify the change listener")
	}

	untyped := transformed.WithTag(Untyped())
	assertTypingPanic(t, untyped.ID(), "A typed expression "+original.ID().String()+
		" was transformed to an untyped expression "+untyped.ID().String(), func() {
		provider.Get().Hold(original, untyped)
	})
}

func assertTypingPanic(t *testing.T, cause UID, message string, action func()) {
	t.Helper()
	defer func() {
		recovered := recover()
		err, ok := recovered.(*TypingException)
		if !ok {
			t.Fatalf("panic = %T %v, want *TypingException", recovered, recovered)
		}
		if err.CauseExprID() != cause || err.Error() != message {
			t.Fatalf("typing panic = %q at %s, want %q at %s", err, err.CauseExprID(), message, cause)
		}
	}()
	action()
	t.Fatal("action did not panic")
}

func TestDeepCopyExpressionTree(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	plus := MustOperExpr(Op("PLUS"), Typed(IntT1), x, one)
	declaration := NewOperDecl("Inc", []OperatorParam{ValueParam("x")}, plus,
		Typed(mustOperatorType(t, []Type{IntT1}, IntT1)))
	declaration.SetRecursive(true)
	let := NewLetInExpr(NewNameExpr("Inc", Typed(mustOperatorType(t, []Type{IntT1}, IntT1))),
		Typed(mustOperatorType(t, []Type{IntT1}, IntT1)), declaration)

	listener := &recordingListener{}
	copy := NewDeepCopier(NewListenerTracker(listener)).Expression(let)
	if !EqualExpr(let, copy) || !let.Tag().Equal(copy.Tag()) {
		t.Fatal("deep copy changed expression structure or tags")
	}
	assertFreshExpressionTree(t, let, copy)
	copyLet := copy.(*LetInExpr)
	copyDecl := copyLet.Declarations()[0]
	if !copyDecl.IsRecursive() || copyDecl == declaration || copyDecl.Body() == declaration.Body() {
		t.Fatal("deep copy aliased or lost operator declaration state")
	}
	// name + value + operator + LET body name + LET = five expressions.
	if len(listener.expressions) != 5 || len(listener.declarations) != 1 {
		t.Fatalf("listener calls = %d expressions, %d declarations", len(listener.expressions), len(listener.declarations))
	}
}

func assertFreshExpressionTree(t *testing.T, original, copied Expr) {
	t.Helper()
	if original == NullExpr() {
		if copied != original {
			t.Fatal("null singleton was copied")
		}
		return
	}
	if original.ID() == copied.ID() {
		t.Fatalf("expression %T retained ID %s", original, original.ID())
	}
	if !original.Tag().Equal(copied.Tag()) {
		t.Fatalf("expression %T changed tag from %s to %s", original, original.Tag(), copied.Tag())
	}
	switch left := original.(type) {
	case *OperExpr:
		right := copied.(*OperExpr)
		leftArgs, rightArgs := left.Arguments(), right.Arguments()
		for index := range leftArgs {
			assertFreshExpressionTree(t, leftArgs[index], rightArgs[index])
		}
	case *LetInExpr:
		right := copied.(*LetInExpr)
		assertFreshExpressionTree(t, left.Body(), right.Body())
		leftDecls, rightDecls := left.Declarations(), right.Declarations()
		for index := range leftDecls {
			assertFreshDeclaration(t, leftDecls[index], rightDecls[index])
		}
	}
}

func assertFreshDeclaration(t *testing.T, original, copied Decl) {
	t.Helper()
	if original.ID() == copied.ID() || !EqualTypedDecl(original, copied) {
		t.Fatalf("declaration %T is not a fresh typed structural copy", original)
	}
	switch left := original.(type) {
	case *AssumeDecl:
		assertFreshExpressionTree(t, left.Body(), copied.(*AssumeDecl).Body())
	case *TheoremDecl:
		assertFreshExpressionTree(t, left.Body(), copied.(*TheoremDecl).Body())
	case *OperDecl:
		right := copied.(*OperDecl)
		if left.IsRecursive() != right.IsRecursive() {
			t.Fatal("operator declaration changed recursive flag")
		}
		assertFreshExpressionTree(t, left.Body(), right.Body())
	}
}

func TestDeepCopyDeclarationsAndModule(t *testing.T) {
	body := MustOperExpr(Op("EQ"), Typed(BoolT1),
		NewNameExpr("x", Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1)))
	declarations := []Decl{
		NewConstDecl("N", Typed(IntT1)),
		NewVarDecl("x", Typed(IntT1)),
		NewNamedAssumeDecl("Init", body, Typed(BoolT1)),
		NewTheoremDecl("Safety", body, Typed(BoolT1)),
		NewOperDecl("IsOne", nil, body, Typed(mustOperatorType(t, nil, BoolT1))),
	}
	module := NewModule("Copy", declarations)
	module.AddProperty(PropertyTypeChecked)
	copy := NewDeepCopier(IdleTracker{}).Module(module)
	if copy == module || copy.Name() != module.Name() || len(copy.Declarations()) != len(declarations) {
		t.Fatal("module copy has wrong identity or shape")
	}
	if copy.HasProperty(PropertyTypeChecked) {
		t.Fatal("deep copy unexpectedly retained Scala module mixin properties")
	}
	for index, declaration := range copy.Declarations() {
		if declaration.ID() == declarations[index].ID() || !EqualDecl(declaration, declarations[index]) || !declaration.Tag().Equal(declarations[index].Tag()) {
			t.Errorf("declaration %d is not a fresh structural copy", index)
		}
	}
}

func TestDeepCopyGeneratedDeclarationsHaveFreshIdentifiers(t *testing.T) {
	random := rand.New(rand.NewSource(0x5ca1a))
	var expression func(depth int) Expr
	expression = func(depth int) Expr {
		if depth == 0 || random.Intn(4) == 0 {
			if random.Intn(2) == 0 {
				return NewNameExpr(fmt.Sprintf("x%d", random.Intn(20)), Typed(IntT1))
			}
			return NewValueExpr(Int64Value(random.Int63n(100)), Typed(IntT1))
		}
		switch random.Intn(6) {
		case 0:
			return MustOperExpr(Op("PLUS"), Typed(IntT1), expression(depth-1), expression(depth-1))
		case 1:
			return MustOperExpr(Op("EQ"), Typed(BoolT1), expression(depth-1), expression(depth-1))
		case 2:
			return MustOperExpr(Op("NOT"), Typed(BoolT1), expression(depth-1))
		case 3:
			return MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1),
				expression(depth-1), expression(depth-1), expression(depth-1))
		case 4:
			return MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)),
				expression(depth-1), expression(depth-1), expression(depth-1))
		default:
			body := expression(depth - 1)
			declaration := NewOperDecl(fmt.Sprintf("Local%d", random.Intn(20)), nil,
				expression(depth-1), Typed(mustOperatorType(t, nil, IntT1)))
			declaration.SetRecursive(random.Intn(2) == 0)
			return NewLetInExpr(body, body.Tag(), declaration)
		}
	}

	for index := 0; index < 1000; index++ {
		name := fmt.Sprintf("d%d", index)
		body := expression(1 + random.Intn(5))
		var original Decl
		switch index % 5 {
		case 0:
			original = NewConstDecl(name, Typed(IntT1))
		case 1:
			original = NewVarDecl(name, Typed(IntT1))
		case 2:
			if index%2 == 0 {
				original = NewNamedAssumeDecl(name, body, Typed(BoolT1))
			} else {
				original = NewUnnamedAssumeDecl(body, Typed(BoolT1))
			}
		case 3:
			original = NewTheoremDecl(name, body, Typed(BoolT1))
		default:
			operator := NewOperDecl(name, []OperatorParam{ValueParam("x")}, body,
				Typed(mustOperatorType(t, []Type{IntT1}, IntT1)))
			operator.SetRecursive(index%2 == 0)
			original = operator
		}
		copied := NewDeepCopier(IdleTracker{}).Declaration(original)
		assertFreshDeclaration(t, original, copied)
	}
}

func TestExpressionToDeclarationTransformation(t *testing.T) {
	listener := &recordingListener{}
	tracker := NewListenerTracker(listener)
	originalBody := NewNameExpr("x", Untyped())
	declaration := NewOperDecl("A", nil, originalBody, Untyped())
	transform := DeclarationTransformFromExpression(tracker, tracker.TrackExpression(func(Expr) Expr {
		return NewNameExpr("y", Untyped())
	}))
	result := transform(declaration).(*OperDecl)
	if result == declaration || result.Body().(*NameExpr).Name() != "y" || len(listener.expressions) != 1 || len(listener.declarations) != 1 {
		t.Fatal("expression-to-declaration adapter did not copy and track both levels")
	}
}

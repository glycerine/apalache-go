package apalache

import "testing"

func TestReplaceEqualIsBottomUpAndFresh(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	nested := MustOperExpr(Op("PLUS"), Untyped(), x,
		MustOperExpr(Op("PLUS"), Untyped(), x, x))
	replacePair := ReplaceMatching(IdleTracker{}, func(expression Expr) bool {
		oper, ok := expression.(*OperExpr)
		return ok && oper.Operator().Name() == "PLUS" && EqualExpr(oper.Arguments()[0], oper.Arguments()[1])
	}, func() Expr { return NewNameExpr("x", Untyped()) })
	if got := replacePair(nested); !EqualExpr(got, x) {
		t.Fatalf("bottom-up replacement got %#v", got)
	}

	plus := MustOperExpr(Op("PLUS"), Untyped(), x, x)
	replaceNames := ReplaceEqual(IdleTracker{}, x, func() Expr { return NewNameExpr("y", Untyped()) })
	got := replaceNames(plus).(*OperExpr).Arguments()
	if !EqualExpr(got[0], got[1]) || got[0].ID() == got[1].ID() {
		t.Fatalf("replacements are not structurally equal with fresh IDs: %v %v", got[0].ID(), got[1].ID())
	}
}

func TestReplaceEqualRewritesLetInBodies(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	declaration := NewOperDecl("A", nil, x, Untyped())
	input := NewLetInExpr(untypedApp("A"), Untyped(), declaration)
	got := ReplaceEqual(IdleTracker{}, x, func() Expr { return NewNameExpr("y", Untyped()) })(input).(*LetInExpr)
	if body := got.Declarations()[0].Body(); !EqualExpr(body, NewNameExpr("y", Untyped())) {
		t.Fatalf("LET-IN declaration body was not replaced: %#v", body)
	}
}

func TestReplaceFixedMatchesScalaCases(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	y := NewNameExpr("y", Untyped())
	z := NewNameExpr("z", Untyped())
	p := NewNameExpr("p", Untyped())
	prime := func(expression Expr) Expr { return MustOperExpr(Op("PRIME"), Untyped(), expression) }
	replace := ReplaceEqual(IdleTracker{}, x, func() Expr { return NewNameExpr("y", Untyped()) })

	for name, pair := range map[string][2]Expr{
		"basic":     {x, y},
		"unmatched": {z, z},
		"nested": {MustOperExpr(Op("PLUS"), Untyped(), x, z),
			MustOperExpr(Op("PLUS"), Untyped(), y, z)},
		"prime": {prime(x), prime(y)},
		"conditional": {MustOperExpr(Op("IF_THEN_ELSE"), Untyped(), p, x, y),
			MustOperExpr(Op("IF_THEN_ELSE"), Untyped(), p, y, y)},
	} {
		t.Run(name, func(t *testing.T) {
			if got := replace(pair[0]); !EqualExpr(got, pair[1]) {
				t.Fatalf("replacement = %#v, want %#v", got, pair[1])
			}
		})
	}

	primeX := prime(x)
	three := intLiteral(3)
	assignment := MustOperExpr(Op("Apalache!:="), Untyped(), primeX, three)
	equality := MustOperExpr(Op("EQ"), Untyped(), primeX, three)
	partial := RewriteExpressions(IdleTracker{}, func(expression Expr) (Expr, bool) {
		operator, ok := expression.(*OperExpr)
		if !ok || operator.Operator().Name() != "Apalache!:=" {
			return nil, false
		}
		arguments := operator.Arguments()
		return MustOperExpr(Op("EQ"), operator.Tag(), arguments...), true
	})
	actualPartial := partial(MustOperExpr(Op("AND"), Untyped(), assignment, equality))
	if !EqualExpr(actualPartial, MustOperExpr(Op("AND"), Untyped(), equality, equality)) {
		t.Fatalf("partial replacement = %#v", actualPartial)
	}

	aDecl := NewOperDecl("A", nil, x, Untyped())
	let := NewLetInExpr(untypedApp("A"), Untyped(), aDecl)
	expectedA := NewOperDecl("A", nil, y, Untyped())
	if got := replace(let); !EqualExpr(got, NewLetInExpr(untypedApp("A"), Untyped(), expectedA)) {
		t.Fatalf("LET replacement = %#v", got)
	}

	unchangedDecl := NewOperDecl("A", nil, NewNameExpr("q", Untyped()), Untyped())
	unchanged := NewLetInExpr(MustOperExpr(Op("PLUS"), Untyped(), z, untypedApp("A")), Untyped(), unchangedDecl)
	if got := replace(unchanged); got != unchanged {
		t.Fatal("unchanged LET was unnecessarily rebuilt")
	}

	bDecl := NewOperDecl("B", []OperatorParam{ValueParam("p")}, p, Untyped())
	multiple := NewLetInExpr(MustOperExpr(Op("SET_ENUM"), Untyped(),
		MustOperExpr(Op("PLUS"), Untyped(), x, untypedApp("A")), untypedApp("B", x)),
		Untyped(), aDecl, bDecl)
	wantMultiple := NewLetInExpr(MustOperExpr(Op("SET_ENUM"), Untyped(),
		MustOperExpr(Op("PLUS"), Untyped(), y, untypedApp("A")), untypedApp("B", y)),
		Untyped(), expectedA, NewOperDecl("B", []OperatorParam{ValueParam("p")}, p, Untyped()))
	if got := replace(multiple); !EqualExpr(got, wantMultiple) {
		t.Fatalf("multi-declaration LET replacement = %#v", got)
	}
}

func TestReplaceFixedTracksUnchangedLetDeclarations(t *testing.T) {
	listener := &recordingListener{}
	declaration := NewOperDecl("A", nil, NewNameExpr("q", Untyped()), Untyped())
	let := NewLetInExpr(untypedApp("A"), Untyped(), declaration)
	pattern := NewNameExpr("x", Untyped())
	got := ReplaceEqual(NewListenerTracker(listener), pattern,
		func() Expr { return NewNameExpr("y", Untyped()) })(let)
	if got != let || len(listener.declarations) != 1 || listener.declarations[0][0] != declaration ||
		listener.declarations[0][1].ID() == declaration.ID() {
		t.Fatalf("unchanged LET tracking = %#v, result identity changed=%v", listener.declarations, got != let)
	}
}

func TestFlattenBooleanConnectives(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	y := NewNameExpr("y", Untyped())
	z := NewNameExpr("z", Untyped())
	nested := MustOperExpr(Op("AND"), Typed(BoolT1), x,
		MustOperExpr(Op("AND"), Typed(BoolT1), y, z))
	got := FlattenBooleanConnectives(IdleTracker{})(nested).(*OperExpr)
	if !got.Tag().Equal(nested.Tag()) || len(got.Arguments()) != 3 {
		t.Fatalf("flatten got tag=%s and %d arguments", got.Tag(), len(got.Arguments()))
	}
	for index, want := range []Expr{x, y, z} {
		if !EqualExpr(got.Arguments()[index], want) {
			t.Fatalf("argument %d changed", index)
		}
	}

	setTag := Typed(mustSetType(t, IntT1))
	parent := MustOperExpr(Op("SET_ENUM"), setTag, nested)
	parentGot := FlattenBooleanConnectives(IdleTracker{})(parent).(*OperExpr)
	if !parentGot.Tag().Equal(setTag) || !parentGot.Arguments()[0].Tag().Equal(Typed(BoolT1)) {
		t.Fatalf("reconstruction lost node-specific tags: parent=%s child=%s", parentGot.Tag(), parentGot.Arguments()[0].Tag())
	}
}

func TestPropagatePrimes(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	y := NewNameExpr("y", Untyped())
	n := NewNameExpr("N", Untyped())
	prime := func(expression Expr) Expr { return MustOperExpr(Op("PRIME"), expression.Tag(), expression) }
	application := MustOperExpr(Op("FUN_APP"), Untyped(), x, y)
	stateVariables := map[string]struct{}{"x": {}, "y": {}}
	transform := PropagatePrimes(IdleTracker{}, stateVariables)

	want := MustOperExpr(Op("FUN_APP"), Untyped(), prime(x), prime(y))
	if got := transform(prime(application)); !EqualExpr(got, want) {
		t.Fatalf("prime propagation got %#v, want %#v", got, want)
	}
	if got := transform(prime(n)); !EqualExpr(got, n) {
		t.Fatalf("constant remained primed: %#v", got)
	}
	literal := NewValueExpr(Int64Value(2021), Typed(IntT1))
	if got := transform(prime(literal)); !EqualExpr(got, literal) {
		t.Fatalf("literal remained primed: %#v", got)
	}
}

func TestPropagatePrimesThroughLetIn(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	y := NewNameExpr("y", Untyped())
	application := MustOperExpr(Op("FUN_APP"), Untyped(), x, y)
	declaration := NewOperDecl("Foo", nil, application, Untyped())
	letIn := NewLetInExpr(untypedApp("Foo"), Untyped(), declaration)
	input := MustOperExpr(Op("PRIME"), Untyped(), letIn)
	got := PropagatePrimes(IdleTracker{}, map[string]struct{}{"x": {}, "y": {}})(input).(*LetInExpr)
	for _, argument := range got.Declarations()[0].Body().(*OperExpr).Arguments() {
		oper, ok := argument.(*OperExpr)
		if !ok || oper.Operator().Name() != "PRIME" {
			t.Fatalf("LET-IN body argument was not primed: %#v", argument)
		}
	}
}

func TestDecorateWithPrimeIsIdempotent(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), x)
	input := MustOperExpr(Op("PLUS"), Typed(IntT1), x, prime)
	got := DecorateWithPrime(IdleTracker{}, map[string]struct{}{"x": {}})(input).(*OperExpr).Arguments()
	for index, argument := range got {
		outer, ok := argument.(*OperExpr)
		if !ok || outer.Operator().Name() != "PRIME" {
			t.Fatalf("argument %d is not singly primed: %#v", index, argument)
		}
		if _, doubled := outer.Arguments()[0].(*OperExpr); doubled {
			t.Fatalf("argument %d was double-primed", index)
		}
	}
}

func TestTransformModuleExpressions(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	operator := NewOperDecl("Op", nil, x, Untyped())
	assumption := NewUnnamedAssumeDecl(x, Untyped())
	module := NewModule("M", []Decl{operator, assumption})
	module.AddProperty(PropertyConfigured)
	transform := ReplaceEqual(IdleTracker{}, x, func() Expr { return NewNameExpr("y", Untyped()) })
	got := TransformModuleExpressions(module, transform, func(declaration Decl) bool {
		_, isOperator := declaration.(*OperDecl)
		return isOperator
	})
	if !EqualExpr(got.OperDeclarations()[0].Body(), NewNameExpr("y", Untyped())) {
		t.Fatal("operator body was not transformed")
	}
	if got.AssumeDeclarations()[0].ID() != assumption.ID() || !got.HasProperty(PropertyConfigured) {
		t.Fatal("excluded declaration or module property was not preserved")
	}
}

func TestUnrollSimpleUnchanged(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(IntT1))
	tupleType, err := NewTupleType(IntT1, IntT1)
	if err != nil {
		t.Fatal(err)
	}
	tuple := MustOperExpr(Op("TUPLE"), Typed(tupleType), x, y)
	unchanged := MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), tuple)
	got := UnrollSimpleUnchanged(IdleTracker{})(unchanged).(*OperExpr)
	if got.Operator().Name() != "AND" || len(got.Arguments()) != 2 || !got.Tag().Equal(Typed(BoolT1)) {
		t.Fatalf("tuple UNCHANGED became %#v", got)
	}
	for _, equality := range got.Arguments() {
		eq := equality.(*OperExpr)
		if eq.Operator().Name() != "EQ" || eq.Arguments()[0].(*OperExpr).Operator().Name() != "PRIME" {
			t.Fatalf("unexpected equality shape: %#v", equality)
		}
		if eq.Arguments()[0].(*OperExpr).Arguments()[0].ID() == eq.Arguments()[1].ID() {
			t.Fatal("UNCHANGED expansion reused expression identity")
		}
	}
}

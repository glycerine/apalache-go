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

func TestFlattenBooleanConnectivesScalaLetAndTagRegressions(t *testing.T) {
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	nestedAnd := func() Expr {
		return MustOperExpr(Op("AND"), Typed(BoolT1),
			MustOperExpr(Op("AND"), Typed(BoolT1), truth, truth), truth)
	}
	nestedOr := func() Expr {
		return MustOperExpr(Op("OR"), Typed(BoolT1),
			MustOperExpr(Op("OR"), Typed(BoolT1), truth, truth), truth)
	}
	flatten := FlattenBooleanConnectives(IdleTracker{})

	local := NewOperDecl("A", nil, nestedOr(), Typed(mustOperatorType(t, nil, BoolT1)))
	let := NewLetInExpr(untypedApp("A"), Typed(BoolT1), local)
	flattenedLet := flatten(let).(*LetInExpr)
	if body := flattenedLet.Declarations()[0].Body().(*OperExpr); body.Operator().Name() != "OR" || len(body.Arguments()) != 3 {
		t.Fatal("nested LET declaration body was not flattened")
	}
	if !flattenedLet.Tag().Equal(let.Tag()) {
		t.Fatal("LET tag changed")
	}

	setInt := mustSetType(t, IntT1)
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	setOne := MustOperExpr(Op("SET_ENUM"), Typed(setInt), one)
	empty := MustOperExpr(Op("SET_ENUM"), Typed(setInt))
	variable := NewNameExpr("x", Typed(IntT1))
	filter := MustOperExpr(Op("SET_FILTER"), Typed(setInt), variable, setOne, nestedOr())
	flattenedFilter := flatten(filter).(*OperExpr)
	if !flattenedFilter.Tag().Equal(Typed(setInt)) || len(flattenedFilter.Arguments()[2].(*OperExpr).Arguments()) != 3 {
		t.Fatal("SET_FILTER lost its type or flattened predicate")
	}

	conditional := MustOperExpr(Op("IF_THEN_ELSE"), Typed(setInt), nestedAnd(), setOne, empty)
	flattenedConditional := flatten(conditional).(*OperExpr)
	if !flattenedConditional.Tag().Equal(Typed(setInt)) || len(flattenedConditional.Arguments()[0].(*OperExpr).Arguments()) != 3 {
		t.Fatal("IF-THEN-ELSE lost its result type or flattened condition")
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), one, conditional)
	flattenedMembership := flatten(membership).(*OperExpr)
	if !flattenedMembership.Tag().Equal(Typed(BoolT1)) ||
		!flattenedMembership.Arguments()[1].Tag().Equal(Typed(setInt)) {
		t.Fatal("rebuilt parent/child tags were conflated")
	}

	functionType := mustFunType(t, IntT1, BoolT1)
	function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType),
		MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), nestedAnd(), truth, truth), variable, setOne)
	if got := flatten(function); !got.Tag().Equal(Typed(functionType)) {
		t.Fatalf("function constructor tag = %v", got.Tag())
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
	operator.SetRecursive(true)
	assumption := NewUnnamedAssumeDecl(x, Untyped())
	namedAssumption := NewNamedAssumeDecl("Named", x, Typed(BoolT1))
	theorem := NewTheoremDecl("T", x, Untyped())
	constant := NewConstDecl("C", Untyped())
	module := NewModule("M", []Decl{operator, assumption, namedAssumption, theorem, constant})
	module.AddProperty(PropertyConfigured)
	transform := ReplaceEqual(IdleTracker{}, x, func() Expr { return NewNameExpr("y", Untyped()) })
	got := TransformModuleExpressions(module, transform, func(declaration Decl) bool {
		_, excluded := declaration.(*AssumeDecl)
		return !excluded
	})
	if !EqualExpr(got.OperDeclarations()[0].Body(), NewNameExpr("y", Untyped())) {
		t.Fatal("operator body was not transformed")
	}
	if !got.OperDeclarations()[0].IsRecursive() || got.OperDeclarations()[0].ID() == operator.ID() {
		t.Fatal("changed operator was not copied with its recursive flag")
	}
	if got.AssumeDeclarations()[0].ID() != assumption.ID() || got.AssumeDeclarations()[1].ID() != namedAssumption.ID() {
		t.Fatal("excluded assumptions were not retained by identity")
	}
	if got.Declarations()[3].ID() != theorem.ID() || got.Declarations()[4].ID() != constant.ID() {
		t.Fatal("unsupported declarations were not retained by identity")
	}
	if got.HasProperty(PropertyConfigured) {
		t.Fatal("fresh Scala module unexpectedly retained mixin properties")
	}
	if got == module || got.Name() != module.Name() {
		t.Fatal("transformer did not produce a fresh same-name module")
	}
}

func TestTransformModuleExpressionsAssumptionNamesAndIdentity(t *testing.T) {
	body := NewNameExpr("p", Typed(BoolT1))
	named := NewNamedAssumeDecl("Safety", body, Typed(BoolT1))
	unnamed := NewUnnamedAssumeDecl(body, Typed(BoolT1))
	module := NewModule("M", []Decl{named, unnamed})

	identity := TransformModuleExpressions(module, func(expression Expr) Expr { return expression }, nil)
	if identity.Declarations()[0].ID() != named.ID() || identity.Declarations()[1].ID() != unnamed.ID() {
		t.Fatal("identity transformation copied declarations")
	}

	changed := TransformModuleExpressions(module, func(Expr) Expr {
		return NewValueExpr(BoolValue(true), Typed(BoolT1))
	}, nil)
	changedNamed := changed.AssumeDeclarations()[0]
	if name, ok := changedNamed.DefinedName(); !ok || name != "Safety" || !changedNamed.Tag().Equal(Typed(BoolT1)) {
		t.Fatal("changed named assumption lost its name or tag")
	}
	if _, ok := changed.AssumeDeclarations()[1].DefinedName(); ok {
		t.Fatal("changed unnamed assumption acquired a name")
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

func TestUnrollSimpleUnchangedScalarRecursionShallowCopiesAndLetBarrier(t *testing.T) {
	leaf := NewNameExpr("x", Typed(IntT1))
	nested := MustOperExpr(Op("PLUS"), Typed(IntT1), leaf, NewValueExpr(Int64Value(1), Typed(IntT1)))
	unchanged := MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), nested)
	listener := &recordingListener{}
	got := UnrollSimpleUnchanged(NewListenerTracker(listener))(unchanged).(*OperExpr)
	if got.Operator().Name() != "EQ" || len(listener.expressions) != 1 || listener.expressions[0][0] != unchanged || listener.expressions[0][1] != got {
		t.Fatal("scalar UNCHANGED was not expanded and tracked")
	}
	leftCopy := got.Arguments()[0].(*OperExpr).Arguments()[0].(*OperExpr)
	rightCopy := got.Arguments()[1].(*OperExpr)
	if leftCopy.ID() == nested.ID() || rightCopy.ID() == nested.ID() || leftCopy.ID() == rightCopy.ID() {
		t.Fatal("Scala copy did not allocate two fresh roots")
	}
	if leftCopy.Arguments()[0] != leaf || rightCopy.Arguments()[0] != leaf {
		t.Fatal("Scala copy was deep instead of retaining nested children")
	}

	decl := NewOperDecl("Local", nil, unchanged, Typed(mustOperatorType(t, nil, BoolT1)))
	let := NewLetInExpr(unchanged, Typed(BoolT1), decl)
	if transformed := UnrollSimpleUnchanged(IdleTracker{})(let); transformed != let {
		t.Fatal("post-inlining unroller descended into LET-IN")
	}
}

func TestUnrollSimpleUnchangedRecursesThroughOrdinaryOperators(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	unchanged := MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), x)
	parent := MustOperExpr(Op("NOT"), Typed(BoolT1), unchanged)
	got := UnrollSimpleUnchanged(IdleTracker{})(parent).(*OperExpr)
	if got == parent || got.Operator().Name() != "NOT" || got.Arguments()[0].(*OperExpr).Operator().Name() != "EQ" {
		t.Fatalf("ordinary recursion produced %#v", got)
	}
	plain := MustOperExpr(Op("EQ"), Typed(BoolT1), x, x)
	if transformed := UnrollSimpleUnchanged(IdleTracker{})(plain); transformed != plain {
		t.Fatal("unchanged ordinary tree was copied")
	}
}

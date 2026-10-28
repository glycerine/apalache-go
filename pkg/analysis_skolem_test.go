package apalache

import "testing"

func TestMarkSkolemizableExpressionTracksPositivePolarity(t *testing.T) {
	setType := mustSet(IntT1)
	binder := NewNameExpr("i", Typed(IntT1))
	exists := func() Expr {
		return MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder,
			MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1)),
			MustOperExpr(Op("EQ"), Typed(BoolT1), binder, intLiteral(1)))
	}
	expression := MustOperExpr(Op("AND"), Typed(BoolT1),
		exists(),
		MustOperExpr(Op("NOT"), Typed(BoolT1), exists()),
		MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), exists(), exists(), exists()))
	marked := MarkSkolemizableExpression(expression, nil).(*OperExpr)
	if marked.Arguments()[0].(*OperExpr).Operator().Name() != "Apalache!Skolem" {
		t.Fatal("positive existential was not marked")
	}
	negated := marked.Arguments()[1].(*OperExpr).Arguments()[0].(*OperExpr)
	if negated.Operator().Name() != "EXISTS3" {
		t.Fatal("existential below NOT was unsafely marked")
	}
	conditional := marked.Arguments()[2].(*OperExpr)
	if conditional.Arguments()[0].(*OperExpr).Operator().Name() != "EXISTS3" ||
		conditional.Arguments()[1].(*OperExpr).Operator().Name() != "Apalache!Skolem" ||
		conditional.Arguments()[2].(*OperExpr).Operator().Name() != "Apalache!Skolem" {
		t.Fatalf("conditional marking = %v", conditional)
	}
}

func TestMarkSkolemizableModuleMarksCardinalityAndProperty(t *testing.T) {
	setType := mustSet(IntT1)
	set := MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1))
	cardinality := MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), set)
	body := MustOperExpr(Op("GE"), Typed(BoolT1), cardinality, intLiteral(1))
	module := NewModule("M", []Decl{boolDefinition("P", body)})
	marked, err := MarkSkolemizableModule(module, nil)
	if err != nil {
		t.Fatal(err)
	}
	markedBody, _ := OperatorBody(marked, "P")
	if !marked.HasProperty(PropertyAnalyzed) || markedBody.(*OperExpr).Operator().Name() != "Apalache!ConstCardinality" {
		t.Fatalf("marked module = %v, %v", marked.HasProperty(PropertyAnalyzed), markedBody)
	}
}

func TestMarkSetExpansionsPropagatesConsumerRequirements(t *testing.T) {
	intSet := mustSet(IntT1)
	powerSet := mustSet(intSet)
	base := MustOperExpr(Op("SET_ENUM"), Typed(intSet), intLiteral(1), intLiteral(2))
	power := func() Expr { return MustOperExpr(Op("SET_POWERSET"), Typed(powerSet), base) }

	union := MustOperExpr(Op("SET_UNARY_UNION"), Typed(intSet), power())
	markedUnion := MarkSetExpansionsExpression(union, nil).(*OperExpr)
	if markedUnion.Arguments()[0].(*OperExpr).Operator().Name() != "Apalache!Expand" {
		t.Fatal("UNION did not request explicit powerset expansion")
	}
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1),
		MustOperExpr(Op("SET_ENUM"), Typed(intSet), intLiteral(1)), power())
	markedMembership := MarkSetExpansionsExpression(membership, nil).(*OperExpr)
	if markedMembership.Arguments()[1].(*OperExpr).Operator().Name() != "SET_POWERSET" {
		t.Fatal("membership unnecessarily expanded its powerset")
	}
	binder := NewNameExpr("s", Typed(intSet))
	skolem := MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1),
		MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder, power(),
			NewValueExpr(BoolValue(true), Typed(BoolT1))))
	markedSkolem := MarkSetExpansionsExpression(skolem, nil).(*OperExpr)
	markedExists := markedSkolem.Arguments()[0].(*OperExpr)
	if markedExists.Arguments()[1].(*OperExpr).Operator().Name() != "SET_POWERSET" {
		t.Fatal("Skolemized existential unnecessarily expanded its powerset")
	}
	functionType := mustFun(IntT1, BoolT1)
	functionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)),
		MustOperExpr(Op("SET_ENUM"), Typed(intSet), intLiteral(1)),
		MustOperExpr(Op("SET_ENUM"), Typed(mustSet(BoolT1)), NewValueExpr(BoolValue(true), Typed(BoolT1))))
	functionBinder := NewNameExpr("f", Typed(functionType))
	forall := MustOperExpr(Op("FORALL3"), Typed(BoolT1), functionBinder, functionSet,
		NewValueExpr(BoolValue(true), Typed(BoolT1)))
	markedForall := MarkSetExpansionsExpression(forall, nil).(*OperExpr)
	if markedForall.Arguments()[1].(*OperExpr).Operator().Name() != "ApalacheInternal!__CompactFunSetExpand" {
		t.Fatal("function quantifier did not use the internal compact expansion marker")
	}
}

func TestMarkedPowersetExpansionFeedsUnaryUnion(t *testing.T) {
	intSet := mustSet(IntT1)
	setOfSets := mustSet(intSet)
	base := MustOperExpr(Op("SET_ENUM"), Typed(setOfSets),
		MustOperExpr(Op("SET_ENUM"), Typed(intSet), intLiteral(1)),
		MustOperExpr(Op("SET_ENUM"), Typed(intSet), intLiteral(2)))
	powerType := mustSet(setOfSets)
	expression := MustOperExpr(Op("SET_UNARY_UNION"), Typed(setOfSets),
		MustOperExpr(Op("SET_POWERSET"), Typed(powerType), base))
	marked := MarkSetExpansionsExpression(expression, nil)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(MustOperExpr(Op("EQ"), Typed(BoolT1), marked, base), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestMarkSetExpansionsScalaOracleCases(t *testing.T) {
	intSetType := mustSet(IntT1)
	setOfIntSetsType := mustSet(intSetType)
	functionType := mustFun(IntT1, IntT1)
	functionSetType := mustSet(functionType)
	setName := func(name string) Expr { return NewNameExpr(name, Typed(intSetType)) }
	power := func() Expr {
		return MustOperExpr(Op("SET_POWERSET"), Typed(setOfIntSetsType), setName("S"))
	}
	functionSet := func() Expr {
		return MustOperExpr(Op("FUN_SET"), Typed(functionSetType), setName("S"), setName("T"))
	}
	expandPower := func() Expr {
		return MustOperExpr(Op("Apalache!Expand"), Typed(setOfIntSetsType), power())
	}
	expandFunctionSet := func() Expr {
		return MustOperExpr(Op("ApalacheInternal!__CompactFunSetExpand"), Typed(functionSetType), functionSet())
	}
	xSet := NewNameExpr("x", Typed(intSetType))
	xFunction := NewNameExpr("x", Typed(functionType))
	predicate := NewNameExpr("P", Typed(BoolT1))
	oneSet := MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intLiteral(1))
	setOfOneSet := MustOperExpr(Op("SET_ENUM"), Typed(setOfIntSetsType), oneSet)
	emptyFunctions := MustOperExpr(Op("SET_ENUM"), Typed(functionSetType))
	operator := NewNameExpr("Op", Typed(IntT1))
	zero := intLiteral(0)

	cases := []struct {
		name  string
		input Expr
		want  Expr
	}{
		{"membership-powerset-unmarked",
			MustOperExpr(Op("SET_IN"), Typed(BoolT1), xSet, power()),
			MustOperExpr(Op("SET_IN"), Typed(BoolT1), xSet, power())},
		{"membership-function-set-unmarked",
			MustOperExpr(Op("SET_IN"), Typed(BoolT1), xFunction, functionSet()),
			MustOperExpr(Op("SET_IN"), Typed(BoolT1), xFunction, functionSet())},
		{"union-powerset-marked",
			MustOperExpr(Op("SET_UNION2"), Typed(setOfIntSetsType), setOfOneSet, power()),
			MustOperExpr(Op("SET_UNION2"), Typed(setOfIntSetsType), setOfOneSet, expandPower())},
		{"union-function-set-marked",
			MustOperExpr(Op("SET_IN"), Typed(BoolT1), xFunction,
				MustOperExpr(Op("SET_UNION2"), Typed(functionSetType), emptyFunctions, functionSet())),
			MustOperExpr(Op("SET_IN"), Typed(BoolT1), xFunction,
				MustOperExpr(Op("SET_UNION2"), Typed(functionSetType), emptyFunctions, expandFunctionSet()))},
		{"forall-powerset-marked",
			MustOperExpr(Op("FORALL3"), Typed(BoolT1), xSet, power(), predicate),
			MustOperExpr(Op("FORALL3"), Typed(BoolT1), xSet, expandPower(), predicate)},
		{"exists-powerset-marked",
			MustOperExpr(Op("EXISTS3"), Typed(BoolT1), xSet, power(), predicate),
			MustOperExpr(Op("EXISTS3"), Typed(BoolT1), xSet, expandPower(), predicate)},
		{"skolem-powerset-unmarked",
			MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1),
				MustOperExpr(Op("EXISTS3"), Typed(BoolT1), xSet, power(), predicate)),
			MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1),
				MustOperExpr(Op("EXISTS3"), Typed(BoolT1), xSet, power(), predicate))},
		{"choose-powerset-unmarked",
			MustOperExpr(Op("CHOOSE3"), Typed(intSetType), xSet, power(), predicate),
			MustOperExpr(Op("CHOOSE3"), Typed(intSetType), xSet, power(), predicate)},
		{"fold-powerset-marked",
			MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1), operator, zero, power()),
			MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1), operator, zero, expandPower())},
		{"fold-function-set-marked",
			MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1), operator, zero, functionSet()),
			MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1), operator, zero, expandFunctionSet())},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := MarkSetExpansionsExpression(test.input, nil)
			if !EqualTypedExpr(got, test.want) {
				t.Fatalf("marked expression = %v, want %v", got, test.want)
			}
		})
	}
}

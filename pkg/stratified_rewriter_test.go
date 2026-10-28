package apalache

import (
	"context"
	"fmt"
	"testing"
)

func TestStratifiedRewriterBuiltinsSubstitutionIntegersAndAssignment(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	scope := InitialRewriterScope()
	for value, expected := range map[Value]int{
		BoolValue(false): 0, BoolValue(true): 1, BoolSet: 2, NatSet: 3, IntSet: 4,
	} {
		_, cell, err := rewriter.Rewrite(NewValueExpr(value, Untyped()), scope)
		if err != nil || cell.ID != expected {
			t.Fatalf("builtin %v = %v, %v", value, cell, err)
		}
	}
	scope.Binding = scope.Binding.With("x", scope.Arena.TrueCell())
	_, substituted, err := rewriter.Rewrite(NewNameExpr("x", Typed(BoolT1)), scope)
	if err != nil || !substituted.Equal(scope.Arena.TrueCell()) {
		t.Fatalf("substitution = %v, %v", substituted, err)
	}
	if _, _, err := rewriter.Rewrite(NewNameExpr("missing", Typed(BoolT1)), scope); err == nil {
		t.Fatal("unbound name was accepted")
	}
	intExpr := NewValueExpr(Int64Value(42), Typed(IntT1))
	intScope, intCell, err := rewriter.Rewrite(intExpr, scope)
	if err != nil || intScope.Arena.CellCount() != 6 || len(rewriter.Constraints()) != 1 {
		t.Fatalf("integer rewrite = %v, count %d, constraints %d, error %v", intCell, intScope.Arena.CellCount(), len(rewriter.Constraints()), err)
	}
}

func TestStratifiedRewriterAssignmentAndBooleanRules(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	scope := InitialRewriterScope()
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	assignment := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, NewValueExpr(Int64Value(7), Typed(IntT1)))
	assignedScope, result, err := rewriter.Rewrite(assignment, scope)
	if err != nil || !result.Equal(assignedScope.Arena.TrueCell()) || !assignedScope.Binding.Contains("x'") {
		t.Fatalf("assignment = scope %#v, cell %v, error %v", assignedScope, result, err)
	}
	if _, _, err := rewriter.Rewrite(assignment, assignedScope); err == nil {
		t.Fatal("duplicate assignment was accepted")
	}
	conjunction := MustOperExpr(Op("AND"), Typed(BoolT1),
		NewValueExpr(BoolValue(true), Typed(BoolT1)),
		NewValueExpr(BoolValue(false), Typed(BoolT1)))
	boolScope, boolCell, err := rewriter.Rewrite(conjunction, assignedScope)
	if err != nil || boolScope.Arena.CellCount() != assignedScope.Arena.CellCount()+1 {
		t.Fatalf("conjunction = %v, count %d, error %v", boolCell, boolScope.Arena.CellCount(), err)
	}
	constraint := rewriter.Constraints()[len(rewriter.Constraints())-1].(*OperExpr)
	if constraint.Operator().Name() != "EQ" || constraint.Arguments()[1].(*OperExpr).Operator().Name() != "AND" {
		t.Fatalf("conjunction constraint = %#v", constraint)
	}
	emptyOr := MustOperExpr(Op("OR"), Typed(BoolT1))
	_, emptyCell, err := rewriter.Rewrite(emptyOr, boolScope)
	if err != nil || !emptyCell.Equal(boolScope.Arena.FalseCell()) {
		t.Fatalf("empty disjunction = %v, %v", emptyCell, err)
	}
}

func TestStratifiedApplicationResolvesResidualResultVariable(t *testing.T) {
	a, _ := NewVarType(1000)
	sequence := MustOperExpr(Op("TUPLE"), Typed(mustSeq(StrT1)),
		NewValueExpr(StringValue("value"), Typed(StrT1)))
	application := MustOperExpr(Op("FUN_APP"), Typed(a), sequence, intLiteral(1))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(application, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	if !EqualTypes(result.Type.TLAType(), StrT1) {
		t.Fatalf("residual application result type = %s", result.Type)
	}
	literal, found := rewriter.LiteralCache().FindLiteral(result)
	if !found || literal.Index != "value" {
		t.Fatalf("residual application result = %v, %v", literal, found)
	}
	_ = scope
}

func TestStratifiedLabelsAndFiniteSetPredicate(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	label := MustOperExpr(Op("LABEL"), Typed(IntT1),
		NewValueExpr(Int64Value(7), Typed(IntT1)), NewValueExpr(StringValue("source-label"), Typed(StrT1)))
	scope, labelled, err := rewriter.Rewrite(label, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	value, found := rewriter.IntegerCache().FindValue(labelled)
	if !found || value.Int64() != 7 {
		t.Fatalf("labelled value = %v, found %v", value, found)
	}
	finite := MustOperExpr(Op("FiniteSets!IsFiniteSet"), Typed(BoolT1),
		NewNameExpr("deliberately_unbound", Typed(mustSet(IntT1))))
	unchangedArena := scope.Arena
	scope, result, err := rewriter.Rewrite(finite, scope)
	if err != nil || !result.Equal(scope.Arena.TrueCell()) || scope.Arena != unchangedArena {
		t.Fatalf("IsFiniteSet = %v, arena changed %v, error %v", result, scope.Arena != unchangedArena, err)
	}
}

func TestStratifiedImplicationAndEquivalenceTruthTables(t *testing.T) {
	boolean := func(value bool) Expr { return NewValueExpr(BoolValue(value), Typed(BoolT1)) }
	for _, operator := range []string{"IMPLIES", "EQUIV"} {
		for _, left := range []bool{false, true} {
			for _, right := range []bool{false, true} {
				want := left == right
				if operator == "IMPLIES" {
					want = !left || right
				}
				name := fmt.Sprintf("%s/%v/%v", operator, left, right)
				t.Run(name, func(t *testing.T) {
					rewriter := NewStratifiedRewriter()
					expression := MustOperExpr(Op(operator), Typed(BoolT1), boolean(left), boolean(right))
					scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
					if err != nil {
						t.Fatal(err)
					}
					checkRewrittenTruth(t, rewriter, scope, result, want)
				})
			}
		}
	}
}

func TestStratifiedConstantExponentiationThroughG3(t *testing.T) {
	for _, test := range []struct {
		base, exponent, expected int64
	}{
		{2, 0, 1},
		{3, 4, 81},
		{-2, 5, -32},
	} {
		rewriter := NewStratifiedRewriter()
		expression := MustOperExpr(Op("POW"), Typed(IntT1), intLiteral(test.base), intLiteral(test.exponent))
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		solver, err := NewG3SolverContext()
		if err != nil {
			t.Fatal(err)
		}
		encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
		if err != nil {
			solver.Close()
			t.Fatal(err)
		}
		claim := MustOperExpr(Op("NE"), Typed(BoolT1), result.NameExpr(), intLiteral(test.expected))
		if err = encoder.Assert(claim); err != nil {
			solver.Close()
			t.Fatal(err)
		}
		status, err := solver.Check(context.Background())
		solver.Close()
		if err != nil || status != SMTUnsatisfiable {
			t.Fatalf("%d ^ %d = %d: %v, %v", test.base, test.exponent, test.expected, status, err)
		}
	}

	rewriter := NewStratifiedRewriter()
	scope, exponent, err := rewriter.Rewrite(intLiteral(3), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	scope.Binding = scope.Binding.With("n", exponent)
	scope, result, err := rewriter.Rewrite(MustOperExpr(Op("POW"), Typed(IntT1), intLiteral(3),
		NewNameExpr("n", Typed(IntT1))), scope)
	if err != nil {
		t.Fatal(err)
	}
	solver, err := NewG3SolverContext()
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		solver.Close()
		t.Fatal(err)
	}
	if err = encoder.Assert(MustOperExpr(Op("NE"), Typed(BoolT1), result.NameExpr(), intLiteral(27))); err != nil {
		solver.Close()
		t.Fatal(err)
	}
	status, err := solver.Check(context.Background())
	solver.Close()
	if err != nil || status != SMTUnsatisfiable {
		t.Fatalf("3 ^ symbolic 3 = 27: %v, %v", status, err)
	}

	rewriter = NewStratifiedRewriter()
	if _, _, err := rewriter.Rewrite(MustOperExpr(Op("POW"), Typed(IntT1), intLiteral(2), intLiteral(-1)),
		InitialRewriterScope()); err == nil {
		t.Fatal("negative exponent was accepted")
	}
}

func TestStratifiedSkolemizedExistentialSupportsAssignments(t *testing.T) {
	setType := mustSet(IntT1)
	binder := NewNameExpr("i", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("y", Typed(IntT1)))
	predicate := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, binder),
		MustOperExpr(Op("EQ"), Typed(BoolT1), binder, intLiteral(2)))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder,
		MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1), intLiteral(2)), predicate)
	skolem := MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1), exists)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(skolem, InitialRewriterScope())
	if err != nil || !scope.Binding.Contains("y'") || scope.Binding.Contains("i") {
		t.Fatalf("Skolem rewrite = %v, binding %v, %v", result, scope.Binding.Map(), err)
	}
	solver, err := NewG3SolverContext()
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		solver.Close()
		t.Fatal(err)
	}
	if err = encoder.Assert(result.NameExpr()); err != nil {
		solver.Close()
		t.Fatal(err)
	}
	assigned := scope.Binding.MustGet("y'")
	if err = encoder.Assert(MustOperExpr(Op("NE"), Typed(BoolT1), assigned.NameExpr(), intLiteral(2))); err != nil {
		solver.Close()
		t.Fatal(err)
	}
	status, err := solver.Check(context.Background())
	solver.Close()
	if err != nil || status != SMTUnsatisfiable {
		t.Fatalf("Skolem assignment = %v, %v", status, err)
	}

	emptyExists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder,
		MustOperExpr(Op("SET_ENUM"), Typed(setType)), NewValueExpr(BoolValue(true), Typed(BoolT1)))
	emptyRewriter := NewStratifiedRewriter()
	emptyScope, emptyResult, err := emptyRewriter.Rewrite(
		MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1), emptyExists), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	emptySolver, err := NewG3SolverContext()
	if err != nil {
		t.Fatal(err)
	}
	emptyEncoder, err := EncodeRewriterScope(emptySolver, emptyScope, emptyRewriter.Constraints())
	if err != nil {
		emptySolver.Close()
		t.Fatal(err)
	}
	if err = emptyEncoder.Assert(emptyResult.NameExpr()); err != nil {
		emptySolver.Close()
		t.Fatal(err)
	}
	status, err = emptySolver.Check(context.Background())
	emptySolver.Close()
	if err != nil || status != SMTUnsatisfiable {
		t.Fatalf("empty Skolem existential = %v, %v", status, err)
	}
}

func TestStratifiedSetNonMembership(t *testing.T) {
	setType := mustSet(IntT1)
	set := MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1), intLiteral(2))
	for _, test := range []struct {
		value int64
		want  bool
	}{{1, false}, {3, true}} {
		rewriter := NewStratifiedRewriter()
		expression := MustOperExpr(Op("SET_NOT_IN"), Typed(BoolT1), intLiteral(test.value), set)
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		checkRewrittenTruth(t, rewriter, scope, result, test.want)
	}
}

func TestStratifiedPrimeAndConstCardinalityWrapper(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	rewriter := NewStratifiedRewriter()
	scope := InitialRewriterScope()
	scope, primedValue, err := rewriter.Rewrite(intValue(11), scope)
	if err != nil {
		t.Fatal(err)
	}
	scope.Binding = scope.Binding.With("x'", primedValue)
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	scope, result, err := rewriter.Rewrite(prime, scope)
	if err != nil || !result.Equal(primedValue) {
		t.Fatalf("prime = %v, want %v, error %v", result, primedValue, err)
	}
	setType := mustSet(IntT1)
	set := MustOperExpr(Op("SET_ENUM"), Typed(setType), intValue(1), intValue(2))
	cardinality := MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), set)
	scope, cardinalityCell, err := rewriter.Rewrite(cardinality, scope)
	cardinalityValue, cardinalityKnown := rewriter.IntegerCache().FindValue(cardinalityCell)
	if err != nil || !cardinalityKnown || cardinalityValue.Int64() != 2 {
		t.Fatalf("fixed cardinality = %v, %v, %v", cardinalityValue, cardinalityKnown, err)
	}
	comparison := MustOperExpr(Op("GE"), Typed(BoolT1), cardinality, intValue(2))
	wrapper := MustOperExpr(Op("Apalache!ConstCardinality"), Typed(BoolT1), comparison)
	scope, result, err = rewriter.Rewrite(wrapper, scope)
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedRewriterSetConstructionUnionAndFilter(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	intSet := Typed(mustSet(IntT1))
	left := MustOperExpr(Op("SET_ENUM"), intSet,
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	right := MustOperExpr(Op("SET_ENUM"), intSet,
		NewValueExpr(Int64Value(2), Typed(IntT1)), NewValueExpr(Int64Value(3), Typed(IntT1)))
	union := MustOperExpr(Op("SET_UNION2"), intSet, left, right)
	scope, unionCell, err := rewriter.Rewrite(union, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	pointers := scope.Arena.GetHas(unionCell)
	if len(pointers) != 3 {
		t.Fatalf("union pointers = %#v", pointers)
	}
	for _, pointer := range pointers {
		if _, fixed := pointer.(FixedElementPointer); !fixed {
			t.Fatalf("fixed union member became %#v", pointer)
		}
	}

	boolSet := Typed(mustSet(BoolT1))
	booleans := MustOperExpr(Op("SET_ENUM"), boolSet,
		NewValueExpr(BoolValue(false), Typed(BoolT1)), NewValueExpr(BoolValue(true), Typed(BoolT1)))
	bound := NewNameExpr("b", Typed(BoolT1))
	filter := MustOperExpr(Op("SET_FILTER"), boolSet, bound, booleans, bound)
	filteredScope, filteredCell, err := rewriter.Rewrite(filter, scope)
	if err != nil {
		t.Fatal(err)
	}
	filtered := filteredScope.Arena.GetHas(filteredCell)
	if len(filtered) != 2 || filteredScope.Binding.Contains("b") {
		t.Fatalf("filtered pointers/binding = %#v / %#v", filtered, filteredScope.Binding.Map())
	}
	for _, pointer := range filtered {
		guarded, ok := pointer.(SMTElementPointer)
		if !ok || !guarded.Element().NameExpr().Tag().Equal(Typed(BoolT1)) {
			t.Fatalf("filtered pointer = %#v", pointer)
		}
	}
	if len(rewriter.Constraints()) == 0 {
		t.Fatal("set rewrites produced no constraints")
	}
}

func TestStratifiedRewriterRejectsInfiniteSetFilter(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	intSet := Typed(mustSet(IntT1))
	bound := NewNameExpr("x", Typed(IntT1))
	filter := MustOperExpr(Op("SET_FILTER"), intSet, bound, NewValueExpr(IntSet, intSet), NewValueExpr(BoolValue(true), Typed(BoolT1)))
	if _, _, err := rewriter.Rewrite(filter, InitialRewriterScope()); err == nil {
		t.Fatal("filter over Int was accepted")
	}
}

func TestStratifiedRewriteScopeDischargesThroughG3(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	expression := MustOperExpr(Op("AND"), Typed(BoolT1),
		NewValueExpr(BoolValue(true), Typed(BoolT1)),
		NewValueExpr(BoolValue(false), Typed(BoolT1)))
	scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(result.NameExpr()); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("rewritten TRUE /\\ FALSE asserted true = %v, %v", status, err)
	}
}

func TestStratifiedArithmeticEqualityAndConditionalThroughG3(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	two := NewValueExpr(Int64Value(2), Typed(IntT1))
	three := NewValueExpr(Int64Value(3), Typed(IntT1))
	ten := NewValueExpr(Int64Value(10), Typed(IntT1))
	sum := MustOperExpr(Op("PLUS"), Typed(IntT1), two, three)
	product := MustOperExpr(Op("MULT"), Typed(IntT1), sum, two)
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1), product, ten)
	scope, equalCell, err := rewriter.Rewrite(equality, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), equalCell.NameExpr())); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("(2 + 3) * 2 = 10 negated = %v, %v", status, err)
	}

	rewriter = NewStratifiedRewriter()
	predicate := MustOperExpr(Op("LT"), Typed(BoolT1),
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	conditional := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1), predicate,
		NewValueExpr(Int64Value(11), Typed(IntT1)), NewValueExpr(Int64Value(22), Typed(IntT1)))
	scope, result, err := rewriter.Rewrite(conditional, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver = newTestG3Solver(t)
	encoder, err = EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	notEleven := MustOperExpr(Op("NE"), Typed(BoolT1), result.NameExpr(), NewValueExpr(Int64Value(11), Typed(IntT1)))
	if err := encoder.Assert(notEleven); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("conditional result != 11 = %v, %v", status, err)
	}
}

func TestStratifiedStringLiteralsAndEqualityThroughG3(t *testing.T) {
	tests := []struct {
		name       string
		left       string
		right      string
		contradict func(Expr) Expr
	}{
		{
			name: "same literal", left: "alpha", right: "alpha",
			contradict: func(result Expr) Expr { return MustOperExpr(Op("NOT"), Typed(BoolT1), result) },
		},
		{name: "distinct literals", left: "red", right: "blue", contradict: func(result Expr) Expr { return result }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1),
				NewValueExpr(StringValue(test.left), Typed(StrT1)),
				NewValueExpr(StringValue(test.right), Typed(StrT1)),
			)
			scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(test.contradict(result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("contradicted string equality = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedStructuralConditionalsThroughG3(t *testing.T) {
	predicate := MustOperExpr(Op("LT"), Typed(BoolT1),
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	falsePredicate := MustOperExpr(Op("GT"), Typed(BoolT1),
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	intSetType := mustSet(IntT1)
	setTag := Typed(intSetType)
	set := func(value int64) Expr {
		return MustOperExpr(Op("SET_ENUM"), setTag, NewValueExpr(Int64Value(value), Typed(IntT1)))
	}
	tupleType, err := NewTupleType(IntT1, intSetType)
	if err != nil {
		t.Fatal(err)
	}
	tupleTag := Typed(tupleType)
	tuple := func(integer, setValue int64) Expr {
		return MustOperExpr(Op("TUPLE"), tupleTag,
			NewValueExpr(Int64Value(integer), Typed(IntT1)), set(setValue))
	}
	sequenceType, err := NewSeqType(IntT1)
	if err != nil {
		t.Fatal(err)
	}
	sequenceTag := Typed(sequenceType)
	sequence := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = NewValueExpr(Int64Value(value), Typed(IntT1))
		}
		return MustOperExpr(Op("TUPLE"), sequenceTag, elements...)
	}
	tests := []struct {
		name      string
		tag       TypeTag
		predicate Expr
		thenExpr  Expr
		elseExpr  Expr
		want      Expr
	}{
		{name: "finite set then branch", tag: setTag, predicate: predicate, thenExpr: set(1), elseExpr: set(2), want: set(1)},
		{name: "finite set else branch", tag: setTag, predicate: falsePredicate, thenExpr: set(1), elseExpr: set(2), want: set(2)},
		{name: "tuple with set field", tag: tupleTag, predicate: predicate, thenExpr: tuple(1, 2), elseExpr: tuple(3, 4), want: tuple(1, 2)},
		{name: "sequence then branch", tag: sequenceTag, predicate: predicate, thenExpr: sequence(1), elseExpr: sequence(2, 3), want: sequence(1)},
		{name: "sequence else branch", tag: sequenceTag, predicate: falsePredicate, thenExpr: sequence(1), elseExpr: sequence(2, 3), want: sequence(2, 3)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			conditional := MustOperExpr(Op("IF_THEN_ELSE"), test.tag, test.predicate, test.thenExpr, test.elseExpr)
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1), conditional, test.want)
			scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("selected structural branch differs from then branch = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedFiniteAndNaturalMembershipThroughG3(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	setType := Typed(mustSet(IntT1))
	set := MustOperExpr(Op("SET_ENUM"), setType,
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), NewValueExpr(Int64Value(2), Typed(IntT1)), set)
	scope, memberCell, err := rewriter.Rewrite(membership, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), memberCell.NameExpr())); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("2 not in {1, 2} = %v, %v", status, err)
	}

	rewriter = NewStratifiedRewriter()
	natural := MustOperExpr(Op("SET_IN"), Typed(BoolT1),
		NewValueExpr(Int64Value(-1), Typed(IntT1)), NewValueExpr(NatSet, setType))
	scope, naturalCell, err := rewriter.Rewrite(natural, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver = newTestG3Solver(t)
	encoder, err = EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(naturalCell.NameExpr()); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("-1 in Nat = %v, %v", status, err)
	}
}

func TestStratifiedFiniteSetAlgebraThroughG3(t *testing.T) {
	setType := mustSet(IntT1)
	setTag := Typed(setType)
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = NewValueExpr(Int64Value(value), Typed(IntT1))
		}
		return MustOperExpr(Op("SET_ENUM"), setTag, elements...)
	}
	tests := []struct {
		name       string
		expression Expr
		contradict func(Expr) Expr
	}{
		{
			name: "intersection",
			expression: MustOperExpr(Op("EQ"), Typed(BoolT1),
				MustOperExpr(Op("SET_INTERSECT"), setTag, set(1, 2), set(2, 3)), set(2)),
			contradict: func(result Expr) Expr { return MustOperExpr(Op("NOT"), Typed(BoolT1), result) },
		},
		{
			name: "difference",
			expression: MustOperExpr(Op("EQ"), Typed(BoolT1),
				MustOperExpr(Op("SET_MINUS"), setTag, set(1, 2), set(2, 3)), set(1)),
			contradict: func(result Expr) Expr { return MustOperExpr(Op("NOT"), Typed(BoolT1), result) },
		},
		{
			name:       "subset",
			expression: MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), set(1), set(1, 2)),
			contradict: func(result Expr) Expr { return MustOperExpr(Op("NOT"), Typed(BoolT1), result) },
		},
		{
			name:       "not subset",
			expression: MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), set(1, 3), set(1, 2)),
			contradict: func(result Expr) Expr { return result },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(test.expression, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(test.contradict(result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("contradicted set algebra result = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedStructuralSetMembershipThroughG3(t *testing.T) {
	tupleType, err := NewTupleType(IntT1)
	if err != nil {
		t.Fatal(err)
	}
	tupleTag := Typed(tupleType)
	tuple := func() Expr {
		return MustOperExpr(Op("TUPLE"), tupleTag, NewValueExpr(Int64Value(1), Typed(IntT1)))
	}
	setTag := Typed(mustSet(tupleType))
	set := MustOperExpr(Op("SET_ENUM"), setTag, tuple())
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), tuple(), set)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(membership, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("structurally equal tuple membership negated = %v, %v", status, err)
	}
}

func TestStratifiedFiniteCardinalityThroughG3(t *testing.T) {
	tupleType, err := NewTupleType(IntT1)
	if err != nil {
		t.Fatal(err)
	}
	tupleTag := Typed(tupleType)
	tuple := func() Expr {
		return MustOperExpr(Op("TUPLE"), tupleTag, NewValueExpr(Int64Value(1), Typed(IntT1)))
	}
	structuralSetTag := Typed(mustSet(tupleType))
	structuralDuplicates := MustOperExpr(Op("SET_ENUM"), structuralSetTag, tuple(), tuple())
	boolSetTag := Typed(mustSet(BoolT1))
	booleans := MustOperExpr(Op("SET_ENUM"), boolSetTag,
		NewValueExpr(BoolValue(false), Typed(BoolT1)), NewValueExpr(BoolValue(true), Typed(BoolT1)))
	bound := NewNameExpr("b", Typed(BoolT1))
	filtered := MustOperExpr(Op("SET_FILTER"), boolSetTag, bound, booleans, bound)
	tests := []struct {
		name string
		set  Expr
		want int64
	}{
		{name: "empty", set: MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1))), want: 0},
		{name: "guarded", set: filtered, want: 1},
		{name: "structural duplicates", set: structuralDuplicates, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cardinality := MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), test.set)
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1), cardinality, NewValueExpr(Int64Value(test.want), Typed(IntT1)))
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("cardinality differs from %d = %v, %v", test.want, status, err)
			}
		})
	}
}

func TestStratifiedSequenceConstructionLengthAndEqualityThroughG3(t *testing.T) {
	sequenceType, err := NewSeqType(IntT1)
	if err != nil {
		t.Fatal(err)
	}
	tag := Typed(sequenceType)
	sequence := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = NewValueExpr(Int64Value(value), Typed(IntT1))
		}
		return MustOperExpr(Op("TUPLE"), tag, elements...)
	}
	rewriter := NewStratifiedRewriter()
	scope, first, err := rewriter.Rewrite(sequence(1, 2), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	length, proto, elements, err := unpackSequence(scope.Arena, first)
	if err != nil || len(elements) != 2 || proto.ID < 0 {
		t.Fatalf("unpacked sequence = length %v, proto %v, elements %#v, error %v", length, proto, elements, err)
	}
	lengthValue, found := rewriter.IntegerCache().FindValue(length)
	if !found || lengthValue.Int64() != 2 {
		t.Fatalf("sequence length cache = %v, %v", lengthValue, found)
	}

	lengthExpression := MustOperExpr(Op("Sequences!Len"), Typed(IntT1), first.NameExpr())
	_, projectedLength, err := rewriter.Rewrite(lengthExpression, scope)
	if err != nil || !projectedLength.Equal(length) {
		t.Fatalf("sequence Len = %v, %v", projectedLength, err)
	}
	application := MustOperExpr(Op("FUN_APP"), Typed(IntT1), first.NameExpr(), NewValueExpr(Int64Value(2), Typed(IntT1)))
	_, secondElement, err := rewriter.Rewrite(application, scope)
	secondValue, secondFound := rewriter.IntegerCache().FindValue(secondElement)
	if err != nil || !secondFound || secondValue.Int64() != 2 {
		t.Fatalf("sequence[2] = %v, %v, %v", secondValue, secondFound, err)
	}
	dynamicIndex := MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1)))
	dynamicApplication := MustOperExpr(Op("FUN_APP"), Typed(IntT1), first.NameExpr(), dynamicIndex)
	scope, dynamicElement, err := rewriter.Rewrite(dynamicApplication, scope)
	if err != nil {
		t.Fatal(err)
	}
	dynamicSolver := newTestG3Solver(t)
	dynamicEncoder, err := EncodeRewriterScope(dynamicSolver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err := dynamicEncoder.Assert(MustOperExpr(Op("NE"), Typed(BoolT1),
		dynamicElement.NameExpr(), NewValueExpr(Int64Value(2), Typed(IntT1)))); err != nil {
		t.Fatal(err)
	}
	if status, err := dynamicSolver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("sequence[1 + 1] differs from 2 = %v, %v", status, err)
	}
	outOfCapacity := MustOperExpr(Op("FUN_APP"), Typed(IntT1), first.NameExpr(), NewValueExpr(Int64Value(9), Typed(IntT1)))
	scope, defaultElement, err := rewriter.Rewrite(outOfCapacity, scope)
	defaultValue, defaultFound := rewriter.IntegerCache().FindValue(defaultElement)
	if err != nil || !defaultFound || defaultValue.Sign() != 0 {
		t.Fatalf("sequence[9] default = %v, %v, %v", defaultValue, defaultFound, err)
	}
	defaultSolver := newTestG3Solver(t)
	defaultEncoder, err := EncodeRewriterScope(defaultSolver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err := defaultEncoder.Assert(MustOperExpr(Op("NE"), Typed(BoolT1),
		defaultElement.NameExpr(), NewValueExpr(Int64Value(0), Typed(IntT1)))); err != nil {
		t.Fatal(err)
	}
	if status, err := defaultSolver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("out-of-capacity default differs from zero = %v, %v", status, err)
	}
	emptyScope, emptySequence, err := rewriter.Rewrite(sequence(), scope)
	if err != nil {
		t.Fatal(err)
	}
	head := MustOperExpr(Op("Sequences!Head"), Typed(IntT1), emptySequence.NameExpr())
	_, emptyHead, err := rewriter.Rewrite(head, emptyScope)
	if err != nil || !emptyHead.Equal(defaultElement) {
		t.Fatalf("Head(<<>>) = %v, want cached default %v, %v", emptyHead, defaultElement, err)
	}

	scope, equalSequence, err := rewriter.Rewrite(sequence(1, 2), scope)
	if err != nil {
		t.Fatal(err)
	}
	scope, differentSequence, err := rewriter.Rewrite(sequence(1, 3), scope)
	if err != nil {
		t.Fatal(err)
	}
	scope.Binding = scope.Binding.With("left", first).With("equal", equalSequence).With("different", differentSequence)
	for _, test := range []struct {
		name       string
		right      string
		contradict func(Expr) Expr
	}{
		{name: "equal", right: "equal", contradict: func(result Expr) Expr { return MustOperExpr(Op("NOT"), Typed(BoolT1), result) }},
		{name: "different", right: "different", contradict: func(result Expr) Expr { return result }},
	} {
		t.Run(test.name, func(t *testing.T) {
			equality := MustOperExpr(Op("EQ"), Typed(BoolT1),
				NewNameExpr("left", tag), NewNameExpr(test.right, tag))
			equalityScope, result, err := rewriter.Rewrite(equality, scope)
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, equalityScope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(test.contradict(result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("contradicted sequence equality = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedSequenceAppendAndTailThroughG3(t *testing.T) {
	sequenceType, err := NewSeqType(IntT1)
	if err != nil {
		t.Fatal(err)
	}
	tag := Typed(sequenceType)
	sequence := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = NewValueExpr(Int64Value(value), Typed(IntT1))
		}
		return MustOperExpr(Op("TUPLE"), tag, elements...)
	}
	tests := []struct {
		name      string
		operation Expr
		want      Expr
	}{
		{
			name: "append", operation: MustOperExpr(Op("Sequences!Append"), tag,
				sequence(1, 2), NewValueExpr(Int64Value(3), Typed(IntT1))), want: sequence(1, 2, 3),
		},
		{name: "tail", operation: MustOperExpr(Op("Sequences!Tail"), tag, sequence(1, 2)), want: sequence(2)},
		{name: "empty tail", operation: MustOperExpr(Op("Sequences!Tail"), tag, sequence()), want: sequence()},
		{name: "concat", operation: MustOperExpr(Op("Sequences!Concat"), tag, sequence(1, 2), sequence(3)), want: sequence(1, 2, 3)},
		{name: "subsequence", operation: MustOperExpr(Op("Sequences!SubSeq"), tag, sequence(1, 2, 3),
			NewValueExpr(Int64Value(2), Typed(IntT1)), NewValueExpr(Int64Value(3), Typed(IntT1))), want: sequence(2, 3)},
		{name: "clipped subsequence", operation: MustOperExpr(Op("Sequences!SubSeq"), tag, sequence(1, 2, 3),
			NewValueExpr(Int64Value(0), Typed(IntT1)), NewValueExpr(Int64Value(99), Typed(IntT1))), want: sequence(1, 2, 3)},
		{name: "reversed subsequence", operation: MustOperExpr(Op("Sequences!SubSeq"), tag, sequence(1, 2, 3),
			NewValueExpr(Int64Value(3), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1))), want: sequence()},
	}
	falsePredicate := MustOperExpr(Op("GT"), Typed(BoolT1),
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	conditionalLeft := MustOperExpr(Op("IF_THEN_ELSE"), tag, falsePredicate, sequence(1), sequence(2, 3))
	tests = append(tests, struct {
		name      string
		operation Expr
		want      Expr
	}{name: "concat symbolic length", operation: MustOperExpr(Op("Sequences!Concat"), tag, conditionalLeft, sequence(4)), want: sequence(2, 3, 4)})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1), test.operation, test.want)
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("sequence operation differs from expected = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedBoundedChoiceThroughG3(t *testing.T) {
	setTag := Typed(mustSet(IntT1))
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = NewValueExpr(Int64Value(value), Typed(IntT1))
		}
		return MustOperExpr(Op("SET_ENUM"), setTag, elements...)
	}
	binder := NewNameExpr("x", Typed(IntT1))
	choose := MustOperExpr(Op("CHOOSE3"), Typed(IntT1), binder, set(1, 2, 3),
		MustOperExpr(Op("GT"), Typed(BoolT1), binder, NewValueExpr(Int64Value(1), Typed(IntT1))))
	rangeSet := MustOperExpr(Op("INT_RANGE"), setTag, intLiteral(1), intLiteral(3))
	rangeChoose := MustOperExpr(Op("CHOOSE3"), Typed(IntT1), binder, rangeSet,
		MustOperExpr(Op("GT"), Typed(BoolT1), binder, intLiteral(1)))
	guess := MustOperExpr(Op("Apalache!Guess"), Typed(IntT1), set(2, 3))
	for _, operation := range []struct {
		name       string
		expression Expr
		allowed    []int64
		forbidden  int64
	}{
		{name: "choose", expression: choose, allowed: []int64{2, 3}, forbidden: 1},
		{name: "range choose", expression: rangeChoose, allowed: []int64{2, 3}, forbidden: 1},
		{name: "guess", expression: guess, allowed: []int64{2, 3}, forbidden: 1},
	} {
		for _, value := range append(append([]int64(nil), operation.allowed...), operation.forbidden) {
			t.Run(fmt.Sprintf("%s_%d", operation.name, value), func(t *testing.T) {
				rewriter := NewStratifiedRewriter()
				scope, picked, err := rewriter.Rewrite(operation.expression, InitialRewriterScope())
				if err != nil {
					t.Fatal(err)
				}
				solver := newTestG3Solver(t)
				encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
				if err != nil {
					t.Fatal(err)
				}
				equal := MustOperExpr(Op("EQ"), Typed(BoolT1), picked.NameExpr(), NewValueExpr(Int64Value(value), Typed(IntT1)))
				if err := encoder.Assert(equal); err != nil {
					t.Fatal(err)
				}
				status, err := solver.Check(context.Background())
				want := SMTSatisfiable
				if value == operation.forbidden {
					want = SMTUnsatisfiable
				}
				if err != nil || status != want {
					t.Fatalf("choice %d = %v, want %v, %v", value, status, want, err)
				}
			})
		}
	}

	emptyGuess := MustOperExpr(Op("Apalache!Guess"), Typed(IntT1), set())
	rewriter := NewStratifiedRewriter()
	_, picked, err := rewriter.Rewrite(emptyGuess, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	value, found := rewriter.IntegerCache().FindValue(picked)
	if !found || value.Sign() != 0 {
		t.Fatalf("empty Guess default = %v, %v", value, found)
	}
}

func TestStratifiedFiniteQuantifiersThroughG3(t *testing.T) {
	setTag := Typed(mustSet(IntT1))
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = NewValueExpr(Int64Value(value), Typed(IntT1))
		}
		return MustOperExpr(Op("SET_ENUM"), setTag, elements...)
	}
	x := NewNameExpr("x", Typed(IntT1))
	tests := []struct {
		name       string
		expression Expr
		want       bool
	}{
		{
			name: "exists witness", want: true,
			expression: MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x, set(1, 2),
				MustOperExpr(Op("EQ"), Typed(BoolT1), x, NewValueExpr(Int64Value(2), Typed(IntT1)))),
		},
		{
			name: "forall positive", want: true,
			expression: MustOperExpr(Op("FORALL3"), Typed(BoolT1), x, set(1, 2),
				MustOperExpr(Op("GT"), Typed(BoolT1), x, NewValueExpr(Int64Value(0), Typed(IntT1)))),
		},
		{
			name: "forall counterexample", want: false,
			expression: MustOperExpr(Op("FORALL3"), Typed(BoolT1), x, set(1, 2),
				MustOperExpr(Op("EQ"), Typed(BoolT1), x, NewValueExpr(Int64Value(1), Typed(IntT1)))),
		},
		{name: "exists empty", want: false, expression: MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x, set(), boolExpr(true))},
		{name: "forall empty", want: true, expression: MustOperExpr(Op("FORALL3"), Typed(BoolT1), x, set(), boolExpr(false))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(test.expression, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			if scope.Binding.Contains("x") {
				t.Fatal("quantifier binding escaped its scope")
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			contradiction := Expr(result.NameExpr())
			if test.want {
				contradiction = MustOperExpr(Op("NOT"), Typed(BoolT1), contradiction)
			}
			if err := encoder.Assert(contradiction); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("contradicted quantifier result = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedFiniteFunctionsThroughG3(t *testing.T) {
	functionType, err := NewFunType(IntT1, IntT1)
	if err != nil {
		t.Fatal(err)
	}
	functionTag := Typed(functionType)
	setTag := Typed(mustSet(IntT1))
	x := NewNameExpr("x", Typed(IntT1))
	constructor := func(offset int64) Expr {
		domain := MustOperExpr(Op("SET_ENUM"), setTag,
			NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
		body := MustOperExpr(Op("PLUS"), Typed(IntT1), x, NewValueExpr(Int64Value(offset), Typed(IntT1)))
		return MustOperExpr(Op("FUN_CTOR"), functionTag, body, x, domain)
	}

	application := MustOperExpr(Op("FUN_APP"), Typed(IntT1), constructor(10), NewValueExpr(Int64Value(2), Typed(IntT1)))
	equalTwelve := MustOperExpr(Op("EQ"), Typed(BoolT1), application, NewValueExpr(Int64Value(12), Typed(IntT1)))
	domain := MustOperExpr(Op("DOMAIN"), setTag, constructor(10))
	expectedDomain := MustOperExpr(Op("SET_ENUM"), setTag,
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	domainEquality := MustOperExpr(Op("EQ"), Typed(BoolT1), domain, expectedDomain)
	reorderedDomain := MustOperExpr(Op("SET_ENUM"), setTag,
		NewValueExpr(Int64Value(2), Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1)))
	reorderedBody := MustOperExpr(Op("PLUS"), Typed(IntT1), NewValueExpr(Int64Value(10), Typed(IntT1)), x)
	reorderedFunction := MustOperExpr(Op("FUN_CTOR"), functionTag, reorderedBody, x, reorderedDomain)
	functionEquality := MustOperExpr(Op("EQ"), Typed(BoolT1), constructor(10), reorderedFunction)
	functionInequality := MustOperExpr(Op("EQ"), Typed(BoolT1), constructor(10), constructor(20))
	predicate := MustOperExpr(Op("LT"), Typed(BoolT1),
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	conditionalFunction := MustOperExpr(Op("IF_THEN_ELSE"), functionTag, predicate, constructor(10), constructor(20))
	conditionalApplication := MustOperExpr(Op("FUN_APP"), Typed(IntT1), conditionalFunction,
		NewValueExpr(Int64Value(1), Typed(IntT1)))
	conditionalEquality := MustOperExpr(Op("EQ"), Typed(BoolT1), conditionalApplication,
		NewValueExpr(Int64Value(11), Typed(IntT1)))
	for _, test := range []struct {
		name       string
		expression Expr
		want       bool
	}{
		{name: "application", expression: equalTwelve, want: true},
		{name: "domain", expression: domainEquality, want: true},
		{name: "equal functions", expression: functionEquality, want: true},
		{name: "different functions", expression: functionInequality, want: false},
		{name: "conditional function", expression: conditionalEquality, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(test.expression, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			contradiction := Expr(result.NameExpr())
			if test.want {
				contradiction = MustOperExpr(Op("NOT"), Typed(BoolT1), contradiction)
			}
			if err := encoder.Assert(contradiction); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("contradicted function result = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedStructuralDomainsThroughG3(t *testing.T) {
	tupleType, _ := NewTupleType(IntT1, BoolT1)
	tuple := MustOperExpr(Op("TUPLE"), Typed(tupleType),
		NewValueExpr(Int64Value(7), Typed(IntT1)), NewValueExpr(BoolValue(true), Typed(BoolT1)))
	intSetTag := Typed(mustSet(IntT1))
	tupleDomain := MustOperExpr(Op("DOMAIN"), intSetTag, tuple)
	wantTupleDomain := MustOperExpr(Op("SET_ENUM"), intSetTag,
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))

	recordType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	record := MustOperExpr(Op("RECORD"), Typed(recordType),
		NewValueExpr(StringValue("b"), Typed(StrT1)), NewValueExpr(BoolValue(true), Typed(BoolT1)),
		NewValueExpr(StringValue("a"), Typed(StrT1)), NewValueExpr(Int64Value(1), Typed(IntT1)))
	stringSetTag := Typed(mustSet(StrT1))
	recordDomain := MustOperExpr(Op("DOMAIN"), stringSetTag, record)
	wantRecordDomain := MustOperExpr(Op("SET_ENUM"), stringSetTag,
		NewValueExpr(StringValue("a"), Typed(StrT1)), NewValueExpr(StringValue("b"), Typed(StrT1)))

	sequenceType, _ := NewSeqType(IntT1)
	sequenceTag := Typed(sequenceType)
	sequence := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = NewValueExpr(Int64Value(value), Typed(IntT1))
		}
		return MustOperExpr(Op("TUPLE"), sequenceTag, elements...)
	}
	falsePredicate := MustOperExpr(Op("GT"), Typed(BoolT1),
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	symbolicSequence := MustOperExpr(Op("IF_THEN_ELSE"), sequenceTag, falsePredicate, sequence(1, 2), sequence(3))
	sequenceDomain := MustOperExpr(Op("DOMAIN"), intSetTag, symbolicSequence)
	wantSequenceDomain := MustOperExpr(Op("SET_ENUM"), intSetTag, NewValueExpr(Int64Value(1), Typed(IntT1)))

	for _, test := range []struct {
		name  string
		left  Expr
		right Expr
	}{
		{name: "tuple", left: tupleDomain, right: wantTupleDomain},
		{name: "record", left: recordDomain, right: wantRecordDomain},
		{name: "symbolic sequence", left: sequenceDomain, right: wantSequenceDomain},
	} {
		t.Run(test.name, func(t *testing.T) {
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1), test.left, test.right)
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("structural domain equality negated = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedFiniteSetEqualityThroughG3(t *testing.T) {
	setType := Typed(mustSet(IntT1))
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = NewValueExpr(Int64Value(value), Typed(IntT1))
		}
		return MustOperExpr(Op("SET_ENUM"), setType, elements...)
	}
	tests := []struct {
		name       string
		left       Expr
		right      Expr
		contradict func(Expr) Expr
	}{
		{
			name: "order independent",
			left: set(1, 2), right: set(2, 1),
			contradict: func(result Expr) Expr {
				return MustOperExpr(Op("NOT"), Typed(BoolT1), result)
			},
		},
		{
			name: "different elements",
			left: set(1), right: set(2),
			contradict: func(result Expr) Expr { return result },
		},
		{
			name: "empty and nonempty",
			left: set(), right: set(1),
			contradict: func(result Expr) Expr { return result },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			equality := MustOperExpr(Op("EQ"), Typed(BoolT1), test.left, test.right)
			scope, result, err := rewriter.Rewrite(equality, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(test.contradict(result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("contradicted set equality = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedNestedSetEqualityThroughG3(t *testing.T) {
	intSetType := mustSet(IntT1)
	innerTag := Typed(intSetType)
	outerTag := Typed(mustSet(intSetType))
	inner := func() Expr {
		return MustOperExpr(Op("SET_ENUM"), innerTag, NewValueExpr(Int64Value(1), Typed(IntT1)))
	}
	left := MustOperExpr(Op("SET_ENUM"), outerTag, inner())
	right := MustOperExpr(Op("SET_ENUM"), outerTag, inner())
	equal := MustOperExpr(Op("EQ"), Typed(BoolT1), left, right)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("nested singleton set equality negated = %v, %v", status, err)
	}
}

func TestStratifiedGuardedSetEqualityThroughG3(t *testing.T) {
	setType := Typed(mustSet(BoolT1))
	allBooleans := MustOperExpr(Op("SET_ENUM"), setType,
		NewValueExpr(BoolValue(false), Typed(BoolT1)),
		NewValueExpr(BoolValue(true), Typed(BoolT1)),
	)
	bound := NewNameExpr("b", Typed(BoolT1))
	filtered := MustOperExpr(Op("SET_FILTER"), setType, bound, allBooleans, bound)
	trueOnly := MustOperExpr(Op("SET_ENUM"), setType,
		NewValueExpr(BoolValue(true), Typed(BoolT1)),
	)
	equal := MustOperExpr(Op("EQ"), Typed(BoolT1), filtered, trueOnly)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("{b \\in BOOLEAN: b} = {TRUE} negated = %v, %v", status, err)
	}
}

func TestStratifiedTupleConstructionAndEqualityThroughG3(t *testing.T) {
	tupleType, err := NewTupleType(IntT1, BoolT1)
	if err != nil {
		t.Fatal(err)
	}
	tag := Typed(tupleType)
	tuple := func(integer int64, boolean bool) Expr {
		return MustOperExpr(Op("TUPLE"), tag,
			NewValueExpr(Int64Value(integer), Typed(IntT1)),
			NewValueExpr(BoolValue(boolean), Typed(BoolT1)),
		)
	}
	tests := []struct {
		name       string
		left       Expr
		right      Expr
		contradict func(Expr) Expr
	}{
		{
			name: "equal fields", left: tuple(1, true), right: tuple(1, true),
			contradict: func(result Expr) Expr {
				return MustOperExpr(Op("NOT"), Typed(BoolT1), result)
			},
		},
		{
			name: "different integer field", left: tuple(1, true), right: tuple(2, true),
			contradict: func(result Expr) Expr { return result },
		},
		{
			name: "different boolean field", left: tuple(1, true), right: tuple(1, false),
			contradict: func(result Expr) Expr { return result },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1), test.left, test.right)
			scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(test.contradict(result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("contradicted tuple equality = %v, %v", status, err)
			}
		})
	}
}

func TestStratifiedTupleConstructorPreservesFieldOrder(t *testing.T) {
	tupleType, err := NewTupleType(BoolT1, IntT1)
	if err != nil {
		t.Fatal(err)
	}
	rewriter := NewStratifiedRewriter()
	expression := MustOperExpr(Op("TUPLE"), Typed(tupleType),
		NewValueExpr(BoolValue(true), Typed(BoolT1)),
		NewValueExpr(Int64Value(7), Typed(IntT1)),
	)
	scope, tuple, err := rewriter.Rewrite(expression, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	fields := scope.Arena.GetHas(tuple)
	if len(fields) != 2 || !fields[0].Element().Equal(scope.Arena.TrueCell()) {
		t.Fatalf("tuple fields = %#v", fields)
	}
	value, found := rewriter.IntegerCache().FindValue(fields[1].Element())
	if !found || value.Int64() != 7 {
		t.Fatalf("tuple integer field = %v, %v", value, found)
	}
}

func TestStratifiedTupleAndRecordApplication(t *testing.T) {
	tupleType, err := NewTupleType(BoolT1, IntT1)
	if err != nil {
		t.Fatal(err)
	}
	tuple := MustOperExpr(Op("TUPLE"), Typed(tupleType),
		NewValueExpr(BoolValue(true), Typed(BoolT1)),
		NewValueExpr(Int64Value(7), Typed(IntT1)),
	)
	second := MustOperExpr(Op("FUN_APP"), Typed(IntT1), tuple, NewValueExpr(Int64Value(2), Typed(IntT1)))
	rewriter := NewStratifiedRewriter()
	_, field, err := rewriter.Rewrite(second, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	value, found := rewriter.IntegerCache().FindValue(field)
	if !found || value.Int64() != 7 {
		t.Fatalf("tuple projection = %v, %v", value, found)
	}
	outOfBounds := MustOperExpr(Op("FUN_APP"), Typed(IntT1), tuple, NewValueExpr(Int64Value(3), Typed(IntT1)))
	if _, _, err := NewStratifiedRewriter().Rewrite(outOfBounds, InitialRewriterScope()); err == nil {
		t.Fatal("out-of-bounds tuple application was accepted")
	}

	recordType, err := NewRecordType(map[string]Type{"a": IntT1, "z": BoolT1})
	if err != nil {
		t.Fatal(err)
	}
	record := MustOperExpr(Op("RECORD"), Typed(recordType),
		NewValueExpr(StringValue("z"), Typed(StrT1)), NewValueExpr(BoolValue(false), Typed(BoolT1)),
		NewValueExpr(StringValue("a"), Typed(StrT1)), NewValueExpr(Int64Value(9), Typed(IntT1)),
	)
	access := MustOperExpr(Op("FUN_APP"), Typed(BoolT1), record, NewValueExpr(StringValue("z"), Typed(StrT1)))
	rewriter = NewStratifiedRewriter()
	_, field, err = rewriter.Rewrite(access, InitialRewriterScope())
	if err != nil || field.ID != 0 {
		t.Fatalf("record projection = %v, %v", field, err)
	}
	missing := MustOperExpr(Op("FUN_APP"), Typed(BoolT1), record, NewValueExpr(StringValue("missing"), Typed(StrT1)))
	missingRewriter := NewStratifiedRewriter()
	_, missingField, err := missingRewriter.Rewrite(missing, InitialRewriterScope())
	if err != nil || !missingField.Equal(InitialPureArena().FalseCell()) {
		t.Fatalf("missing record field default = %v, %v", missingField, err)
	}
}

func TestStratifiedRecordConstructionAndEqualityThroughG3(t *testing.T) {
	setType := mustSet(IntT1)
	recordType, err := NewRecordType(map[string]Type{"a": IntT1, "b": setType})
	if err != nil {
		t.Fatal(err)
	}
	recordTag := Typed(recordType)
	setTag := Typed(setType)
	key := func(name string) Expr { return NewValueExpr(StringValue(name), Typed(StrT1)) }
	record := func(integer, setValue int64, reverse bool) Expr {
		set := MustOperExpr(Op("SET_ENUM"), setTag, NewValueExpr(Int64Value(setValue), Typed(IntT1)))
		integerExpr := NewValueExpr(Int64Value(integer), Typed(IntT1))
		if reverse {
			return MustOperExpr(Op("RECORD"), recordTag, key("b"), set, key("a"), integerExpr)
		}
		return MustOperExpr(Op("RECORD"), recordTag, key("a"), integerExpr, key("b"), set)
	}
	tests := []struct {
		name       string
		left       Expr
		right      Expr
		contradict func(Expr) Expr
	}{
		{
			name: "source field order is irrelevant", left: record(1, 2, false), right: record(1, 2, true),
			contradict: func(result Expr) Expr {
				return MustOperExpr(Op("NOT"), Typed(BoolT1), result)
			},
		},
		{
			name: "nested set field differs", left: record(1, 2, false), right: record(1, 3, false),
			contradict: func(result Expr) Expr { return result },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1), test.left, test.right)
			scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Assert(test.contradict(result.NameExpr())); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
				t.Fatalf("contradicted record equality = %v, %v", status, err)
			}
		})
	}

	t.Run("conditional record chooses else branch", func(t *testing.T) {
		predicate := MustOperExpr(Op("GT"), Typed(BoolT1),
			NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
		conditional := MustOperExpr(Op("IF_THEN_ELSE"), recordTag,
			predicate, record(1, 2, false), record(3, 4, true))
		equal := MustOperExpr(Op("EQ"), Typed(BoolT1), conditional, record(3, 4, false))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		solver := newTestG3Solver(t)
		encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
		if err != nil {
			t.Fatal(err)
		}
		if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
			t.Fatal(err)
		}
		if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
			t.Fatalf("conditional record differs from else branch = %v, %v", status, err)
		}
	})

	rewriter := NewStratifiedRewriter()
	scope, recordCell, err := rewriter.Rewrite(record(7, 8, true), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	fields := scope.Arena.GetHas(recordCell)
	if len(fields) != 2 {
		t.Fatalf("sorted record fields = %#v", fields)
	}
	integer, found := rewriter.IntegerCache().FindValue(fields[0].Element())
	if !found || integer.Int64() != 7 {
		t.Fatalf("sorted record fields = %#v, integer %v, found %v", fields, integer, found)
	}
	missing := MustOperExpr(Op("RECORD"), recordTag, key("a"), NewValueExpr(Int64Value(1), Typed(IntT1)))
	missingRewriter := NewStratifiedRewriter()
	missingScope, missingCell, err := missingRewriter.Rewrite(missing, InitialRewriterScope())
	if err != nil {
		t.Fatalf("legacy record with a missing static field: %v", err)
	}
	if len(missingScope.Arena.GetHas(missingCell)) != 2 || !missingScope.Arena.HasDomain(missingCell) {
		t.Fatalf("legacy record was not totalized with a dynamic domain: %s", missingCell)
	}
	duplicate := MustOperExpr(Op("RECORD"), recordTag,
		key("a"), NewValueExpr(Int64Value(1), Typed(IntT1)),
		key("a"), NewValueExpr(Int64Value(2), Typed(IntT1)),
		key("b"), MustOperExpr(Op("SET_ENUM"), setTag),
	)
	if _, _, err := NewStratifiedRewriter().Rewrite(duplicate, InitialRewriterScope()); err == nil {
		t.Fatal("record constructor accepted a duplicate field")
	}
}

func TestStratifiedClosedRowRecordEqualityThroughG3(t *testing.T) {
	row, err := NewRowType(map[string]Type{"x": BoolT1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	recordType := NewRecordRowType(row)
	tag := Typed(recordType)
	record := func(value bool) Expr {
		return MustOperExpr(Op("RECORD"), tag,
			NewValueExpr(StringValue("x"), Typed(StrT1)),
			NewValueExpr(BoolValue(value), Typed(BoolT1)),
		)
	}
	equal := MustOperExpr(Op("EQ"), Typed(BoolT1), record(true), record(true))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("closed row-record equality negated = %v, %v", status, err)
	}
}

func TestStratifiedEqualityRejectsUnsupportedStructuralCells(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	sequenceType, err := NewSeqType(IntT1)
	if err != nil {
		t.Fatal(err)
	}
	scope := InitialRewriterScope()
	newArena, cells := scope.Arena.AppendCellTypes(
		BasicCellType{Type: sequenceType}, BasicCellType{Type: sequenceType},
	)
	scope.Arena = newArena
	scope.Binding = scope.Binding.With("left", cells[0]).With("right", cells[1])
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1),
		NewNameExpr("left", Typed(sequenceType)), NewNameExpr("right", Typed(sequenceType)))
	if _, _, err := rewriter.Rewrite(equality, scope); err == nil {
		t.Fatal("unsupported sequence equality was approximated as cell equality")
	}
}

func TestStratifiedRewritesRetainedNullaryLet(t *testing.T) {
	valueType := mustOperator(nil, IntT1)
	value := NewOperDecl("value", nil, intLiteral(2), Typed(valueType))
	application := MustOperExpr(Op("OPER_APP"), Typed(IntT1), NewNameExpr("value", Typed(valueType)))
	let := NewLetInExpr(
		MustOperExpr(Op("EQ"), Typed(BoolT1), application, intLiteral(2)),
		Typed(BoolT1), value)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(let, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
	if _, leaked := scope.Binding.Get("value"); leaked {
		t.Fatal("LET binding escaped its lexical scope")
	}
}

func TestStratifiedLetPreservesAssignmentsFromBody(t *testing.T) {
	valueType := mustOperator(nil, IntT1)
	value := NewOperDecl("value", nil, intLiteral(2), Typed(valueType))
	application := MustOperExpr(Op("OPER_APP"), Typed(IntT1), NewNameExpr("value", Typed(valueType)))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	let := NewLetInExpr(MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, application), Typed(BoolT1), value)
	rewriter := NewStratifiedRewriter()
	scope, _, err := rewriter.Rewrite(let, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	assigned, found := scope.Binding.Get("x'")
	if !found {
		t.Fatal("assignment made in LET body was discarded")
	}
	if _, leaked := scope.Binding.Get("value"); leaked {
		t.Fatal("LET binding escaped its lexical scope")
	}
	if integer, ok := rewriter.IntegerCache().FindValue(assigned); !ok || integer.Int64() != 2 {
		t.Fatalf("assigned x' = %v", assigned)
	}
}

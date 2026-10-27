package apalache

import (
	"context"
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

func TestStratifiedEqualityRejectsUnsupportedStructuralCells(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	recordType, err := NewRecordType(map[string]Type{"x": IntT1})
	if err != nil {
		t.Fatal(err)
	}
	scope := InitialRewriterScope()
	newArena, cells := scope.Arena.AppendCellTypes(
		BasicCellType{Type: recordType}, BasicCellType{Type: recordType},
	)
	scope.Arena = newArena
	scope.Binding = scope.Binding.With("left", cells[0]).With("right", cells[1])
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1),
		NewNameExpr("left", Typed(recordType)), NewNameExpr("right", Typed(recordType)))
	if _, _, err := rewriter.Rewrite(equality, scope); err == nil {
		t.Fatal("unsupported record equality was approximated as cell equality")
	}
}

package apalache

import (
	"context"
	"testing"
)

func TestStratifiedAssignmentScalaScalarAndSetOracleCases(t *testing.T) {
	assignment := func(name string, typ Type, value Expr) Expr {
		prime := MustOperExpr(Op("PRIME"), Typed(typ), NewNameExpr(name, Typed(typ)))
		return MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, value)
	}
	skolem := func(binder *NameExpr, domain, predicate Expr) Expr {
		exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder, domain, predicate)
		return MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1), exists)
	}
	set := func(elementType Type, elements ...Expr) Expr {
		return MustOperExpr(Op("SET_ENUM"), Typed(mustSet(elementType)), elements...)
	}
	checkCandidates := func(t *testing.T, expression Expr, candidates []Expr, satisfiable []bool) {
		t.Helper()
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		assigned, found := scope.Binding.Get("x'")
		if !found {
			t.Fatal("assignment did not bind x'")
		}
		comparisons := make([]ArenaCell, len(candidates))
		for index, candidate := range candidates {
			scope, comparisons[index], err = rewriter.Rewrite(MustOperExpr(Op("EQ"), Typed(BoolT1),
				assigned.NameExpr(), candidate), scope)
			if err != nil {
				t.Fatal(err)
			}
		}
		solver := newTestG3Solver(t)
		encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
		if err != nil {
			t.Fatal(err)
		}
		if err = encoder.Assert(result.NameExpr()); err != nil {
			t.Fatal(err)
		}
		for index, comparison := range comparisons {
			if err = solver.Push(); err != nil {
				t.Fatal(err)
			}
			if err = encoder.Assert(comparison.NameExpr()); err != nil {
				t.Fatal(err)
			}
			status, checkErr := solver.Check(context.Background())
			want := SMTUnsatisfiable
			if satisfiable[index] {
				want = SMTSatisfiable
			}
			if checkErr != nil || status != want {
				t.Fatalf("candidate %d = %v, %v; want %v", index, status, checkErr, want)
			}
			if err = solver.Pop(); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("integer Skolem assignment chooses exactly the domain", func(t *testing.T) {
		binder := NewNameExpr("t", Typed(IntT1))
		domain := set(IntT1, intLiteral(1), intLiteral(2))
		checkCandidates(t, skolem(binder, domain, assignment("x", IntT1, binder)),
			[]Expr{intLiteral(1), intLiteral(2), intLiteral(3)}, []bool{true, true, false})
	})

	t.Run("conjunction retains both assignments", func(t *testing.T) {
		expression := MustOperExpr(Op("AND"), Typed(BoolT1),
			assignment("x", IntT1, intLiteral(1)), assignment("y", IntT1, intLiteral(2)))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		x, xFound := scope.Binding.Get("x'")
		y, yFound := scope.Binding.Get("y'")
		if !xFound || !yFound {
			t.Fatalf("conjunction binding = %v", scope.Binding.Map())
		}
		claim := booleanAnd(result.NameExpr(),
			MustOperExpr(Op("EQ"), Typed(BoolT1), x.NameExpr(), intLiteral(1)),
			MustOperExpr(Op("EQ"), Typed(BoolT1), y.NameExpr(), intLiteral(2)))
		scope, proved, err := rewriter.Rewrite(claim, scope)
		if err != nil {
			t.Fatal(err)
		}
		checkRewrittenTruth(t, rewriter, scope, proved, true)
	})

	t.Run("empty static and dynamic domains are false but retain binding", func(t *testing.T) {
		binder := NewNameExpr("t", Typed(IntT1))
		domains := []Expr{
			set(IntT1),
			MustOperExpr(Op("SET_FILTER"), Typed(mustSet(IntT1)),
				NewNameExpr("u", Typed(IntT1)), set(IntT1, intLiteral(1)), boolExpr(false)),
		}
		for index, domain := range domains {
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(
				skolem(binder, domain, assignment("x", IntT1, binder)), InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			if !scope.Binding.Contains("x'") {
				t.Fatalf("empty domain %d dropped assignment", index)
			}
			checkRewrittenTruth(t, rewriter, scope, result, false)
		}
	})

	t.Run("assignment is visible to the following prime equality", func(t *testing.T) {
		prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
		expression := MustOperExpr(Op("AND"), Typed(BoolT1), assignment("x", IntT1, intLiteral(1)),
			MustOperExpr(Op("EQ"), Typed(BoolT1), prime, intLiteral(1)))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		checkRewrittenTruth(t, rewriter, scope, result, true)
	})

	t.Run("set Skolem assignment chooses exactly the domain", func(t *testing.T) {
		setInt := mustSet(IntT1)
		binder := NewNameExpr("t", Typed(setInt))
		set12 := set(IntT1, intLiteral(1), intLiteral(2))
		set23 := set(IntT1, intLiteral(2), intLiteral(3))
		set13 := set(IntT1, intLiteral(1), intLiteral(3))
		domain := set(setInt, set12, set23)
		checkCandidates(t, skolem(binder, domain, assignment("x", setInt, binder)),
			[]Expr{set12, set23, set13}, []bool{true, true, false})
	})

	t.Run("filtered set assignment respects structural equality", func(t *testing.T) {
		setInt := mustSet(IntT1)
		set12 := set(IntT1, intLiteral(1), intLiteral(2))
		set23WithDuplicate := set(IntT1,
			MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1), intLiteral(1)),
			intLiteral(2), intLiteral(3))
		set23 := set(IntT1, intLiteral(2), intLiteral(3))
		set13 := set(IntT1, intLiteral(1), intLiteral(3))
		filterBinder := NewNameExpr("u", Typed(setInt))
		domain := MustOperExpr(Op("SET_FILTER"), Typed(mustSet(setInt)), filterBinder,
			set(setInt, set12, set23WithDuplicate),
			MustOperExpr(Op("NE"), Typed(BoolT1), filterBinder, set23))
		binder := NewNameExpr("t", Typed(setInt))
		checkCandidates(t, skolem(binder, domain, assignment("x", setInt, binder)),
			[]Expr{set12, set13, set23}, []bool{true, false, false})
	})

	t.Run("powerset assignment admits exactly the four subsets", func(t *testing.T) {
		setInt := mustSet(IntT1)
		binder := NewNameExpr("t", Typed(setInt))
		set12 := set(IntT1, intLiteral(1), intLiteral(2))
		domain := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(setInt)), set12)
		checkCandidates(t, skolem(binder, domain, assignment("x", setInt, binder)), []Expr{
			set(IntT1), set(IntT1, intLiteral(1)), set(IntT1, intLiteral(2)), set12,
			set(IntT1, intLiteral(1), intLiteral(2), intLiteral(3)),
		}, []bool{true, true, true, true, false})
	})

	t.Run("enumerated function assignment chooses exactly the domain", func(t *testing.T) {
		functionType := mustFun(BoolT1, IntT1)
		constructor := func(value int64, binderName string) Expr {
			binder := NewNameExpr(binderName, Typed(BoolT1))
			return MustOperExpr(Op("FUN_CTOR"), Typed(functionType), intLiteral(value), binder,
				NewValueExpr(BoolSet, Typed(mustSet(BoolT1))))
		}
		fun0, fun1, fun2 := constructor(0, "b0"), constructor(1, "b1"), constructor(2, "b2")
		binder := NewNameExpr("t", Typed(functionType))
		domain := set(functionType, fun0, fun1)
		checkCandidates(t, skolem(binder, domain, assignment("x", functionType, binder)),
			[]Expr{fun0, fun1, fun2}, []bool{true, true, false})
	})

	t.Run("tuple assignment preserves correlated fields", func(t *testing.T) {
		setInt := mustSet(IntT1)
		tupleType, err := NewTupleType(IntT1, BoolT1, setInt)
		if err != nil {
			t.Fatal(err)
		}
		tuple := func(integer int64, boolean bool, elements ...int64) Expr {
			setElements := make([]Expr, len(elements))
			for index, element := range elements {
				setElements[index] = intLiteral(element)
			}
			return MustOperExpr(Op("TUPLE"), Typed(tupleType), intLiteral(integer), boolExpr(boolean),
				set(IntT1, setElements...))
		}
		first := tuple(1, false, 1, 3)
		second := tuple(2, true, 4)
		outside := tuple(1, true, 4)
		binder := NewNameExpr("t", Typed(tupleType))
		checkCandidates(t, skolem(binder, set(tupleType, first, second), assignment("x", tupleType, binder)),
			[]Expr{first, second, outside}, []bool{true, true, false})
	})

	t.Run("finite function-set assignment preserves result alternatives", func(t *testing.T) {
		functionType := mustFun(BoolT1, IntT1)
		constructor := func(value int64, name string) Expr {
			binder := NewNameExpr(name, Typed(BoolT1))
			return MustOperExpr(Op("FUN_CTOR"), Typed(functionType), intLiteral(value), binder,
				NewValueExpr(BoolSet, Typed(mustSet(BoolT1))))
		}
		functionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)),
			NewValueExpr(BoolSet, Typed(mustSet(BoolT1))), set(IntT1, intLiteral(0), intLiteral(1)))
		binder := NewNameExpr("t", Typed(functionType))
		checkCandidates(t, skolem(binder, functionSet, assignment("x", functionType, binder)),
			[]Expr{constructor(0, "b0"), constructor(1, "b1"), constructor(2, "b2")},
			[]bool{true, true, false})
	})

	t.Run("empty-domain function set still supplies its unique function", func(t *testing.T) {
		functionType := mustFun(IntT1, IntT1)
		functionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)),
			set(IntT1), set(IntT1, intLiteral(0), intLiteral(1)))
		binder := NewNameExpr("t", Typed(functionType))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(
			skolem(binder, functionSet, assignment("x", functionType, binder)), InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		if !scope.Binding.Contains("x'") {
			t.Fatal("empty-domain function assignment dropped x'")
		}
		checkRewrittenTruth(t, rewriter, scope, result, true)
	})

	t.Run("finite Boolean-valued function set supplies a witness", func(t *testing.T) {
		functionType := mustFun(IntT1, BoolT1)
		domain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), intLiteral(0),
			MustOperExpr(Op("MINUS"), Typed(IntT1), intLiteral(5), intLiteral(1)))
		functionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)), domain,
			NewValueExpr(BoolSet, Typed(mustSet(BoolT1))))
		binder := NewNameExpr("t", Typed(functionType))
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(
			skolem(binder, functionSet, assignment("x", functionType, binder)), InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		if !scope.Binding.Contains("x'") {
			t.Fatal("finite function assignment dropped x'")
		}
		checkRewrittenTruth(t, rewriter, scope, result, true)
	})

	for _, codomain := range []struct {
		name string
		set  Expr
	}{
		{name: "Nat", set: NewValueExpr(NatSet, Typed(mustSet(IntT1)))},
		{name: "Int", set: NewValueExpr(IntSet, Typed(mustSet(IntT1)))},
	} {
		t.Run("integer-valued function assignment over "+codomain.name, func(t *testing.T) {
			functionType := mustFun(IntT1, IntT1)
			domain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), intLiteral(0), intLiteral(4))
			functionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)), domain, codomain.set)
			binder := NewNameExpr("t", Typed(functionType))
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(
				skolem(binder, functionSet, assignment("x", functionType, binder)), InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			assigned, found := scope.Binding.Get("x'")
			if !found {
				t.Fatal("integer function assignment dropped x'")
			}
			var claim Expr = result.NameExpr()
			if codomain.name == "Nat" {
				application := MustOperExpr(Op("FUN_APP"), Typed(IntT1), assigned.NameExpr(), intLiteral(1))
				claim = booleanAnd(claim,
					MustOperExpr(Op("GE"), Typed(BoolT1), application, intLiteral(0)))
			}
			scope, checked, err := rewriter.Rewrite(claim, scope)
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, checked, true)
		})
	}
}

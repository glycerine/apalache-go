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
}

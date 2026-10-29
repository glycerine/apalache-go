package apalache

import (
	"context"
	"testing"
)

func TestStratifiedSequenceScalaOracleCases(t *testing.T) {
	sequenceType := mustSeq(IntT1)
	tag := Typed(sequenceType)
	sequence := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intLiteral(value)
		}
		return MustOperExpr(Op("TUPLE"), tag, elements...)
	}
	length := func(value Expr) Expr {
		return MustOperExpr(Op("Sequences!Len"), Typed(IntT1), value)
	}
	application := func(value, index Expr) Expr {
		return MustOperExpr(Op("FUN_APP"), Typed(IntT1), value, index)
	}
	subsequence := func(value Expr, from, to int64) Expr {
		return MustOperExpr(Op("Sequences!SubSeq"), tag, value, intLiteral(from), intLiteral(to))
	}
	equal := func(left, right Expr) Expr {
		return MustOperExpr(Op("EQ"), Typed(BoolT1), left, right)
	}
	check := func(t *testing.T, expression Expr, want bool) {
		t.Helper()
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(expression, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		checkRewrittenTruth(t, rewriter, scope, result, want)
	}

	for _, test := range []struct {
		name       string
		expression Expr
		want       bool
	}{
		{name: "concrete head", expression: equal(
			MustOperExpr(Op("Sequences!Head"), Typed(IntT1), sequence(3, 4, 5)), intLiteral(3)), want: true},
		{name: "concrete length", expression: equal(length(sequence(3, 4, 5)), intLiteral(3)), want: true},
		{name: "concrete tail", expression: equal(
			MustOperExpr(Op("Sequences!Tail"), tag, sequence(3, 4, 5)), sequence(4, 5)), want: true},
		{name: "empty tail", expression: equal(length(
			MustOperExpr(Op("Sequences!Tail"), tag, sequence())), intLiteral(0)), want: true},
		{name: "tail of empty subsequence retains Scala undefined length", expression: equal(length(
			MustOperExpr(Op("Sequences!Tail"), tag, subsequence(sequence(1), 1, 0))), intLiteral(-1)), want: true},
		{name: "middle subsequence", expression: equal(subsequence(sequence(3, 4, 5, 6), 2, 3), sequence(4, 5)), want: true},
		{name: "reversed subsequence", expression: equal(length(
			subsequence(sequence(3, 4, 5, 6), 3, 1)), intLiteral(0)), want: true},
		{name: "subsequence starts after capacity", expression: equal(length(
			subsequence(sequence(3, 4, 5, 6), 5, 6)), intLiteral(0)), want: true},
		{name: "subsequence clips upper bound", expression: equal(
			subsequence(sequence(3, 4, 5, 6), 1, 10), sequence(3, 4, 5, 6)), want: true},
		{name: "subsequence clips lower bound", expression: equal(
			subsequence(sequence(3, 4, 5, 6), 0, 4), sequence(3, 4, 5, 6)), want: true},
		{name: "zero-width subsequence", expression: equal(
			subsequence(sequence(3, 4, 5, 6), 1, 0), sequence()), want: true},
		{name: "equal prefixes", expression: equal(
			subsequence(sequence(1, 2, 3), 1, 2), subsequence(sequence(1, 2, 4), 1, 2)), want: true},
		{name: "different full subsequences", expression: equal(
			subsequence(sequence(1, 2, 3), 1, 3), subsequence(sequence(1, 2, 4), 1, 3)), want: false},
		{name: "append", expression: equal(
			MustOperExpr(Op("Sequences!Append"), tag, sequence(4, 5), intLiteral(10)), sequence(4, 5, 10)), want: true},
		{name: "append subsequence", expression: equal(
			MustOperExpr(Op("Sequences!Append"), tag, subsequence(sequence(3, 4, 5, 6), 2, 3), intLiteral(10)),
			sequence(4, 5, 10)), want: true},
		{name: "subsequence equality", expression: equal(sequence(4, 5),
			subsequence(sequence(3, 4, 5, 6), 2, 3)), want: true},
		{name: "subsequence domain", expression: equal(
			MustOperExpr(Op("DOMAIN"), Typed(mustSet(IntT1)), subsequence(sequence(3, 4, 5, 6), 2, 3)),
			MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1), intLiteral(2))), want: true},
		{name: "concat subsequence", expression: equal(
			MustOperExpr(Op("Sequences!Concat"), tag, sequence(9, 10), subsequence(sequence(3, 4, 5, 6), 2, 3)),
			sequence(9, 10, 4, 5)), want: true},
		{name: "sequence EXCEPT", expression: equal(
			exceptExpression(sequenceType, sequence(3, 4, 5, 6), intLiteral(2), intLiteral(7)),
			sequence(3, 7, 5, 6)), want: true},
		{name: "out-of-domain sequence EXCEPT", expression: equal(
			exceptExpression(sequenceType, sequence(3, 4, 5, 6), intLiteral(10), intLiteral(7)),
			sequence(3, 4, 5, 6)), want: true},
		{name: "empty sequence EXCEPT", expression: equal(
			exceptExpression(sequenceType, sequence(), intLiteral(10), intLiteral(7)), sequence()), want: true},
		{name: "capacity", expression: equal(
			MustOperExpr(Op("ApalacheInternal!__ApalacheSeqCapacity"), Typed(IntT1), sequence(3, 4, 5, 6)),
			intLiteral(4)), want: true},
	} {
		t.Run(test.name, func(t *testing.T) { check(t, test.expression, test.want) })
	}

	t.Run("concrete sequence cells preserve all elements", func(t *testing.T) {
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(sequence(3, 4, 5), InitialRewriterScope())
		if err != nil || !EqualTypes(result.Type.TLAType(), sequenceType) {
			t.Fatalf("sequence result = %v, %v", result, err)
		}
		for index, expected := range []int64{3, 4, 5} {
			scope, element, rewriteErr := rewriter.Rewrite(application(result.NameExpr(), intLiteral(int64(index+1))), scope)
			if rewriteErr != nil {
				t.Fatal(rewriteErr)
			}
			scope, claim, rewriteErr := rewriter.Rewrite(equal(element.NameExpr(), intLiteral(expected)), scope)
			if rewriteErr != nil {
				t.Fatal(rewriteErr)
			}
			checkRewrittenTruth(t, rewriter, scope, claim, true)
		}
	})
}

func TestStratifiedSequenceScalaUndefinedApplicationsRemainSatisfiable(t *testing.T) {
	sequenceType := mustSeq(IntT1)
	tag := Typed(sequenceType)
	sequence := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intLiteral(value)
		}
		return MustOperExpr(Op("TUPLE"), tag, elements...)
	}
	application := func(value, index Expr) Expr {
		return MustOperExpr(Op("FUN_APP"), Typed(IntT1), value, index)
	}

	for _, test := range []struct {
		name       string
		expression Expr
	}{
		{name: "empty static", expression: application(sequence(), intLiteral(1))},
		{name: "outside static", expression: application(sequence(1, 2), intLiteral(7))},
		{name: "undefined concat", expression: MustOperExpr(Op("Sequences!Concat"), tag, sequence(9, 10),
			MustOperExpr(Op("Sequences!Tail"), tag, MustOperExpr(Op("Sequences!SubSeq"), tag,
				sequence(1), intLiteral(1), intLiteral(0))))},
	} {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope, _, err := rewriter.Rewrite(test.expression, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			solver := newTestG3Solver(t)
			if _, err = EncodeRewriterScope(solver, scope, rewriter.Constraints()); err != nil {
				t.Fatal(err)
			}
			if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
				t.Fatalf("undefined sequence operation = %v, %v", status, checkErr)
			}
		})
	}

	t.Run("outside symbolic index", func(t *testing.T) {
		rewriter := NewStratifiedRewriter()
		scope, seven, err := rewriter.Rewrite(intLiteral(7), InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		scope.Binding = scope.Binding.With("x", seven)
		scope, _, err = rewriter.Rewrite(application(sequence(1, 2), NewNameExpr("x", Typed(IntT1))), scope)
		if err != nil {
			t.Fatal(err)
		}
		solver := newTestG3Solver(t)
		if _, err = EncodeRewriterScope(solver, scope, rewriter.Constraints()); err != nil {
			t.Fatal(err)
		}
		if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTSatisfiable {
			t.Fatalf("symbolic out-of-domain sequence application = %v, %v", status, checkErr)
		}
	})
}

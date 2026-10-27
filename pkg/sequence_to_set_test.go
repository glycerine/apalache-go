package apalache

import "testing"

func TestStratifiedSequenceToSetThroughG3(t *testing.T) {
	sequenceType := mustSeq(IntT1)
	setType := mustSet(IntT1)
	integer := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	sequence := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = integer(value)
		}
		return MustOperExpr(Op("TUPLE"), Typed(sequenceType), elements...)
	}
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = integer(value)
		}
		return MustOperExpr(Op("SET_ENUM"), Typed(setType), elements...)
	}
	tests := []struct {
		name     string
		sequence Expr
		set      Expr
	}{
		{"empty", sequence(), set()},
		{"duplicates", sequence(1, 1, 2), set(1, 2)},
		{"symbolic length", MustOperExpr(Op("Sequences!SubSeq"), Typed(sequenceType),
			sequence(1, 2, 3), integer(1), integer(2)), set(1, 2)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			converted := MustOperExpr(Op("SequencesExt!ToSet"), Typed(setType), test.sequence)
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(
				MustOperExpr(Op("EQ"), Typed(BoolT1), converted, test.set), InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, true)
		})
	}
}

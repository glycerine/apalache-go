package apalache

import "testing"

func TestSetDeduplicationPredicatesGuardMembershipAndDuplicates(t *testing.T) {
	setType := mustSet(IntT1)
	x := NewNameExpr("x", Typed(IntT1))
	oneAgain := MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1), intLiteral(0))
	base := MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1), oneAgain, intLiteral(2))
	filtered := MustOperExpr(Op("SET_FILTER"), Typed(setType), x, base,
		MustOperExpr(Op("LE"), Typed(BoolT1), x, intLiteral(1)))
	rewriter := NewStratifiedRewriter()
	scope, set, err := rewriter.Rewrite(filtered, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	scope, predicates, err := rewriter.setDeduplicationPredicates(scope, set)
	if err != nil {
		t.Fatal(err)
	}
	if len(predicates) != 3 {
		t.Fatalf("deduplication produced %d predicates, want 3", len(predicates))
	}
	for index, want := range []bool{true, false, false} {
		checkRewrittenTruth(t, rewriter, scope, predicates[index], want)
	}
}

func TestSetDeduplicationRejectsNonSetWithoutMutation(t *testing.T) {
	base := InitialRewriterScope()
	returned, _, err := NewStratifiedRewriter().setDeduplicationPredicates(base, base.Arena.TrueCell())
	if err == nil || returned.Arena != base.Arena {
		t.Fatalf("invalid deduplication returned scope=%v, err=%v", returned, err)
	}
}

package apalache

import (
	"errors"
	"testing"
)

func TestCheckerExceptionHierarchyMatchesScala(t *testing.T) {
	expression := NewNameExpr("x", Typed(IntT1))
	noRule := NewNoRuleException("missing rule", expression)
	var rewriter *RewriterException
	var checker *CheckerException
	if !errors.As(noRule, &rewriter) || !errors.As(noRule, &checker) {
		t.Fatal("NoRuleException did not inherit rewriter and checker errors")
	}
	if checker.CauseExpr() != expression || checker.Error() != "missing rule" {
		t.Fatalf("checker root = %q at %v", checker, checker.CauseExpr())
	}

	for _, err := range []error{
		NewRewriterKnownLimitationError("limit", expression),
		NewSmtEncodingException("encoding", expression),
		NewInvalidTlaExException("invalid", expression),
		NewInternalCheckerError("internal", expression),
	} {
		checker = nil
		if !errors.As(err, &checker) || checker.CauseExpr() != expression {
			t.Errorf("%T did not inherit CheckerException", err)
		}
	}

	if got := NewCheckerException("generic", nil).CauseExpr(); got != NullExpr() {
		t.Fatalf("nil checker cause = %v, want NullExpr", got)
	}
}

package apalache

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestSymbolicRangeEnumerationRejectsUnknownBounds(t *testing.T) {
	for _, inv := range []string{`[i \in 1..x |-> i][1]=1`, `[i \in 1..x |-> i][1]=0`, `((1..x) \cup {3})={1,2,3}`, `((1..x) \cup {3})={3}`, `({3} \cup (1..x))={3}`} {
		t.Run(inv, func(t *testing.T) {
			prepared := prepareSafetyModelFixture(t, fmt.Sprintf("---- MODULE Review ----\nEXTENDS Integers\nVARIABLE\n\\* @type: Int;\nx\nInit == x=2\nNext == x'=x\nInv == %s\n====", inv), []string{"Inv"}, "")
			_, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates, BoundedSafetyOptions{MaxSteps: 0})
			if err == nil || !strings.Contains(err.Error(), "enumerat") {
				t.Fatalf("want unsupported enumeration error, got %v", err)
			}
		})
	}
}

func TestCompactChoiceRejectsUnenumeratedDomains(t *testing.T) {
	for _, inv := range []string{`Guess(SUBSET (1..x)) = {}`} {
		t.Run(inv, func(t *testing.T) {
			prepared := prepareSafetyModelFixture(t, fmt.Sprintf("---- MODULE Review ----\nEXTENDS Integers, Apalache\nVARIABLE\n\\* @type: Int;\nx\nInit == x=2\nNext == x'=x\nInv == %s\n====", inv), []string{"Inv"}, "")
			_, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates, BoundedSafetyOptions{MaxSteps: 0})
			if err == nil || !strings.Contains(err.Error(), "enumerat") {
				t.Fatalf("want unsupported enumeration, got %v", err)
			}
		})
	}
}

func TestFunctionChoiceRejectsCompactDomainWithoutPreprocessing(t *testing.T) {
	ints := mustSet(IntT1)
	base := MustOperExpr(Op("SET_ENUM"), Typed(ints), intLiteral(1))
	domain := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(ints)), base)
	function := mustFun(ints, IntT1)
	functions := MustOperExpr(Op("FUN_SET"), Typed(mustSet(function)), domain, base)
	guess := MustOperExpr(Op("Apalache!Guess"), Typed(function), functions)
	_, _, err := NewStratifiedRewriter().Rewrite(guess, InitialRewriterScope())
	if err == nil || !strings.Contains(err.Error(), "enumerat") {
		t.Fatalf("want unsupported enumeration, got %v", err)
	}
}

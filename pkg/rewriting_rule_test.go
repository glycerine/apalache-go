package apalache

import (
	"reflect"
	"strings"
	"testing"
)

func TestFunctionalRewritingRuleContractAndLogging(t *testing.T) {
	arena, err := NewArenaSolverAdapter(newTestG3Solver(t))
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewSymbolicState(intLiteral(1), arena, EmptyBinding())
	if err != nil {
		t.Fatal(err)
	}
	rule := FunctionalRewritingRule{
		RuleName: "IncrementRule",
		Applicable: func(current *SymbolicState) bool {
			_, ok := current.Expression.(*ValueExpr)
			return ok
		},
		Rewrite: func(current *SymbolicState) (*SymbolicState, error) {
			return current.SetExpression(intLiteral(2))
		},
	}
	if !rule.IsApplicable(state) || rule.Name() != "IncrementRule" {
		t.Fatalf("rule applicability/name = %v/%q", rule.IsApplicable(state), rule.Name())
	}
	result, err := rule.Apply(state)
	if err != nil {
		t.Fatal(err)
	}
	if original, _ := FormatTLAExpression(state.Expression); original != "1" {
		t.Fatalf("rule mutated input state: %s", original)
	}
	if rewritten, _ := FormatTLAExpression(result.Expression); rewritten != "2" {
		t.Fatalf("rewritten expression = %s", rewritten)
	}

	logs := make([]string, 0, 2)
	log := SMTLogFunc(func(message string) { logs = append(logs, message) })
	if err = LogRewritingRuleEntry(log, rule, state); err != nil {
		t.Fatal(err)
	}
	if err = LogRewritingRuleReturn(log, rule, result); err != nil {
		t.Fatal(err)
	}
	want := []string{"; IncrementRule(1) {", "; } IncrementRule returns 2 [5 arena cells])"}
	if !reflect.DeepEqual(logs, want) {
		t.Fatalf("rule logs = %#v, want %#v", logs, want)
	}
}

func TestFunctionalRewritingRuleRejectsMissingOperationsAndInvalidLogs(t *testing.T) {
	rule := FunctionalRewritingRule{RuleName: "Empty"}
	if rule.IsApplicable(nil) {
		t.Fatal("rule without applicability function applied")
	}
	if _, err := rule.Apply(nil); err == nil || !strings.Contains(err.Error(), "Empty") {
		t.Fatalf("missing rewrite error = %v", err)
	}
	if err := LogRewritingRuleEntry(nil, rule, nil); err == nil {
		t.Fatal("invalid entry log succeeded")
	}
	if err := LogRewritingRuleReturn(nil, rule, nil); err == nil {
		t.Fatal("invalid return log succeeded")
	}
}

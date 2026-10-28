package apalache

import "fmt"

// RewritingRule is the legacy symbolic-state operational-semantics boundary.
// Go returns an error instead of relying on Scala exceptions.
type RewritingRule interface {
	Name() string
	IsApplicable(*SymbolicState) bool
	Apply(*SymbolicState) (*SymbolicState, error)
}

type SMTLogSink interface {
	Log(string)
}

type SMTLogFunc func(string)

func (log SMTLogFunc) Log(message string) { log(message) }

// FunctionalRewritingRule adapts ordinary Go functions to RewritingRule.
type FunctionalRewritingRule struct {
	RuleName   string
	Applicable func(*SymbolicState) bool
	Rewrite    func(*SymbolicState) (*SymbolicState, error)
}

func (rule FunctionalRewritingRule) Name() string { return rule.RuleName }

func (rule FunctionalRewritingRule) IsApplicable(state *SymbolicState) bool {
	return rule.Applicable != nil && rule.Applicable(state)
}

func (rule FunctionalRewritingRule) Apply(state *SymbolicState) (*SymbolicState, error) {
	if rule.Rewrite == nil {
		return nil, fmt.Errorf("rewriting rule %s has no implementation", rule.Name())
	}
	return rule.Rewrite(state)
}

func LogRewritingRuleEntry(log SMTLogSink, rule RewritingRule, state *SymbolicState) error {
	if log == nil || rule == nil || state == nil || state.Expression == nil {
		return fmt.Errorf("rule entry logging requires a log, rule, and symbolic state")
	}
	log.Log(fmt.Sprintf("; %s(%s) {", rule.Name(), debugIRExpression(state.Expression)))
	return nil
}

func LogRewritingRuleReturn(log SMTLogSink, rule RewritingRule, state *SymbolicState) error {
	if log == nil || rule == nil || state == nil || state.Expression == nil || state.Arena == nil {
		return fmt.Errorf("rule return logging requires a log, rule, and symbolic state with arena")
	}
	log.Log(fmt.Sprintf("; } %s returns %s [%d arena cells])",
		rule.Name(), debugIRExpression(state.Expression), state.Arena.PureArena().CellCount()))
	return nil
}

var _ RewritingRule = FunctionalRewritingRule{}

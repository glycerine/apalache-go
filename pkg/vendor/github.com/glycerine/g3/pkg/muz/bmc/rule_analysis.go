package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, bmc::is_linear.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// IsLinear selects the source BMC linear compiler only when every rule has at
// most one predicate in its tail and no quantifiers in interpreted tails.
func IsLinear(rules *base.RuleSet) (bool, error) {
	if rules == nil {
		return false, ast.ErrASTHandle
	}
	manager, err := rules.RuleManager()
	if err != nil {
		return false, err
	}
	count, err := rules.Len()
	if err != nil {
		return false, err
	}
	for i := 0; i < count; i++ {
		rule, err := rules.Rule(i)
		if err != nil {
			return false, err
		}
		tailCount, err := rule.UninterpretedTailSize()
		var quantified bool
		if err == nil && tailCount <= 1 {
			quantified, err = manager.HasRuleQuantifiers(rule)
		}
		err = errors.Join(err, rule.Close())
		if err != nil {
			return false, err
		}
		if tailCount > 1 || quantified {
			return false, nil
		}
	}
	return true, nil
}

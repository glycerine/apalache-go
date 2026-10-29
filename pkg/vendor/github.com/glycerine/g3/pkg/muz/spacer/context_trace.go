package spacer

// Source: context::get_rules_along_trace in
// src/muz/spacer/spacer_context.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// RulesAlongTrace returns owning rule references in preorder for the latest
// successful query. The caller closes every returned rule.
func (c *Context) RulesAlongTrace() (result []*base.Rule, resultErr error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if !c.solved || c.lastResult != lbool.True || c.query == nil {
		return nil, ErrContextOwner
	}
	roots, err := c.query.ReachFacts()
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return nil, nil
	}
	queue := []*ReachFact{roots[len(roots)-1]}
	for index := 0; index < len(roots)-1; index++ {
		resultErr = errors.Join(resultErr, roots[index].Close())
	}
	defer func() {
		for index := len(queue) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, queue[index].Close())
		}
		if resultErr != nil {
			for index := len(result) - 1; index >= 0; index-- {
				resultErr = errors.Join(resultErr, result[index].Close())
			}
			result = nil
		}
	}()
	for current := 0; current < len(queue); current++ {
		rule, err := queue[current].Rule()
		if err != nil {
			return result, err
		}
		result = append(result, rule)
		children, err := queue[current].Justifications()
		if err != nil {
			return result, err
		}
		queue = append(queue, children...)
	}
	return result, resultErr
}

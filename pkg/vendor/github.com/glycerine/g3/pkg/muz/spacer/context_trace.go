package spacer

// Source: context::get_rules_along_trace in
// src/muz/spacer/spacer_context.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// RulesAlongTrace returns owning rule references in the source queue order for
// the latest successful query. Non-SAT results return no rules.
// The caller closes every returned rule.
func (c *Context) RulesAlongTrace() (result []*base.Rule, resultErr error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if c.lastResult != lbool.True {
		return nil, writeSpacerVerbose(1, "Trace unavailable when result is false\n")
	}
	if !c.solved || c.query == nil {
		return nil, ErrContextOwner
	}
	roots, err := c.query.ReachFacts()
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return nil, nil
	}
	attempt := &spacerTraceCleanupAttempt{manager: c.manager, facts: roots}
	queue := []*ReachFact{roots[len(roots)-1]}
	defer func() {
		if resultErr == nil {
			if cleanup := attempt.closeFacts(); cleanup == nil {
				attempt.rules = nil // transfer successful rule owners to caller
				return
			} else {
				resultErr = &spacerTraceCleanupFailure{attempt: attempt, cause: cleanup, cleanupErr: cleanup, reported: cleanup, reportText: cleanup.Error()}
				result = nil
				return
			}
		}
		resultErr = attempt.finish(resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
	for current := 0; current < len(queue); current++ {
		rule, err := queue[current].Rule()
		if err != nil {
			return result, err
		}
		result = append(result, rule)
		attempt.rules = append(attempt.rules, rule)
		children, err := queue[current].Justifications()
		if err != nil {
			return result, err
		}
		queue = append(queue, children...)
		attempt.facts = append(attempt.facts, children...)
		if current == 0 && len(children) != 1 {
			return result, errors.Join(resultErr, writeSpacerVerbose(1, "Warning: counterexample is trivial or non-existent\n"))
		}
	}
	return result, resultErr
}

// counterexampleDepth follows get_cex_depth's breadth-first traversal, excluding
// the query wrapper. A query with zero or multiple premises has source depth zero.
func (c *Context) counterexampleDepth() (depth uint32, resultErr error) {
	if c.lastResult != lbool.True {
		return 0, nil
	}
	facts, err := c.query.ReachFacts()
	if err != nil {
		return 0, err
	}
	// Every reference acquired below stays owned until the traversal completes.
	defer func() {
		for i := len(facts) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, facts[i].Close())
		}
	}()
	if len(facts) == 0 {
		return 0, nil
	}
	children, err := facts[len(facts)-1].Justifications()
	if err != nil {
		return 0, err
	}
	start := len(facts)
	facts = append(facts, children...)
	if len(children) != 1 {
		return 0, nil
	}
	// Frontier boundaries serve as the source queue's null depth markers.
	for start < len(facts) {
		end := len(facts)
		for i := start; i < end; i++ {
			children, err = facts[i].Justifications()
			if err != nil {
				return 0, err
			}
			facts = append(facts, children...)
		}
		depth++
		start = end
	}
	return depth, nil
}

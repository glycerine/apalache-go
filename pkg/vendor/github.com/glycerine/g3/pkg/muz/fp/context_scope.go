package fp

// Source: src/muz/base/dl_context.cpp:153-195. The trail undoes background
// assertions, raw formulas, and then rules. A returning Go cleanup failure
// keeps the current trail step and its owners reachable until retry.

import "github.com/glycerine/g3/pkg/ast"

type contextPopPhase uint8

const (
	contextPopNone contextPopPhase = iota
	contextPopAssertions
	contextPopRawRules
	contextPopRules
	contextPopSnapshot
)

// ref_vector::shrink decrements the removed suffix from front to back. The
// consumed prefix is compacted even when a finalizer reports an error, so retry
// starts with the first remaining owner without replaying completed callbacks.
func (e *PropositionalEngine) releasePoppedAssertions(start int) error {
	next := start
	defer func() {
		remaining := copy(e.assertions[start:], e.assertions[next:])
		clear(e.assertions[start+remaining:])
		e.assertions = e.assertions[:start+remaining]
	}()
	for next < len(e.assertions) {
		err := e.manager.DelRef(&e.assertions[next])
		if e.assertions[next] == (ast.ASTRef{}) {
			next++
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *PropositionalEngine) releasePoppedRawRules() error {
	next := 0
	defer func() {
		remaining := copy(e.retiredRawRules, e.retiredRawRules[next:])
		clear(e.retiredRawRules[remaining:])
		e.retiredRawRules = e.retiredRawRules[:remaining]
	}()
	for next < len(e.retiredRawRules) {
		err := e.manager.DelRef(&e.retiredRawRules[next].formula)
		if e.retiredRawRules[next].formula == (ast.ASTRef{}) {
			next++
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *PropositionalEngine) resumeContextPop() error {
	last := len(e.scopes) - 1
	if last < 0 || len(e.rawRuleScopes) != last+1 || len(e.assertionScopes) != last+1 {
		return ErrPropositionalEngine
	}
	if e.popPhase == contextPopAssertions {
		if err := e.releasePoppedAssertions(e.assertionScopes[last]); err != nil {
			return err
		}
		e.retiredRawRules = append(e.retiredRawRules, e.rawRules...)
		e.rawRules = e.rawRuleScopes[last]
		e.rawRuleScopes[last] = nil
		e.popPhase = contextPopRawRules
	}
	if e.popPhase == contextPopRawRules {
		if err := e.releasePoppedRawRules(); err != nil {
			return err
		}
		e.popPhase = contextPopRules
	}
	if e.popPhase == contextPopRules {
		if err := e.set.ReplaceRules(e.scopes[last]); err != nil {
			return err
		}
		e.popPhase = contextPopSnapshot
	}
	if e.popPhase != contextPopSnapshot {
		return ErrPropositionalEngine
	}
	if err := e.scopes[last].Close(); err != nil {
		return err
	}
	e.scopes[last] = nil
	e.scopes = e.scopes[:last]
	e.rawRuleScopes = e.rawRuleScopes[:last]
	e.assertionScopes = e.assertionScopes[:last]
	e.popPhase = contextPopNone
	return nil
}

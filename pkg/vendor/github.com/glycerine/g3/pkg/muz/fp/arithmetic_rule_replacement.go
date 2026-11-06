package fp

import "github.com/glycerine/g3/pkg/muz/base"

// Arithmetic stores borrow the current source program. Retire their old
// result before committing a replacement, keeping facts independently owned.
func (e *PropositionalEngine) rebindArithmeticRules(replacement *base.RuleSet) error {
	if e.karrDatalog == nil && e.intervalDatalog == nil && e.boundDatalog == nil && e.sparseIntervalDatalog == nil && e.boundIntervalDatalog == nil {
		return nil
	}
	e.invalidateQueryForMutation()
	if e.karrDatalog != nil {
		e.karrDatalog.rules = replacement
		e.karrDatalog.results = nil
	}
	if e.sparseIntervalDatalog != nil {
		if err := e.sparseIntervalDatalog.discardEvaluator(); err != nil {
			return err
		}
	}
	if e.intervalDatalog != nil {
		if err := e.intervalDatalog.discardEvaluator(); err != nil {
			return err
		}
	}
	if e.intervalDatalog != nil {
		e.intervalDatalog.rules = replacement
	}
	if e.sparseIntervalDatalog != nil {
		e.sparseIntervalDatalog.rules = replacement
	}
	if e.boundDatalog != nil {
		e.boundDatalog.rules = replacement
		e.boundDatalog.evaluated = nil
	}
	if e.boundIntervalDatalog != nil {
		e.boundIntervalDatalog.rules = replacement
		e.boundIntervalDatalog.evaluated = nil
	}
	return nil
}

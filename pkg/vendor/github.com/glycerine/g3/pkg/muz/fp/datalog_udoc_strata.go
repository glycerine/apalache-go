package fp

// Source: dl_compiler.cpp stratified Datalog execution: every negative
// dependency is saturated before rules in the consuming stratum execute.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (c *datalogUDocContext) saturateStratified(set *base.RuleSet) (resultErr error) {
	finalized, err := set.IsFinalized()
	if err != nil {
		return err
	}
	if !finalized {
		valid, err := set.Finalize()
		if err != nil {
			return errors.Join(err, ErrUDocDatalogFragment)
		}
		if !valid {
			return ErrUnstratifiedNegation
		}
		// Source scoped_query restores the caller's open/closed rule state.
		defer func() { resultErr = errors.Join(resultErr, set.Reopen()) }()
	}
	count, err := set.Len()
	if err != nil {
		return err
	}
	groups := make(map[uint32][]int)
	var highest uint32
	for index := 0; index < count; index++ {
		rule, err := set.Rule(index)
		if err != nil {
			return err
		}
		head, headErr := rule.HeadDeclaration()
		shapeErr := c.preflightUDocRule(rule)
		stratum, stratumErr := set.PredicateStratum(head)
		if err = errors.Join(headErr, shapeErr, stratumErr, rule.Close()); err != nil {
			return err
		}
		groups[stratum] = append(groups[stratum], index)
		if stratum > highest {
			highest = stratum
		}
	}
	for stratum := uint32(0); stratum <= highest; stratum++ {
		for changed := true; changed; {
			changed = false
			for _, index := range groups[stratum] {
				if err = c.checkpoint(); err != nil {
					return err
				}
				rule, err := set.Rule(index)
				if err != nil {
					return err
				}
				ruleChanged, err := c.evaluateRule(rule)
				if err != nil {
					return fmt.Errorf("muz/fp: evaluate UDoc rule %d: %w", index, err)
				}
				changed = changed || ruleChanged
			}
		}
	}
	return nil
}

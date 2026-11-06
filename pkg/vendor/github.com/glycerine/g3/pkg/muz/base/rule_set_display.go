package base

// Source: src/muz/base/dl_rule_set.cpp rule_set::display.

import (
	"fmt"
	"io"
)

// WriteDisplay writes the rule-set diagnostic in grouped-head order. The
// source defaults to profile output disabled and a zero timing threshold.
func (s *RuleSet) WriteDisplay(out io.Writer) error {
	return s.WriteDisplayWithProfile(out, false, 0)
}

// WriteDisplayWithProfile writes the source Datalog rule listing with its
// profile annotation and millisecond threshold controls.
func (s *RuleSet) WriteDisplayWithProfile(out io.Writer, outputProfile bool, profileTimeoutMilliseconds uint32) error {
	if out == nil {
		return io.ErrClosedPipe
	}
	if err := s.open(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "; rule count: %d\n; predicate count: %d\n", len(s.rules), len(s.byHead)); err != nil {
		return err
	}
	for _, declaration := range s.outputOrder {
		name, err := s.manager.manager.GetASTName(declaration)
		if err != nil {
			return err
		}
		text, err := s.manager.manager.Symbols.String(name)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "; output: %s\n", text); err != nil {
			return err
		}
	}
	for _, declaration := range s.headOrder {
		for _, rule := range s.byHead[declaration] {
			ruleState, err := rule.open()
			if err != nil {
				return err
			}
			if ruleState.manager != s.manager.manager {
				return ErrRuleOwner
			}
			passes, err := ruleState.accounting.PassesOutputThreshold(profileTimeoutMilliseconds)
			if err != nil {
				return err
			}
			if !passes {
				continue
			}
			if err := s.manager.WriteRuleDisplayWithProfile(out, rule, outputProfile); err != nil {
				return err
			}
		}
	}
	return nil
}

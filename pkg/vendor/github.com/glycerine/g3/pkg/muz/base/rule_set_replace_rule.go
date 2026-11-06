package base

import "fmt"

func (s *RuleSet) ReplaceRule(oldRule, replacement *Rule) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.finalized {
		return fmt.Errorf("muz/base: reopen rule set before mutation")
	}
	oldState, err := oldRule.open()
	if err != nil {
		return err
	}
	newState, err := replacement.open()
	if err != nil {
		return err
	}
	if oldState == newState {
		return nil
	}
	oldDeclaration, err := s.ruleHeadDeclaration(oldRule)
	if err != nil {
		return err
	}
	newDeclaration, err := s.ruleHeadDeclaration(replacement)
	if err != nil {
		return err
	}
	if oldDeclaration != newDeclaration {
		return fmt.Errorf("muz/base: replacement rule changes head predicate")
	}
	index := -1
	for i, candidate := range s.rules {
		if candidate.state == oldState {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("muz/base: rule is not in rule set")
	}
	owned, err := replacement.Retain()
	if err != nil {
		return err
	}
	bucket := s.byHead[oldDeclaration]
	bucketIndex := -1
	for i, candidate := range bucket {
		if candidate.state == oldState {
			bucketIndex = i
			break
		}
	}
	if bucketIndex < 0 {
		_ = owned.Close()
		return fmt.Errorf("muz/base: grouped rule index is inconsistent")
	}
	previous := s.rules[index]
	s.rules[index] = owned
	bucket[bucketIndex] = owned
	s.byHead[oldDeclaration] = bucket
	return previous.Close()
}

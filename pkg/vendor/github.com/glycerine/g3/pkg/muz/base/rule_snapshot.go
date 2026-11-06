package base

// Source: src/muz/pdr/pdr_dl_interface.cpp and
// src/muz/spacer/spacer_dl_interface.cpp check_reset.

import "errors"

var ErrRuleSetSnapshot = errors.New("muz/base: invalid or closed rule-set snapshot")

// RuleSetSnapshot owns the previous rule set used by PDR/Spacer check_reset.
// CheckAndReplace returns the reuse decision against the prior snapshot and
// then atomically publishes an owned copy of current for the next check.
type RuleSetSnapshot struct {
	self    *RuleSetSnapshot
	manager *RuleManager
	set     *RuleSet
	closed  bool
}

func NewRuleSetSnapshot(manager *RuleManager) (*RuleSetSnapshot, error) {
	if manager == nil {
		return nil, ErrRuleSetSnapshot
	}
	if err := manager.open(); err != nil {
		return nil, err
	}
	set, err := NewRuleSet(manager)
	if err != nil {
		return nil, err
	}
	snapshot := &RuleSetSnapshot{manager: manager, set: set}
	snapshot.self = snapshot
	return snapshot, nil
}

func (s *RuleSetSnapshot) open() error {
	if s == nil || s.self != s || s.closed || s.manager == nil || s.set == nil {
		return ErrRuleSetSnapshot
	}
	return s.manager.open()
}

func (s *RuleSetSnapshot) CheckAndReplace(current *RuleSet) (reusable bool, resultErr error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if current == nil || current.manager != s.manager {
		return false, ErrRuleSetSnapshot
	}
	reusable, err := RuleSetIsWeaker(s.set, current)
	if err != nil {
		return false, err
	}
	replacement, err := NewRuleSet(s.manager)
	if err != nil {
		return false, err
	}
	if err = replacement.AddRules(current); err != nil {
		return false, errors.Join(err, replacement.Close())
	}
	prior := s.set
	s.set = replacement
	if err = prior.Close(); err != nil {
		return false, err
	}
	return reusable, nil
}

func (s *RuleSetSnapshot) Reset() error {
	if err := s.open(); err != nil {
		return err
	}
	empty, err := NewRuleSet(s.manager)
	if err != nil {
		return err
	}
	prior := s.set
	s.set = empty
	return prior.Close()
}

func (s *RuleSetSnapshot) Close() error {
	if s == nil || s.self != s {
		return ErrRuleSetSnapshot
	}
	if s.closed {
		return nil
	}
	err := s.set.Close()
	if err != nil {
		return err
	}
	s.closed = true
	s.manager = nil
	s.set = nil
	return nil
}

// RuleSetIsWeaker reports the source check_reset condition: the old set must
// be nonempty and each new rule must be basically subsumed by some old rule.
// It is intentionally asymmetric.
func RuleSetIsWeaker(oldSet, newSet *RuleSet) (weaker bool, resultErr error) {
	if oldSet == nil || newSet == nil || oldSet.manager != newSet.manager {
		return false, ErrRuleSetOwner
	}
	oldCount, err := oldSet.Len()
	if err != nil {
		return false, err
	}
	newCount, err := newSet.Len()
	if err != nil {
		return false, err
	}
	if oldCount == 0 {
		return false, nil
	}
	oldRules := make([]*Rule, 0, oldCount)
	defer func() {
		for position := len(oldRules) - 1; position >= 0; position-- {
			resultErr = errors.Join(resultErr, oldRules[position].Close())
		}
	}()
	for i := 0; i < oldCount; i++ {
		rule, err := oldSet.Rule(i)
		if err != nil {
			return false, err
		}
		oldRules = append(oldRules, rule)
	}
	for i := 0; i < newCount; i++ {
		candidate, err := newSet.Rule(i)
		if err != nil {
			return false, err
		}
		found := false
		for _, prior := range oldRules {
			found, err = BasicRuleSubsumes(prior, candidate)
			if err != nil || found {
				break
			}
		}
		resultErr = errors.Join(resultErr, candidate.Close())
		if err != nil {
			return false, err
		}
		if !found {
			return false, resultErr
		}
	}
	return true, resultErr
}

package base

// ExchangeContents publishes a completely prepared program without changing
// either RuleSet owner. The fixedpoint context uses this to preserve its source
// m_rule_set identity while leaving superseded contents in a temporary owner
// whose native cleanup can be retried. No references are acquired or released.
func (s *RuleSet) ExchangeContents(other *RuleSet) error {
	if err := s.open(); err != nil {
		return err
	}
	if other == nil || other.manager != s.manager {
		return ErrRuleSetOwner
	}
	if err := other.open(); err != nil {
		return err
	}
	s.exchangeContents(other)
	return nil
}

func (s *RuleSet) exchangeContents(other *RuleSet) {
	owner, otherOwner := s.contextOwner, other.contextOwner
	*s, *other = *other, *s
	s.contextOwner, other.contextOwner = owner, otherOwner
}

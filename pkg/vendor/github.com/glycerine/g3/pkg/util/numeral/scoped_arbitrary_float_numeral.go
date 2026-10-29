package numeral

// Source: src/util/scoped_numeral.h:25-55, instantiated with
// f2n<mpf_manager> by src/math/interval/interval_def.h:197-370.

import "github.com/glycerine/g3/pkg/arena"

// ScopedArbitraryFloatNumeral borrows the concrete f2n manager and owns one
// MPF value. Close is the source scoped-numeral destructor boundary.
type ScopedArbitraryFloatNumeral struct {
	self        *ScopedArbitraryFloatNumeral
	manager     *ArbitraryFloatNumeralManager
	value       ArbitraryFloat
	sourceCause error
	closed      bool
}

func NewScopedArbitraryFloatNumeral(m *ArbitraryFloatNumeralManager) (*ScopedArbitraryFloatNumeral, error) {
	if e := m.guard(); e != nil {
		return nil, e
	}
	s := &ScopedArbitraryFloatNumeral{manager: m}
	s.self = s
	return s, nil
}

func (s *ScopedArbitraryFloatNumeral) Get() (*ArbitraryFloat, error) {
	if s == nil || s.self != s || s.manager == nil {
		return nil, ErrScopedCopy
	}
	if s.closed {
		return nil, arena.ErrClosed
	}
	if e := s.manager.guard(); e != nil {
		return nil, e
	}
	return &s.value, nil
}

func (s *ScopedArbitraryFloatNumeral) Close() error {
	if s == nil || s.self != s || s.manager == nil {
		return ErrScopedCopy
	}
	if s.closed {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if e := s.manager.guard(); e != nil {
		s.sourceCause = e
		return e
	}
	if e := s.manager.core.valueGuard(&s.value); e != nil {
		s.sourceCause = e
		return e
	}
	if _, e := sourceNumeralDel(s.manager.core.integer, &s.value.significand); e != nil {
		s.sourceCause = e
		return e
	}
	s.closed = true
	return nil
}

// Cleanup owns a stopped scoped destructor without replaying source Del.
func (s *ScopedArbitraryFloatNumeral) Cleanup() error {
	if s == nil || s.self != s || s.manager == nil {
		return ErrScopedCopy
	}
	if s.closed {
		return nil
	}
	if e := s.manager.core.CleanupValue(&s.value); e != nil {
		return e
	}
	s.closed = true
	return nil
}

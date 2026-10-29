package polynomial

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// Actual RAW and physical children reached by the retained to_mpq call.
type algebraicConversionScope struct {
	owner      *AlgebraicRootCore
	hooks      unumeral.NumericInvocationHooks
	roots      unumeral.NumericAllocationRoots
	invocation unumeral.BinaryToRationalInvocation
	raw        []unumeral.IntegerRawInvocationReceipt
	stop       error
	executing  bool
}

func (s *algebraicConversionScope) NumericBound(executing bool) bool {
	m := s.owner
	if m == nil || m.self != m || m.closed || s.hooks.Generation == 0 {
		return false
	}
	if s == &m.binary.conversion {
		return !executing || s.executing && m.binary.executing && m.binary.qConstructed
	}
	if s == &m.bounds.conversion {
		return !executing || s.executing && m.bounds.executing
	}
	for i := range m.inversion.conversions {
		if s == &m.inversion.conversions[i] {
			return !executing || s.executing && m.inversion.executing
		}
	}
	return s == &m.refinementConversion && (!executing || s.executing && m.localActive)
}
func (s *algebraicConversionScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.roots
}
func (s *algebraicConversionScope) NumericRecord(z *unumeral.IntegerManager, _ string, r unumeral.NumericInvocationReceipt) {
	if s.NumericBound(true) && s.owner.config.Rationals.Integers() == z && r.Raw != nil {
		s.raw = append(s.raw, *r.Raw)
	}
}
func (s *algebraicConversionScope) pending(m *AlgebraicRootCore) bool {
	if s.owner != m {
		return false
	}
	if s.stop != nil || unumeral.NumericAllocationPending(&s.roots) || m.config.Rationals.MatchesRawPowerInvocation(&s.invocation.Power) || m.config.Rationals.MatchesRawNormalizeInvocation(&s.invocation.Normalize) {
		return true
	}
	for i := range s.raw {
		if m.config.Rationals.Integers().MatchesRawInvocation(&s.raw[i]) {
			return true
		}
	}
	return false
}
func (s *algebraicConversionScope) cleanup() error {
	if s.owner == nil {
		return nil
	}
	if !s.NumericBound(false) {
		return ErrAlgebraicOwner
	}
	if s.executing {
		return ErrAlgebraicPending
	}
	z := s.owner.config.Rationals.Integers()
	if e := s.roots.Cleanup(z); e != nil {
		return e
	}
	for i := len(s.raw) - 1; i >= 0; i-- {
		if e := z.CleanupRawInvocation(&s.raw[i], true); e != nil {
			return e
		}
	}
	if e := s.owner.config.Rationals.CleanupRawNormalizeInvocation(&s.invocation.Normalize); e != nil {
		return e
	}
	if e := s.owner.config.Rationals.CleanupRawPowerInvocation(&s.invocation.Power); e != nil {
		return e
	}
	clear(s.raw)
	s.raw = s.raw[:0]
	s.stop = nil
	return nil
}
func (m *AlgebraicRootCore) boundToRational(a *unumeral.BinaryRational, out *unumeral.Rational) error {
	return m.bounds.conversion.run(m, a, out)
}
func (s *algebraicConversionScope) run(m *AlgebraicRootCore, a *unumeral.BinaryRational, out *unumeral.Rational) error {
	if m == nil || m.self != m {
		return ErrAlgebraicOwner
	}
	bound := s == &m.bounds.conversion || s == &m.refinementConversion || s == &m.binary.conversion
	if s == &m.binary.conversion && (!m.binary.running || !m.binary.executing || !m.binary.qConstructed || out != &m.binary.localQ) {
		return ErrAlgebraicOwner
	}
	for i := range m.inversion.conversions {
		if s == &m.inversion.conversions[i] {
			if !m.inversion.executing || out != &m.inversion.values[i] {
				return ErrAlgebraicOwner
			}
			bound = true
		}
	}
	if !bound {
		return ErrAlgebraicOwner
	}
	if s == &m.refinementConversion && (!m.localActive || out != &m.localQ) {
		return ErrAlgebraicOwner
	}
	if s.pending(m) || s.hooks.Generation == ^uint64(0) {
		return ErrAlgebraicPending
	}
	clear(s.raw)
	s.raw = s.raw[:0]
	s.invocation = unumeral.BinaryToRationalInvocation{}
	s.owner = m
	// Contexts retain this actual call generation across reused scope storage.
	s.hooks = unumeral.NumericInvocationHooks{Generation: s.hooks.Generation + 1, Policy: s}
	s.executing = true
	defer func() { s.executing = false }()
	context := unumeral.NumericAllocationContext{}.WithCaller(&s.hooks)
	e := m.config.Binary.ToRationalWithInvocation(m.config.Rationals, a, out, context, &s.invocation)
	z := m.config.Rationals.Integers()
	for i := len(s.raw) - 1; i >= 0 && s.stop == nil; i-- {
		s.stop = z.RawInvocationSourceCause(&s.raw[i])
	}
	if s.stop == nil {
		s.stop = m.config.Rationals.RawNormalizeInvocationSourceCause(&s.invocation.Normalize)
	}
	if s.stop == nil {
		s.stop = m.config.Rationals.RawPowerInvocationSourceCause(&s.invocation.Power)
	}
	return e
}

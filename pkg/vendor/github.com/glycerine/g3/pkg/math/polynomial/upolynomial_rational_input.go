package polynomial

// Source: src/math/polynomial/upolynomial.cpp:204-211.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// Receiving numerical children outlive a stopped global-rational borrow. The
// source temporary remains with that service's existing lexical owner.
type upolyRationalInputScope struct {
	invocation uint64
	copy       unumeral.IntegerCopyAllocationReceipt
	normalize  unumeral.IntegerNormalizeInvocationReceipt
	bindings   [4]unumeral.NumericCallerBinding
	bound      uint8
	running    bool
	source     *unumeral.IntegerManager
}

// Issued by actual accepted input-call entry. A refused entry leaves the
// caller's prior receipt unchanged and grants no cleanup of another call.
type upolyRationalInputInvocationReceipt struct {
	manager    *UPolynomialManager
	frame      *upolyRationalInputScope
	invocation uint64
}

func (m *UPolynomialManager) matchesRationalInput(r *upolyRationalInputInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.rationalInput && r.invocation != 0 && r.invocation == m.rationalInput.invocation
}
func (m *UPolynomialManager) rationalInputPending(r *upolyRationalInputInvocationReceipt) bool {
	return m.matchesRationalInput(r) && m.rationalInput.pending(m)
}
func (m *UPolynomialManager) cleanupRationalInputInvocation(r *upolyRationalInputInvocationReceipt) error {
	if !m.matchesRationalInput(r) {
		return nil
	}
	return m.CleanupRationalInput()
}

func (s *upolyRationalInputScope) NumericExecuting() bool { return s.running }
func (s *upolyRationalInputScope) pending(m *UPolynomialManager) bool {
	return m.config.Integer.MatchesCopyAllocationInvocation(&s.copy) || m.modular.MatchesPNormalizeInvocation(&s.normalize)
}
func (s *upolyRationalInputScope) bind(z, source *unumeral.IntegerManager) error {
	for i, owner := range [2]*unumeral.IntegerManager{z, source} {
		if owner == nil || i == 1 && owner == z {
			continue
		}
		if e := owner.BindCaller(&s.bindings[s.bound], s, 0); e != nil {
			s.unbind()
			return e
		}
		s.bound++
		if e := owner.Natural.BindCaller(&s.bindings[s.bound], s, 0); e != nil {
			s.unbind()
			return e
		}
		s.bound++
	}
	return nil
}
func (s *upolyRationalInputScope) unbind() {
	for s.bound != 0 {
		s.bound--
		if e := s.bindings[s.bound].Unbind(); e != nil {
			panic(e)
		}
	}
}

// CleanupRationalInput acknowledges only the receiving children actually issued
// by SetRationals/the S-expression constant conversion, before vector cleanup.
func (m *UPolynomialManager) CleanupRationalInput() error {
	if m == nil || m.self != m {
		return ErrUPolynomialOwner
	}
	s := &m.rationalInput
	if s.running {
		return ErrUPolynomialCleanupPending
	}
	if !s.pending(m) {
		s.source = nil
		return nil
	}
	s.running = true
	if e := s.bind(m.config.Integer, s.source); e != nil {
		s.running = false
		return e
	}
	defer func() {
		s.unbind()
		s.running = false
		if !s.pending(m) {
			s.source = nil
		}
	}()
	if e := m.config.Integer.CleanupCopyAllocationInvocation(&s.copy); e != nil {
		return e
	}
	return m.modular.CleanupPNormalizeInvocation(&s.normalize, true)
}

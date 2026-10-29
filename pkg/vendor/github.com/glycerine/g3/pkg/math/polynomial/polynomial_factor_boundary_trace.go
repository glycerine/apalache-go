package polynomial

// Source: src/math/polynomial/polynomial.cpp:6755-6756,6829.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// These records run outside factor_core's lexical locals, including on its
// constant branch. Recursive entry belongs to the actual parent's child call.
type polynomialFactorDiagnosticScope struct {
	bindings                             [2]unumeral.NumericCallerBinding
	bound                                uint8
	self                                 *polynomialFactorDiagnosticScope
	owner                                *MultivariatePolynomialManager
	parent                               *polynomialFactorCoreFrame
	target                               *PolynomialFactors
	invocation                           uint64
	active, executing                    bool
	polynomial                           polynomialDisplayInvocationReceipt
	number                               unumeral.NumeralInsertionReceipt
	bodyCause, sourceCause, cleanupCause error
}

func (s *polynomialFactorDiagnosticScope) NumericExecuting() bool { return s.executing }
func (m *MultivariatePolynomialManager) factorDiagnosticActual(s *polynomialFactorDiagnosticScope) bool {
	if m == nil || m.self != m || s == nil || s.owner != m || s.self != s || s.invocation == 0 {
		return false
	}
	if s == &m.factorPrelude || s == &m.factorEnd {
		return s.parent == nil
	}
	p := s.parent
	return p != nil && s == &p.childPrelude && m.matchesFactorCore(&polynomialFactorCoreInvocationReceipt{m, p, p.invocation})
}
func (m *MultivariatePolynomialManager) beginFactorDiagnostic(s *polynomialFactorDiagnosticScope, f *PolynomialFactors, parent *polynomialFactorCoreFrame) error {
	if s != &m.factorPrelude && s != &m.factorEnd {
		if parent == nil || s != &parent.childPrelude || !m.matchesFactorCore(&polynomialFactorCoreInvocationReceipt{m, parent, parent.invocation}) || !parent.executing {
			return ErrPolynomialOwner
		}
	}
	if s.active || s.executing {
		return ErrPolynomialPending
	}
	if s.invocation == ^uint64(0) {
		return arena.ErrBounds
	}
	*s = polynomialFactorDiagnosticScope{self: s, owner: m, parent: parent, target: f, invocation: s.invocation + 1, active: true, executing: true}
	if e := m.core.integer.BindCaller(&s.bindings[0], s, 0); e != nil {
		s.active = false
		s.executing = false
		return e
	}
	s.bound = 1
	if e := m.core.integer.Natural.BindCaller(&s.bindings[1], s, 0); e != nil {
		if unbind := s.bindings[0].Unbind(); unbind != nil {
			panic(unbind)
		}
		s.bound = 0
		s.active = false
		s.executing = false
		return e
	}
	s.bound = 2
	return nil
}
func (m *MultivariatePolynomialManager) finishFactorDiagnostic(s *polynomialFactorDiagnosticScope, body error) error {
	for s.bound != 0 {
		s.bound--
		if e := s.bindings[s.bound].Unbind(); e != nil {
			panic(e)
		}
	}
	s.executing = false
	s.bodyCause = body
	s.sourceCause = m.core.displayReceiptSourceCause(&s.polynomial)
	if s.sourceCause == nil {
		s.sourceCause = s.number.SourceCause()
	}
	s.active = m.core.displayReceiptPending(&s.polynomial) || s.number.Active() || s.sourceCause != nil
	return errors.Join(body, s.sourceCause)
}
func (m *MultivariatePolynomialManager) cleanupFactorDiagnostic(s *polynomialFactorDiagnosticScope) (err error) {
	if !m.factorDiagnosticActual(s) {
		return nil
	}
	if s.executing {
		return ErrPolynomialPending
	}
	if !s.active {
		return nil
	}
	defer func() {
		if err != nil && s.cleanupCause == nil {
			s.cleanupCause = err
		}
	}()
	if err = m.core.cleanupDisplayReceipt(&s.polynomial); err != nil {
		return err
	}
	if err = m.core.modular.CleanupSourceString(&s.number); err != nil {
		return err
	}
	s.active = false
	return nil
}
func (m *MultivariatePolynomialManager) traceFactorCoreEntry(p Polynomial, f *PolynomialFactors, s *polynomialFactorDiagnosticScope, parent *polynomialFactorCoreFrame) (err error) {
	tags := m.core.monomials.monomialTraceTags()
	if !tags.ShouldTrace("factor") && !tags.ShouldTrace("factor_bug") {
		return nil
	}
	if err = m.beginFactorDiagnostic(s, f, parent); err != nil {
		return err
	}
	defer func() { err = m.finishFactorDiagnostic(s, err) }()
	if err = m.emitFactorTrace(f, "factor_core", 6755, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral("factor_core\np: ")
		if e := m.core.displayPolynomialDirectWithReceipt(p, out, false, &s.polynomial); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	}); err != nil {
		return err
	}
	return m.traceFactorBoundaryConstant(s, 6756, "factor_core", "factors r.get_constant(): ")
}
func (m *MultivariatePolynomialManager) traceFactorBoundaryConstant(s *polynomialFactorDiagnosticScope, line int, function, label string) error {
	if !m.core.monomials.monomialTraceTags().ShouldTrace("factor_bug") {
		return nil
	}
	return m.emitFactorTraceTag(s.target, "factor_bug", function, line, func(out *polynomialTraceSink) error {
		_ = out.WriteLiteral(label)
		text, e := m.core.modular.SourceStringTracked(&s.target.constant, &s.number)
		if e != nil {
			return e
		}
		_ = out.WriteLiteral(text)
		return out.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceFactorEnd(f *PolynomialFactors) (err error) {
	if !m.core.monomials.monomialTraceTags().ShouldTrace("factor_bug") {
		return nil
	}
	s := &m.factorEnd
	if err = m.beginFactorDiagnostic(s, f, nil); err != nil {
		return err
	}
	defer func() { err = m.finishFactorDiagnostic(s, err) }()
	return m.traceFactorBoundaryConstant(s, 6829, "factor", "[factor] end, r.get_constant(): ")
}

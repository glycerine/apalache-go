package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:645-680,2887-2893.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usexpr "github.com/glycerine/g3/pkg/util/sexpr"
)

type algebraicRootConstructionScope struct {
	owner                                *AlgebraicRootCore
	conversion                           algebraicIsolationConversionScope
	parser                               *SexprPolynomialConverter
	parserAccepted                       bool
	parserStop                           error
	selection                            *AlgebraicVector
	pm                                   *MultivariatePolynomialManager
	calls                                polynomialAlgebraicEvaluationScope
	active, running, finishing, borrowed bool
	sourceStop                           error
}

func (s *algebraicRootConstructionScope) childSourceCause() error {
	if s.owner != nil {
		if e := s.owner.rootTraceSourceCause(); e != nil {
			return e
		}
		if e := s.owner.config.Polynomials.refinableReceiptSourceCause(&s.conversion.refinable); e != nil {
			return e
		}
	}
	if e := s.calls.childSourceCause(); e != nil {
		return e
	}
	if r := &s.conversion.factor; r.service != nil && r.service.matchesFactorFrontendHistory(r) && r.service.frontend.receipt == *r && r.service.frontend.sourceCause != nil {
		return r.service.frontend.sourceCause
	}
	if r := &s.conversion.receipt; r.manager != nil && r.manager.matchesFactorNumeralVector(r) && r.frame.sourceCause != nil {
		return r.frame.sourceCause
	}
	return s.parserStop
}
func (m *AlgebraicRootCore) beginRootConstruction(pm *MultivariatePolynomialManager) error {
	s := &m.rootConstruction
	if s.active || m.isolationConversion.active || m.isolationConversion.running || m.multivariateIsolationRunning || m.isolationSigns.active {
		return ErrAlgebraicPending
	}
	if pm != nil && pm.algebraicIsolationBorrowers == math.MaxUint32 {
		return arena.ErrSlotLimit
	}
	*s = algebraicRootConstructionScope{owner: m, pm: pm, active: true, running: true}
	s.calls.vm = m
	if pm != nil {
		pm.algebraicIsolationBorrowers++
		s.borrowed = true
	}
	return nil
}
func (m *AlgebraicRootCore) cleanupRootConstruction() error {
	s := &m.rootConstruction
	if s.running || s.finishing {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if e := m.cleanupRootTrace(); e != nil {
		return e
	}
	if s.parser != nil {
		if s.parserAccepted {
			if e := s.parser.Close(); e != nil {
				return e
			}
		} else {
			if e := s.parser.closeHeader(); e != nil {
				return e
			}
		}
		s.parser = nil
	}
	// Factor locals are younger than the source scoped root vector. Conversion
	// receipts name only the actual accepted dense/factor invocations.
	if e := m.cleanupIsolationConversionScope(&s.conversion); e != nil {
		return e
	}
	if s.selection != nil {
		if m.rootSelection != nil && m.rootSelection != s.selection {
			return ErrAlgebraicPending
		}
		if e := s.selection.Close(); e != nil {
			return e
		}
		if m.rootSelection == s.selection {
			m.rootSelection = nil
		}
		s.selection = nil
	}
	if s.borrowed {
		s.pm.algebraicIsolationBorrowers--
		s.borrowed = false
	}
	s.active = false
	return nil
}
func (m *AlgebraicRootCore) finishRootConstruction(cause error) error {
	s := &m.rootConstruction
	s.running = false
	if s.parser != nil {
		if s.parserAccepted {
			// Probe only after the synchronous numeric borrow has ended, so this
			// records its actual final lexical cleanup state, including destructor DT.
			s.parserStop = s.parser.sexpr.Configuration().Rationals.WithNumericBorrow(func(b *unumeral.RationalNumericBorrow) error { return b.Ready() })
		}
		if s.parser.cause != nil {
			s.parserStop = errors.Join(s.parserStop, s.parser.cause)
		}
		if s.parserStop == nil {
			if e := s.parser.closeHeader(); e != nil {
				return errors.Join(cause, e)
			}
			s.parser = nil
		}
	}
	if e := s.childSourceCause(); e != nil {
		s.sourceStop = e
		return errors.Join(cause, e)
	}
	if cause == nil {
		return m.cleanupRootConstruction()
	}
	return cause
}
func (m *AlgebraicRootCore) rootConstructionWorkspace() (UPolynomial, error) {
	if m.isolateTmp1.slot == 0 {
		var e error
		m.isolateTmp1, e = m.config.Polynomials.NewPolynomial()
		if e != nil {
			return UPolynomial{}, e
		}
	}
	return m.isolateTmp1, nil
}
func (m *AlgebraicRootCore) rootIndexAssertion(index uint32, line int32) error {
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled && index == 0 {
		return diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", line, "i != 0")
	}
	return nil
}

// MkPolynomialRoot converts the source univariate polynomial directly into
// persistent m_isolate_tmp1 before selecting the one-based real root. Errors
// preserve conversion publications and require CleanupRetained before reuse.
func (m *AlgebraicRootCore) MkPolynomialRoot(pm *MultivariatePolynomialManager, p Polynomial, index uint32, out *Algebraic) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	if pm == nil {
		return arena.ErrConfig
	}
	if e := pm.ready(); e != nil {
		return e
	}
	if m.rootConstruction.active || m.isolationConversion.active {
		return ErrAlgebraicPending
	}
	if e := m.rootIndexAssertion(index, 666); e != nil {
		return e
	}
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled {
		yes, e := pm.core.IsUnivariate(p)
		if e != nil {
			return e
		}
		if !yes {
			if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 667, "is_univariate(p)"); e != nil {
				return e
			}
		}
	}
	if e = m.beginRootConstruction(pm); e != nil {
		return e
	}
	defer func() { cause = m.finishRootConstruction(cause) }()
	if e = m.traceRootPolynomial(pm, p); e != nil {
		return e
	}
	up, e := m.rootConstructionWorkspace()
	if e != nil {
		return e
	}
	s := &m.rootConstruction
	s.conversion.active = true
	target, e := m.config.Polynomials.descriptor(up)
	if e != nil {
		return e
	}
	if e = pm.toNumeralVectorIntoSource(p, 0, false, m.config.Polynomials, &target.state, &s.conversion.receipt); e != nil {
		return e
	}
	view, e := m.config.Polynomials.wholeView(up)
	if e != nil {
		return e
	}
	previous := m.polynomialCaller
	m.polynomialCaller = &s.calls
	defer func() { m.polynomialCaller = previous }()
	return m.mkRootWithConstruction(view, index, out, s)
}

// MkSexprRoot follows the expression overload used by AST arithmetic. Parsing
// writes the persistent isolation buffer before the root-index/runtime checks;
// debug assertions retain their earlier source position. Use CleanupRetained
// after an error, without resetting the published polynomial prefix.
func (m *AlgebraicRootCore) MkSexprRoot(sx *usexpr.SexprManager, p usexpr.SexprHandle, index uint32, out *Algebraic) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	if sx == nil {
		return arena.ErrConfig
	}
	if m.rootConstruction.active || m.isolationConversion.active {
		return ErrAlgebraicPending
	}
	if e := m.rootIndexAssertion(index, 675); e != nil {
		return e
	}
	if e := m.beginRootConstruction(nil); e != nil {
		return e
	}
	defer func() { cause = m.finishRootConstruction(cause) }()
	up, e := m.rootConstructionWorkspace()
	if e != nil {
		return e
	}
	s := &m.rootConstruction
	s.parser, e = NewSexprPolynomialConverter(sx, m.config.Polynomials)
	if e != nil {
		return e
	}
	if e = s.parser.convertAtDepthForRoot(p, up, 0, s); e != nil {
		return e
	}
	// Parser source locals have ended before the later root-isolation call.
	if e = s.parser.closeHeader(); e != nil {
		return e
	}
	s.parser = nil
	s.conversion.active = true
	view, e := m.config.Polynomials.wholeView(up)
	if e != nil {
		return e
	}
	if e = m.traceRootExpression(view, index); e != nil {
		return e
	}
	previous := m.polynomialCaller
	m.polynomialCaller = &s.calls
	defer func() { m.polynomialCaller = previous }()
	return m.mkRootWithConstruction(view, index, out, s)
}

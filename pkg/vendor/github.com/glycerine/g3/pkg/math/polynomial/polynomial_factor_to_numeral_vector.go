package polynomial

// Source: src/math/polynomial/upolynomial.h:401-450.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialFactorNumeralVectorReceipt struct {
	diagnostic polynomialFactorNumeralVectorHistory
	returned   bool
	manager    *MultivariatePolynomialManager
	frame      *polynomialFactorNumeralVectorScope
	invocation uint64
}
type polynomialFactorNumeralVectorScope struct {
	owner                                     *MultivariatePolynomialManager
	historyRecorded                           bool
	historyIndex                              int
	phase                                     uint8
	referenceConstructed, sourceReferenceDone bool
	invocation                                uint64
	active, executing, finishing, constructed bool
	direct                                    polynomialFactorNumeralVectorReceipt
	output                                    *upolyVectorState
	outputManager                             *UPolynomialManager
	np                                        polynomialReference
	normalization                             polynomialNormalizeInvocationReceipt
	numeric                                   polynomialNumericRoots
	normalize                                 unumeral.IntegerNormalizeInvocationReceipt
	bodyCause, sourceCause, cleanupCause      error
}

func (m *MultivariatePolynomialManager) matchesFactorNumeralVector(r *polynomialFactorNumeralVectorReceipt) bool {
	return m != nil && m.self == m && m.core != nil && m.core.self == m.core && r != nil && r.manager == m && r.frame == &m.factorNumeralVector && r.invocation != 0 && r.invocation == r.frame.invocation
}
func numericIntegerManager_factorNumeralVectorRoots(m *unumeral.IntegerManager, context integerAllocationInvocationContext) *polynomialNumericRoots {
	r := context.factorNumeralVector
	if m == nil || !m.ValidOwner() || r == nil || !r.manager.matchesFactorNumeralVector(r) || context.factorNumeralVectorInvocation != r.invocation || r.manager.core.integer != m || !r.frame.active || !r.frame.executing || r.manager.core.callbackDepth != 0 {
		return nil
	}
	output := r.frame.outputManager
	if output == nil || output.self != output || output.config.Integer != m {
		return nil
	}
	return &r.frame.numeric
}
func (m *MultivariatePolynomialManager) toNumeralVectorWithReceipt(p Polynomial, output *upolyVectorState, r *polynomialFactorNumeralVectorReceipt) error {
	return m.toNumeralVectorSource(p, 0, false, output, r)
}
func (m *MultivariatePolynomialManager) toNumeralVectorVariableWithReceipt(p Polynomial, variable uint32, output *upolyVectorState, r *polynomialFactorNumeralVectorReceipt) error {
	return m.toNumeralVectorSource(p, variable, true, output, r)
}
func (m *MultivariatePolynomialManager) toNumeralVectorSource(p Polynomial, variable uint32, selected bool, output *upolyVectorState, r *polynomialFactorNumeralVectorReceipt) (cause error) {
	if m == nil {
		return ErrPolynomialOwner
	}
	return m.toNumeralVectorIntoSource(p, variable, selected, m.upm, output, r)
}
func (m *MultivariatePolynomialManager) toNumeralVectorIntoSource(p Polynomial, variable uint32, selected bool, outputManager *UPolynomialManager, output *upolyVectorState, r *polynomialFactorNumeralVectorReceipt) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if output == nil || outputManager == nil || outputManager.config.Integer != m.core.integer {
		return ErrUPolynomialOwner
	}
	if e := outputManager.check(); e != nil {
		return e
	}
	c := m.core
	if !selected {
		if e := polynomialToNumeralSourceAssert(404, "pm.is_univariate(p)", func() (bool, error) {
			return c.IsUnivariate(p)
		}); e != nil {
			return e
		}
	}
	s := &m.factorNumeralVector
	if s.active || s.executing || s.finishing || c.callbackDepth != 0 || s.invocation == ^uint64(0) {
		return ErrPolynomialPending
	}
	*s = polynomialFactorNumeralVectorScope{owner: m, invocation: s.invocation + 1, active: true, executing: true, constructed: true, referenceConstructed: true, output: output, outputManager: outputManager}
	s.direct = polynomialFactorNumeralVectorReceipt{manager: m, frame: s, invocation: s.invocation}
	previousCaller := factorNumeralVectorCaller(c.integer)
	setFactorNumeralVectorCaller(c.integer, m)
	defer setFactorNumeralVectorCaller(c.integer, previousCaller)
	if r != nil {
		*r = s.direct
	}
	context := integerAllocationInvocationContext{factorNumeralVector: &s.direct, factorNumeralVectorInvocation: s.invocation}
	defer func() {
		s.executing = false
		if s.bodyCause == nil {
			s.bodyCause = cause
		}
		cause = errors.Join(cause, m.finishFactorNumeralVectorSource())
		m.updateFactorNumeralVectorHistory()
		s.direct.diagnostic = m.factorNumeralVectorSnapshot()
		s.direct.returned = true
		if r != nil {
			*r = s.direct
		}
	}()
	np, e := m.normalizeWithFactorNumeralVectorHistory(p, &s.normalization, &s.direct)
	if e != nil {
		return e
	}
	if e = s.np.assign(c, np); e != nil {
		return e
	}
	// Literal reads remain on ORIGINAL p, after assigning unused np.
	size, e := c.readWord(p, 8)
	if e != nil {
		return e
	}
	var degree uint32
	if selected {
		degree, e = c.Degree(p, variable)
	} else {
		degree, e = c.TotalDegree(p)
	}
	if e != nil {
		return e
	}
	if e = outputManager.reserve(output, degree+1); e != nil {
		return e
	}
	for i := uint32(0); i <= degree; i++ {
		value, e := outputManager.read(outputManager.stateView(output), i)
		if e != nil {
			return e
		}
		if e = outputManager.modular.Reset(value); e != nil {
			return e
		}
	}
	for i := uint32(0); i < size; i++ {
		term, e := c.monomialCore(p, i)
		if e != nil {
			return e
		}
		var k uint32
		if selected {
			monomialSize, err := c.monomials.sizeCore(term)
			if err != nil {
				return err
			}
			if monomialSize != 0 {
				if monomialSize != 1 {
					continue
				}
				power, err := c.monomials.powerCore(term, 0)
				if err != nil {
					return err
				}
				if power.Var != variable {
					continue
				}
				k = power.Degree
			}
		} else {
			k, e = c.monomials.degreeCore(term)
			if e != nil {
				return e
			}
		}
		if !selected {
			if e = polynomialToNumeralSourceAssert(415, "k <= deg", func() (bool, error) {
				return k <= degree, nil
			}); e != nil {
				return e
			}
		}
		value, e := outputManager.read(outputManager.stateView(output), k)
		if e != nil {
			return e
		}
		coefficient, e := c.coefficientCore(p, i)
		if e != nil {
			return e
		}
		s.phase = 1
		if e := s.numeric.operation.enter(c.integer); e != nil {
			return e
		}
		e = outputManager.modular.WithAllocationContext((context).utility(outputManager.modular.Integers())).Copy(value, &coefficient)
		s.numeric.operation.leave()
		if e != nil {
			return e
		}
	}
	return outputManager.setSize(output, degree+1)
}
func (m *MultivariatePolynomialManager) finishFactorNumeralVectorSource() error {
	s := &m.factorNumeralVector
	if !m.matchesFactorNumeralVector(&s.direct) {
		return ErrPolynomialOwner
	}
	if s.executing || s.finishing || m.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	previousCaller := factorNumeralVectorCaller(m.core.integer)
	setFactorNumeralVectorCaller(m.core.integer, m)
	defer setFactorNumeralVectorCaller(m.core.integer, previousCaller)
	s.finishing = true
	defer func() { s.finishing = false }()

	if e := m.normalizeInvocationSourceCause(&s.normalization); e != nil {
		s.sourceCause = e
		return e
	}
	if s.constructed {
		if e := s.np.closeValue(m.core, false); e != nil {
			s.sourceCause = e
			return e
		}
		s.constructed = false
		s.sourceReferenceDone = true
	}
	if !polynomialNegNumericPending(&s.numeric) && !s.outputManager.modular.MatchesPNormalizeInvocation(&s.normalize) && !m.matchesNormalizeInvocation(&s.normalization) {
		s.active = false
	}
	return nil
}
func (m *MultivariatePolynomialManager) cleanupFactorNumeralVector(r *polynomialFactorNumeralVectorReceipt, abort bool) (cause error) {
	if !m.matchesFactorNumeralVector(r) {
		return nil
	}
	s := r.frame
	if !s.active {
		return nil
	}
	if s.executing || s.finishing || m.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	previousCaller := factorNumeralVectorCaller(m.core.integer)
	setFactorNumeralVectorCaller(m.core.integer, m)
	defer setFactorNumeralVectorCaller(m.core.integer, previousCaller)
	s.finishing = true
	defer func() {
		s.finishing = false
		if cause != nil && s.cleanupCause == nil {
			s.cleanupCause = cause
		}
		m.updateFactorNumeralVectorHistory()
		s.direct.diagnostic = m.factorNumeralVectorSnapshot()
		s.direct.returned = true
		r.diagnostic = s.direct.diagnostic
		r.returned = true
	}()
	if e := s.outputManager.modular.CleanupPNormalizeInvocation(&s.normalize, false); e != nil {
		return e
	}
	if e := m.cleanupNormalizeInvocation(&s.normalization, false); e != nil {
		return e
	}
	if s.constructed {
		if e := s.np.close(m.core); e != nil {
			return e
		}
		s.constructed = false
	}
	if !abort {
		return nil
	}
	if e := s.outputManager.modular.CleanupPNormalizeInvocation(&s.normalize, true); e != nil {
		return e
	}
	if e := m.cleanupNormalizeInvocation(&s.normalization, true); e != nil {
		return e
	}
	if e := s.numeric.cleanup(m.core.integer); e != nil {
		return e
	}
	s.active = false
	return nil
}

type polynomialFactorNumeralVectorHistory struct {
	invocation                                uint64
	phase                                     uint8
	referenceConstructed, sourceReferenceDone bool
	normalization                             polynomialNormalizeInvocationReceipt
	normalize                                 unumeral.IntegerNormalizeInvocationReceipt
	bodyCause, sourceCause, cleanupCause      error
}

func (m *MultivariatePolynomialManager) archiveFactorNumeralVector() error {
	s := &m.factorNumeralVector
	if s.historyRecorded {
		return nil
	}
	if !s.active || !s.executing || m.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	s.historyIndex, s.historyRecorded = len(m.factorNumeralVectorHistory), true
	m.factorNumeralVectorHistory = append(m.factorNumeralVectorHistory, m.factorNumeralVectorSnapshot())
	return nil
}
func (m *MultivariatePolynomialManager) factorNumeralVectorSnapshot() polynomialFactorNumeralVectorHistory {
	s := &m.factorNumeralVector
	return polynomialFactorNumeralVectorHistory{s.invocation, s.phase, s.referenceConstructed, s.sourceReferenceDone, s.normalization, s.normalize, s.bodyCause, s.sourceCause, s.cleanupCause}
}
func (m *MultivariatePolynomialManager) updateFactorNumeralVectorHistory() {
	s := &m.factorNumeralVector
	if s.historyRecorded {
		m.factorNumeralVectorHistory[s.historyIndex] = m.factorNumeralVectorSnapshot()
	}
}
func (m *MultivariatePolynomialManager) factorNumeralVectorDiagnostic(r *polynomialFactorNumeralVectorReceipt) (polynomialFactorNumeralVectorHistory, bool) {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame != &m.factorNumeralVector || r.invocation == 0 {
		return polynomialFactorNumeralVectorHistory{}, false
	}
	s := &m.factorNumeralVector
	if s.invocation == r.invocation {
		return polynomialFactorNumeralVectorHistory{s.invocation, s.phase, s.referenceConstructed, s.sourceReferenceDone, s.normalization, s.normalize, s.bodyCause, s.sourceCause, s.cleanupCause}, true
	}
	for _, row := range m.factorNumeralVectorHistory {
		if row.invocation == r.invocation {
			return row, true
		}
	}
	// Completed allocation-free calls return a value-only diagnostic snapshot.
	// It grants no source/own cleanup authority; active matching stays exact.
	if r.returned && r.invocation <= s.invocation && r.diagnostic.invocation == r.invocation {
		return r.diagnostic, true
	}
	return polynomialFactorNumeralVectorHistory{}, false
}
func (m *MultivariatePolynomialManager) closeFactorNumeralVectorHistory() {
	m.factorNumeralVectorHistory = nil
}

// This actual executing caller excludes owner retirement only. Numeric authority
// remains the explicit converter or its reached Normalize child receipt.
func numericIntegerManager_factorNumeralVectorCallerExecuting(m *unumeral.IntegerManager) bool {
	if m == nil || !m.ValidOwner() {
		return false
	}
	f := factorNumeralVectorCaller(m)
	return f != nil && f.self == f && f.core != nil && f.core.integer == m && f.matchesFactorNumeralVector(&f.factorNumeralVector.direct) && (f.factorNumeralVector.executing || f.factorNumeralVector.finishing)
}

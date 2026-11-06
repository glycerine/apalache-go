package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1097-1145;
// src/math/polynomial/upolynomial_factorization_int.h:42-52;
// src/math/polynomial/upolynomial.cpp:32-54; src/util/scoped_numeral_vector.h:29-40.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyPrimeTrialInvocationReceipt struct {
	service    *UPolynomialFactorizationService
	parent     *upolyFactorSquareFreeScope
	frame      *upolyPrimeTrialScope
	invocation uint64
}

// The RAW transport is the actual p_i, not an additional scoped numeral.
// All storage is charged once by the enclosing reflected cold header.
type upolyPrimeTrialScope struct {
	sourceExecuting          bool
	rootPrevious, zpPrevious *upolyPrimeTrialScope
	callDepth                uint32
	sourceEnded              bool
	degreesStarted           bool
	factorStage              uint8
	factorIndex, factorValue uint32
	self                     *upolyPrimeTrialScope
	service                  *UPolynomialFactorizationService
	zp                       *UPolynomialManager
	receipt                  upolyPrimeTrialInvocationReceipt
	invocation               uint64
	active, executing        bool
	constructed              uint8
	fzp                      upolyVectorState
	current                  *UPolynomialFactors
	degrees                  upolyFactorDegreeState
	transport                unumeral.Int
	transportBound           bool
	numeric                  polynomialNumericRoots
	normalize                unumeral.IntegerNormalizeInvocationReceipt
	sourceIndex              uint32
	sourceCause, bodyCause   error
}

func (s *UPolynomialFactorizationService) matchesPrimeTrialHistory(r *upolyPrimeTrialInvocationReceipt) bool {
	return s != nil && s.self == s && s.manager != nil && s.manager.self == s.manager && r != nil && r.service == s && r.parent != nil && r.parent.self == r.parent && r.parent.service == s && r.frame == &r.parent.primeTrial && r.frame.self == r.frame && r.frame.service == s && r.frame.zp != nil && r.frame.zp.self == r.frame.zp && r.frame.zp.config.Integer == s.manager.config.Integer && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (s *UPolynomialFactorizationService) matchesPrimeTrialInvocation(r *upolyPrimeTrialInvocationReceipt) bool {
	return s.matchesPrimeTrialHistory(r) && s.scope == r.parent && r.frame.active
}
func (s *UPolynomialFactorizationService) beginPrimeTrialInvocation(zp *UPolynomialManager) (upolyPrimeTrialInvocationReceipt, error) {
	if e := s.checkOwner(); e != nil {
		return upolyPrimeTrialInvocationReceipt{}, e
	}
	if s.scope == nil || zp == nil || zp.self != zp || s.scope.zp != zp {
		return upolyPrimeTrialInvocationReceipt{}, ErrUPolynomialOwner
	}
	q := &s.scope.primeTrial
	if q.active || q.invocation == ^uint64(0) {
		return upolyPrimeTrialInvocationReceipt{}, ErrUPolynomialCleanupPending
	}
	*q = upolyPrimeTrialScope{service: s, zp: zp, invocation: q.invocation + 1, active: true, executing: true}
	q.self = q
	q.receipt = upolyPrimeTrialInvocationReceipt{s, s.scope, q, q.invocation}
	return q.receipt, nil
}
func (s *UPolynomialFactorizationService) finishPrimeTrialVectorSource(r *upolyPrimeTrialInvocationReceipt, body error) error {
	if !s.matchesPrimeTrialHistory(r) {
		return ErrUPolynomialOwner
	}
	q := r.frame
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	// Source scoped vector DT is ascending numeral deletion then backing DT.
	for q.sourceIndex < q.fzp.size {
		a, e := q.zp.read(q.zp.stateView(&q.fzp), q.sourceIndex)
		if e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		if _, e = upolynomialSourceNumeralDel(q.zp.config.Integer, a); e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		q.sourceIndex++
	}
	q.fzp.size = 0
	q.zp.freeBacking(&q.fzp)
	return q.bodyCause
}
func (s *UPolynomialFactorizationService) cleanupPrimeTrialInvocation(r *upolyPrimeTrialInvocationReceipt, abort bool) error {
	if !s.matchesPrimeTrialInvocation(r) {
		return nil
	}
	q := r.frame
	if q.executing || q.callDepth != 0 {
		return ErrUPolynomialCleanupPending
	}
	if e := q.zp.modular.CleanupPNormalizeInvocation(&q.normalize, false); e != nil {
		return e
	}
	q.enterCall()
	defer q.leaveCall()
	if q.degreesStarted {
		q.degrees.close()
		q.degreesStarted = false
	}
	if q.current != nil {
		if !q.current.closed {
			if e := q.current.Close(); e != nil {
				return e
			}
		}
		q.current = nil
	}
	for q.sourceIndex < q.fzp.size {
		a, e := q.zp.read(q.zp.stateView(&q.fzp), q.sourceIndex)
		if e != nil {
			return e
		}
		if e = q.zp.config.Integer.CleanupRawNumeral(a); e != nil {
			return e
		}
		q.sourceIndex++
	}
	q.fzp.size = 0
	q.zp.freeBacking(&q.fzp)
	if abort {
		if q.transportBound {
			if e := q.zp.config.Integer.CleanupRawNumeral(&q.transport); e != nil {
				return e
			}
			q.transportBound = false
		}
		if e := q.zp.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
			return e
		}
		if e := q.cleanupNumeric(q.zp.config.Integer); e != nil {
			return e
		}
		q.active = false
	}
	return nil
}

func (q *upolyPrimeTrialScope) cleanupNumeric(z *unumeral.IntegerManager) error {
	e := unumeral.CleanupNumericAllocationRoots(z, &q.numeric.NumericAllocationRoots)
	if errors.Is(e, unumeral.ErrNumericPending) {
		return errors.Join(ErrUPolynomialCleanupPending, e)
	}
	return e
}

// Source current_fs destructor body: ascending reset, outer reset, degrees
// reset, SET(constant,1), counters, DEL(constant), reverse member backing DT.
// Cursors belong to this actual constructed child, not a global factors sweep.
func (s *UPolynomialFactorizationService) finishPrimeCurrentSource(q *upolyPrimeTrialScope) error {
	fs := q.current
	if fs == nil || fs.closed {
		return nil
	}
	m := q.zp
	z := m.config.Integer
	for q.factorStage < 5 {
		switch q.factorStage {
		case 0:
			for q.factorIndex < fs.count {
				v := &fs.factors[q.factorIndex]
				for q.factorValue < v.size {
					a, e := m.read(m.stateView(v), q.factorValue)
					if e != nil {
						return e
					}
					if _, e = upolynomialSourceNumeralDel(z, a); e != nil {
						return e
					}
					q.factorValue++
				}
				v.size = 0
				q.factorValue = 0
				q.factorIndex++
			}
			q.factorStage++
		case 1:
			for i := uint32(0); i < fs.count; i++ {
				m.freeBacking(&fs.factors[i])
			}
			fs.count = 0
			fs.factors = fs.factors[:0]
			fs.degrees = fs.degrees[:0]
			q.factorStage++
		case 2:
			// A stopped source DEL is not an owning acknowledgement at this SET.
			if fs.constant.NativeCellToken() != 0 && z.IsLiveCellToken(fs.constant.NativeCellToken()) {
				if z.InterruptedNumeral(&fs.constant) {
					return arena.ErrInvalidSlot
				}
			}
			context := integerAllocationInvocationContext{primeTrial: &q.receipt, primeTrialInvocation: q.invocation}
			if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SetInt64(&fs.constant, 1); e != nil {
				return e
			}
			fs.totalFactors = 0
			fs.totalDegree = 0
			q.factorStage++
		case 3:
			if _, e := upolynomialSourceNumeralDel(z, &fs.constant); e != nil {
				return e
			}
			q.factorStage++
		case 4:
			b := z.Budget()
			fs.degrees = nil
			fs.factors = nil
			b.Release(fs.header)
			m.liveFactors--
			fs.closed = true
			q.factorStage++
		}
	}
	return nil
}
func (s *UPolynomialFactorizationService) finishPrimeTrialSource(r *upolyPrimeTrialInvocationReceipt, body error) error {
	if !s.matchesPrimeTrialHistory(r) {
		return ErrUPolynomialOwner
	}
	q := r.frame
	q.enterCall()
	defer q.leaveCall()
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceEnded || q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	if e := r.parent.traceSourceBlocker(); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}

	if q.degreesStarted {
		q.degrees.close()
		q.degreesStarted = false
	}
	if e := s.finishPrimeCurrentSource(q); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	q.current = nil
	if e := s.finishPrimeTrialVectorSource(r, body); q.sourceCause != nil {
		return e
	}
	q.sourceEnded = true
	if !q.transportBound && !polynomialNegNumericPending(&q.numeric) && !q.zp.modular.MatchesPNormalizeInvocation(&q.normalize) {
		q.active = false
	}
	return q.bodyCause
}
func (q *upolyPrimeTrialScope) enterCall() {
	if q.callDepth == 0 {
		m := q.service.manager
		q.rootPrevious = m.primeTrialCaller
		m.primeTrialCaller = q
		if q.zp != m {
			q.zpPrevious = q.zp.primeTrialCaller
			q.zp.primeTrialCaller = q
		}
	}
	q.callDepth++
}
func (q *upolyPrimeTrialScope) leaveCall() {
	q.callDepth--
	if q.callDepth == 0 {
		m := q.service.manager
		m.primeTrialCaller = q.rootPrevious
		if q.zp != m {
			q.zp.primeTrialCaller = q.zpPrevious
		}
	}
}

func (s *UPolynomialFactorizationService) primeTrialInvocationSourceCause(r *upolyPrimeTrialInvocationReceipt) error {
	if !s.matchesPrimeTrialHistory(r) {
		return nil
	}
	return r.frame.sourceCause
}

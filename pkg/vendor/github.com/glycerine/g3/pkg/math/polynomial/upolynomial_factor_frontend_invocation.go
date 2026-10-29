package polynomial

// Source: src/math/polynomial/upolynomial.cpp:3042-3118.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

type upolyFactorFrontendInvocationReceipt struct {
	service    *UPolynomialFactorizationService
	frame      *upolyFactorFrontendScope
	invocation uint64
	history    *upolyFactorFrontendHistory
}
type upolyFactorFrontendHistory struct {
	scalarTrace                                                                       [4]unumeral.NumeralInsertionReceipt
	coreTrace                                                                         upolyDisplayInvocationReceipt
	euclidTrace                                                                       upolyDisplayInvocationReceipt
	modGCDTrace                                                                       upolyDisplayInvocationReceipt
	exactTrace                                                                        upolyDisplayInvocationReceipt
	perfectSquare                                                                     unumeral.IntegerPerfectSquareInvocationReceipt
	perfectSquareBodyCause, perfectSquareSourceCause, perfectSquareOwnCause           error
	vectorNormalizeInvocation                                                         uint64
	vectorNormalizeBodyCause, vectorNormalizeSourceCause, vectorNormalizeOwnCause     error
	vectorNormalizePeak, vectorNormalizeStoppedStage                                  uint8
	modularPowerInvocation                                                            uint64
	modularPowerBodyCause, modularPowerSourceCause, modularPowerOwnCause              error
	modularPowerEndEntered                                                            bool
	quadraticInvocation                                                               uint64
	quadraticBodyCause, quadraticSourceCause, quadraticOwnCause                       error
	quadraticPeak, quadraticStoppedStage                                              uint8
	quadraticStoppedIndex                                                             uint32
	flipInvocation, primitiveInvocation, divisibilityInvocation, derivativeInvocation uint64
	flipBodyCause, flipSourceCause, flipOwnCause                                      error
	primitiveBodyCause, primitiveSourceCause, primitiveOwnCause                       error
	divisibilityBodyCause, divisibilitySourceCause, divisibilityOwnCause              error
	derivativeBodyCause, derivativeOwnCause                                           error
	derivativePhase                                                                   uint8
	divisibilityAddMul                                                                unumeral.IntegerAddMulInvocationReceipt
	boolNumeral                                                                       unumeral.IntegerDividesNumeralInvocationReceipt
	addMul                                                                            unumeral.IntegerAddMulInvocationReceipt

	remainderInvocation, monicInvocation, craInvocation            uint64
	remainderBodyCause, remainderSourceCause, remainderOwnCause    error
	monicBodyCause, monicSourceCause, monicOwnCause                error
	craBodyCause, craSourceCause, craOwnCause                      error
	remainderAddMul                                                unumeral.IntegerAddMulInvocationReceipt
	monicEndEntered                                                bool
	monicSourceIndex                                               uint8
	imageFrame                                                     *upolyFactorImageScope
	imageInvocation                                                uint64
	imageBodyCause, imageSourceCause, imageOwnCause                error
	imageSourceStoppedStage                                        uint8
	modularGCDFrame                                                *upolyFactorModularGCDScope
	modularGCDInvocation                                           uint64
	modularGCDBodyCause, modularGCDSourceCause, modularGCDOwnCause error
	issued                                                         upolyFactorFrontendInvocationReceipt
	complete                                                       bool
	gcdRaw, divisionRaw, extendedRaw                               unumeral.IntegerRawInvocationReceipt
	normalize                                                      unumeral.IntegerNormalizeInvocationReceipt
	squareFree                                                     upolyFactorSquareFreeInvocationReceipt
	self                                                           *upolyFactorFrontendHistory
	service                                                        *UPolynomialFactorizationService
	next                                                           *upolyFactorFrontendHistory
	header, invocation                                             uint64
	bodyCause, sourceCause, ownCause                               error
	sourceStoppedStage                                             uint8
	sourceStoppedIndex                                             uint32
}
type upolyFactorModularGCDScope struct {
	executing, sourceExecuting, ownExecuting bool
	owner                                    *upolyFactorFrontendScope
	invocation                               uint64
	values                                   [6]unumeral.Int
	constructed                              uint8
	bodyCause, sourceCause, ownCause         error
}
type upolyFactorImageScope struct {
	executing, sourceExecuting, ownExecuting         bool
	owner                                            *upolyFactorFrontendScope
	invocation                                       uint64
	old, c                                           unumeral.Int
	oldMode                                          bool
	savedConstructed, classConstructed, cConstructed bool
	sourceStoppedStage                               uint8
	bodyCause, sourceCause, ownCause                 error
}
type upolyFactorRemainderScope struct {
	addMul                                   unumeral.IntegerAddMulInvocationReceipt
	owner                                    *upolyFactorFrontendScope
	invocation                               uint64
	am                                       unumeral.Int
	constructed                              bool
	executing, sourceExecuting, ownExecuting bool
	bodyCause, sourceCause, ownCause         error
}

type upolyFactorMonicScope struct {
	owner                                                        *upolyFactorFrontendScope
	invocation                                                   uint64
	lc, inv                                                      unumeral.Int
	active, executing, sourceExecuting, ownExecuting, endEntered bool
	sourceIndex, ownIndex                                        uint8
	bodyCause, sourceCause, ownCause                             error
}

type upolyFactorCRAScope struct {
	trace                                    upolyCRATrace
	owner                                    *upolyFactorFrontendScope
	invocation                               uint64
	values                                   [12]unumeral.Int
	constructed, peak                        uint8
	executing, sourceExecuting, ownExecuting bool
	bodyCause, sourceCause, ownCause         error
}

type upolyFactorDerivativeRaw struct {
	self               *upolyFactorDerivativeRaw
	owner              *upolyFactorDerivativeScope
	next               *upolyFactorDerivativeRaw
	invocation, header uint64
	value              unumeral.Int
}
type upolyFactorDerivativeScope struct {
	retired                              *upolyFactorDerivativeRaw
	ownCause                             error
	owner                                *upolyFactorFrontendScope
	invocation                           uint64
	d                                    unumeral.Int
	executing, constructed, bodyReturned bool
	phase                                uint8
	bodyCause                            error
}

type upolyFactorDivisibilityScope struct {
	owner                                                 *upolyFactorFrontendScope
	invocation                                            uint64
	ratio                                                 unumeral.Int
	addMul                                                unumeral.IntegerAddMulInvocationReceipt
	executing, sourceExecuting, ownExecuting, constructed bool
	bodyCause, sourceCause, ownCause                      error
}

type upolyFactorPrimitiveScope struct {
	owner                                                 *upolyFactorFrontendScope
	invocation                                            uint64
	cont                                                  unumeral.Int
	executing, sourceExecuting, ownExecuting, constructed bool
	bodyCause, sourceCause, ownCause                      error
}

type upolyFactorVectorNormalizeScope struct {
	owner                                                          *upolyFactorFrontendScope
	invocation                                                     uint64
	g, old, tmp                                                    unumeral.Int
	constructed, peak                                              uint8
	executing, sourceExecuting, ownExecuting, coefficientExecuting bool
	bodyCause, sourceCause, ownCause                               error
}

type upolyFactorModularPowerScope struct {
	owner                                                        *upolyFactorFrontendScope
	invocation                                                   uint64
	power                                                        unumeral.Int
	active, executing, sourceExecuting, ownExecuting, endEntered bool
	bodyCause, sourceCause, ownCause                             error
}

type upolyFactorQuadraticScope struct {
	owner                                    *upolyFactorFrontendScope
	invocation                               uint64
	values                                   [4]unumeral.Int
	f1, f2, product                          upolyVectorState
	constructed, peak                        uint8
	sourceIndex                              uint32
	executing, sourceExecuting, ownExecuting bool
	bodyCause, sourceCause, ownCause         error
}

type upolyFactorFlipScope struct {
	owner                                                 *upolyFactorFrontendScope
	invocation                                            uint64
	c                                                     unumeral.Int
	constructed, executing, sourceExecuting, ownExecuting bool
	bodyCause, sourceCause, ownCause                      error
}

type upolyFactorFrontendScope struct {
	scalarTrace     [4]unumeral.NumeralInsertionReceipt
	coreTrace       upolyDisplayInvocationReceipt
	euclidTrace     upolyDisplayInvocationReceipt
	modGCDTrace     upolyDisplayInvocationReceipt
	exactTrace      upolyDisplayInvocationReceipt
	perfectSquare   unumeral.IntegerPerfectSquareInvocationReceipt
	vectorNormalize upolyFactorVectorNormalizeScope
	modularPower    upolyFactorModularPowerScope
	quadratic       upolyFactorQuadraticScope
	flip            upolyFactorFlipScope
	result          *UPolynomialFactors

	primitive                                        upolyFactorPrimitiveScope
	divisibility                                     upolyFactorDivisibilityScope
	boolNumeral                                      unumeral.IntegerDividesNumeralInvocationReceipt
	derivative                                       upolyFactorDerivativeScope
	cra                                              upolyFactorCRAScope
	monic                                            upolyFactorMonicScope
	remainder                                        upolyFactorRemainderScope
	image                                            upolyFactorImageScope
	receipt                                          upolyFactorFrontendInvocationReceipt
	modularGCD                                       upolyFactorModularGCDScope
	history                                          *upolyFactorFrontendHistory
	self                                             *upolyFactorFrontendScope
	service                                          *UPolynomialFactorizationService
	previous                                         *upolyFactorFrontendScope
	previousInteger                                  *upolyFactorFrontendScope
	invocation                                       uint64
	active, executing, sourceExecuting, ownExecuting bool
	content                                          unumeral.Int
	numeric                                          polynomialNumericRoots
	normalize                                        unumeral.IntegerNormalizeInvocationReceipt
	gcdRaw, divisionRaw, extendedRaw                 unumeral.IntegerRawInvocationReceipt
	addMul                                           unumeral.IntegerAddMulInvocationReceipt
	pp, prime, A, B, D                               upolyVectorState
	constructed                                      uint8
	sourceIndex                                      uint32
	sourceStoppedStage                               uint8
	sourceStoppedIndex                               uint32
	squareFree                                       upolyFactorSquareFreeInvocationReceipt
	bodyCause, sourceCause, ownCause                 error
}

func (s *UPolynomialFactorizationService) beginFactorFrontend() (*upolyFactorFrontendScope, error) {
	if s == nil || s.self != s || s.manager == nil || s.manager.self != s.manager || s.closed {
		return nil, ErrUPolynomialOwner
	}
	q := &s.frontend
	if q.active || q.invocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	n := uint64(reflect.TypeOf(upolyFactorFrontendHistory{}).Size())
	if e := s.manager.config.Integer.Budget().Reserve(n); e != nil {
		return nil, e
	}
	h := &upolyFactorFrontendHistory{service: s, header: n, invocation: q.invocation + 1, next: s.frontendHistory}
	h.self = h
	s.frontendHistory = h
	*q = upolyFactorFrontendScope{history: h, service: s, invocation: q.invocation + 1, active: true, executing: true}
	q.self = q
	q.receipt = upolyFactorFrontendInvocationReceipt{s, q, q.invocation, h}
	h.issued = q.receipt
	q.enterCall()
	return q, nil
}
func (q *upolyFactorFrontendScope) vector(n uint8) *upolyVectorState {
	switch n {
	case 2:
		return &q.pp
	case 3:
		return &q.prime
	case 4:
		return &q.A
	case 5:
		return &q.B
	case 6:
		return &q.D
	}
	return nil
}
func (q *upolyFactorFrontendScope) bodyReturned() {
	q.executing = false
	q.leaveCall()
}
func (q *upolyFactorFrontendScope) finish(body error) error {
	if q == nil || q.self != q || q.service == nil || q.service.self != q.service || q != &q.service.frontend || q.service.manager == nil || q.service.manager.self != q.service.manager {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if !q.active {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	defer q.recordHistory()
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause == nil {
		q.sourceCause = q.scalarTraceBlocker()
	}
	if q.sourceCause == nil {
		q.sourceCause = q.coreTrace.sourceBlocker()
	}
	if q.sourceCause == nil {
		q.sourceCause = q.euclidTrace.sourceBlocker()
	}
	if q.sourceCause == nil {
		q.sourceCause = q.modGCDTrace.sourceBlocker()
	}
	if q.sourceCause == nil {
		q.sourceCause = q.exactTrace.sourceBlocker()
	}
	if q.sourceCause == nil {
		q.sourceCause = q.quadratic.sourceCause
	}
	if q.sourceCause == nil {
		q.sourceCause = q.flip.sourceCause
	}
	if q.sourceCause == nil {
		q.sourceCause = q.service.manager.config.Integer.DividesNumeralInvocationSourceCause(&q.boolNumeral)
	}
	if q.sourceCause == nil {
		q.sourceCause = q.service.manager.config.Integer.AddMulInvocationSourceCause(&q.addMul)
	}
	if q.sourceCause == nil {
		q.sourceCause = q.modularGCD.sourceCause
	}
	if q.sourceCause == nil {
		q.sourceCause = q.service.manager.config.Integer.RawInvocationSourceCause(&q.gcdRaw)
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	if e := q.service.factorSquareFreeInvocationSourceCause(&q.squareFree); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	q.sourceExecuting = true
	q.enterCall()
	defer func() {
		q.sourceExecuting = false
		q.leaveCall()
	}()
	for q.constructed > 0 {
		var e error
		if q.constructed == 1 {
			_, e = upolynomialSourceNumeralDel(q.service.manager.config.Integer, &q.content)
		} else {
			e = upolynomialScopedVectorSourceDel(q.service.manager, q.vector(q.constructed), &q.sourceIndex)
		}
		if e != nil {
			q.sourceCause = e
			q.sourceStoppedStage = q.constructed
			q.sourceStoppedIndex = q.sourceIndex
			return errors.Join(q.bodyCause, e)
		}
		q.constructed--
		q.sourceIndex = 0
	}
	if !q.hasUnjoined() {
		q.active = false
	}
	return q.bodyCause
}

func (q *upolyFactorFrontendScope) own(abort bool) (cause error) {
	if q.self != q || q.service == nil || q.service.self != q.service || q != &q.service.frontend {
		return ErrUPolynomialOwner
	}
	if !q.active {
		return nil
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.ownExecuting = true
	q.enterCall()
	defer func() {
		q.ownExecuting = false
		q.leaveCall()
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
		q.recordHistory()
	}()
	if e := q.cra.trace.cleanup(); e != nil {
		return e
	}
	if e := q.cleanupScalarTrace(); e != nil {
		return e
	}
	if e := q.coreTrace.cleanupOwned(); e != nil {
		return e
	}
	if e := q.euclidTrace.cleanupOwned(); e != nil {
		return e
	}
	if e := q.modGCDTrace.cleanupOwned(); e != nil {
		return e
	}
	if e := q.exactTrace.cleanupOwned(); e != nil {
		return e
	}
	z := q.service.manager.config.Integer

	if e := z.CleanupDividesNumeralInvocation(&q.boolNumeral, false); e != nil {
		return e
	}
	if e := q.service.manager.modular.CleanupPNormalizeInvocation(&q.normalize, false); e != nil {
		return e
	}
	for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.extendedRaw, &q.divisionRaw, &q.gcdRaw} {
		if e := z.CleanupRawInvocation(r, false); e != nil {
			return e
		}
	}
	if e := z.CleanupPerfectSquareInvocation(&q.perfectSquare, false); e != nil {
		return e
	}
	if e := q.modularPower.own(false); e != nil {
		return e
	}
	if e := q.vectorNormalize.own(); e != nil {
		return e
	}
	if e := q.quadratic.own(); e != nil {
		return e
	}
	if e := q.flip.own(); e != nil {
		return e
	}
	if e := q.primitive.own(); e != nil {
		return e
	}
	if e := q.divisibility.own(false); e != nil {
		return e
	}
	if e := z.CleanupAddMulInvocation(&q.addMul, false); e != nil {
		return e
	}
	if e := q.cra.own(); e != nil {
		return e
	}
	if e := q.monic.own(false); e != nil {
		return e
	}
	if e := q.remainder.own(false); e != nil {
		return e
	}
	if e := q.image.own(); e != nil {
		return e
	}
	if e := q.modularGCD.own(); e != nil {
		return e
	}
	if e := q.service.cleanupFactorSquareFreeInvocation(&q.squareFree, false); e != nil {
		return e
	}
	for q.constructed > 0 {
		var e error
		if q.constructed == 1 {
			e = q.service.manager.config.Integer.CleanupRawNumeral(&q.content)
		} else {
			v := q.vector(q.constructed)
			e = q.service.manager.reset(v)
			if e == nil {
				q.service.manager.freeBacking(v)
			}
		}
		if e != nil {
			return e
		}
		q.constructed--
	}
	if abort {
		if e := z.CleanupPerfectSquareInvocation(&q.perfectSquare, true); e != nil {
			return e
		}
		if e := q.modularPower.own(true); e != nil {
			return e
		}

		if e := q.divisibility.own(true); e != nil {
			return e
		}
		if e := z.CleanupDividesNumeralInvocation(&q.boolNumeral, true); e != nil {
			return e
		}
		if e := z.CleanupAddMulInvocation(&q.addMul, true); e != nil {
			return e
		}
		if e := q.derivative.own(); e != nil {
			return e
		}
		if e := q.monic.own(true); e != nil {
			return e
		}
		if e := q.remainder.own(true); e != nil {
			return e
		}
		for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.extendedRaw, &q.divisionRaw, &q.gcdRaw} {
			if e := z.CleanupRawInvocation(r, true); e != nil {
				return e
			}
		}
		if e := q.service.manager.modular.CleanupPNormalizeInvocation(&q.normalize, true); e != nil {
			return e
		}
		if e := cleanupUPolynomialNumericRoots(z, &q.numeric); e != nil {
			return e
		}
		if e := q.service.cleanupFactorSquareFreeInvocation(&q.squareFree, true); e != nil {
			return e
		}
		q.active = false
	}
	return nil
}

// Retention is decided from exact captured child tuples and directly stamped
// leaves; a nil square-free class frame does not imply diagnostic completion.
func (q *upolyFactorFrontendScope) hasUnjoined() bool {
	if q.scalarTracePending() {
		return true
	}
	if q.service.manager.displayReceiptPending(&q.coreTrace) {
		return true
	}
	if q.service.manager.displayReceiptPending(&q.euclidTrace) {
		return true
	}
	if q.service.manager.displayReceiptPending(&q.modGCDTrace) {
		return true
	}
	if q.cra.trace.pending() {
		return true
	}
	z := q.service.manager.config.Integer
	if z.MatchesPerfectSquareInvocation(&q.perfectSquare) {
		return true
	}
	if q.modularPower.active {
		return true
	}

	if z.MatchesDividesNumeralInvocation(&q.boolNumeral) || z.MatchesAddMulInvocation(&q.addMul) || z.MatchesAddMulInvocation(&q.divisibility.addMul) {
		return true
	}
	if q.derivative.retired != nil {
		return true
	}
	if q.derivative.d.NativeCellToken() != 0 {
		return true
	}
	if q.monic.active {
		return true
	}
	if z.MatchesAddMulInvocation(&q.remainder.addMul) {
		return true
	}
	if polynomialNegNumericPending(&q.numeric) {
		return true
	}
	for _, r := range []*unumeral.IntegerRawInvocationReceipt{&q.gcdRaw, &q.divisionRaw, &q.extendedRaw} {
		if z.MatchesRawInvocation(r) {
			return true
		}
	}
	if q.service.manager.modular.MatchesPNormalizeInvocation(&q.normalize) {
		return true
	}
	r := &q.squareFree
	if q.service.matchesSquareFreeHistory(r) {
		if r.frame != nil && !r.frame.retired {
			return true
		}
		if q.service.diagnostic.matches(&r.diagnostic) && q.service.diagnostic.active {
			return true
		}
	}
	return false
}

// Diagnostic-only immutable invocation identity; it carries no NUM, backing,
// lower context, or authority to clean any frame after reuse.
func (q *upolyFactorFrontendScope) recordHistory() {
	h := q.history
	if h == nil || h.self != h || h.service != q.service || h.invocation != q.invocation || h.header == 0 {
		return
	}
	h.scalarTrace = q.scalarTrace
	h.coreTrace = q.coreTrace
	h.euclidTrace = q.euclidTrace
	h.modGCDTrace = q.modGCDTrace
	h.exactTrace = q.exactTrace
	h.perfectSquare = q.perfectSquare
	if z := q.service.manager.config.Integer; z.MatchesPerfectSquareHistory(&q.perfectSquare) {
		h.perfectSquareBodyCause, h.perfectSquareSourceCause, h.perfectSquareOwnCause = q.perfectSquare.Causes()
	}
	h.vectorNormalizeInvocation = q.vectorNormalize.invocation
	h.vectorNormalizeBodyCause = q.vectorNormalize.bodyCause
	h.vectorNormalizeSourceCause = q.vectorNormalize.sourceCause
	h.vectorNormalizeOwnCause = q.vectorNormalize.ownCause
	h.vectorNormalizePeak = q.vectorNormalize.peak
	h.vectorNormalizeStoppedStage = q.vectorNormalize.constructed
	h.modularPowerInvocation = q.modularPower.invocation
	h.modularPowerBodyCause = q.modularPower.bodyCause
	h.modularPowerSourceCause = q.modularPower.sourceCause
	h.modularPowerOwnCause = q.modularPower.ownCause
	h.modularPowerEndEntered = q.modularPower.endEntered
	h.quadraticInvocation = q.quadratic.invocation
	h.quadraticBodyCause = q.quadratic.bodyCause
	h.quadraticSourceCause = q.quadratic.sourceCause
	h.quadraticOwnCause = q.quadratic.ownCause
	h.quadraticPeak = q.quadratic.peak
	h.quadraticStoppedStage = q.quadratic.constructed
	h.quadraticStoppedIndex = q.quadratic.sourceIndex
	h.complete = !q.active
	h.flipInvocation = q.flip.invocation
	h.flipBodyCause = q.flip.bodyCause
	h.flipSourceCause = q.flip.sourceCause
	h.flipOwnCause = q.flip.ownCause
	h.primitiveInvocation = q.primitive.invocation
	h.primitiveBodyCause = q.primitive.bodyCause
	h.primitiveSourceCause = q.primitive.sourceCause
	h.primitiveOwnCause = q.primitive.ownCause
	h.divisibilityInvocation = q.divisibility.invocation
	h.divisibilityBodyCause = q.divisibility.bodyCause
	h.divisibilitySourceCause = q.divisibility.sourceCause
	h.divisibilityOwnCause = q.divisibility.ownCause
	h.divisibilityAddMul = q.divisibility.addMul
	h.derivativeInvocation = q.derivative.invocation
	h.derivativeBodyCause = q.derivative.bodyCause
	h.derivativeOwnCause = q.derivative.ownCause
	h.derivativePhase = q.derivative.phase
	h.boolNumeral = q.boolNumeral
	h.addMul = q.addMul

	h.remainderInvocation = q.remainder.invocation
	h.remainderBodyCause = q.remainder.bodyCause
	h.remainderSourceCause = q.remainder.sourceCause
	h.remainderOwnCause = q.remainder.ownCause
	h.remainderAddMul = q.remainder.addMul
	h.monicInvocation = q.monic.invocation
	h.monicBodyCause = q.monic.bodyCause
	h.monicSourceCause = q.monic.sourceCause
	h.monicOwnCause = q.monic.ownCause
	h.monicEndEntered = q.monic.endEntered
	h.monicSourceIndex = q.monic.sourceIndex
	h.craInvocation = q.cra.invocation
	h.craBodyCause = q.cra.bodyCause
	h.craSourceCause = q.cra.sourceCause
	h.craOwnCause = q.cra.ownCause
	h.imageFrame = &q.image
	h.imageInvocation = q.image.invocation
	h.imageBodyCause = q.image.bodyCause
	h.imageSourceCause = q.image.sourceCause
	h.imageOwnCause = q.image.ownCause
	h.imageSourceStoppedStage = q.image.sourceStoppedStage
	h.modularGCDFrame = &q.modularGCD
	h.modularGCDInvocation = q.modularGCD.invocation
	h.modularGCDBodyCause = q.modularGCD.bodyCause
	h.modularGCDSourceCause = q.modularGCD.sourceCause
	h.modularGCDOwnCause = q.modularGCD.ownCause
	h.gcdRaw = q.gcdRaw
	h.divisionRaw = q.divisionRaw
	h.extendedRaw = q.extendedRaw
	h.normalize = q.normalize
	h.squareFree = q.squareFree
	h.bodyCause = q.bodyCause
	h.sourceCause = q.sourceCause
	h.ownCause = q.ownCause
	h.sourceStoppedStage = q.sourceStoppedStage
	h.sourceStoppedIndex = q.sourceStoppedIndex
}

func (q *upolyFactorModularGCDScope) finish(body error) error {
	if q.owner == nil || q != &q.owner.modularGCD || !q.owner.actual() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause == nil {
		q.sourceCause = q.owner.euclidTrace.sourceBlocker()
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	if e := q.owner.modGCDTrace.sourceBlocker(); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	z := q.owner.service.manager.config.Integer
	if e := z.DividesNumeralInvocationSourceCause(&q.owner.boolNumeral); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	if e := q.owner.primitive.sourceCause; e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	if e := q.owner.divisibility.sourceCause; e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	if e := q.owner.cra.sourceCause; e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	if e := q.owner.image.sourceCause; e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	if e := q.owner.numericSourceCause(); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	if e := z.RawInvocationSourceCause(&q.owner.gcdRaw); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	for q.constructed > 0 {
		_, e := upolynomialSourceNumeralDel(z, &q.values[q.constructed-1])
		if e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		q.constructed--
	}
	return q.bodyCause
}
func (q *upolyFactorModularGCDScope) own() (cause error) {
	if q.owner == nil {
		return nil
	}
	if q != &q.owner.modularGCD || !q.owner.actual() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.ownExecuting = true
	defer func() { q.ownExecuting = false }()
	defer func() {
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	for q.constructed > 0 {
		if e := q.owner.service.manager.config.Integer.CleanupRawNumeral(&q.values[q.constructed-1]); e != nil {
			return e
		}
		q.constructed--
	}
	return nil
}

func (s *UPolynomialFactorizationService) matchesFactorFrontendHistory(r *upolyFactorFrontendInvocationReceipt) bool {
	if s == nil || s.self != s || s.closed || s.manager == nil || s.manager.self != s.manager || r == nil || r.service != s || r.frame != &s.frontend || r.invocation == 0 || r.history == nil {
		return false
	}
	h := r.history
	if h.self != h || h.service != s || h.header == 0 || h.invocation != r.invocation || h.issued != *r {
		return false
	}
	for old := s.frontendHistory; old != nil; old = old.next {
		if old == h {
			return true
		}
	}
	return false
}
func (s *UPolynomialFactorizationService) cleanupFactorFrontendInvocation(r *upolyFactorFrontendInvocationReceipt, abort bool) error {
	if !s.matchesFactorFrontendHistory(r) {
		return ErrUPolynomialOwner
	}
	if r.invocation != s.frontend.invocation {
		if r.history.complete {
			return nil
		}
		return ErrUPolynomialOwner
	}
	if s.frontend.receipt != *r {
		return ErrUPolynomialOwner
	}
	return s.frontend.own(abort)
}

func (q *upolyFactorImageScope) finish(body error) error {
	if q.owner == nil || q != &q.owner.image || !q.owner.actual() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause == nil {
		q.sourceCause = q.owner.euclidTrace.sourceBlocker()
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	if q.owner.monic.sourceCause != nil {
		q.sourceCause = q.owner.monic.sourceCause
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	if q.owner.remainder.sourceCause != nil {
		q.sourceCause = q.owner.remainder.sourceCause
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	if q.owner.numericSourceCause() != nil {
		q.sourceCause = q.owner.numericSourceCause()
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	m := q.owner.service.manager
	stop := func(e error) error {
		if q.sourceCause == nil {
			if q.cConstructed {
				q.sourceStoppedStage = 3
			} else if q.classConstructed {
				q.sourceStoppedStage = 2
			} else if q.savedConstructed {
				q.sourceStoppedStage = 1
			}
		}
		if q.sourceCause == nil {
			q.sourceCause = e
		}
		return errors.Join(q.bodyCause, e)
	}
	if q.cConstructed {
		if _, e := upolynomialSourceNumeralDel(m.config.Integer, &q.c); e != nil {
			return stop(e)
		}
		q.cConstructed = false
	}
	if q.classConstructed {
		var e error
		if q.oldMode {
			e = m.modular.WithAllocationContext((numericIntegerModularManager_factorFrontendAllocationContext(m.modular, q.owner)).utility(m.modular.Integers())).SetZP(&q.old)
		} else {
			e = m.modular.SetZ()
		}
		if e != nil {
			return stop(e)
		}
		q.classConstructed = false
	}
	if q.savedConstructed {
		if _, e := upolynomialSourceNumeralDel(m.config.Integer, &q.old); e != nil {
			return stop(e)
		}
		q.savedConstructed = false
	}
	return q.bodyCause
}
func (q *upolyFactorImageScope) own() (cause error) {
	if q.owner == nil {
		return nil
	}
	if q != &q.owner.image || !q.owner.actual() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.ownExecuting = true
	defer func() { q.ownExecuting = false }()
	defer func() {
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	m := q.owner.service.manager
	if q.cConstructed {
		if e := m.config.Integer.CleanupRawNumeral(&q.c); e != nil {
			return e
		}
		q.cConstructed = false
	}
	if q.classConstructed {
		var e error
		if q.oldMode {
			e = m.modular.WithAllocationContext((numericIntegerModularManager_factorFrontendAllocationContext(m.modular, q.owner)).utility(m.modular.Integers())).SetZP(&q.old)
		} else {
			e = m.modular.SetZ()
		}
		if e != nil {
			return e
		}
		q.classConstructed = false
	}
	if q.savedConstructed {
		if e := m.config.Integer.CleanupRawNumeral(&q.old); e != nil {
			return e
		}
		q.savedConstructed = false
	}
	return nil
}

// Lifecycle-only registered actual caller. Numeric acquisition continues to
// require the explicit immutable invocation context at every lower leaf.
func (q *upolyFactorFrontendScope) enterCall() {
	m := q.service.manager
	q.previous = m.factorFrontendCaller
	m.factorFrontendCaller = q
	q.previousInteger = factorFrontendCaller(m.config.Integer)
	setFactorFrontendCaller(m.config.Integer, q)
}
func (q *upolyFactorFrontendScope) leaveCall() {
	m := q.service.manager
	m.factorFrontendCaller = q.previous
	q.previous = nil
	setFactorFrontendCaller(m.config.Integer, q.previousInteger)
	q.previousInteger = nil
}
func numericIntegerManager_factorFrontendCallerExecuting(m *unumeral.IntegerManager) bool {
	if m == nil || !m.ValidOwner() {
		return false
	}
	if s := factorFrontendConstantCaller(m); s != nil && s.manager != nil && s.manager.config.Integer == m && s.constantCallerExecuting() {
		return true
	}
	for q := factorFrontendCaller(m); q != nil; q = q.previousInteger {
		if q.self == q && q.service != nil && q.service.self == q.service && q == &q.service.frontend && q.invocation != 0 && q.service.manager != nil && q.service.manager.self == q.service.manager && q.service.manager.config.Integer == m && !q.service.closed && (q.executing || q.sourceExecuting || q.ownExecuting) {
			return true
		}
	}
	return false
}

// Source: src/math/polynomial/upolynomial.cpp:516-567.
func (q *upolyFactorRemainderScope) valid() bool {
	o := q.owner
	return o != nil && o.self == o && o.service != nil && o.service.self == o.service && o == &o.service.frontend && q == &o.remainder && o.service.manager != nil && o.service.manager.self == o.service.manager
}
func (q *upolyFactorRemainderScope) finish(body error) error {
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	z := q.owner.service.manager.config.Integer
	if e := z.AddMulInvocationSourceCause(&q.addMul); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	if e := q.owner.numericSourceCause(); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	if q.constructed {
		if _, e := upolynomialSourceNumeralDel(z, &q.am); e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		q.constructed = false
	}
	return q.bodyCause
}
func (q *upolyFactorRemainderScope) own(abort bool) (cause error) {
	if q.owner == nil {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	z := q.owner.service.manager.config.Integer
	if e := z.CleanupAddMulInvocation(&q.addMul, false); e != nil {
		return e
	}
	if q.constructed {
		if e := z.CleanupRawNumeral(&q.am); e != nil {
			return e
		}
		q.constructed = false
	}
	if abort {
		return z.CleanupAddMulInvocation(&q.addMul, true)
	}
	return nil
}

// Source: src/math/polynomial/upolynomial.h:361-364.
func (q *upolyFactorMonicScope) valid() bool {
	o := q.owner
	return o != nil && o.self == o && o.service != nil && o.service.self == o.service && o == &o.service.frontend && q == &o.monic && o.service.manager != nil && o.service.manager.self == o.service.manager
}
func (q *upolyFactorMonicScope) finishEnd() error {
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	// The actual internal end is reached while its own body is executing. It is
	// not an externally callable source-finalize adapter.
	if !q.executing || q.sourceExecuting || q.ownExecuting || !q.endEntered {
		return ErrUPolynomialCleanupPending
	}
	if q.sourceCause != nil {
		return q.sourceCause
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	z := q.owner.service.manager.config.Integer
	if q.owner.numericSourceCause() != nil {
		q.sourceCause = q.owner.numericSourceCause()
		return q.sourceCause
	}
	for q.sourceIndex < 2 {
		v := &q.lc
		if q.sourceIndex == 1 {
			v = &q.inv
		}
		if _, e := upolynomialSourceNumeralDel(z, v); e != nil {
			q.sourceCause = e
			return e
		}
		q.sourceIndex++
	}
	q.active = false
	return nil
}
func (q *upolyFactorMonicScope) own(abort bool) (cause error) {
	if q.owner == nil {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if !q.active {
		return nil
	}
	if !abort && !q.endEntered {
		return nil
	}
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	z := q.owner.service.manager.config.Integer
	for q.ownIndex < 2 {
		v := &q.lc
		if q.ownIndex == 1 {
			v = &q.inv
		}
		if e := z.CleanupRawNumeral(v); e != nil {
			return e
		}
		q.ownIndex++
	}
	q.active = false
	return nil
}

// Source: src/math/polynomial/upolynomial.cpp:663-721.
func (q *upolyFactorCRAScope) valid() bool {
	o := q.owner
	return o != nil && o.self == o && o.service != nil && o.service.self == o.service && o == &o.service.frontend && q == &o.cra && o.service.manager != nil && o.service.manager.self == o.service.manager
}
func (q *upolyFactorCRAScope) finish(body error) error {
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	if e := q.trace.sourceFailure(); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	z := q.owner.service.manager.config.Integer
	if e := q.owner.numericSourceCause(); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	if e := z.RawInvocationSourceCause(&q.owner.extendedRaw); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	for q.constructed > 0 {
		if _, e := upolynomialSourceNumeralDel(z, &q.values[q.constructed-1]); e != nil {
			q.sourceCause = e
			q.trace.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		q.constructed--
	}
	return q.bodyCause
}
func (q *upolyFactorCRAScope) own() (cause error) {
	if q.owner == nil {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if e := q.trace.cleanup(); e != nil {
		return e
	}
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	z := q.owner.service.manager.config.Integer
	for q.constructed > 0 {
		if e := z.CleanupRawNumeral(&q.values[q.constructed-1]); e != nil {
			return e
		}
		q.constructed--
	}
	return nil
}

func (q *upolyFactorFrontendScope) actual() bool {
	return q != nil && q.self == q && q.service != nil && q.service.self == q.service && q == &q.service.frontend && q.service.manager != nil && q.service.manager.self == q.service.manager && !q.service.closed
}

// Source: src/math/polynomial/upolynomial.cpp:357-361. This is transport
// storage for an actually published RAW d before the next source declaration;
// it neither constructs a second NUM nor gives the earlier RAW a destructor.
func (q *upolyFactorDerivativeScope) retireMultiplier() error {
	o := q.owner
	if o == nil || !o.actual() || q != &o.derivative || !q.executing {
		return ErrUPolynomialOwner
	}
	if q.d.NativeCellToken() == 0 {
		return nil
	}
	if !q.constructed {
		return ErrUPolynomialOwner
	}
	n := uint64(reflect.TypeOf(upolyFactorDerivativeRaw{}).Size())
	z := o.service.manager.config.Integer
	if e := z.Budget().Reserve(n); e != nil {
		return e
	}
	r := &upolyFactorDerivativeRaw{owner: q, invocation: q.invocation, header: n, value: q.d, next: q.retired}
	r.self = r
	q.retired = r
	q.d = unumeral.Int{}
	return nil
}
func (q *upolyFactorDerivativeScope) own() (cause error) {
	if q.owner == nil {
		return nil
	}
	o := q.owner
	if !o.actual() || q != &o.derivative || q.executing {
		return ErrUPolynomialOwner
	}
	defer func() {
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	z := o.service.manager.config.Integer
	if e := z.CleanupRawNumeral(&q.d); e != nil {
		return e
	}
	q.constructed = false
	for q.retired != nil {
		r := q.retired
		if r.self != r || r.owner != q || r.invocation != q.invocation || r.header == 0 {
			return ErrUPolynomialOwner
		}
		if e := z.CleanupRawNumeral(&r.value); e != nil {
			return e
		}
		z.Budget().Release(r.header)
		r.header = 0
		q.retired = r.next
	}
	return nil
}

// Source: src/math/polynomial/upolynomial.cpp:579-607.
func (q *upolyFactorDivisibilityScope) valid() bool {
	return q.owner != nil && q.owner.actual() && q == &q.owner.divisibility
}
func (q *upolyFactorDivisibilityScope) finish(body error) error {
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	z := q.owner.service.manager.config.Integer
	for _, e := range []error{z.DividesNumeralInvocationSourceCause(&q.owner.boolNumeral), z.AddMulInvocationSourceCause(&q.addMul), q.owner.numericSourceCause()} {
		if e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
	}
	if q.constructed {
		if _, e := upolynomialSourceNumeralDel(z, &q.ratio); e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		q.constructed = false
	}
	return q.bodyCause
}
func (q *upolyFactorDivisibilityScope) own(abort bool) (cause error) {
	if q.owner == nil {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	z := q.owner.service.manager.config.Integer
	if e := z.CleanupAddMulInvocation(&q.addMul, false); e != nil {
		return e
	}
	if q.constructed {
		if e := z.CleanupRawNumeral(&q.ratio); e != nil {
			return e
		}
		q.constructed = false
	}
	if abort {
		return z.CleanupAddMulInvocation(&q.addMul, true)
	}
	return nil
}

// Source: src/math/polynomial/upolynomial.h:212-215.
func (q *upolyFactorPrimitiveScope) valid() bool {
	return q.owner != nil && q.owner.actual() && q == &q.owner.primitive
}
func (q *upolyFactorPrimitiveScope) finish(body error) error {
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	z := q.owner.service.manager.config.Integer
	for _, e := range []error{z.RawInvocationSourceCause(&q.owner.gcdRaw), z.RawInvocationSourceCause(&q.owner.divisionRaw), q.owner.numericSourceCause()} {
		if e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
	}
	if q.constructed {
		if _, e := upolynomialSourceNumeralDel(z, &q.cont); e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		q.constructed = false
	}
	return q.bodyCause
}
func (q *upolyFactorPrimitiveScope) own() (cause error) {
	if q.owner == nil {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	if q.constructed {
		if e := q.owner.service.manager.config.Integer.CleanupRawNumeral(&q.cont); e != nil {
			return e
		}
		q.constructed = false
	}
	return nil
}

func (q *upolyFactorFrontendScope) numericSourceCause() error {
	if e := q.perfectSquare.SourceCause(); e != nil {
		return e
	}
	if q.vectorNormalize.sourceCause != nil {
		return q.vectorNormalize.sourceCause
	}
	if q.modularPower.sourceCause != nil {
		return q.modularPower.sourceCause
	}
	if q.sourceCause != nil {
		return q.sourceCause
	}
	z := q.service.manager.config.Integer

	if e := z.DividesNumeralInvocationSourceCause(&q.boolNumeral); e != nil {
		return e
	}
	return z.AddMulInvocationSourceCause(&q.addMul)
}

// Source: src/math/polynomial/upolynomial.cpp:3042-3118.
// The caller's actual output object remains live across reached source calls.
// Other Factors objects sharing this manager do not belong to this invocation.
func (fs *UPolynomialFactors) factorFrontendExecuting() bool {
	if fs == nil || fs.self != fs || fs.manager == nil || fs.manager.self != fs.manager {
		return false
	}
	if s := fs.manager.factorFrontendConstantCaller; s != nil && s.constantCallerExecuting() {
		if s.frontendConstant.executing && s.frontendConstant.result == fs {
			return true
		}
		if q := s.frontendConstantOwn; q != nil && q.result == fs && q.ownExecuting {
			return true
		}
	}
	for q := fs.manager.factorFrontendCaller; q != nil; q = q.previous {
		if q.actual() && q.invocation != 0 && q.result == fs && (q.executing || q.sourceExecuting || q.ownExecuting) {
			return true
		}
	}
	return false
}

// Source: src/math/polynomial/upolynomial.cpp:2960-2965.
func (q *upolyFactorFlipScope) valid() bool {
	return q.owner != nil && q.owner.actual() && q == &q.owner.flip
}
func (q *upolyFactorFlipScope) finish(body error) error {
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	z := q.owner.service.manager.config.Integer
	for _, e := range []error{q.owner.numericSourceCause()} {
		if e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
	}
	if q.constructed {
		if _, e := upolynomialSourceNumeralDel(z, &q.c); e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		q.constructed = false
	}
	return q.bodyCause
}
func (q *upolyFactorFlipScope) own() (cause error) {
	if q.owner == nil {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	if q.constructed {
		if e := q.owner.service.manager.config.Integer.CleanupRawNumeral(&q.c); e != nil {
			return e
		}
		q.constructed = false
	}
	return nil
}

// Source: src/math/polynomial/upolynomial.cpp:2967-3011.
func (q *upolyFactorQuadraticScope) valid() bool {
	return q.owner != nil && q.owner.actual() && q == &q.owner.quadratic
}
func (q *upolyFactorQuadraticScope) vector(n uint8) *upolyVectorState {
	switch n {
	case 5:
		return &q.f1
	case 6:
		return &q.f2
	case 7:
		return &q.product
	}
	return nil
}
func (q *upolyFactorQuadraticScope) finishProduct(body error) error {
	if !q.valid() || !q.executing {
		return ErrUPolynomialOwner
	}
	if q.sourceCause != nil {
		return errors.Join(body, q.sourceCause)
	}
	if e := q.owner.numericSourceCause(); e != nil {
		q.sourceCause = e
		return errors.Join(body, e)
	}
	if e := upolynomialScopedVectorSourceDel(q.owner.service.manager, &q.product, &q.sourceIndex); e != nil {
		q.sourceCause = e
		return errors.Join(body, e)
	}
	q.sourceIndex = 0
	q.constructed = 6
	return body
}
func (q *upolyFactorQuadraticScope) finish(body error) error {
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	if q.sourceCause == nil {
		q.sourceCause = q.owner.scalarTraceBlocker()
	}
	if q.sourceCause == nil {
		q.sourceCause = q.owner.coreTrace.sourceBlocker()
	}
	if q.sourceCause != nil {
		return errors.Join(q.bodyCause, q.sourceCause)
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	if e := q.owner.numericSourceCause(); e != nil {
		q.sourceCause = e
		return errors.Join(q.bodyCause, e)
	}
	m := q.owner.service.manager
	for q.constructed > 0 {
		var e error
		if q.constructed <= 4 {
			_, e = upolynomialSourceNumeralDel(m.config.Integer, &q.values[q.constructed-1])
		} else {
			e = upolynomialScopedVectorSourceDel(m, q.vector(q.constructed), &q.sourceIndex)
		}
		if e != nil {
			q.sourceCause = e
			return errors.Join(q.bodyCause, e)
		}
		q.constructed--
		q.sourceIndex = 0
	}
	return q.bodyCause
}
func (q *upolyFactorQuadraticScope) own() (cause error) {
	if q.owner == nil {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	m := q.owner.service.manager
	for q.constructed > 0 {
		var e error
		if q.constructed <= 4 {
			e = m.config.Integer.CleanupRawNumeral(&q.values[q.constructed-1])
		} else {
			v := q.vector(q.constructed)
			e = m.reset(v)
			if e == nil {
				m.freeBacking(v)
			}
		}
		if e != nil {
			return e
		}
		q.constructed--
	}
	return nil
}

// Source: src/util/mpzzp.h:242-254.
func (q *upolyFactorModularPowerScope) valid() bool {
	return q.owner != nil && q.owner.actual() && q == &q.owner.modularPower
}
func (q *upolyFactorModularPowerScope) finishEnd() error {
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if !q.executing || !q.endEntered || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if q.sourceCause != nil {
		return q.sourceCause
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()

	if _, e := upolynomialSourceNumeralDel(q.owner.service.manager.config.Integer, &q.power); e != nil {
		q.sourceCause = e
		return e
	}
	q.active = false
	return nil
}
func (q *upolyFactorModularPowerScope) own(abort bool) (cause error) {
	if q.owner == nil {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if !q.active || (!abort && !q.endEntered) {
		return nil
	}
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	if e := q.owner.service.manager.config.Integer.CleanupRawNumeral(&q.power); e != nil {
		return e
	}
	q.active = false
	return nil
}

// Source: src/math/polynomial/upolynomial.cpp:366-401.
func (q *upolyFactorVectorNormalizeScope) valid() bool {
	return q.owner != nil && q.owner.actual() && q == &q.owner.vectorNormalize
}
func (q *upolyFactorVectorNormalizeScope) value(n uint8) *unumeral.Int {
	switch n {
	case 1:
		return &q.g
	case 2:
		return &q.old
	case 3:
		return &q.tmp
	}
	return nil
}
func (q *upolyFactorVectorNormalizeScope) destroySource(until uint8, body error) error {
	if q.sourceCause != nil {
		return errors.Join(body, q.sourceCause)
	}
	q.sourceExecuting = true
	defer func() { q.sourceExecuting = false }()
	z := q.owner.service.manager.config.Integer
	for _, e := range []error{q.owner.scalarTraceBlocker(), q.owner.numericSourceCause(), z.RawInvocationSourceCause(&q.owner.divisionRaw), z.RawInvocationSourceCause(&q.owner.gcdRaw)} {
		if e != nil {
			q.sourceCause = e
			return errors.Join(body, e)
		}
	}
	for q.constructed > until {
		if _, e := upolynomialSourceNumeralDel(z, q.value(q.constructed)); e != nil {
			q.sourceCause = e
			return errors.Join(body, e)
		}
		q.constructed--
	}
	return body
}
func (q *upolyFactorVectorNormalizeScope) finishCoefficient(body error) error {
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if !q.executing || q.coefficientExecuting || q.sourceExecuting || q.ownExecuting || q.constructed < 2 {
		return ErrUPolynomialCleanupPending
	}
	return q.destroySource(1, body)
}
func (q *upolyFactorVectorNormalizeScope) finish(body error) error {
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.coefficientExecuting || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	if q.bodyCause == nil {
		q.bodyCause = body
	}
	return q.destroySource(0, q.bodyCause)
}
func (q *upolyFactorVectorNormalizeScope) own() (cause error) {
	if q.owner == nil {
		return nil
	}
	if !q.valid() {
		return ErrUPolynomialOwner
	}
	if q.executing || q.coefficientExecuting || q.sourceExecuting || q.ownExecuting {
		return ErrUPolynomialCleanupPending
	}
	q.ownExecuting = true
	defer func() {
		q.ownExecuting = false
		if cause != nil && q.ownCause == nil {
			q.ownCause = cause
		}
	}()
	for q.constructed > 0 {
		if e := q.owner.service.manager.config.Integer.CleanupRawNumeral(q.value(q.constructed)); e != nil {
			return e
		}
		q.constructed--
	}
	return nil
}

package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:979-1299.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

var ErrUPolynomialCleanupPending = unumeral.ErrNumericCleanupPending

// Actual cold ownership for factor_square_free's lexical objects. The source
// cannot safely continue a destructor through an invalid numeral. Go retains
// the unfinished reverse destruction cursor instead of losing its owner. This
// additional charged header is a Go exception/lifetime adapter, not C++ malloc.
type upolyFactorSquareFreeScope struct {
	traceBorrow                                       upolyFactorTraceBorrow
	tracePolynomial                                   upolyDisplayInvocationReceipt
	trace                                             upolyFactorsDisplayReceipt
	issuedReceipt                                     upolyFactorSquareFreeInvocationReceipt
	historyNext                                       *upolyFactorSquareFreeScope
	historyRequired                                   bool
	invocation                                        uint64
	executing, sourceExecuting, ownExecuting, retired bool
	bodyCause, sourceCause, ownCause                  error
	primePreparation                                  upolyFactorPrimePreparationScope
	iterator                                          primegen.Iterator
	hensel                                            upolyHenselCallerScope
	trialHelper                                       upolyRecombinationTrialScope
	recombinationStep                                 upolyRecombinationStepScope
	self                                              *upolyFactorSquareFreeScope
	service                                           *UPolynomialFactorizationService
	callerPrevious                                    *upolyFactorSquareFreeScope
	callDepth                                         uint32
	mignotte                                          upolyMignotteScope
	primeTrial                                        upolyPrimeTrialScope
	manager                                           *UPolynomialManager
	fpp                                               upolyVectorState
	p                                                 unumeral.Int
	zp                                                *UPolynomialManager
	best                                              *UPolynomialFactors
	bestP                                             unumeral.Int
	ds                                                upolyFactorDegreeState
	gcdTmp                                            unumeral.Int
	zpe                                               *UPolynomialManager
	lifted                                            *UPolynomialFactors
	leading                                           unumeral.Int
	it                                                upolyFactorCombinationState
	trial, quotient                                   upolyVectorState
	trialContent                                      unumeral.Int
	constructed                                       uint8
	header                                            uint64
}

func (s *UPolynomialFactorizationService) newSquareFreeScope() (*upolyFactorSquareFreeScope, error) {
	if s.scope != nil || s.squareFreeInvocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	n := uint64(reflect.TypeOf(upolyFactorSquareFreeScope{}).Size())
	if e := s.manager.config.Integer.Budget().Reserve(n); e != nil {
		return nil, e
	}
	s.squareFreeInvocation++
	c := &upolyFactorSquareFreeScope{manager: s.manager, service: s, header: n, invocation: s.squareFreeInvocation}
	c.self = c
	s.scope = c
	return c, nil
}

// Reverse lexical source order: content, quotient, trial, combination iterator,
// leading coefficient, lifted factors, lifted core, gcd, degree set, saved
// prime, prime factors, prime core, prime, primitive vector. Only constructed
// objects participate; publication/cleanup advances one completed action.
func (c *upolyFactorSquareFreeScope) cleanup() error {
	if e := c.traceSourceBlocker(); e != nil {
		return e
	}
	if q := &c.primePreparation; q.active {
		if q.executing || q.sourceExecuting || q.ownExecuting {
			return ErrUPolynomialCleanupPending
		}
		if e := q.finish(&q.receipt, q.bodyCause); e != nil && q.sourceCause != nil {
			return e
		}
	}
	if q := &c.service.preparation; q.active && q.parent == c {
		if e := q.finish(&q.receipt, q.bodyCause); e != nil && q.sourceCause != nil {
			return e
		}
	}
	if q := &c.recombinationStep; q.active {
		if q.executing || q.sourceExecuting || q.ownExecuting || q.callDepth != 0 {
			return ErrUPolynomialCleanupPending
		}
		q.finish(&q.receipt, q.bodyCause)
		if q.sourceCause != nil {
			return q.sourceCause
		}
	}
	if q := &c.trialHelper; q.active {
		if q.executing || q.sourceExecuting || q.ownExecuting || q.callDepth != 0 {
			return ErrUPolynomialCleanupPending
		}
		q.finish(&q.receipt, q.bodyCause)
		if q.sourceCause != nil {
			return q.sourceCause
		}
	}
	if c.hensel.active {
		if e := c.hensel.finish(); e != nil {
			return e
		}
	}
	if q := &c.primeTrial; q.active {
		if q.executing || q.callDepth != 0 {
			return ErrUPolynomialCleanupPending
		}
		q.service.finishPrimeTrialSource(&q.receipt, q.bodyCause)
		if q.sourceCause != nil {
			return q.sourceCause
		}
	}
	if q := &c.mignotte; q.active && q.service != nil {
		if q.executing || q.callDepth != 0 {
			return ErrUPolynomialCleanupPending
		}
		q.service.finishMignotteSource(&q.receipt, q.bodyCause)
		if q.sourceCause != nil {
			return q.sourceCause
		}
	}
	if e := c.cleanupMembersPhase(true); e != nil {
		return e
	}
	if c.hensel.active || c.primePreparation.active || c.mignotte.active || c.primeTrial.active || c.trialHelper.active || c.recombinationStep.active || c.service.preparation.active {
		return ErrUPolynomialCleanupPending
	}
	return nil
}
func (c *upolyFactorSquareFreeScope) cleanupMembers() error { return c.cleanupMembersPhase(false) }
func (c *upolyFactorSquareFreeScope) cleanupMembersPhase(source bool) error {
	m := c.manager
	for c.constructed != 0 {
		var e error
		switch c.constructed {
		case 14:
			e = m.modular.Del(&c.trialContent)
		case 13:
			e = m.reset(&c.quotient)
			if e == nil {
				m.freeBacking(&c.quotient)
			}
		case 12:
			e = m.reset(&c.trial)
			if e == nil {
				m.freeBacking(&c.trial)
			}
		case 11:
			c.it.close()
			c.it = upolyFactorCombinationState{}
		case 10:
			e = m.modular.Del(&c.leading)
		case 9:
			if c.primePreparation.matches() {
				e = c.closePreparedFactors(c.lifted, 1, source)
			} else {
				e = c.lifted.Close()
			}
			if e == nil {
				c.lifted = nil
			}
		case 8:
			if c.primePreparation.matches() {
				e = c.closePrimeRelated(source)
			} else {
				e = c.zpe.Close()
			}
			if e == nil {
				c.zpe = nil
			}
		case 7:
			if source {
				e = c.sourcePrimeNumeral(&c.gcdTmp)
			} else {
				e = m.modular.Del(&c.gcdTmp)
			}
		case 6:
			c.ds.close()
			c.ds = upolyFactorDegreeState{}
		case 5:
			if source {
				e = c.sourcePrimeNumeral(&c.bestP)
			} else {
				e = m.modular.Del(&c.bestP)
			}
		case 4:
			if c.primePreparation.matches() {
				e = c.closePreparedFactors(c.best, 0, source)
			} else {
				e = c.best.Close()
			}
			if e == nil {
				c.best = nil
			}
		case 3:
			if c.primePreparation.matches() {
				e = c.closePrimeRelated(source)
			} else {
				e = c.zp.Close()
			}
			if e == nil {
				c.zp = nil
			}
		case 2:
			if source {
				e = c.sourcePrimeNumeral(&c.p)
			} else {
				e = m.modular.Del(&c.p)
			}
		case 1:
			e = m.reset(&c.fpp)
			if e == nil {
				m.freeBacking(&c.fpp)
			}
		}
		if e != nil {
			return e
		}
		c.constructed--
	}
	return nil
}

// CleanupRetained retries actual unfinished source destruction. It never
// changes modular mode, reconstructs a numeral or rolls back caller outputs.
func (s *UPolynomialFactorizationService) CleanupRetained() error {
	if s == nil || s.self != s || s.manager == nil {
		return ErrUPolynomialOwner
	}
	if s.entryTraceBorrow.executing {
		return ErrUPolynomialCleanupPending
	}
	if s.constantCallerExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if q := &s.frontend; q.active && (q.executing || q.sourceExecuting || q.ownExecuting) {
		return ErrUPolynomialCleanupPending
	}
	if s.closed {
		return arena.ErrClosed
	}
	if c := s.scope; c != nil && (c.executing || c.sourceExecuting || c.ownExecuting || c.callDepth != 0) {
		return ErrUPolynomialCleanupPending
	}
	if e := s.manager.cleanupDisplayReceipt(&s.entryTrace); e != nil {
		return e
	}
	if c := s.scope; c != nil {
		if e := c.cleanupTrace(); e != nil {
			return e
		}
	}
	if q := &s.frontend; q.active {
		if e := q.own(false); e != nil {
			return e
		}
		if e := q.own(true); e != nil {
			return e
		}
	}
	if e := s.cleanupFrontendConstantCalls(); e != nil {
		return e
	}
	for _, q := range []*upolyFactorPreparationScope{&s.diagnostic, &s.preparation} {
		if q.active {
			if e := q.own(&q.receipt, false); e != nil {
				return e
			}
		}
	}
	if s.scope != nil {
		c := &s.scope.hensel
		if c.active && (c.executing || c.sourceExecuting || c.ownExecuting || c.callDepth != 0) {
			return ErrUPolynomialCleanupPending
		}
	}
	if s.scope == nil || !s.scope.hensel.active {
		if e := s.cleanupHenselPairs(); e != nil {
			return e
		}
	}
	// Empty-service Close still refunds its own header after a borrowed owner
	// closes; no manager/prime access is needed without a retained scope.
	if s.scope == nil {
		for _, q := range []*upolyFactorPreparationScope{&s.diagnostic, &s.preparation} {
			if q.active {
				if e := q.own(&q.receipt, true); e != nil {
					return e
				}
			}
		}
		return nil
	}
	if e := s.checkOwner(); e != nil {
		return e
	}
	if s.scope.callDepth != 0 {
		return ErrUPolynomialCleanupPending
	}
	caller := s.scope
	if e := s.cleanupSquareFreeMembers(caller, false); e != nil {
		return e
	}
	if e := s.cleanupSquareFreeMembers(caller, true); e != nil {
		return e
	}
	for _, q := range []*upolyFactorPreparationScope{&s.diagnostic, &s.preparation} {
		if q.active {
			if e := q.own(&q.receipt, true); e != nil {
				return e
			}
		}
	}
	s.releaseSquareFreeScope(caller)
	return nil
}

// RetainedCleanupCore borrows a still-owned lexical core. It lets the caller
// explicitly select set_z after auditing a guarded invalid modular bound.
// The borrow expires when CleanupRetained consumes that core.
func (s *UPolynomialFactorizationService) RetainedCleanupCore(index uint32) (*UPolynomialManager, error) {
	if e := s.checkOwner(); e != nil {
		return nil, e
	}
	if s.scope != nil {
		for _, m := range [3]*UPolynomialManager{s.scope.zpe, s.scope.zp, s.scope.hensel.related} {
			if m != nil {
				if index == 0 {
					return m, nil
				}
				index--
			}
		}
	}
	for h := s.henselPairs; h != nil; h = h.next {
		if h.executing || h.sourceExecuting || h.callDepth != 0 {
			return nil, ErrUPolynomialCleanupPending
		}
		for _, m := range []*UPolynomialManager{h.check.bCore, h.check.brCore, h.rm} {
			if m != nil && !m.closed {
				if index == 0 {
					return m, nil
				}
				index--
			}
		}
	}
	return nil, arena.ErrBounds
}
func (s *UPolynomialFactorizationService) RetainedCleanupFactors(index uint32) (*UPolynomialFactors, error) {
	if e := s.checkOwner(); e != nil {
		return nil, e
	}
	if s.scope != nil {
		for _, f := range [3]*UPolynomialFactors{s.scope.primeTrial.current, s.scope.lifted, s.scope.best} {
			if f != nil {
				if f == s.scope.primeTrial.current && (s.scope.primeTrial.executing || s.scope.primeTrial.sourceExecuting || s.scope.primeTrial.callDepth != 0) {
					return nil, ErrUPolynomialCleanupPending
				}
				if index == 0 {
					return f, nil
				}
				index--
			}
		}
	}
	return nil, arena.ErrBounds
}

// The automatic enclosing epilogue is source-only. Explicit CleanupRetained
// separately acknowledges acquired unjoined child roots after older locals.
func (s *UPolynomialFactorizationService) finishSquareFreeScopeSource() error {
	if e := s.checkOwner(); e != nil {
		return e
	}
	if s.manager.primePreparationCallPending() {
		return ErrUPolynomialCleanupPending
	}
	c := s.scope
	if c == nil {
		return nil
	}
	if c.executing || c.sourceExecuting || c.ownExecuting || c.callDepth != 0 {
		return ErrUPolynomialCleanupPending
	}
	c.enterCall()
	defer c.leaveCall()
	if c.sourceCause != nil {
		return c.sourceCause
	}
	c.sourceExecuting = true
	e := c.cleanup()
	c.sourceExecuting = false
	if e != nil {
		if e != ErrUPolynomialCleanupPending && c.sourceCause == nil {
			c.sourceCause = c.squareFreeChildSourceCause()
			if c.sourceCause == nil {
				c.sourceCause = e
			}
		}
		return e
	}
	s.releaseSquareFreeScope(c)
	return nil
}

// Actual enclosing destructor calls carry their own concrete caller identity;
// an idle retained scope is never treated as an executing caller.
func (c *upolyFactorSquareFreeScope) enterCall() {
	if c.callDepth == 0 {
		c.callerPrevious = c.manager.squareFreeCaller
		c.manager.squareFreeCaller = c
	}
	c.callDepth++
}
func (c *upolyFactorSquareFreeScope) leaveCall() {
	c.callDepth--
	if c.callDepth == 0 {
		c.manager.squareFreeCaller = c.callerPrevious
		c.callerPrevious = nil
	}
}

func (s *UPolynomialFactorizationService) finishSquareFreeScopeError(body error) error {
	return errors.Join(body, s.finishSquareFreeScopeSource())
}

func (s *UPolynomialFactorizationService) cleanupSquareFreeMembers(caller *upolyFactorSquareFreeScope, abort bool) error {
	if e := caller.cleanupTrace(); e != nil {
		return e
	}
	if !abort {
		if q := &caller.primePreparation; q.active {
			if e := q.own(&q.receipt, false); e != nil {
				return e
			}
		}
		if caller.recombinationStep.active {
			if e := caller.recombinationStep.own(&caller.recombinationStep.receipt, false); e != nil {
				return e
			}
		}
		if caller.trialHelper.active {
			if e := caller.trialHelper.own(&caller.trialHelper.receipt, false); e != nil {
				return e
			}
		}
		caller.enterCall()
		defer caller.leaveCall()
		if caller.hensel.active {
			if e := caller.hensel.own(false); e != nil {
				return e
			}
		}
		prime := &caller.primeTrial
		if prime.active {
			if e := s.cleanupPrimeTrialInvocation(&prime.receipt, false); e != nil {
				return e
			}
		}
		q := &caller.mignotte
		if q.executing || q.callDepth != 0 {
			return ErrUPolynomialCleanupPending
		}
		if q.active {
			if e := s.cleanupMignotteInvocation(&q.receipt, false); e != nil {
				return e
			}
		}
		if e := caller.cleanupMembers(); e != nil {
			return e
		}
		return nil
	}
	caller.enterCall()
	defer caller.leaveCall()
	q := &caller.mignotte
	prime := &caller.primeTrial

	if q.active {
		if e := s.cleanupMignotteInvocation(&q.receipt, true); e != nil {
			return e
		}
	}
	if prime.active {
		if e := s.cleanupPrimeTrialInvocation(&prime.receipt, true); e != nil {
			return e
		}
	}
	if caller.hensel.active {
		if e := caller.hensel.own(true); e != nil {
			return e
		}
	}
	if caller.recombinationStep.active {
		if e := caller.recombinationStep.own(&caller.recombinationStep.receipt, true); e != nil {
			return e
		}
	}
	if caller.trialHelper.active {
		if e := caller.trialHelper.own(&caller.trialHelper.receipt, true); e != nil {
			return e
		}
	}
	for _, q := range []*upolyFactorPreparationScope{&s.preparation} {
		if q.active && q.parent == caller {
			if e := q.own(&q.receipt, true); e != nil {
				return e
			}
		}
	}
	if q := &caller.primePreparation; q.active {
		if e := q.own(&q.receipt, true); e != nil {
			return e
		}
	}

	return nil
}

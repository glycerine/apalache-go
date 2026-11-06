package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1030-1050,1153-1175,1238-1292;
// src/math/polynomial/upolynomial_factorization.h:85-89.
// Actual explicit Go service binding and reflected enclosing cold storage.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

type UPolynomialFactorParams struct{ MaxPrime, PrimeTrials, MaxSearchSize uint32 }

func DefaultUPolynomialFactorParams() UPolynomialFactorParams {
	return UPolynomialFactorParams{MaxPrime: 4294967295, PrimeTrials: 1, MaxSearchSize: 0xffffffff}
}

type UPolynomialFactorizationConfig struct {
	Primes                *primegen.Generator
	FactorizationDebugTag bool
}
type UPolynomialFactorizationService struct {
	entryTrace                  upolyDisplayInvocationReceipt
	entryTraceBorrow            upolyFactorTraceBorrow
	frontend                    upolyFactorFrontendScope
	frontendHistory             *upolyFactorFrontendHistory
	frontendConstant            upolyFactorConstantBinding
	frontendConstantCalls       *upolyFactorConstantCall
	frontendConstantOwn         *upolyFactorConstantCall
	squareFreeHistory           *upolyFactorSquareFreeScope
	squareFreeDiagnosticHistory *upolySquareFreeDiagnosticHistory
	squareFreeInvocation        uint64
	diagnostic, preparation     upolyFactorPreparationScope
	self                        *UPolynomialFactorizationService
	manager                     *UPolynomialManager
	config                      UPolynomialFactorizationConfig
	scope                       *upolyFactorSquareFreeScope
	henselPairs                 *upolyHenselPairScope
	header                      uint64
	closed                      bool
}
type UPolynomialFactorizationStats struct {
	OwnerBytes, RetainedOwnerBytes uint64
	CachedOwnerBytes               uint64
	CachedScopes                   uint32
	RetainedScopes                 uint32
	MignottePending                bool
	PrimeTrialPending              bool
	RecombinationPending           bool
	HenselPending                  bool
}

func NewUPolynomialFactorizationService(m *UPolynomialManager, c UPolynomialFactorizationConfig) (*UPolynomialFactorizationService, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	if c.Primes == nil {
		var e error
		c.Primes, e = primegen.BorrowProcessGenerator()
		if e != nil {
			return nil, e
		}
	}
	if e := c.Primes.Check(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(UPolynomialFactorizationService{}).Size())
	if e := m.config.Integer.Budget().Reserve(n); e != nil {
		return nil, e
	}
	s := &UPolynomialFactorizationService{manager: m, config: c, header: n}
	s.self = s
	return s, nil
}
func (s *UPolynomialFactorizationService) checkOwner() error {
	if s == nil || s.self != s || s.manager == nil {
		return ErrUPolynomialOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	if e := s.manager.check(); e != nil {
		return e
	}
	return s.config.Primes.Check()
}
func (s *UPolynomialFactorizationService) check() error {
	if e := s.checkOwner(); e != nil {
		return e
	}
	if s.scope != nil || s.entryTraceBorrow.executing || s.manager.displayReceiptPending(&s.entryTrace) {
		return ErrUPolynomialCleanupPending
	}
	return nil
}

func (s *UPolynomialFactorizationService) Close() error {
	if s == nil || s.self != s || s.manager == nil {
		return ErrUPolynomialOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	if e := s.CleanupRetained(); e != nil {
		return e
	}
	for h := s.henselPairs; h != nil; h = h.next {
		s.manager.config.Integer.Budget().Release(h.header)
	}
	s.henselPairs = nil
	for c := s.squareFreeHistory; c != nil; c = c.historyNext {
		s.manager.config.Integer.Budget().Release(c.header)
		c.header = 0
	}
	s.squareFreeHistory = nil
	for h := s.squareFreeDiagnosticHistory; h != nil; h = h.next {
		s.manager.config.Integer.Budget().Release(h.header)
		h.header = 0
	}
	s.squareFreeDiagnosticHistory = nil
	for h := s.frontendHistory; h != nil; h = h.next {
		s.manager.config.Integer.Budget().Release(h.header)
		h.header = 0
	}
	s.frontendHistory = nil
	for q := s.frontendConstantCalls; q != nil; q = q.next {
		s.manager.config.Integer.Budget().Release(q.header)
		q.header = 0
	}
	s.frontendConstantCalls = nil
	s.manager.config.Integer.Budget().Release(s.header)
	s.closed = true
	return nil
}
func (s *UPolynomialFactorizationService) Stats() UPolynomialFactorizationStats {
	if s == nil || s.self != s || s.closed {
		return UPolynomialFactorizationStats{}
	}
	r := UPolynomialFactorizationStats{OwnerBytes: s.header}
	for q := s.frontendConstantCalls; q != nil; q = q.next {
		r.CachedOwnerBytes += q.header
		r.CachedScopes++
	}
	for t := s.frontend.derivative.retired; t != nil; t = t.next {
		r.CachedOwnerBytes += t.header
		r.CachedScopes++
	}
	for h := s.frontendHistory; h != nil; h = h.next {
		r.CachedOwnerBytes += h.header
		r.CachedScopes++
	}
	for c := s.squareFreeHistory; c != nil; c = c.historyNext {
		r.CachedOwnerBytes += c.header
		r.CachedScopes++
	}
	for h := s.squareFreeDiagnosticHistory; h != nil; h = h.next {
		r.CachedOwnerBytes += h.header
		r.CachedScopes++
	}
	if s.scope != nil {
		r.RetainedScopes = 1
		r.RetainedOwnerBytes = s.scope.header
		r.MignottePending = s.scope.mignotte.active
		r.PrimeTrialPending = s.scope.primeTrial.active
		r.RecombinationPending = s.scope.recombinationStep.active
		r.HenselPending = s.scope.hensel.active
	}
	for h := s.henselPairs; h != nil; h = h.next {
		r.CachedOwnerBytes += h.header
		r.CachedScopes++
		if h.active {
			r.RetainedOwnerBytes += h.header
			r.RetainedScopes++
			r.HenselPending = true
		}
	}
	return r
}
func (s *UPolynomialFactorizationService) PrimeSquareFreeFactors(m *UPolynomialManager, f UPolynomialView, out *UPolynomialFactors) error {
	if e := s.check(); e != nil {
		return e
	}
	return m.SquareFreeFactorZP(f, out)
}
func (s *UPolynomialFactorizationService) FactorPrime(m *UPolynomialManager, f UPolynomialView, out *UPolynomialFactors) (bool, error) {
	if e := s.check(); e != nil {
		return false, e
	}
	return m.FactorZP(f, out)
}
func (s *UPolynomialFactorizationService) FactorPrimeSquareFree(m *UPolynomialManager, f UPolynomialView, out *UPolynomialFactors) (bool, error) {
	if e := s.check(); e != nil {
		return false, e
	}
	return m.FactorZPSquareFree(f, out)
}
func (s *UPolynomialFactorizationService) BerlekampPrimeSquareFree(m *UPolynomialManager, f UPolynomialView, out *UPolynomialFactors, randomized bool) (bool, error) {
	if e := s.check(); e != nil {
		return false, e
	}
	return m.FactorZPSquareFreeBerlekamp(f, out, randomized)
}

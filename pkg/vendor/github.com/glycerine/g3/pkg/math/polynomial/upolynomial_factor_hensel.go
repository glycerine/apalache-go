package polynomial

// Source: src/math/polynomial/upolynomial_factorization_int.h:32-52;
// src/math/polynomial/upolynomial_factorization.cpp:583-977.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Both literal to_zp_manager overloads, with reads at each source dereference.

func (m *UPolynomialManager) toZPInPlace(p *upolyVectorState) error {
	return m.toZPInPlaceWithHensel(p, nil)
}
func (m *UPolynomialManager) toZPInPlaceWithHensel(p *upolyVectorState, receipt *upolyHenselPairInvocationReceipt) error {
	context := numericIntegerModularManager_henselPairAllocationContext(m.modular, receipt)
	if receipt != nil && context.henselPairReceipt(m.config.Integer) == nil {
		return ErrUPolynomialOwner
	}
	return m.toZPInPlaceWithSourceContext(p, context)
}
func (m *UPolynomialManager) toZPInPlaceWithSourceContext(p *upolyVectorState, context integerAllocationInvocationContext) error {
	for i := uint32(0); i < p.size; i++ {
		a, e := m.read(m.stateView(p), i)
		if e != nil {
			return e
		}
		if e = numericIntegerModularManager_normalizeWithSourceContext(m.modular, a, context); e != nil {
			return e
		}
	}
	return m.trim(p)
}
func (m *UPolynomialManager) toZPCopy(p UPolynomialView, out *upolyVectorState) error {
	return m.toZPCopyEngine(p, out, nil, nil)
}
func (m *UPolynomialManager) toZPCopyEngine(p UPolynomialView, out *upolyVectorState, r *upolyPrimeTrialInvocationReceipt, h *upolyHenselPairInvocationReceipt) error {
	return m.toZPCopyWithOwners(p, out, r, h, nil)
}
func (m *UPolynomialManager) toZPCopyWithOwners(p UPolynomialView, out *upolyVectorState, r *upolyPrimeTrialInvocationReceipt, h *upolyHenselPairInvocationReceipt, caller *upolyHenselCallerScope) error {
	if e := m.reset(out); e != nil {
		return e
	}
	for i := uint32(0); i < p.count; i++ {
		var local unumeral.Int
		value := &local
		if r != nil {
			r.frame.transport = unumeral.Int{}
			r.frame.transportBound = true
			value = &r.frame.transport
		}
		if h != nil {
			h.frame.check.transport = unumeral.Int{}
			h.frame.check.transportBound = true
			value = &h.frame.check.transport
		}
		if caller != nil {
			caller.transport = unumeral.Int{}
			caller.transportBound = true
			value = &caller.transport
		}
		a, e := m.read(p, i)
		if e != nil {
			return e
		}
		context := integerAllocationInvocationContext{}
		if r != nil {
			context = numericIntegerModularManager_primeTrialAllocationContext(m.modular, r)
			if context.primeTrialReceipt(m.config.Integer) == nil {
				return ErrUPolynomialOwner
			}
		}
		if h != nil {
			context = numericIntegerModularManager_henselPairAllocationContext(m.modular, h)
			if context.henselPairReceipt(m.config.Integer) == nil {
				return ErrUPolynomialOwner
			}
		}
		if caller != nil {
			context = numericIntegerModularManager_henselCallerAllocationContext(m.modular, caller)
			if context.henselCallerFrame(m.config.Integer) == nil {
				return ErrUPolynomialOwner
			}
		}
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(value, a); e != nil {
			if r == nil && h == nil && caller == nil {
				m.modular.Del(value)
			}
			return e
		}
		target, e := m.pushDefault(out)
		if e != nil {
			if r == nil && h == nil && caller == nil {
				m.modular.Del(value)
			}
			return e
		}
		*target = *value // literal nontrivial mpz move, no coefficient copy or replay
		if caller != nil {
			caller.transport = unumeral.Int{}
			caller.transportBound = false
		}
		if r != nil {
			r.frame.transport = unumeral.Int{}
			r.frame.transportBound = false
		}
		if h != nil {
			h.frame.check.transport = unumeral.Int{}
			h.frame.check.transportBound = false
		}
	}
	return m.trim(out)
}
func (s *UPolynomialFactorizationService) HenselLiftPair(a, b, r *unumeral.Int, u, A, v, B, C UPolynomialView, outA, outB UPolynomial) error {
	if e := s.check(); e != nil {
		return e
	}
	m := s.manager
	da, e := m.descriptor(outA)
	if e != nil {
		return e
	}
	db, e := m.descriptor(outB)
	if e != nil {
		return e
	}
	return s.henselPair(a, b, r, u, A, v, B, C, &da.state, &db.state)
}
func (s *UPolynomialFactorizationService) henselPair(a, b, r *unumeral.Int, u, A, v, B, C UPolynomialView, outA, outB *upolyVectorState) (failure error) {
	return s.henselPairWithCaller(a, b, r, u, A, v, B, C, outA, outB, nil)
}
func (s *UPolynomialFactorizationService) henselPairWithCaller(a, b, r *unumeral.Int, u, A, v, B, C UPolynomialView, outA, outB *upolyVectorState, caller *upolyHenselCallerScope) (failure error) {
	m := s.manager
	receipt, e := s.beginHenselPairWithCaller(caller)
	if e != nil {
		return e
	}
	h := receipt.frame
	h.bindInputs([5]UPolynomialView{u, A, v, B, C}, outA, outB)
	h.enterCall()
	defer func() {
		h.executing = false
		h.leaveCall()
		failure = s.finishHenselPairSource(&receipt, failure)
		if failure == nil {
			failure = s.cleanupHenselPairInvocation(&receipt, false)
			if failure == nil {
				failure = s.cleanupHenselPairInvocation(&receipt, true)
			}
		}
	}()
	if e = s.traceHenselPairEntry(h, a, b, r); e != nil {
		return e
	}
	rm, e := NewRelatedUPolynomialManager(m)
	if e != nil {
		return e
	}
	h.rm = rm
	h.attachRelated()
	h.constructed = 1
	if e = rm.modular.WithAllocationContext((numericIntegerModularManager_henselPairAllocationContext(rm.modular, &receipt)).utility(rm.modular.Integers())).SetZP(r); e != nil {
		return e
	}
	enabled, e := factorHeaderAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && !(upolyViewDegree(h.borrowedInput(4)) == upolyViewDegree(h.borrowedInput(1))+upolyViewDegree(h.borrowedInput(3))) {
		if e := m.factorizationAssertionFailure(604, "upm.degree(C) == upm.degree(A) + upm.degree(B)"); e != nil {
			return e
		}
	}
	enabled, e = factorHeaderAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && !(upolyViewDegree(h.borrowedInput(0)) < upolyViewDegree(h.borrowedInput(3)) && upolyViewDegree(h.borrowedInput(2)) < upolyViewDegree(h.borrowedInput(1))) {
		if e := m.factorizationAssertionFailure(605, "upm.degree(U) < upm.degree(B) && upm.degree(V) < upm.degree(A)"); e != nil {
			return e
		}
	}
	h.constructed = 2
	if e = m.mulWithHensel(h.borrowedInput(1), h.borrowedInput(3), &h.f, &receipt); e != nil {
		return e
	}
	if e = m.addSubStateWithHensel(h.borrowedInput(4), m.stateView(&h.f), &h.f, true, &receipt); e != nil {
		return e
	}
	if e = m.divScalarWithHensel(m.stateView(&h.f), b, &receipt); e != nil {
		return e
	}
	if e = rm.toZPInPlaceWithHensel(&h.f, &receipt); e != nil {
		return e
	}
	if e = s.traceHenselPairStage(h, 614); e != nil {
		return e
	}
	h.constructed = 5
	if e = s.traceHenselPairStage(h, 637); e != nil {
		return e
	}
	if e = rm.mulWithHensel(h.borrowedInput(2), rm.stateView(&h.f), &h.Vf, &receipt); e != nil {
		return e
	}
	if e = s.traceHenselPairStage(h, 641); e != nil {
		return e
	}
	var d uint32
	if e = rm.divRemCoreWithHensel(rm.stateView(&h.Vf), h.borrowedInput(1), &d, &h.t, &h.S, &receipt); e != nil {
		return e
	}
	if e = s.traceHenselPairStage(h, 646); e != nil {
		return e
	}
	h.constructed = 7
	if e = rm.mulWithHensel(h.borrowedInput(0), rm.stateView(&h.f), &h.T, &receipt); e != nil {
		return e
	}
	if e = s.traceHenselPairStage(h, 653); e != nil {
		return e
	}
	if e = rm.mulWithHensel(h.borrowedInput(3), rm.stateView(&h.t), &h.tmp, &receipt); e != nil {
		return e
	}
	if e = s.traceHenselPairStage(h, 658); e != nil {
		return e
	}
	if e = rm.addSubStateWithHensel(rm.stateView(&h.T), rm.stateView(&h.tmp), &h.T, false, &receipt); e != nil {
		return e
	}
	if e = s.traceHenselPairStage(h, 663); e != nil {
		return e
	}
	if e = m.mulScalarStateWithHensel(&h.S, b, &receipt); e != nil {
		return e
	}
	if e = m.mulScalarStateWithHensel(&h.T, b, &receipt); e != nil {
		return e
	}
	if e = m.addSubStateWithHensel(h.borrowedInput(1), m.stateView(&h.S), outA, false, &receipt); e != nil {
		return e
	}
	if e = m.addSubStateWithHensel(h.borrowedInput(3), m.stateView(&h.T), outB, false, &receipt); e != nil {
		return e
	}
	conditional, e := s.henselConditionalAssertionEnabled()
	if e != nil {
		return e
	}
	if conditional {
		ok, e := s.checkHenselPair(h.borrowedInput(4), a, b, r, h.borrowedInput(1), h.borrowedInput(3), m.stateView(outA), m.stateView(outB), &receipt)
		if e != nil {
			return e
		}
		if !ok {
			if e := henselConditionalAssertionFailure(674, "check_hansel_lift(upm, C, a, b, r, A, B, A_lifted, B_lifted)"); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *UPolynomialFactorizationService) henselQuadratic(C UPolynomialView, zpe *UPolynomialManager, A, B *upolyVectorState, e uint32) error {
	return s.henselQuadraticWithCaller(C, zpe, A, B, e, nil)
}
func (s *UPolynomialFactorizationService) henselQuadraticWithCaller(C UPolynomialView, zpe *UPolynomialManager, A, B *upolyVectorState, e uint32, caller *upolyHenselCallerScope) (failure error) {
	m := s.manager
	if caller != nil {
		caller.enterCall()
		defer caller.leaveCall()
	}
	if err := s.traceHenselQuadraticEntry(caller, zpe, m.stateView(A), m.stateView(B), C, e); err != nil {
		return err
	}
	zp, err := NewRelatedUPolynomialManager(m)
	if err != nil {
		return err
	}
	if caller == nil {
		defer zp.Close()
	} else {
		caller.related = zp
		caller.ownRelatedStage = 0
		caller.ownRelatedIndex = 0
		caller.attachRelated()
		defer func() {
			body := failure
			failure = joinHenselCaller(failure, caller.finishQuadratic())
			if body == nil && failure == nil {
				failure = caller.completeQuadraticStorage()
			}
		}()
	}
	p, err := zpe.modular.BorrowedModulus()
	if err != nil {
		return err
	}
	if err = zp.modular.WithAllocationContext((numericIntegerModularManager_henselCallerAllocationContext(zp.modular, caller)).utility(zp.modular.Integers())).SetZP(p); err != nil {
		return err
	}
	var locals [5]upolyVectorState
	U, V, D, Al, Bl := &locals[0], &locals[1], &locals[2], &locals[3], &locals[4]
	if caller != nil {
		U, V, D, Al, Bl = &caller.quadratic[0], &caller.quadratic[1], &caller.quadratic[2], &caller.quadratic[3], &caller.quadratic[4]
		// This is a new reached U/V/D declaration block, after SetZP.
		caller.quadraticSourceStarted = false
		caller.quadraticOwnStarted = false
		caller.quadraticConstructed = 3
	}
	if caller == nil {
		defer m.dropLocalVector(U)
		defer m.dropLocalVector(V)
		defer m.dropLocalVector(D)
	}
	if err = zp.extendedGCDWithCaller(zp.stateView(A), zp.stateView(B), U, V, D, caller); err != nil {
		return err
	}
	enabled, err := factorHeaderAssertionsEnabled()
	if err != nil {
		return err
	}
	if enabled {
		one := false
		if D.size == 1 {
			one, err = zp.modular.IsOne(&zp.values(D)[0])
			if err != nil {
				return err
			}
		}
		if !one {
			if err = m.factorizationAssertionFailure(728, "D.size() == 1 && zp_upm.m().is_one(D[0])"); err != nil {
				return err
			}
		}
	}
	if caller == nil {
		defer m.dropLocalVector(Al)
		defer m.dropLocalVector(Bl)
	} else {
		caller.quadraticConstructed = 5
	}
	for k := uint32(1); k < e; {
		if err = m.checkpoint(); err != nil {
			return err
		}
		pe, err := zpe.modular.BorrowedModulus()
		if err != nil {
			return err
		}
		if err = s.henselPairWithCaller(pe, pe, pe, m.stateView(U), m.stateView(A), m.stateView(V), m.stateView(B), C, Al, Bl, caller); err != nil {
			return err
		}
		if err = s.traceHenselQuadratic(caller, C, pe, 744); err != nil {
			return err
		}
		if err = s.henselQuadraticIterationWithCaller(zpe, pe, U, V, A, B, Al, Bl, caller); err != nil {
			return err
		}
		k *= 2
	}
	return nil
}
func (s *UPolynomialFactorizationService) henselQuadraticIteration(zpe *UPolynomialManager, pe *unumeral.Int, U, V, A, B, Al, Bl *upolyVectorState) error {
	return s.henselQuadraticIterationWithCaller(zpe, pe, U, V, A, B, Al, Bl, nil)
}
func (s *UPolynomialFactorizationService) henselQuadraticIterationWithCaller(zpe *UPolynomialManager, pe *unumeral.Int, U, V, A, B, Al, Bl *upolyVectorState, caller *upolyHenselCallerScope) (failure error) {
	m := s.manager
	var locals [6]upolyVectorState
	tmp1, g, S, T, t, tmp2 := &locals[0], &locals[1], &locals[2], &locals[3], &locals[4], &locals[5]
	if caller != nil {
		tmp1, g, S, T, t, tmp2 = &caller.iteration[0], &caller.iteration[1], &caller.iteration[2], &caller.iteration[3], &caller.iteration[4], &caller.iteration[5]
		caller.iterationConstructed = 2
		caller.iterationSourceStarted = false
		caller.iterationOwnStarted = false
		defer func() { failure = joinHenselCaller(failure, caller.finishIteration()) }()
	}
	if caller == nil {
		defer m.dropLocalVector(tmp1)
		defer m.dropLocalVector(g)
	}
	zero := unumeral.NewInt(0)
	if e := m.scopedPush(g, &zero); e != nil {
		return e
	}
	if e := m.config.Integer.SetInt64(&m.values(g)[g.size-1], 1); e != nil {
		return e
	}
	if e := m.mulWithCaller(m.stateView(Al), m.stateView(U), tmp1, caller); e != nil {
		return e
	}
	if e := m.addSubWithCaller(m.stateView(g), m.stateView(tmp1), g, true, caller); e != nil {
		return e
	}
	if e := m.mulWithCaller(m.stateView(Bl), m.stateView(V), tmp1, caller); e != nil {
		return e
	}
	if e := m.addSubWithCaller(m.stateView(g), m.stateView(tmp1), g, true, caller); e != nil {
		return e
	}
	if e := m.divScalarWithCaller(m.stateView(g), pe, caller); e != nil {
		return e
	}
	if e := zpe.toZPInPlaceWithCaller(g, caller); e != nil {
		return e
	}
	if e := s.traceHenselQuadratic(caller, UPolynomialView{}, pe, 771); e != nil {
		return e
	}
	if caller == nil {
		defer m.dropLocalVector(S)
		defer m.dropLocalVector(T)
		defer m.dropLocalVector(t)
		defer m.dropLocalVector(tmp2)
	} else {
		caller.iterationConstructed = 6
	}
	if e := zpe.mulWithCaller(m.stateView(g), m.stateView(V), tmp1, caller); e != nil {
		return e
	}
	var d uint32
	// A can be newly allocated by the prime core even when input factors
	// borrow a fourth store. Bind its actual origin at this lifted-core read.
	if e := zpe.adoptState(m, A); e != nil {
		return e
	}
	if e := zpe.divRemWithCaller(m.stateView(tmp1), m.stateView(A), &d, t, T, caller); e != nil {
		return e
	}
	if e := zpe.mulWithCaller(m.stateView(g), m.stateView(U), tmp1, caller); e != nil {
		return e
	}
	if e := zpe.adoptState(m, B); e != nil {
		return e
	}
	if e := zpe.mulWithCaller(m.stateView(t), m.stateView(B), tmp2, caller); e != nil {
		return e
	}
	if e := zpe.addSubWithCaller(m.stateView(tmp1), m.stateView(tmp2), S, false, caller); e != nil {
		return e
	}
	if e := m.scalarWithCaller(S, pe, caller); e != nil {
		return e
	}
	if e := m.scalarWithCaller(T, pe, caller); e != nil {
		return e
	}
	if e := m.addSubWithCaller(m.stateView(U), m.stateView(S), U, false, caller); e != nil {
		return e
	}
	if e := m.addSubWithCaller(m.stateView(V), m.stateView(T), V, false, caller); e != nil {
		return e
	}
	if e := zpe.modular.WithAllocationContext((numericIntegerModularManager_henselCallerAllocationContext(zpe.modular, caller)).utility(zpe.modular.Integers())).SetPSquare(); e != nil {
		return e
	}
	for _, vec := range [4]*upolyVectorState{U, V, Al, Bl} {
		if e := zpe.toZPInPlaceWithCaller(vec, caller); e != nil {
			return e
		}
	}
	*A, *Al = *Al, *A
	*B, *Bl = *Bl, *B
	conditional, err := s.henselConditionalAssertionEnabled()
	if err != nil {
		return err
	}
	if conditional {
		ok, e := s.checkQuadraticHenselWithCaller(zpe, m.stateView(U), m.stateView(A), m.stateView(V), m.stateView(B), caller)
		if e != nil {
			return e
		}
		if !ok {
			if e := henselConditionalAssertionFailure(801, "check_quadratic_hensel(zpe_upm, U, A, V, B)"); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *UPolynomialFactorizationService) HenselLiftFactors(f UPolynomialView, pf *UPolynomialFactors, e uint32, lf *UPolynomialFactors) error {
	if err := s.check(); err != nil {
		return err
	}
	if err := pf.check(); err != nil {
		return err
	}
	if err := lf.check(); err != nil {
		return err
	}
	if pf.manager.config.Integer != s.manager.config.Integer || lf.manager.config.Integer != s.manager.config.Integer {
		return ErrUPolynomialOwner
	}
	// Explicit physical provenance adapter for actual independent cores. These
	// cold retain prefixes precede the source numeric body, without copying
	// coefficients or changing either source arithmetic manager's modulus.
	m, prime, lift := s.manager, pf.manager, lf.manager
	for _, binding := range [4]struct {
		receiver *UPolynomialManager
		store    *UPolynomialCoefficientStore
	}{
		{m, prime.store}, {m, lift.store}, {prime, m.store}, {lift, m.store},
	} {
		if err := binding.receiver.BorrowCoefficientStore(binding.store); err != nil {
			return err
		}
	}
	for i := uint32(0); i < pf.count; i++ {
		if err := m.adoptState(prime, &pf.factors[i]); err != nil {
			return err
		}
		if err := lift.adoptState(prime, &pf.factors[i]); err != nil {
			return err
		}
	}
	for i := uint32(0); i < lf.count; i++ {
		if err := m.adoptState(lift, &lf.factors[i]); err != nil {
			return err
		}
	}
	return s.henselFactors(f, pf, e, lf)
}
func (s *UPolynomialFactorizationService) henselFactors(f UPolynomialView, pf *UPolynomialFactors, e uint32, lf *UPolynomialFactors) error {
	// The public facade owns a factual charged Go lifetime adapter, not pf/lf.
	// Internal factor_square_free passes its actual caller directly instead.
	owner, err := s.newSquareFreeScope()
	if err != nil {
		return err
	}
	if err = s.beginHenselCaller(owner); err != nil {
		return err
	}
	err = s.henselFactorsWithCaller(f, pf, e, lf, &owner.hensel)
	if err == nil {
		// Successful source/own completion consumed only this exact adapter's fields.
		// Do not invoke manager-wide cleanup of independent cached pair invocations.
		if s.scope != owner || owner.hensel.active || owner.constructed != 0 {
			return ErrUPolynomialCleanupPending
		}
		s.manager.config.Integer.Budget().Release(owner.header)
		s.scope = nil
	}
	return err
}
func (s *UPolynomialFactorizationService) henselFactorsWithCaller(f UPolynomialView, pf *UPolynomialFactors, e uint32, lf *UPolynomialFactors, caller *upolyHenselCallerScope) (failure error) {
	m := s.manager
	zp := pf.manager
	zpe := lf.manager
	if caller != nil {
		caller.prime = zp
		caller.lift = zpe
		caller.enterCall()
		defer caller.leaveCall()
		defer func() {
			if caller.outerConstructed == 0 && caller.bodyCause == nil {
				caller.bodyCause = failure
			}
			caller.executing = false
		}()
	}
	enabled, err := factorHeaderAssertionsEnabled()
	if err != nil {
		return err
	}
	if enabled && !(pf.totalFactors > 0) {
		if err = m.factorizationAssertionFailure(874, "zp_fs.total_factors() > 0"); err != nil {
			return err
		}
	}
	enabled, err = factorHeaderAssertionsEnabled()
	if err != nil {
		return err
	}
	if enabled {
		one, err := m.config.Integer.IsOne(&pf.constant)
		if err != nil {
			return err
		}
		if !one {
			if err = m.factorizationAssertionFailure(880, "nm.is_one(zp_fs.get_constant())"); err != nil {
				return err
			}
		}
	}
	p, err := zp.modular.BorrowedModulus()
	if err != nil {
		return err
	}
	if err = zpe.modular.WithAllocationContext((numericIntegerModularManager_henselCallerAllocationContext(zpe.modular, caller)).utility(zpe.modular.Integers())).SetZP(p); err != nil {
		return err
	}
	if err = s.traceHenselFactors(caller, f, pf, lf, false); err != nil {
		return err
	}
	var locals [4]upolyVectorState
	A, B, C, parts := &locals[0], &locals[1], &locals[2], &locals[3]
	if caller == nil {
		defer m.dropLocalVector(A)
		defer m.dropLocalVector(B)
		defer m.dropLocalVector(C)
		defer m.dropLocalVector(parts)
	} else {
		A, B, C, parts = &caller.outer[0], &caller.outer[1], &caller.outer[2], &caller.outer[3]
		caller.outerConstructed = 4
		defer func() {
			if caller.bodyCause == nil {
				caller.bodyCause = failure
			}
			caller.executing = false
			failure = joinHenselCaller(failure, caller.finish())
			if failure == nil {
				failure = caller.own(false)
				if failure == nil {
					failure = caller.own(true)
				}
			}
		}()
	}
	if err = m.setViewWithCaller(parts, f, caller); err != nil {
		return err
	}
	// Source converts unsigned count-1 to signed int for loop endpoint.
	end := int32(pf.count - 1)
	for i := int32(0); i < end; i++ {
		enabled, err = factorHeaderAssertionsEnabled()
		if err != nil {
			return err
		}
		if enabled {
			mult, err := pf.multiplicity(uint32(i))
			if err != nil {
				return err
			}
			if mult != 1 {
				if err = m.factorizationAssertionFailure(902, "zp_fs.get_degree(i) == 1"); err != nil {
					return err
				}
			}
		}
		if err = zp.setViewWithCaller(A, zp.stateView(&pf.factors[i]), caller); err != nil {
			return err
		}
		if err = s.traceHenselVectors(caller, 906); err != nil {
			return err
		}
		if i > 0 {
			// parts was grown by the lifted core in the prior source division.
			// Adopt its actual physical origin at this subsequent cross-core read.
			if err = zp.adoptState(m, parts); err != nil {
				return err
			}
			if err = zp.toZPCopyWithOwners(m.stateView(parts), C, nil, nil, caller); err != nil {
				return err
			}
		} else {
			if err = s.henselInitialCWithCaller(f, pf, C, caller); err != nil {
				return err
			}
		}
		if err = s.traceHenselVectors(caller, 921); err != nil {
			return err
		}
		if err = zp.divWithCaller(m.stateView(C), m.stateView(A), B, caller); err != nil {
			return err
		}
		if err = s.traceHenselVectors(caller, 927); err != nil {
			return err
		}
		p, err := zp.modular.BorrowedModulus()
		if err != nil {
			return err
		}
		if err = zpe.modular.WithAllocationContext((numericIntegerModularManager_henselCallerAllocationContext(zpe.modular, caller)).utility(zpe.modular.Integers())).SetZP(p); err != nil {
			return err
		}
		if err = s.henselQuadraticWithCaller(m.stateView(parts), zpe, A, B, e, caller); err != nil {
			return err
		}
		conditional, err := s.henselConditionalAssertionEnabled()
		if err != nil {
			return err
		}
		if conditional {
			ok, err := s.checkIndividualLiftWithCaller(zp, zp.stateView(&pf.factors[i]), zpe, m.stateView(A), caller)
			if err != nil {
				return err
			}
			if !ok {
				if e := henselConditionalAssertionFailure(934, "check_individual_lift(zp_upm, zp_fs[i], zpe_upm, A)"); e != nil {
					return e
				}
			}
		}
		if err = s.traceHenselVectors(caller, 935); err != nil {
			return err
		}
		if i == 0 {
			if err = zpe.toZPCopyWithOwners(f, parts, nil, nil, caller); err != nil {
				return err
			}
		}
		if err = zpe.divWithCaller(m.stateView(parts), m.stateView(A), parts, caller); err != nil {
			return err
		}
		if err = lf.pushSwap(m, A, 1); err != nil {
			return err
		}
	}
	var localLCInv unumeral.Int
	lcInv := &localLCInv
	if caller == nil {
		defer m.config.Integer.Del(lcInv)
	} else {
		lcInv = &caller.finalLCInv
		caller.finalLCInvConstructed = true
		defer func() { failure = joinHenselCaller(failure, caller.finishCallerNumeral(false)) }()
	}
	top, err := m.read(f, f.count-1)
	if err != nil {
		return err
	}
	if err = zpe.modular.WithAllocationContext((numericIntegerModularManager_henselCallerAllocationContext(zpe.modular, caller)).utility(zpe.modular.Integers())).Copy(lcInv, top); err != nil {
		return err
	}
	if err = zpe.modular.WithAllocationContext((numericIntegerModularManager_henselCallerAllocationContext(zpe.modular, caller)).utility(zpe.modular.Integers())).Inv(lcInv); err != nil {
		return err
	}
	if err = zpe.scalarWithCaller(B, lcInv, caller); err != nil {
		return err
	}
	if err = lf.pushSwap(m, B, 1); err != nil {
		return err
	}
	if err = s.traceHenselFactors(caller, f, pf, lf, true); err != nil {
		return err
	}
	conditional, err := s.henselConditionalAssertionEnabled()
	if err != nil {
		return err
	}
	if conditional {
		ok, err := s.checkHenselFactorsWithCaller(f, pf, lf, e, caller)
		if err != nil {
			return err
		}
		if !ok {
			if e := henselConditionalAssertionFailure(964, "check_hensel_lift(upm, f, zp_fs, zpe_fs, e)"); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *UPolynomialFactorizationService) henselInitialC(f UPolynomialView, pf *UPolynomialFactors, C *upolyVectorState) error {
	return s.henselInitialCWithCaller(f, pf, C, nil)
}
func (s *UPolynomialFactorizationService) henselInitialCWithCaller(f UPolynomialView, pf *UPolynomialFactors, C *upolyVectorState, caller *upolyHenselCallerScope) (failure error) {
	m := s.manager
	zp := pf.manager
	if e := pf.multiplyWithCaller(C, caller); e != nil {
		return e
	}
	var localLC unumeral.Int
	lc := &localLC
	if caller == nil {
		defer m.config.Integer.Del(lc)
	} else {
		lc = &caller.initialLC
		caller.initialLCConstructed = true
		defer func() { failure = joinHenselCaller(failure, caller.finishCallerNumeral(true)) }()
	}
	top, e := m.read(f, f.count-1)
	if e != nil {
		return e
	}
	if e = zp.modular.WithAllocationContext((numericIntegerModularManager_henselCallerAllocationContext(zp.modular, caller)).utility(zp.modular.Integers())).Copy(lc, top); e != nil {
		return e
	}
	return zp.scalarWithCaller(C, lc, caller)
}

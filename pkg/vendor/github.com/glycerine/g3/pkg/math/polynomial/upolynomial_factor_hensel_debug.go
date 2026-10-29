package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:513-568,677-700,805-871.
// Source runtime CASSERT tag helpers, independent of g3debug build selection.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The source CASSERT tag is spelled "factorizatio::bughunt". The explicit
// service option represents that tag, while the process debug context retains
// the independent assertions_enabled switch and debugger action.
func (s *UPolynomialFactorizationService) henselConditionalAssertionEnabled() (bool, error) {
	if !polynomialSourceDebug || !s.config.FactorizationDebugTag {
		return false, nil
	}
	return diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
}

func henselConditionalAssertionFailure(line int32, expression string) error {
	e := diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/math/polynomial/upolynomial_factorization.cpp", line, expression)
	if e != nil {
		return errors.Join(ErrUPolynomialAssertion, e)
	}
	return nil
}

func (s *UPolynomialFactorizationService) checkHenselPair(C UPolynomialView, a, b, r *unumeral.Int, A, B, Al, Bl UPolynomialView, receipt *upolyHenselPairInvocationReceipt) (ok bool, failure error) {
	m := s.manager
	h := receipt.frame
	c := &h.check
	defer func() { failure = h.finishHenselCheckSource(failure) }()
	br := &c.br
	c.constructed = 1
	context := numericIntegerModularManager_henselPairAllocationContext(m.modular, receipt)
	if e := m.config.Integer.WithAllocationContext((context).utility(m.config.Integer)).Mul(b, r, br); e != nil {
		return false, e
	}
	brCore, e := NewRelatedUPolynomialManager(m)
	if e != nil {
		return false, e
	}
	c.brCore = brCore
	c.constructed = 2
	h.attachCheckManager(brCore, true)
	if e = brCore.modular.WithAllocationContext((numericIntegerModularManager_henselPairAllocationContext(brCore.modular, receipt)).utility(brCore.modular.Integers())).SetZP(br); e != nil {
		return false, e
	}
	if Al.count != A.count {
		return false, nil
	}
	if Bl.count != B.count {
		return false, nil
	}
	atop, e := m.read(A, A.count-1)
	if e != nil {
		return false, e
	}
	ltop, e := m.read(Al, Al.count-1)
	if e != nil {
		return false, e
	}
	same, e := m.config.Integer.Equal(atop, ltop)
	if e != nil || !same {
		return false, e
	}
	test1 := &c.test1
	c.constructed = 3
	if e = m.mulWithHensel(Al, Bl, test1, receipt); e != nil {
		return false, e
	}
	if e = m.addSubStateWithHensel(C, m.stateView(test1), test1, true, receipt); e != nil {
		return false, e
	}
	if e = brCore.toZPInPlaceWithHensel(test1, receipt); e != nil {
		return false, e
	}
	if test1.size != 0 {
		if e := s.traceHenselPairCheckFailure(h, A, B, C); e != nil {
			return false, e
		}
		return false, nil
	}
	bCore, e := NewRelatedUPolynomialManager(m)
	if e != nil {
		return false, e
	}
	c.bCore = bCore
	c.constructed = 4
	h.attachCheckManager(bCore, false)
	if e = bCore.modular.WithAllocationContext((numericIntegerModularManager_henselPairAllocationContext(bCore.modular, receipt)).utility(bCore.modular.Integers())).SetZP(b); e != nil {
		return false, e
	}
	test2a, test2b := &c.test2a, &c.test2b
	c.constructed = 6
	if e = bCore.toZPCopyWithHensel(A, test2a, receipt); e != nil {
		return false, e
	}
	if e = bCore.toZPCopyWithHensel(Al, test2b, receipt); e != nil {
		return false, e
	}
	same, e = m.EqualViews(m.stateView(test2a), m.stateView(test2b))
	if e != nil || !same {
		return false, e
	}
	test3a, test3b := &c.test3a, &c.test3b
	c.constructed = 8
	if e = bCore.toZPCopyWithHensel(B, test3a, receipt); e != nil {
		return false, e
	}
	if e = bCore.toZPCopyWithHensel(Bl, test3b, receipt); e != nil {
		return false, e
	}
	return m.EqualViews(m.stateView(test3a), m.stateView(test3b))
}
func (s *UPolynomialFactorizationService) checkQuadraticHensel(zpe *UPolynomialManager, U, A, V, B UPolynomialView) (bool, error) {
	return s.checkQuadraticHenselWithCaller(zpe, U, A, V, B, nil)
}
func (s *UPolynomialFactorizationService) checkQuadraticHenselWithCaller(zpe *UPolynomialManager, U, A, V, B UPolynomialView, caller *upolyHenselCallerScope) (ok bool, failure error) {
	m := s.manager
	var locals [3]upolyVectorState
	tmp1, tmp2, one := &locals[0], &locals[1], &locals[2]
	if caller == nil {
		defer m.dropLocalVector(tmp1)
		defer m.dropLocalVector(tmp2)
		defer m.dropLocalVector(one)
	} else {
		tmp1, tmp2, one = &caller.quadraticCheck[0], &caller.quadraticCheck[1], &caller.quadraticCheck[2]
		caller.quadraticCheckConstructed = 2
		caller.quadraticCheckSource = 2
		caller.quadraticCheckOwn = 2
		defer func() { failure = joinHenselCaller(failure, caller.finishQuadraticCheckSource()) }()
	}
	if e := zpe.mulWithCaller(U, A, tmp1, caller); e != nil {
		return false, e
	}
	if e := zpe.mulWithCaller(V, B, tmp2, caller); e != nil {
		return false, e
	}
	if caller != nil {
		caller.quadraticCheckConstructed = 3
		caller.quadraticCheckSource = 3
		caller.quadraticCheckOwn = 3
	}
	if e := zpe.addSubWithCaller(m.stateView(tmp1), m.stateView(tmp2), one, false, caller); e != nil {
		return false, e
	}
	if one.size == 1 {
		ok, failure = m.config.Integer.IsOne(&m.values(one)[0])
		if failure != nil {
			return false, failure
		}
	}
	if !ok {
		if e := s.traceQuadraticCheckFailure(caller, zpe, U, A, V, B); e != nil {
			return false, e
		}
	}
	return ok, nil
}
func (s *UPolynomialFactorizationService) checkIndividualLift(zp *UPolynomialManager, A UPolynomialView, zpe *UPolynomialManager, Al UPolynomialView) (bool, error) {
	return s.checkIndividualLiftWithCaller(zp, A, zpe, Al, nil)
}
func (s *UPolynomialFactorizationService) checkIndividualLiftWithCaller(zp *UPolynomialManager, A UPolynomialView, zpe *UPolynomialManager, Al UPolynomialView, caller *upolyHenselCallerScope) (ok bool, failure error) {
	var local upolyVectorState
	converted := &local
	if caller == nil {
		defer zp.dropLocalVector(converted)
	} else {
		converted = &caller.individual[0]
		caller.individualSource = 1
		caller.individualOwn = 1
		defer func() { failure = joinHenselCaller(failure, caller.finishIndividualSource()) }()
	}
	if e := zp.toZPCopyWithOwners(Al, converted, nil, nil, caller); e != nil {
		return false, e
	}
	return zp.EqualViews(A, zp.stateView(converted))
}
func (s *UPolynomialFactorizationService) checkHenselFactors(f UPolynomialView, pf, lf *UPolynomialFactors, e uint32) (bool, error) {
	return s.checkHenselFactorsWithCaller(f, pf, lf, e, nil)
}
func (s *UPolynomialFactorizationService) checkHenselFactorsWithCaller(f UPolynomialView, pf, lf *UPolynomialFactors, e uint32, caller *upolyHenselCallerScope) (ok bool, failure error) {
	m := s.manager
	zp, zpe := pf.manager, lf.manager
	p, err := zp.modular.BorrowedModulus()
	if err != nil {
		return false, err
	}
	pe, err := zpe.modular.BorrowedModulus()
	if err != nil {
		return false, err
	}
	var localPower unumeral.Int
	power := &localPower
	context := integerAllocationInvocationContext{}
	var receipt *unumeral.IntegerPowerInvocationReceipt
	if caller == nil {
		defer m.config.Integer.Del(power)
	} else {
		power = &caller.checkPower
		caller.checkPowerConstructed = true
		caller.checkPowerEnded = false
		caller.checkPowerOwned = false
		context = numericIntegerModularManager_henselCallerAllocationContext(m.modular, caller)
		receipt = &caller.checkPowerInvocation
		defer func() { failure = joinHenselCaller(failure, caller.finishConditionalSource()) }()
	}
	if err = m.config.Integer.WithAllocationContext((context).utility(m.config.Integer)).PowerTracked(p, e, power, receipt); err != nil {
		return false, err
	}
	ge, err := m.config.Integer.GreaterEqual(pe, power)
	if err != nil || !ge {
		return false, err
	}
	var first [2]upolyVectorState
	multp, fp := &first[0], &first[1]
	if caller == nil {
		defer m.dropLocalVector(multp)
		defer m.dropLocalVector(fp)
	} else {
		multp, fp = &caller.checkVectors[0], &caller.checkVectors[1]
		caller.checkSource = 2
		caller.checkOwn = 2
	}
	if err = pf.multiplyWithCaller(multp, caller); err != nil {
		return false, err
	}
	if err = zp.toZPCopyWithOwners(f, fp, nil, nil, caller); err != nil {
		return false, err
	}
	top, err := m.read(m.stateView(fp), fp.size-1)
	if err != nil {
		return false, err
	}
	if err = zp.scalarWithCaller(multp, top, caller); err != nil {
		return false, err
	}
	same, err := m.EqualViews(m.stateView(multp), m.stateView(fp))
	if err != nil {
		return false, err
	}
	if !same {
		if err = s.traceHenselProductFailure(caller, f, pf, p, false); err != nil {
			return false, err
		}
		return false, nil
	}
	if lf.count != pf.count {
		return false, nil
	}
	var second [2]upolyVectorState
	multpe, fpe := &second[0], &second[1]
	if caller == nil {
		defer m.dropLocalVector(multpe)
		defer m.dropLocalVector(fpe)
	} else {
		multpe, fpe = &caller.checkVectors[2], &caller.checkVectors[3]
		caller.checkSource = 4
		caller.checkOwn = 4
	}
	if err = lf.multiplyWithCaller(multpe, caller); err != nil {
		return false, err
	}
	if err = zpe.toZPCopyWithOwners(f, fpe, nil, nil, caller); err != nil {
		return false, err
	}
	top, err = m.read(m.stateView(fpe), fpe.size-1)
	if err != nil {
		return false, err
	}
	if err = zpe.scalarWithCaller(multpe, top, caller); err != nil {
		return false, err
	}
	same, err = m.EqualViews(m.stateView(multpe), m.stateView(fpe))
	if err != nil {
		return false, err
	}
	if !same {
		if err = s.traceHenselProductFailure(caller, f, lf, pe, true); err != nil {
			return false, err
		}
	}
	return same, nil
}

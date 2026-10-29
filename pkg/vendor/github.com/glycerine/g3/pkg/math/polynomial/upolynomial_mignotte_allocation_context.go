package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:979-1017;
// src/util/mpz.h:124-149,393-407; src/util/mpzzp.h:132,150-163,229-235,271-279.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c integerAllocationInvocationContext) mignotteReceipt(m *unumeral.IntegerManager) *upolyMignotteInvocationReceipt {
	r := c.mignotte
	if m == nil || !m.ValidOwner() || r == nil || r.service == nil || !r.service.matchesMignotteInvocation(r) || r.service.manager.config.Integer != m || !r.frame.executing || r.frame.constructed == 0 || r.service.manager.callbackDepth != 0 || c.mignotteInvocation == 0 || c.mignotteInvocation != r.invocation {
		return nil
	}
	return r
}
func numericIntegerModularManager_mignotteAllocationContext(m *unumeral.IntegerModularManager, r *upolyMignotteInvocationReceipt) integerAllocationInvocationContext {
	if m == nil || m.Integers() == nil || r == nil || r.service == nil || r.service.manager == nil || r.service.manager.modular != m {
		return integerAllocationInvocationContext{}
	}
	c := integerAllocationInvocationContext{mignotte: r, mignotteInvocation: r.invocation}
	if c.mignotteReceipt(m.Integers()) == nil {
		return integerAllocationInvocationContext{}
	}
	return c
}

// One source call dispatches into the original scalar engines. The explicit
// context reaches only actual allocation leaves; successful NUM publication
// remains in the declared norm/bound/tmp, never a mirrored numeral array.
func (s *UPolynomialFactorizationService) mignotteNumericCall(r *upolyMignotteInvocationReceipt, op uint8, a, b, out *unumeral.Int, k uint32) error {
	if s == nil || s.self != s || r == nil || r.service != s {
		return ErrUPolynomialOwner
	}
	m := s.manager.modular
	if e := m.Check(); e != nil {
		return e
	}
	c := numericIntegerModularManager_mignotteAllocationContext(m, r)
	if c.mignotteReceipt(m.Integers()) == nil {
		return ErrUPolynomialOwner
	}
	r.frame.enterCall(s.manager)
	defer r.frame.leaveCall(s.manager)
	q := r.frame
	if !((out == &q.norm && q.constructed >= 1) || (out == &q.bound && q.constructed >= 2) || (out == &q.tmp && q.constructed >= 3)) {
		return ErrUPolynomialOwner
	}
	var e error
	switch op {
	case 0:
		e = m.WithAllocationContext((c).utility(m.Integers())).AddMulTracked(out, a, b, out, &r.frame.addMul)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = m.Integers().AddMulInvocationSourceCause(&r.frame.addMul)
		}
		return e
	case 1:
		_, e = m.RootInPlaceWithReceipt(out, k, &r.frame.root)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = m.Integers().RootInvocationSourceCause(&r.frame.root)
		}
		return e
	case 2:
		return m.WithAllocationContext((c).utility(m.Integers())).SetInt64(out, int64(int32(k)))
	case 3:
		return m.WithAllocationContext((c).utility(m.Integers())).Mul2k(a, k, out)
	case 4:
		return m.WithAllocationContext((c).utility(m.Integers())).Copy(out, a)
	case 5:
		return m.WithAllocationContext((c).utility(m.Integers())).Abs(out)
	case 6:
		return m.WithAllocationContext((c).utility(m.Integers())).Add(a, b, out)
	case 7:
		return m.WithAllocationContext((c).utility(m.Integers())).Mul(a, b, out)
	default:
		return ErrUPolynomialAssertion
	}
}

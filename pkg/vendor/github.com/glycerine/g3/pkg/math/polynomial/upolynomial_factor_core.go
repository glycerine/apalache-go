package polynomial

// Source: src/math/polynomial/upolynomial.cpp:2960-3118;
// src/math/polynomial/upolynomial_factorization.cpp:1036-1050.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (s *UPolynomialFactorizationService) Factor(f UPolynomialView, out *UPolynomialFactors, params UPolynomialFactorParams) (bool, error) {
	if e := s.check(); e != nil {
		return false, e
	}
	if e := s.manager.checkFactors(out); e != nil {
		return false, e
	}
	result, e := s.factorCore(f, out, params)
	if e != nil {
		return result, e
	}
	return result, s.verboseFactorCount(out)
}
func (s *UPolynomialFactorizationService) flipFactorConstant(fs *UPolynomialFactors) (cause error) {
	m := s.manager
	o := m.factorFrontendCaller
	if o == nil || !o.actual() || !o.executing || o.result != fs {
		return ErrUPolynomialOwner
	}
	q := &o.flip
	if q.constructed || q.executing || q.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	*q = upolyFactorFlipScope{owner: o, invocation: q.invocation + 1, constructed: true, executing: true}
	defer func() { q.executing = false; cause = q.finish(cause) }()
	context := numericIntegerModularManager_factorFrontendAllocationContext(m.modular, o)
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(&q.c, &fs.constant); e != nil {
		return e
	}
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Neg(&q.c); e != nil {
		return e
	}
	return fs.setConstantWithSourceContext(&q.c, context)
}
func (s *UPolynomialFactorizationService) flipLeadingIfNegative(p *upolyVectorState, fs *UPolynomialFactors, k uint32) error {
	m := s.manager
	if p.size == 0 {
		return nil
	}
	top, e := m.read(m.stateView(p), p.size-1)
	if e != nil {
		return e
	}
	negative, e := m.modular.IsNeg(top)
	if e != nil || !negative {
		return e
	}
	if e = m.negViewInPlaceWithSourceContext(m.stateView(p), numericIntegerModularManager_factorFrontendAllocationContext(m.modular, m.factorFrontendCaller)); e != nil {
		return e
	}
	if k%2 == 1 {
		return s.flipFactorConstant(fs)
	}
	return nil
}
func (s *UPolynomialFactorizationService) factorQuadratic(p *upolyVectorState, fs *UPolynomialFactors, k uint32) (cause error) {
	m := s.manager
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && p.size != 3 {
		if e := m.scalarDivisionAssertionFailure(2968, "p.size() == 3"); e != nil {
			return e
		}
	}
	if e := s.traceFactorQuadraticPolynomials(m.factorFrontendCaller, p, 2969); e != nil {
		return e
	}
	a, e := m.read(m.stateView(p), 2)
	if e != nil {
		return e
	}
	b, e := m.read(m.stateView(p), 1)
	if e != nil {
		return e
	}
	c, e := m.read(m.stateView(p), 0)
	if e != nil {
		return e
	}
	o := m.factorFrontendCaller
	if o == nil || !o.actual() || !o.executing || o.result != fs {
		return ErrUPolynomialOwner
	}
	if e := s.traceFactorQuadraticScalars(o, [3]*unumeral.Int{a, b, c}, 2974); e != nil {
		return e
	}
	q := &o.quadratic
	if q.constructed != 0 || q.executing || q.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	*q = upolyFactorQuadraticScope{owner: o, invocation: q.invocation + 1, constructed: 3, peak: 3, executing: true}
	defer func() { q.executing = false; cause = q.finish(cause) }()
	b2, ac, disc, root := &q.values[0], &q.values[1], &q.values[2], &q.values[3]
	context := numericIntegerModularManager_factorFrontendAllocationContext(m.modular, o)
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Power(b, 2, b2); e != nil {
		return e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(a, c, ac); e != nil {
		return e
	}
	minus4 := unumeral.NewInt(-4)
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).AddMulTracked(b2, &minus4, ac, disc, &o.addMul); e != nil {
		return e
	}
	if e := m.quadraticDiscriminantAssertion(disc); e != nil {
		return e
	}

	q.constructed = 4
	q.peak = 4
	square, e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).IsPerfectSquare(disc, root)
	if e != nil {
		return e
	}
	if !square {
		return fs.pushCopyWithSourceContext(m.stateView(p), k, numericIntegerModularManager_factorFrontendAllocationContext(m.modular, m.factorFrontendCaller))
	}
	if e := s.traceFactorQuadraticScalars(o, [3]*unumeral.Int{root}, 2990); e != nil {
		return e
	}
	f1, f2 := &q.f1, &q.f2
	q.constructed = 6
	q.peak = 6
	if e = m.reserve(f1, 2); e != nil {
		return e
	}
	if e = m.reserve(f2, 2); e != nil {
		return e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Sub(b, root, &m.values(f1)[0]); e != nil {
		return e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Add(b, root, &m.values(f2)[0]); e != nil {
		return e
	}
	two := unumeral.NewInt(2)
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(a, &two, &m.values(f1)[1]); e != nil {
		return e
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(a, &two, &m.values(f2)[1]); e != nil {
		return e
	}
	if e = m.setSize(f1, 2); e != nil {
		return e
	}
	if e = m.setSize(f2, 2); e != nil {
		return e
	}
	if e = (m).normalizeWithSourceContext(m.stateView(f1), context); e != nil {
		return e
	}
	if e = (m).normalizeWithSourceContext(m.stateView(f2), context); e != nil {
		return e
	}
	if e := s.traceFactorQuadraticPolynomials(o, p, 3003); e != nil {
		return e
	}
	if polynomialSourceDebug {
		if e = s.quadraticDiagnosticWithFrontend(p, f1, f2, q, context); e != nil {
			return e
		}
	}
	if e = fs.pushCopyWithSourceContext(m.stateView(f1), k, context); e != nil {
		return e
	}
	return fs.pushCopyWithSourceContext(m.stateView(f2), k, context)
}
func (s *UPolynomialFactorizationService) quadraticDiagnosticWithFrontend(p, f1, f2 *upolyVectorState, q *upolyFactorQuadraticScope, context integerAllocationInvocationContext) (cause error) {
	m := s.manager
	q.constructed = 7
	q.peak = 7
	defer func() { cause = q.finishProduct(cause) }()
	if e := m.mulWithSourceContext(m.stateView(f1), m.stateView(f2), &q.product, &q.owner.addMul, context); e != nil {
		return e
	}
	return m.quadraticProductAssertion(m.stateView(&q.product), m.stateView(p))
}
func (s *UPolynomialFactorizationService) factorSqfPrimitive(p *upolyVectorState, fs *UPolynomialFactors, k uint32, params UPolynomialFactorParams) (bool, error) {
	return s.factorSqfPrimitiveWithReceipt(p, fs, k, params, nil)
}
func (s *UPolynomialFactorizationService) factorSqfPrimitiveWithReceipt(p *upolyVectorState, fs *UPolynomialFactors, k uint32, params UPolynomialFactorParams, r *upolyFactorSquareFreeInvocationReceipt) (bool, error) {
	m := s.manager
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return false, e
	}
	if enabled && p.size != 0 {
		top, e := m.read(m.stateView(p), p.size-1)
		if e != nil {
			return false, e
		}
		positive, e := m.modular.IsPos(top)
		if e != nil {
			return false, e
		}
		if !positive {
			if e := m.scalarDivisionAssertionFailure(3015, "sz == 0 || m().is_pos(p[sz-1])"); e != nil {
				return false, e
			}
		}
	}
	if p.size <= 2 {
		return true, fs.pushCopyWithSourceContext(m.stateView(p), k, numericIntegerModularManager_factorFrontendAllocationContext(m.modular, m.factorFrontendCaller))
	}
	if p.size == 3 {
		return true, s.factorQuadratic(p, fs, k)
	}
	return s.factorSquareFreeWithReceipt(m.stateView(p), fs, k, params, r)
}
func (s *UPolynomialFactorizationService) exactDivVerifyWithSourceContext(a, b UPolynomialView, out *upolyVectorState, child *unumeral.IntegerAddMulInvocationReceipt, context integerAllocationInvocationContext, line int32) error {
	yes, e := s.manager.exactDivStateWithSourceContext(a, b, out, child, context)
	if e != nil {
		return e
	}
	if !yes {
		return s.manager.factorExactDivisionVerifyFailure(line)
	}
	return nil
}
func (s *UPolynomialFactorizationService) factorCore(f UPolynomialView, fs *UPolynomialFactors, params UPolynomialFactorParams) (completed bool, failure error) {
	m := s.manager
	if f.count == 0 {
		zero := unumeral.NewInt(0)
		return true, s.setFrontendConstant(fs, &zero)
	}
	if f.count == 1 {
		a, e := m.read(f, 0)
		if e != nil {
			return false, e
		}
		return true, s.setFrontendConstant(fs, a)
	}
	q, e := s.beginFactorFrontend()
	if e != nil {
		return false, e
	}
	q.result = fs
	defer func() { q.bodyReturned(); failure = q.finish(failure) }()
	content := &q.content
	pp, prime, A, B, D := &q.pp, &q.prime, &q.A, &q.B, &q.D
	q.constructed = 2
	if e := m.primitiveStateWithSourceContext(f, pp, content, numericIntegerModularManager_factorFrontendAllocationContext(m.modular, q)); e != nil {
		return false, e
	}
	if e := fs.setConstantWithSourceContext(content, numericIntegerModularManager_factorFrontendAllocationContext(m.modular, q)); e != nil {
		return false, e
	}
	q.constructed = 3
	if e := m.derivativeStateWithSourceContext(m.stateView(pp), prime, numericIntegerModularManager_factorFrontendAllocationContext(m.modular, q)); e != nil {
		return false, e
	}
	q.constructed = 6
	if e := m.gcdWithSourceContext(m.stateView(pp), m.stateView(prime), B, numericIntegerModularManager_factorFrontendAllocationContext(m.modular, q)); e != nil {
		return false, e
	}
	result := true
	if B.size <= 1 {
		if e := m.factorNonconstantAssertion(pp); e != nil {
			return false, e
		}
		if e := s.flipLeadingIfNegative(pp, fs, 1); e != nil {
			return false, e
		}
		yes, e := s.factorSqfPrimitiveWithReceipt(pp, fs, 1, params, &q.squareFree)
		if e != nil {
			return false, e
		}
		if !yes {
			result = false
		}
	} else {
		if e := s.exactDivVerifyWithSourceContext(m.stateView(pp), m.stateView(B), A, &q.addMul, numericIntegerModularManager_factorFrontendAllocationContext(m.modular, q), 3074); e != nil {
			return false, e
		}
		if e := s.traceFactorCore(q, 3075, 0); e != nil {
			return false, e
		}
		for j := uint32(1); A.size > 1; j++ {
			if e := m.checkpoint(); e != nil {
				return false, e
			}
			if e := s.traceFactorCore(q, 3080, j); e != nil {
				return false, e
			}
			if e := m.gcdWithSourceContext(m.stateView(A), m.stateView(B), D, numericIntegerModularManager_factorFrontendAllocationContext(m.modular, q)); e != nil {
				return false, e
			}
			if e := s.exactDivVerifyWithSourceContext(m.stateView(A), m.stateView(D), pp, &q.addMul, numericIntegerModularManager_factorFrontendAllocationContext(m.modular, q), 3085); e != nil {
				return false, e
			}
			if pp.size > 1 {
				if e := s.flipLeadingIfNegative(pp, fs, j); e != nil {
					return false, e
				}
				yes, e := s.factorSqfPrimitiveWithReceipt(pp, fs, j, params, &q.squareFree)
				if e != nil {
					return false, e
				}
				if !yes {
					result = false
				}
			} else {
				if e := s.traceFactorCore(q, 3093, j); e != nil {
					return false, e
				}
				if e := m.factorConstantSizeAssertion(pp); e != nil {
					return false, e
				}
				if e := m.factorConstantUnitAssertion(pp); e != nil {
					return false, e
				}
				a, e := m.read(m.stateView(pp), 0)
				if e != nil {
					return false, e
				}

				minus, e := m.modular.IsMinusOne(a)
				if e != nil {
					return false, e
				}
				if minus && j%2 == 1 {
					if e = s.flipFactorConstant(fs); e != nil {
						return false, e
					}
				}
			}
			if e := s.traceFactorCore(q, 3099, j); e != nil {
				return false, e
			}
			if e := s.exactDivVerifyWithSourceContext(m.stateView(B), m.stateView(D), B, &q.addMul, numericIntegerModularManager_factorFrontendAllocationContext(m.modular, q), 3100); e != nil {
				return false, e
			}
			*A, *D = *D, *A
		}
		if e := s.traceFactorCore(q, 3106, 0); e != nil {
			return false, e
		}
		if e := m.factorFinalOneAssertion(A); e != nil {
			return false, e
		}

	}
	return result, nil
}

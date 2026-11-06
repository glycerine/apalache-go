package polynomial

// Source: src/math/polynomial/polynomial.cpp:3555-4506;
// src/util/obj_ref.h; src/util/ref_buffer.h; src/util/scoped_numeral.h.

// Explicit owning acknowledgement. Completed concrete local destruction is
// separate from partial constructor/incoming-INC abort; no source cursor is
// rolled back and no registry supplies roots.
func (m *MultivariatePolynomialManager) CleanupGCD() error {
	if m == nil || m.self != m || m.core == nil {
		return ErrPolynomialOwner
	}
	if m.gcdDepth != 0 || m.core.quadraticTraceCallback() || m.core.evaluationTraceCallback() {
		return ErrPolynomialPending
	}
	if e := m.cleanupGCDScopes(m.gcdFrames, false); e != nil {
		return e
	}
	return m.cleanupGCDScopes(m.gcdFrames, true)
}
func (m *MultivariatePolynomialManager) cleanupGCDScopes(scopes []*polynomialGCDFunctionScope, abort bool) error {
	c := m.core
	if !abort {
		for j := len(scopes) - 1; j >= 0; j-- {
			s := scopes[j]
			if !s.active {
				continue
			}
			if e := m.cleanupLexSortInvocation(&s.lexSort); e != nil {
				return e
			}
			if e := c.cleanupDisplayReceipt(&s.traceDisplay); e != nil {
				return e
			}
			if e := c.cleanupArithmeticInvocation(&s.product, false); e != nil {
				return e
			}
			if e := m.cleanupGCDDirectChildren(s, false); e != nil {
				return e
			}
			if e := m.cleanupResultantConcreteChildren(s, false); e != nil {
				return e
			}
			if e := c.cleanupArithmeticInvocation(&s.resultantArithmetic, false); e != nil {
				return e
			}
			if e := c.cleanupPseudoInvocation(&s.resultantPseudo, false); e != nil {
				return e
			}
			if e := c.cleanupPowerInvocation(&s.resultantPower, false); e != nil {
				return e
			}
			if e := m.cleanupQuadraticChildren(s, false); e != nil {
				return e
			}
			if e := c.cleanupContentInvocation(&s.contentInvocation, false); e != nil {
				return e
			}
			if e := m.cleanupICCPInvocation(&s.iccpInvocation, false); e != nil {
				return e
			}
			if e := m.cleanupDerivativeInvocation(&s.derivativeInvocation, false); e != nil {
				return e
			}
			if e := c.cleanupDividesInvocation(&s.dividesInvocation, false); e != nil {
				return e
			}
			if e := c.cleanupExactDivInvocation(&s.divisionInvocation, false); e != nil {
				return e
			}
			if e := m.cleanupNormalizeInvocation(&s.normalizeInvocation, false); e != nil {
				return e
			}
			if s.topOwner && s.generation == c.pscReceipts.generation {
				if e := c.cleanupPSCCompleted(s.generation); e != nil {
					return e
				}
			}
			for s.depth != 0 {
				id := s.stack[s.depth-1]
				var e error
				switch {
				case id == gcdMonomialBufferID:
					e = s.monomialBuffer.close(c, true)
				case id >= gcdMonomialBase:
					r := &s.monomials[id-gcdMonomialBase]
					e = r.drop(c, &r.value, &r.decrementStarted, true)
				case id == gcdVariablesId:
					s.vars = nil
				case id == gcdSparseId:
					e = s.sparse.CloseChecked()
				case id == gcdZPId:
					// A completed older class whose body was never reached has its literal
					// restore action once. A previously stopped member is owned-acknowledged.
					if s.zp.completed && s.zp.stage == 0 && s.zp.sourceCause == nil {
						e = s.zp.sourceEnd()
					} else {
						e = s.zp.cleanup()
					}
				case id < gcdNumeralBase:
					s.powers[id] = nil
				case id < gcdReferenceBase:
					e = c.deleteNumeral(&s.nums[id-gcdNumeralBase])
				default:
					e = s.refs[id-gcdReferenceBase].root.closeValue(c, true)
				}
				if e != nil {
					return e
				}
				s.depth--
			}
			if e := s.returned.closeValue(c, true); e != nil {
				return e
			}
		}
		return nil
	}
	// Only after ALL completed younger/outer objects have been acknowledged do
	// actual unjoined incoming references and failed-constructor roots abort.
	for j := len(scopes) - 1; j >= 0; j-- {
		s := scopes[j]
		if !s.active {
			continue
		}
		if e := c.cleanupArithmeticInvocation(&s.product, true); e != nil {
			return e
		}
		if e := m.cleanupGCDZPXMonomial(s); e != nil {
			return e
		}
		if e := m.cleanupGCDDirectChildren(s, true); e != nil {
			return e
		}
		if e := m.cleanupResultantConcreteChildren(s, true); e != nil {
			return e
		}
		if e := c.cleanupArithmeticInvocation(&s.resultantArithmetic, true); e != nil {
			return e
		}
		if e := c.cleanupPseudoInvocation(&s.resultantPseudo, true); e != nil {
			return e
		}
		if e := c.cleanupPowerInvocation(&s.resultantPower, true); e != nil {
			return e
		}
		if e := m.cleanupQuadraticChildren(s, true); e != nil {
			return e
		}
		if e := c.cleanupContentInvocation(&s.contentInvocation, true); e != nil {
			return e
		}
		if e := m.cleanupICCPInvocation(&s.iccpInvocation, true); e != nil {
			return e
		}
		if e := m.cleanupDerivativeInvocation(&s.derivativeInvocation, true); e != nil {
			return e
		}
		if e := c.cleanupDividesInvocation(&s.dividesInvocation, true); e != nil {
			return e
		}
		if e := c.cleanupExactDivInvocation(&s.divisionInvocation, true); e != nil {
			return e
		}
		if e := m.cleanupNormalizeInvocation(&s.normalizeInvocation, true); e != nil {
			return e
		}
		if e := s.returned.abort(c); e != nil {
			return e
		}
		for i := len(s.refs) - 1; i >= 0; i-- {
			if e := s.refs[i].root.abort(c); e != nil {
				return e
			}
		}
		for i := len(s.monomials) - 1; i >= 0; i-- {
			r := &s.monomials[i]
			if e := r.drop(c, &r.incoming, &r.incomingStarted, true); e != nil {
				return e
			}
			r.assigning = false
		}
		if e := s.monomialBuffer.close(c, true); e != nil {
			return e
		}
		if s.sparse.self != nil && !s.sparse.closed {
			if e := s.sparse.CloseChecked(); e != nil {
				return e
			}
		}
		if e := s.zp.cleanup(); e != nil {
			return e
		}
		s.vars = nil
		for j := range s.powers {
			s.powers[j] = nil
		}
		if e := s.numeric.cleanup(c.integer); e != nil {
			return e
		}
		if s.newSkeleton != nil {
			if e := s.newSkeleton.CloseChecked(); e != nil {
				return e
			}
			s.newSkeleton = nil
		}
		if s.topOwner && s.generation == c.pscReceipts.generation {
			if e := c.cleanupPSCAborts(s.generation); e != nil {
				return e
			}
		}
	}
	for _, s := range scopes {
		if s.active {
			h := s.header
			// Retain captured diagnostic causes only; the next guarded begin resets them.
			*s = polynomialGCDFunctionScope{header: h, invocation: s.invocation, traceDisplay: s.traceDisplay, bodyCause: s.bodyCause, cleanupCause: s.cleanupCause, normalizeInvocation: s.normalizeInvocation, resultantPower: s.resultantPower, resultantPseudo: s.resultantPseudo, resultantArithmetic: s.resultantArithmetic, resultantCoefficient: s.resultantCoefficient, resultantAppend: s.resultantAppend, resultantFactory: s.resultantFactory, resultantNeg: s.resultantNeg}
		}
	}
	return nil
}

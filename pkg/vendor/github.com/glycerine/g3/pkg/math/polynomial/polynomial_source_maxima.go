package polynomial

// Source: src/math/polynomial/polynomial.cpp:1479-1491,1964-1981,2048-2068.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialCore) gradedLexMaxPosition(p Polynomial) (uint32, error) {
	n, e := m.readWord(p, 8)
	if e != nil {
		return 0, e
	}
	if n == 0 {
		return ^uint32(0), nil
	}
	max := uint32(0)
	for i := uint32(1); i < n; i++ {
		a, e := m.monomialCore(p, i)
		if e != nil {
			return 0, e
		}
		b, e := m.monomialCore(p, max)
		if e != nil {
			return 0, e
		}
		cmp, e := m.monomials.GradedLex(a, b)
		if e != nil {
			return 0, e
		}
		if cmp > 0 {
			max = i
		}
	}
	return max, nil
}
func (b *polynomialSOM) gradedLexMaxPosition() (uint32, error) {
	m := b.owner
	max := ^uint32(0)
	n, e := b.as.size(m)
	if e != nil {
		return 0, e
	}
	for i := uint32(0); i < n; i++ {
		a, e := b.as.coefficient(m, i)
		if e != nil {
			return 0, e
		}
		zero, e := m.modular.IsZero(&a)
		if e != nil {
			return 0, e
		}
		if !zero {
			if max == ^uint32(0) {
				max = i
			} else {
				x, e := b.ms.monomial(m, i)
				if e != nil {
					return 0, e
				}
				y, e := b.ms.monomial(m, max)
				if e != nil {
					return 0, e
				}
				cmp, e := m.monomials.GradedLex(x, y)
				if e != nil {
					return 0, e
				}
				if cmp > 0 {
					max = i
				}
			}
		}
	}
	return max, nil
}

// Unlike add(a,m), this source overload publishes the borrowed monomial slot
// before INC. A guarded INC failure leaves that exact slot nonowning.
func (b *polynomialSOM) addPolynomial(p Polynomial) error {
	return b.addPolynomialWithArithmeticReceipt(p, nil)
}
func (b *polynomialSOM) addPolynomialWithArithmeticReceipt(p Polynomial, receipt *polynomialArithmeticInvocationReceipt) (cause error) {
	m := b.owner
	if receipt != nil {
		if !m.matchesArithmeticInvocation(receipt) || receipt.frame.buffer != b || receipt.frame.executing {
			return ErrPolynomialOwner
		}
		receipt.frame.executing = true
		defer func() { receipt.frame.executing = false; receipt.frame.bodyCause = cause }()
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return e
		}
		pos, e := b.positions.Get(term)
		if e != nil {
			return e
		}
		if pos == ^uint32(0) {
			sz, e := b.ms.size(m)
			if e != nil {
				return e
			}
			var appendScope *polynomialCheapAppendScope
			if receipt != nil {
				if e = b.acquireAppendInvocation(&receipt.frame.append, sz); e != nil {
					return e
				}
				appendScope = &b.appendScope
			}
			if b.appendScope.active {
				if e = b.prepareAppendTags(sz + 1); e != nil {
					return e
				}
			}
			if e = b.positions.Set(term, sz); e != nil {
				return e
			}
			if e = b.ms.appendMonomial(m, term); e != nil {
				return e
			}
			if b.appendScope.active {
				b.monomialOwners[sz], b.monomialTransferred[sz] = 0, 0
			}
			b.borrowedAppend = true
			b.borrowedAppendIndex = sz
			if e = m.monomials.IncRef(term); e != nil {
				return e
			}
			b.borrowedAppend = false
			if appendScope != nil {
				b.monomialOwners[sz] = appendScope.invocation
				appendScope.monomialClaims++
				appendScope.monomials = sz + 1
			} else if b.appendScope.active {
				b.appendScope.foreignMixed = true
			}
			observer := appendScope
			if observer == nil && b.appendScope.active {
				observer = &b.appendScope
			}
			if e = b.as.appendCoefficientWithAppendScope(m, unumeral.Int{}, observer, appendScope != nil); e != nil {
				return e
			}
			a, e := b.as.coefficient(m, sz)
			if e != nil {
				return e
			}
			source, e := m.coefficientCore(p, i)
			if e != nil {
				return e
			}
			if receipt == nil {
				e = m.modular.Copy(&a, &source)
			} else {
				e = numericIntegerModularManager_copyWithArithmeticReceipt(m.modular, &a, &source, receipt)
			}
			if w := b.as.putCoefficient(m, sz, a); w != nil {
				return w
			}
			if e != nil {
				return e
			}
		} else {
			a, e := b.as.coefficient(m, pos)
			if e != nil {
				return e
			}
			source, e := m.coefficientCore(p, i)
			if e != nil {
				return e
			}
			if receipt == nil {
				e = m.modular.Add(&a, &source, &a)
			} else {
				e = numericIntegerModularManager_addWithArithmeticReceipt(m.modular, &a, &source, &a, receipt)
			}
			if w := b.as.putCoefficient(m, pos, a); w != nil {
				return w
			}
			if e != nil {
				return e
			}
		}
	}
	return nil
}

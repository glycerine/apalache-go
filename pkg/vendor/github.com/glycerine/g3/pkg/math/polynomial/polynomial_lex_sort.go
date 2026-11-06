package polynomial

// Source: src/math/polynomial/polynomial.cpp:1315-1366,1440-1460,2462-2464,7022-7024;
// src/util/permutation.h:57-91.

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (m *MultivariatePolynomialManager) lexMaximumBelow(p Polynomial, start, end, x uint32) (uint32, error) {
	max := ^uint32(0)
	ms, e := m.core.captureMonomialPointer(p, 0)
	if e != nil {
		return 0, e
	}
	for i := start; i < end; i++ {
		a, e := ms.read(i)
		if e != nil {
			return 0, e
		}
		y, e := m.core.monomials.MaxSmallerThan(a, x)
		if e != nil {
			return 0, e
		}
		if y != ^uint32(0) && (max == ^uint32(0) || y > max) {
			max = y
		}
	}
	return max, nil
}
func (m *MultivariatePolynomialManager) lexRange(p Polynomial, start, end, x uint32) error {
	if e := polynomialLexSortSourceAssert(1316, "end > start", func() (bool, error) {
		return end > start, nil
	}); e != nil {
		return e
	}
	c := m.core
	ms, e := c.captureMonomialPointer(p, 0)
	if e != nil {
		return e
	}
	maxDegree := uint32(0)
	for i, j := start, uint32(0); i < end; i, j = i+1, j+1 {
		a, e := ms.read(i)
		if e != nil {
			return e
		}
		d, e := c.monomials.DegreeOf(a, x)
		if e != nil {
			return e
		}
		n := d + 1
		if uint64(n) > uint64(len(m.lexBuckets)) {
			m.lexBuckets = append(m.lexBuckets, make([][]uint32, int(n)-len(m.lexBuckets))...)
		}
		if uint64(d) >= uint64(len(m.lexBuckets)) {
			if polynomialSourceDebug {
				return ErrPolynomialAssertion
			}
			return arena.ErrBounds
		}
		m.lexBuckets[d] = append(m.lexBuckets[d], j)
		if d > maxDegree {
			maxDegree = d
		}
	}
	m.lexPermutation = m.lexPermutation[:0]
	for i := maxDegree + 1; i > 0; {
		i--
		m.lexPermutation = append(m.lexPermutation, m.lexBuckets[i]...)
		m.lexBuckets[i] = m.lexBuckets[i][:0]
	}
	n := uint32(len(m.lexPermutation))
	if e = polynomialLexSortSourceAssert(1333, "p.size() == end - start", func() (bool, error) {
		return n == end-start, nil
	}); e != nil {
		return e
	}
	if e = m.lexApplyPermutation(p, start, n, true); e != nil {
		return e
	}
	if e = m.lexApplyPermutation(p, start, n, false); e != nil {
		return e
	}
	for i := start; i < end; {
		a, e := ms.read(i)
		if e != nil {
			return e
		}
		d, e := c.monomials.DegreeOf(a, x)
		if e != nil {
			return e
		}
		if d == 0 {
			y, e := m.lexMaximumBelow(p, i, end, x)
			if e != nil {
				return e
			}
			if y != ^uint32(0) {
				return m.lexRange(p, i, end, y)
			}
			return nil
		}
		j := i + 1
		for ; j < end; j++ {
			a, e := ms.read(j)
			if e != nil {
				return e
			}
			next, e := c.monomials.DegreeOf(a, x)
			if e != nil {
				return e
			}
			if e = polynomialLexSortSourceAssert(1352, "d_j <= d", func() (bool, error) {
				return next <= d, nil
			}); e != nil {
				return e
			}
			if next < d {
				break
			}
		}
		if e = polynomialLexSortSourceAssert(1356, "j == end || m_ms[j]->degree_of(x) < d", func() (bool, error) {
			if j == end {
				return true, nil
			}
			a, e := ms.read(j)
			if e != nil {
				return false, e
			}
			next, e := c.monomials.DegreeOf(a, x)
			if e != nil {
				return false, e
			}
			return next < d, nil
		}); e != nil {
			return e
		}
		if j > i+1 {
			y, e := m.lexMaximumBelow(p, i, j, x)
			if e != nil {
				return e
			}
			if y != ^uint32(0) {
				if e = m.lexRange(p, i, j, y); e != nil {
					return e
				}
			}
		}
		i = j
	}
	return nil
}
func (m *MultivariatePolynomialManager) LexSort(p Polynomial) error {
	return m.lexSortForAlgebraic(p, nil)
}
func (m *MultivariatePolynomialManager) lexSortForAlgebraic(p Polynomial, caller *polynomialAlgebraicEvaluationScope) error {
	return m.lexSortWithReceipt(p, caller, nil)
}
func (m *MultivariatePolynomialManager) lexSortWithReceipt(p Polynomial, caller *polynomialAlgebraicEvaluationScope, receipt *polynomialLexSortInvocationReceipt) error {
	if e := m.ready(); e != nil {
		return e
	}
	if m.lexTracePending() {
		return ErrPolynomialPending
	}
	flags, e := m.core.readWord(p, 4)
	if e != nil {
		return e
	}
	if flags>>31 != 0 {
		return nil
	}
	n, e := m.core.readWord(p, 8)
	if e != nil {
		return e
	}
	if n <= 1 {
		return m.core.writeWord(p, 4, flags|0x80000000)
	}
	a, e := m.core.monomialCore(p, 0)
	if e != nil {
		return e
	}
	x, e := m.core.monomials.MaxVariable(a)
	if e != nil {
		return e
	}
	if m.lexPending {
		return ErrPolynomialPending
	}
	if m.lexGeneration == ^uint64(0) {
		return ErrPolynomialPending
	}
	m.lexGeneration++
	if receipt != nil {
		*receipt = polynomialLexSortInvocationReceipt{m, m.lexGeneration}
	}
	if caller != nil {
		caller.lexGeneration = m.lexGeneration
	}
	if e = m.lexRange(p, 0, n, x); e != nil {
		m.lexPending = true
		m.lexCause = e
		return e
	}
	if e = m.core.writeWord(p, 4, flags|0x80000000); e != nil {
		return e
	}
	if polynomialSourceDebug {
		if e = m.lexDebugCheck(p); e != nil {
			if m.lexTracePending() {
				m.lexPending = true
				m.lexCause = e
			}
			return e
		}
	}
	return nil
}

// CleanupLexSort releases native scratch retained by a stopped invocation.
// It does not undo polynomial publications or reexecute the stopped source call.
func (m *MultivariatePolynomialManager) CleanupLexSort() error {
	if e := m.identity(); e != nil {
		return e
	}
	if m.core.quadraticTraceCallback() {
		return ErrPolynomialPending
	}
	if !m.lexTracePending() {
		if e := m.ready(); e != nil {
			return e
		}
	}
	return m.cleanupLexSort()
}
func (m *MultivariatePolynomialManager) cleanupLexSort() error {
	if m.core.quadraticTraceCallback() {
		return ErrPolynomialPending
	}
	if m.lexTraceGeneration == m.lexGeneration {
		if e := m.core.cleanupDisplayReceipt(&m.lexTraceDisplay); e != nil {
			return e
		}
		m.lexTraceGeneration = 0
		if m.core.pendingLexSort == m {
			m.core.pendingLexSort = nil
		}
	}
	if !m.lexPending {
		return nil
	}
	m.lexBuckets = nil
	m.lexPermutation = nil
	m.lexPending = false
	return nil
}

package polynomial

// Source: src/math/polynomial/polynomial.cpp:2100-2113.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"sort"
)

// Go's sort supplies the source's descending graded-lex result. The C++
// standard-library implementation and its internal movement are not ported.
func (b *PolynomialSOMBuffer) sortMonomials() error {
	var cause error
	sort.Slice(b.buffer.ms, func(i, j int) bool {
		if cause != nil {
			return false
		}
		var order int
		order, cause = b.storage.monomials.GradedLex(b.buffer.ms[i], b.buffer.ms[j])
		return cause == nil && order > 0
	})
	return cause
}

// Source mpz has no destructor. Retain actual abandoned numeric fields for
// explicit owning cleanup, without emulating a raw container destructor.
func (b *PolynomialSOMBuffer) finishSortSource() error {
	if !b.sortActive {
		return nil
	}
	b.retiredNumerals = append(b.retiredNumerals, b.sortNumerals.retired...)
	for _, a := range b.sortNumerals.values {
		if a.NativeCellToken() != 0 {
			b.retiredNumerals = append(b.retiredNumerals, a)
		}
	}
	b.retiredNumeralCount = uint32(len(b.retiredNumerals))
	b.sortNumerals = polynomialNumerals{}
	b.sortActive = false
	return nil
}
func (b *PolynomialSOMBuffer) SortGradedLex() (result error) {
	if e := b.ready(); e != nil {
		return e
	}
	if b.buffer.owner == nil {
		return ErrPolynomialOwner
	}
	if e := b.sortMonomials(); e != nil {
		return e
	}
	// The default local is constructed after the sort, at the actual lexical
	// site. A previous stopped local is not silently repaired or reused.
	if b.sortActive {
		return ErrPolynomialPending
	}
	b.sortActive = true
	b.sortOriginalCause = nil
	b.sortNumerals = polynomialNumerals{}
	defer func() {
		b.sortOriginalCause = result
		e := b.finishSortSource()
		if result == nil {
			result = e
		}
	}()
	n, e := b.buffer.ms.size(b.storage)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		term, e := b.buffer.ms.monomial(b.storage, i)
		if e != nil {
			return e
		}
		pos, e := b.buffer.positions.Get(term)
		if e != nil {
			return e
		}
		if e = b.sortNumerals.appendCoefficient(b.storage, unumeral.Int{}); e != nil {
			return e
		}
		old, e := b.buffer.as.coefficient(b.storage, pos)
		if e != nil {
			return e
		}
		local, e := b.sortNumerals.coefficient(b.storage, i)
		if e != nil {
			return e
		}
		if e = b.storage.modular.Swap(&local, &old); e != nil {
			return e
		}
		if e = b.sortNumerals.putCoefficient(b.storage, i, local); e != nil {
			return e
		}
		if e = b.buffer.as.putCoefficient(b.storage, pos, old); e != nil {
			return e
		}
		if e = b.buffer.positions.Reset(term); e != nil {
			return e
		}
		if e = b.buffer.positions.Set(term, i); e != nil {
			return e
		}
	}
	b.buffer.as, b.sortNumerals = b.sortNumerals, b.buffer.as
	return nil
}

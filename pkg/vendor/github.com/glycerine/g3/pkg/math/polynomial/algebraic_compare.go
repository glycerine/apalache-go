package polynomial

// Source: algebraic_numbers.cpp comparison, including Sturm–Tarski fallback.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *AlgebraicRootCore) comparisonCleanup() error {
	if m.compareSequence == nil {
		return nil
	}
	if e := m.compareSequence.Close(); e != nil {
		return e
	}
	m.compareSequence = nil
	return nil
}

func (m *AlgebraicRootCore) compareBasic(a, b *unumeral.Rational) (int, error) {
	same, e := m.config.Rationals.Equal(a, b)
	if e != nil {
		return 0, e
	}
	if same {
		return 0, nil
	}
	order, e := m.config.Rationals.Compare(a, b)
	if e != nil {
		return 0, e
	}
	if order < 0 {
		return -1, nil
	}
	return 1, nil
}
func (m *AlgebraicRootCore) compareRootRational(r *algebraicRecord, b *unumeral.Rational) (int, error) {
	bm, qm, p := m.config.Binary, m.config.Rationals, m.config.Polynomials
	below, e := bm.LessEqualRational(&(*r.interval.BorrowUpper()), qm, b)
	if e != nil {
		return 0, e
	}
	if below {
		return -1, nil
	}
	above, e := bm.GreaterEqualRational(&(*r.interval.BorrowLower()), qm, b)
	if e != nil {
		return 0, e
	}
	if above {
		return 1, nil
	}
	view, e := m.sourcePolynomial(r)
	if e != nil {
		return 0, e
	}
	sign, e := p.EvalSignRational(view, b)
	if e != nil {
		return 0, e
	}
	if sign == 0 {
		return 0, nil
	}
	lowerSign := 1
	if r.flags&2 != 0 {
		lowerSign = -1
	}
	if sign == lowerSign {
		return 1, nil
	}
	return -1, nil
}

// CompareRational uses source basic equality before less-than, or the actual
// isolating-interval tests followed by the polynomial's sign at the rational.
func (m *AlgebraicRootCore) CompareRational(a *Algebraic, b *unumeral.Rational) (int, error) {
	l, e := m.location(a)
	if e != nil {
		return 0, e
	}
	if l == nil {
		zero := unumeral.NewRational()
		return m.compareBasic(&zero, b)
	}
	r, e := m.load(l)
	if e != nil {
		return 0, e
	}
	if l.kind == 0 {
		return m.compareBasic(&r.value, b)
	}
	return m.compareRootRational(&r, b)
}

func (m *AlgebraicRootCore) Compare(a, b *Algebraic) (int, error) {
	if e := m.traceComparison(a, b); e != nil {
		return 0, e
	}
	al, e := m.location(a)
	if e != nil {
		return 0, e
	}
	bl, e := m.location(b)
	if e != nil {
		return 0, e
	}
	// Root/root dispatch reaches compare_core's assertion gates before any
	// algebraic-cell member reads.
	if al != nil && al.kind == 1 && bl != nil && bl.kind == 1 {
		return m.compareCore(a, b)
	}
	ar, br := algebraicRecord{value: unumeral.NewRational()}, algebraicRecord{value: unumeral.NewRational()}
	if al != nil {
		ar, e = m.load(al)
		if e != nil {
			return 0, e
		}
	}
	if bl != nil {
		br, e = m.load(bl)
		if e != nil {
			return 0, e
		}
	}
	if al == nil || al.kind == 0 {
		if bl == nil || bl.kind == 0 {
			return m.compareBasic(&ar.value, &br.value)
		}
		v, e := m.compareRootRational(&br, &ar.value)
		return -v, e
	}
	if bl == nil || bl.kind == 0 {
		return m.compareRootRational(&ar, &br.value)
	}
	return m.compareCore(a, b)
}

// Source interval references see mutations made by refine. Always reload both
// actual encoded cells here; cached scalar copies would retain old bounds.
func (m *AlgebraicRootCore) comparisonRecords(a, b *Algebraic) (algebraicRecord, algebraicRecord, error) {
	var ar, br algebraicRecord
	al, e := m.location(a)
	if e != nil {
		return ar, br, e
	}
	bl, e := m.location(b)
	if e != nil {
		return ar, br, e
	}
	if al == nil || bl == nil || al.kind != 1 || bl.kind != 1 {
		return ar, br, ErrAlgebraicOwner
	}
	ar, e = m.load(al)
	if e != nil {
		return ar, br, e
	}
	br, e = m.load(bl)
	return ar, br, e
}
func (m *AlgebraicRootCore) comparisonIntervals(a, b *Algebraic) (int, bool, error) {
	ar, br, e := m.comparisonRecords(a, b)
	if e != nil {
		return 0, false, e
	}
	left, e := m.config.Binary.LessEqual(&(*ar.interval.BorrowUpper()), &(*br.interval.BorrowLower()))
	if e != nil {
		return 0, false, e
	}
	if left {
		m.compareCheap++
		return -1, true, nil
	}
	right, e := m.config.Binary.GreaterEqual(&(*ar.interval.BorrowLower()), &(*br.interval.BorrowUpper()))
	if e != nil {
		return 0, false, e
	}
	if right {
		m.compareCheap++
		return 1, true, nil
	}
	return 0, false, nil
}
func (m *AlgebraicRootCore) compareCore(a, b *Algebraic) (_value int, _err error) {
	if e := m.assertComparisonPair(a, b); e != nil {
		return 0, e
	}
	// Each established to_algebraic access reaches its own source switch.
	if e := algebraicQueryTagAssertionSwitch(); e != nil {
		return 0, e
	}
	if e := algebraicQueryTagAssertionSwitch(); e != nil {
		return 0, e
	}

	if v, done, e := m.comparisonIntervals(a, b); e != nil || done {
		return v, e
	}
	ar, br, e := m.comparisonRecords(a, b)
	if e != nil {
		return 0, e
	}
	p := m.config.Polynomials
	av, e := m.sourcePolynomial(&ar)
	if e != nil {
		return 0, e
	}
	bv, e := m.sourcePolynomial(&br)
	if e != nil {
		return 0, e
	}
	same, e := p.EqualViews(av, bv)
	if e != nil {
		return 0, e
	}
	if same {
		m.comparePoly++
		return 0, nil
	}
	if e = m.traceComparisonRoots(&ar, &br); e != nil {
		return 0, e
	}
	if ar.flags&1 != 0 && br.flags&1 != 0 {
		for {
			if e = m.checkpoint(); e != nil {
				return 0, e
			}
			if _, e = m.refine(a); e != nil {
				return 0, e
			}
			if _, e = m.refine(b); e != nil {
				return 0, e
			}
			m.compareRefine++
			if e = m.assertComparisonNonbasic(a, 1761, "!a.is_basic()"); e != nil {
				return 0, e
			}
			if e = m.assertComparisonNonbasic(b, 1762, "!b.is_basic()"); e != nil {
				return 0, e
			}
			if v, done, e := m.comparisonIntervals(a, b); e != nil || done {
				return v, e
			}
		}
	}
	am, e := m.magnitude(&(*ar.interval.BorrowLower()), &(*ar.interval.BorrowUpper()))
	if e != nil {
		return 0, e
	}
	bm, e := m.magnitude(&(*br.interval.BorrowLower()), &(*br.interval.BorrowUpper()))
	if e != nil {
		return 0, e
	}
	target := am
	if bm < target {
		target = bm
	}
	if m.minMagnitude > target {
		target = m.minMagnitude
	}
	if bm > target {
		refined, e := m.refineN(b, uint32(bm-target))
		if e != nil {
			return 0, e
		}
		if !refined {
			return m.Compare(a, b)
		}
		m.compareRefine += uint32(bm - target)
		if v, done, e := m.comparisonIntervals(a, b); e != nil || done {
			return v, e
		}
	}
	if am > target {
		refined, e := m.refineN(a, uint32(am-target))
		if e != nil {
			return 0, e
		}
		if !refined {
			return m.Compare(a, b)
		}
		m.compareRefine += uint32(am - target)
		if v, done, e := m.comparisonIntervals(a, b); e != nil || done {
			return v, e
		}
	}
	if target > m.minMagnitude {
		for i := int32(0); i < target-m.minMagnitude; i++ {
			refined, e := m.refine(a)
			if e != nil {
				return 0, e
			}
			if !refined {
				return m.Compare(a, b)
			}
			refined, e = m.refine(b)
			if e != nil {
				return 0, e
			}
			if !refined {
				return m.Compare(a, b)
			}
			m.compareRefine++
			if v, done, e := m.comparisonIntervals(a, b); e != nil || done {
				return v, e
			}
		}
	}
	m.compareSturm++
	if m.compareSequence != nil {
		return 0, ErrAlgebraicPending
	}
	seq, e := p.NewSequence()
	if e != nil {
		return 0, e
	}
	m.compareSequence = seq
	defer func() {
		if cleanup := m.comparisonCleanup(); cleanup != nil {
			_value = 0
			_err = errors.Join(_err, cleanup)
		}
	}()
	ar, br, e = m.comparisonRecords(a, b)
	if e != nil {
		return 0, e
	}
	av, e = m.sourcePolynomial(&ar)
	if e != nil {
		return 0, e
	}
	bv, e = m.sourcePolynomial(&br)
	if e != nil {
		return 0, e
	}
	if e = p.SturmTarskiSequence(av, bv, seq); e != nil {
		return 0, e
	}
	// Re-read actual interval fields at the two actual sign-variation accesses.
	ar, br, e = m.comparisonRecords(a, b)
	if e != nil {
		return 0, e
	}
	lower, e := p.SignVariationsBinary(seq, &(*ar.interval.BorrowLower()))
	if e != nil {
		return 0, e
	}
	ar, br, e = m.comparisonRecords(a, b)
	if e != nil {
		return 0, e
	}
	upper, e := p.SignVariationsBinary(seq, &(*ar.interval.BorrowUpper()))
	if e != nil {
		return 0, e
	}
	v := int32(lower) - int32(upper)
	if e = m.traceComparisonSturm(a, b, v); e != nil {
		return 0, e
	}
	if v == 0 {
		return 0, nil
	}
	negative, e := m.comparisonLowerSign(b)
	if e != nil {
		return 0, e
	}
	if (v < 0) == (negative < 0) {
		return -1, nil
	}
	return 1, nil
}

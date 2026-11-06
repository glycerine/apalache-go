package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2462-2548.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicSelectionScope struct {
	polynomialCaller                         *polynomialAlgebraicEvaluationScope
	sourceStop                               error
	integer                                  unumeral.Int
	binary                                   unumeral.BinaryRational
	rational                                 unumeral.Rational
	integerOwned, binaryOwned, rationalOwned bool
	active, running                          bool
}

func (m *AlgebraicRootCore) cleanupSelection() error {
	s := &m.selection
	if s.running {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	if s.rationalOwned {
		if e := m.config.Rationals.Del(&s.rational); e != nil {
			return e
		}
		s.rationalOwned = false
	}
	if s.binaryOwned {
		if e := m.config.Binary.Del(&s.binary); e != nil {
			return e
		}
		s.binaryOwned = false
	}
	if s.integerOwned {
		if e := m.config.Rationals.Integers().Del(&s.integer); e != nil {
			return e
		}
		s.integerOwned = false
	}
	s.active = false
	return nil
}

func (m *AlgebraicRootCore) finishSelectionSource() (cause error) {
	s := &m.selection
	if s.running {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	if s.sourceStop != nil {
		return s.sourceStop
	}
	if m.localSelection == s && m.localActive && m.localCleanup != nil {
		s.sourceStop = m.localCleanup
		return s.sourceStop
	}
	if m.integerSet.selection == s && m.integerSet.active && m.integerSet.sourceStop != nil {
		s.sourceStop = m.integerSet.sourceStop
		return s.sourceStop
	}
	cause = m.cleanupSelection()
	if cause != nil {
		s.sourceStop = cause
	}
	return cause
}

// selectionValue treats the source null algebraic cell as its basic zero.
func (m *AlgebraicRootCore) selectionValue(a *Algebraic) (r algebraicRecord, basic bool, e error) {
	l, e := m.location(a)
	if e != nil {
		return r, false, e
	}
	if l == nil {
		r.value = m.zero
		return r, true, nil
	}
	r, e = m.load(l)
	return r, l.kind == 0, e
}

func (m *AlgebraicRootCore) assertOrdered(prev, curr *Algebraic, line int32) error {
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	cmp, e := m.Compare(prev, curr)
	if e != nil {
		return e
	}
	if cmp >= 0 {
		return diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", line, "lt(prev, curr)")
	}
	return nil
}

func (m *AlgebraicRootCore) separate(prev, curr *Algebraic) error {
	if e := m.assertOrdered(prev, curr, 2464); e != nil {
		return e
	}
	a, ab, e := m.selectionValue(prev)
	if e != nil {
		return e
	}
	b, bb, e := m.selectionValue(curr)
	if e != nil {
		return e
	}
	if ab && bb {
		return nil
	}
	bm, q := m.config.Binary, m.config.Rationals
	for {
		var overlap bool
		if ab {
			overlap, e = bm.LessEqualRational(b.interval.BorrowLower(), q, &a.value)
		} else if bb {
			overlap, e = bm.GreaterEqualRational(a.interval.BorrowUpper(), q, &b.value)
		} else {
			overlap, e = bm.GreaterEqual(a.interval.BorrowUpper(), b.interval.BorrowLower())
		}
		if e != nil || !overlap {
			return e
		}
		if !ab {
			if _, e = m.refine(prev); e != nil {
				return e
			}
		}
		if !bb {
			if _, e = m.refine(curr); e != nil {
				return e
			}
		}
		// The source breaks as soon as either refined cell becomes basic.
		var nextAB, nextBB bool
		a, nextAB, e = m.selectionValue(prev)
		if e != nil {
			return e
		}
		b, nextBB, e = m.selectionValue(curr)
		if e != nil {
			return e
		}
		if !ab && nextAB || !bb && nextBB {
			return nil
		}
	}
}

// IntLT and IntGT return an integer strictly outside the given value. Root
// cells use their stored binary bounds; basic cells use floor-1 or ceil+1.
func (m *AlgebraicRootCore) IntLT(a, out *Algebraic) error { return m.selectInteger(a, out, false) }
func (m *AlgebraicRootCore) IntGT(a, out *Algebraic) error { return m.selectInteger(a, out, true) }
func (m *AlgebraicRootCore) selectInteger(a, out *Algebraic, upper bool) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	s := &m.selection
	if s.active {
		return ErrAlgebraicPending
	}
	*s = algebraicSelectionScope{polynomialCaller: m.polynomialCaller, active: true, running: true, integerOwned: true}
	defer func() { s.running = false; cause = errors.Join(cause, m.finishSelectionSource()) }()
	r, basic, e := m.selectionValue(a)
	if e != nil {
		return e
	}
	q, b := m.config.Rationals, m.config.Binary
	if basic {
		if upper {
			e = q.CeilToInt(&r.value, &s.integer)
		} else {
			e = q.FloorToInt(&r.value, &s.integer)
		}
		if e != nil {
			return e
		}
		if upper {
			e = q.Integers().Inc(&s.integer)
		} else {
			e = q.Integers().Dec(&s.integer)
		}
	} else if upper {
		e = b.Ceil(r.interval.BorrowUpper(), &s.integer)
	} else {
		e = b.Floor(r.interval.BorrowLower(), &s.integer)
	}
	if e != nil {
		return e
	}
	return m.SetInteger(out, &s.integer)
}

// Select chooses the source small binary rational strictly between prev and
// curr. Bound separation may refine either input; output may alias an input.
func (m *AlgebraicRootCore) Select(prev, curr, out *Algebraic) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	s := &m.selection
	if s.active {
		return ErrAlgebraicPending
	}
	if e := m.traceSelect(prev, curr); e != nil {
		return e
	}
	// Both source assertions and separation precede the scoped binary value.
	if e := m.assertOrdered(prev, curr, 2526); e != nil {
		return e
	}
	if e := m.separate(prev, curr); e != nil {
		return e
	}
	*s = algebraicSelectionScope{polynomialCaller: m.polynomialCaller, active: true, running: true, binaryOwned: true}
	defer func() { s.running = false; cause = errors.Join(cause, m.finishSelectionSource()) }()
	a, ab, e := m.selectionValue(prev)
	if e != nil {
		return e
	}
	b, bb, e := m.selectionValue(curr)
	if e != nil {
		return e
	}
	bm, q := m.config.Binary, m.config.Rationals
	if ab {
		if bb {
			e = bm.SelectSmallRationalBounds(q, &a.value, &b.value, &s.binary)
		} else {
			e = bm.SelectSmallRationalLower(q, &a.value, b.interval.BorrowLower(), &s.binary)
		}
	} else if bb {
		e = bm.SelectSmallRationalUpper(q, a.interval.BorrowUpper(), &b.value, &s.binary)
	} else {
		e = bm.SelectSmallCore(a.interval.BorrowUpper(), b.interval.BorrowLower(), &s.binary)
	}
	if e != nil {
		return e
	}
	s.rational = unumeral.NewRational()
	s.rationalOwned = true
	if e = bm.ToRational(q, &s.binary, &s.rational); e != nil {
		return e
	}
	return m.SetRational(out, &s.rational)
}

package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2658-2674.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// DisplayMathematicaInto emits the source rational or Root[polynomial &, i]
// expression. Root indices share the actual cell cache with the other displays.
func (m *AlgebraicRootCore) DisplayMathematicaInto(out unumeral.RationalSourceStream, a *Algebraic) (cause error) {
	if out == nil {
		return arena.ErrConfig
	}
	l, e := m.location(a)
	if e != nil {
		return e
	}
	s := &m.rootDisplay
	if s.active {
		return ErrAlgebraicPending
	}
	if s.invocation == ^uint64(0) {
		return arena.ErrBounds
	}
	*s = algebraicRootDisplayScope{invocation: s.invocation + 1, active: true, running: true}
	defer func() { s.running = false; cause = errors.Join(cause, m.finishRootDisplay(false)) }()
	q := unumeral.NewRational()
	var r algebraicRecord
	if l != nil {
		r, e = m.load(l)
		if e != nil {
			return e
		}
		q = r.value
	}
	if l == nil || l.kind == 0 {
		integer, e := m.config.Rationals.IsInt(&q)
		if e != nil {
			return e
		}
		z := m.config.Rationals.Integers()
		if e = z.InsertSourceIntegerTracked(out, &q.Num, &s.insertions[0], nil); e != nil {
			return e
		}
		// Retire the completed numerator before the denominator can reuse the
		// shared character frame. An interrupted insertion stays bound instead.
		if s.insertions[0].Active() {
			if e = s.insertions[0].Complete(z.Natural, false); e != nil {
				return e
			}
			if e = s.insertions[0].Complete(z.Natural, true); e != nil {
				return e
			}
		}
		if integer {
			return nil
		}
		if e = out.AppendCString("/"); e != nil {
			return e
		}
		if e = z.InsertSourceIntegerTracked(out, &q.Den, &s.insertions[1], nil); e != nil {
			return e
		}
		if s.insertions[1].Active() {
			if e = s.insertions[1].Complete(z.Natural, false); e != nil {
				return e
			}
			return s.insertions[1].Complete(z.Natural, true)
		}
		return nil
	}
	if e = out.AppendCString("Root["); e != nil {
		return e
	}
	view, e := m.sourcePolynomial(&r)
	if e != nil {
		return e
	}
	if e = m.config.Polynomials.DisplayInto(view, out, "#1", true); e != nil {
		return e
	}
	index, e := m.displayRootIndex(l, view, 2670)
	if e != nil {
		return e
	}
	if e = out.AppendCString(" &, "); e != nil {
		return e
	}
	if e = displayUnsigned(out, index); e != nil {
		return e
	}
	return out.AppendCString("]")
}

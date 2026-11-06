package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2626-2656,2676-2706.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicRootDisplayScope struct {
	invocation      uint64
	display         upolyDisplayInvocationReceipt
	sourceStop      error
	rootID          upolyRootIDInvocationReceipt
	insertions      [2]unumeral.NumeralInsertionReceipt
	negative        unumeral.Int
	coefficients    UPolynomial
	negativeOwned   bool
	active, running bool
}

func (m *AlgebraicRootCore) cleanupRootDisplay() error { return m.finishRootDisplay(true) }
func (m *AlgebraicRootCore) finishRootDisplay(owning bool) error {
	s := &m.rootDisplay
	if s.running {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	p := m.config.Polynomials
	if p.matchesRootID(&s.rootID) && p.rootID.active {
		if !owning {
			if e := errors.Join(p.rootID.sourceStop, p.rootID.childStop); e != nil {
				return e
			}
		} else if e := p.cleanupRootIDWithReceipt(&s.rootID); e != nil {
			return e
		}
	}
	if owning {
		if e := p.cleanupDisplayReceipt(&s.display); e != nil {
			return e
		}
	} else if e := p.displayReceiptSourceCause(&s.display); e != nil {
		return e
	}
	for i := len(s.insertions) - 1; i >= 0; i-- {
		if !s.insertions[i].Active() {
			continue
		}
		if !owning {
			if e := s.insertions[i].SourceCause(); e != nil {
				return e
			}
		}
		if e := s.insertions[i].Complete(m.config.Rationals.Integers().Natural, false); e != nil {
			return e
		}
		if owning {
			if e := s.insertions[i].Complete(m.config.Rationals.Integers().Natural, true); e != nil {
				return e
			}
		}
	}

	if s.coefficients.slot != 0 {
		if e := m.delRootDisplayPolynomial(); e != nil {
			return e
		}
	}
	if s.negativeOwned {
		if e := m.config.Rationals.Integers().Del(&s.negative); e != nil {
			return e
		}
		s.negativeOwned = false
	}
	s.active = p.matchesRootID(&s.rootID) && p.rootID.active || p.displayReceiptPending(&s.display)
	for i := range s.insertions {
		if s.insertions[i].Active() {
			s.active = true
		}
	}
	return nil
}

// DisplayRootInto writes (#-polynomial, index); DisplayRootSMT2Into writes
// (root-obj polynomial index). Each stream call is one reached insertion.
func (m *AlgebraicRootCore) DisplayRootInto(out unumeral.RationalSourceStream, a *Algebraic) error {
	return m.displayRootInto(out, a, false)
}
func (m *AlgebraicRootCore) DisplayRootSMT2Into(out unumeral.RationalSourceStream, a *Algebraic) error {
	return m.displayRootInto(out, a, true)
}
func (m *AlgebraicRootCore) displayRootInto(out unumeral.RationalSourceStream, a *Algebraic, smt bool) (cause error) {
	if out == nil {
		return arena.ErrConfig
	}
	l, e := m.location(a)
	if e != nil {
		return e
	}
	return m.displayRootAtLocation(out, l, smt)
}
func (m *AlgebraicRootCore) displayRootAtLocation(out unumeral.RationalSourceStream, l *algebraicLocation, smt bool) (cause error) {
	return m.displayRootAtLocationWithReceipt(out, l, smt, nil)
}
func (m *AlgebraicRootCore) displayRootAtLocationWithReceipt(out unumeral.RationalSourceStream, l *algebraicLocation, smt bool, receipt *algebraicRootDisplayInvocationReceipt) (cause error) {
	var e error
	if m.rootDisplay.active {
		return ErrAlgebraicPending
	}
	if l == nil {
		if smt {
			return out.AppendCString("(root-obj x 1)")
		}
		return out.AppendCString("(#, 1)")
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	p := m.config.Polynomials
	var view UPolynomialView
	s := &m.rootDisplay
	if s.invocation == ^uint64(0) {
		return arena.ErrBounds
	}
	*s = algebraicRootDisplayScope{invocation: s.invocation + 1, active: true, running: true}
	if receipt != nil {
		*receipt = algebraicRootDisplayInvocationReceipt{m, s.invocation}
	}
	defer func() {
		s.running = false
		cleanup := m.finishRootDisplay(false)
		if cleanup != nil {
			s.sourceStop = cleanup
		}
		cause = errors.Join(cause, cleanup)
	}()
	if l.kind == 0 {
		s.negativeOwned = true
		z := m.config.Rationals.Integers()
		if e = z.Copy(&s.negative, &r.value.Num); e != nil {
			return e
		}
		if e = z.Neg(&s.negative); e != nil {
			return e
		}
		// The two source dup coefficients have actual independent numeral fields.
		// Native UP storage supplies their checked view without container emulation.
		s.coefficients, e = m.newRootDisplayPolynomial()
		if e != nil {
			return e
		}
		if e = p.PushCoefficient(s.coefficients, &s.negative); e != nil {
			return e
		}
		if e = p.PushCoefficient(s.coefficients, &r.value.Den); e != nil {
			return e
		}
		view, e = p.wholeView(s.coefficients)
		if e != nil {
			return e
		}
	} else {
		view, e = m.sourcePolynomial(&r)
		if e != nil {
			return e
		}
	}
	prefix, variable, separator, suffix := "(", "#", ", ", ")"
	if smt {
		prefix, variable, separator = "(root-obj ", "x", " "
	}
	if e = out.AppendCString(prefix); e != nil {
		return e
	}
	if smt {
		e = p.displaySMT2IntoWithReceipt(view, out, variable, &s.display)
	} else {
		e = p.displayIntoWithReceipt(view, out, variable, false, &s.display)
	}
	if e != nil {
		return e
	}
	if l.kind == 0 {
		if smt {
			return out.AppendCString(" 1)")
		}
		return out.AppendCString(", 1)")
	}

	line := int32(2652)
	if smt {
		line = 2702
	}
	index, e := m.displayRootIndex(l, view, line)
	if e != nil {
		return e
	}

	if e = out.AppendCString(separator); e != nil {
		return e
	}
	if e = displayUnsigned(out, index); e != nil {
		return e
	}
	return out.AppendCString(suffix)
}

func (m *AlgebraicRootCore) displayRootIndex(l *algebraicLocation, view UPolynomialView, line int32) (uint32, error) {
	// m_i occupies the upper 29 bits of the source packed cell word.
	// Publish only after get_root_id returns, and before the following output.
	flags, e := m.config.Allocator.ReadUint32(l.raw, 16)
	if e != nil {
		return 0, e
	}
	index := flags >> 3
	if index == 0 {
		index, e = m.displayRootIDAtLower(l, view)
		if e != nil {
			return 0, e
		}
		index = (index + 1) & 0x1fffffff
		flags = flags&7 | index<<3
		if e = m.config.Allocator.WriteUint32(l.raw, 16, flags); e != nil {
			return 0, e
		}
	}

	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return 0, e
	}
	if enabled && index == 0 {
		if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", line, "c->m_i > 0"); e != nil {
			return 0, e
		}
	}
	return index, nil
}

// Source get_root_id builds its scoped Sturm sequence before evaluating the
// borrowed lower endpoint. This concrete encoded-cell entry preserves that
// read frontier without an any-valued numeral or callback adapter.
func (m *AlgebraicRootCore) displayRootIDAtLower(l *algebraicLocation, view UPolynomialView) (index uint32, cause error) {
	q, e := m.config.Polynomials.beginRootID(view, &m.rootDisplay.rootID)
	if e != nil {
		return 0, e
	}
	defer func() { cause = q.finish(cause) }()
	if e = q.sturm(view); e != nil {
		return 0, e
	}
	s0, e := q.owner.sequenceVariations(&q.sequence, 1, nil)
	if e != nil {
		return 0, e
	}
	var words [4]uint32
	for i := range words {
		words[i], e = m.config.Allocator.ReadUint32(l.raw, 24+uint64(i)*4)
		if e != nil {
			return 0, e
		}
	}
	lower := unumeral.BinaryRationalFromBorrowedNumeral(algebraicWordsInt(words[:3]), words[3])
	return q.evaluateLower(&lower, s0)
}

// Basic root rendering owns two source duplicate coefficients. Its accepted
// trace body may construct/destroy this local while public UP mutation remains
// forbidden across header/body/footer callbacks.
func (m *AlgebraicRootCore) rootDisplayPolynomialSourceAllowed() bool {
	p := m.config.Polynomials
	return m.rootTrace.running && p.factorTrace == &m.rootTrace.borrow && !p.factorTrace.callback()
}
func (m *AlgebraicRootCore) newRootDisplayPolynomial() (UPolynomial, error) {
	p := m.config.Polynomials
	if !m.rootDisplay.running {
		return UPolynomial{}, ErrAlgebraicPending
	}
	if m.rootDisplayPolynomialSourceAllowed() {
		return p.newPolynomial()
	}
	return p.NewPolynomial()
}
func (m *AlgebraicRootCore) delRootDisplayPolynomial() error {
	p := m.config.Polynomials
	if m.rootDisplayPolynomialSourceAllowed() {
		return p.delPolynomial(&m.rootDisplay.coefficients)
	}
	return p.DelPolynomial(&m.rootDisplay.coefficients)
}

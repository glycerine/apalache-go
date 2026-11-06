package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2721-2747.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicDecimalWriter struct{ out io.Writer }

func (w algebraicDecimalWriter) AppendInt32(n int32) error {
	return writeAlgebraicIntervalPart(w.out, strconv.FormatInt(int64(n), 10))
}
func (w algebraicDecimalWriter) AppendCString(s string) error {
	return writeAlgebraicIntervalPart(w.out, s)
}
func (w algebraicDecimalWriter) AppendNumeralText(s string) error {
	return writeAlgebraicIntervalPart(w.out, s)
}

// DisplayDecimal refines local copies of a root cell's bounds before selecting
// the source's upper approximation or exact lower root. The stored cell is not
// refined by this display operation. A stopped numeral insertion retains its
// manager's source locals for CleanupDecimalStream or CleanupDisplay. Root
// bound and accepted display/refinement continuations also have core owning
// cleanup through CleanupRetained; stopped source destruction is not replayed.
func (m *AlgebraicRootCore) DisplayDecimal(w io.Writer, a *Algebraic, precision uint32) (err error) {
	return m.displayDecimalWithReceipt(w, a, precision, nil)
}
func (m *AlgebraicRootCore) displayDecimalWithReceipt(w io.Writer, a *Algebraic, precision uint32, receipt *algebraicDecimalDisplayInvocationReceipt) (err error) {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	l, err := m.location(a)
	if err != nil {
		return err
	}
	if m.decimalDisplay.pending(m) {
		return ErrAlgebraicPending
	}
	if m.decimalDisplay.invocation == ^uint64(0) {
		return arena.ErrBounds
	}
	m.decimalDisplay.invocation++
	m.decimalDisplay.basic = unumeral.RationalDecimalInvocationReceipt{}
	m.decimalDisplay.sourceStop = nil
	m.decimalDisplay.displayStop = nil
	if receipt != nil {
		*receipt = algebraicDecimalDisplayInvocationReceipt{m, m.decimalDisplay.invocation}
	}
	m.decimalDisplayRunning = true
	defer func() { m.decimalDisplayRunning = false }()
	if l == nil || l.kind == 0 {
		value, e := m.copySourceBasicValue(m, l)
		if e != nil {
			return e
		}
		if value.cell == nil {
			return m.config.Rationals.DisplayDecimalIntoWithInvocation(algebraicDecimalWriter{w}, value.zero, precision, false, &m.decimalDisplay.basic)
		}
		return m.config.Rationals.DisplayDecimalStoredIntoWithInvocation(algebraicDecimalWriter{w}, value.source.config.Allocator, value.cell.raw, precision, false, &m.decimalDisplay.basic)
	}

	s := &m.decimalDisplay
	s.values = [2]unumeral.BinaryRational{}
	s.remaining = 2
	s.sourceStop = nil
	s.displayStop = nil
	s.display = unumeral.BinaryDisplayInvocationReceipt{}
	// A failed new bound copy must not inherit a retired evaluation stop.
	s.refinement.evaluation = upolyBinaryEvaluationInvocationReceipt{}
	defer func() {
		m.decimalDisplayRunning = false
		cleanup := m.finishDecimalDisplay(false)
		if cleanup != nil {
			s.sourceStop = cleanup
		}
		err = errors.Join(err, cleanup)
	}()
	interval, e := m.copyIntervalAtPrecisionFromCellWithRefinement(l, &s.values[0], &s.values[1], precision, &s.copy, &s.refinement)
	if e != nil {
		return e
	}
	selected := &s.values[0]
	if interval {
		selected = &s.values[1]
	}
	e = m.config.Binary.DisplayDecimalToWithInvocation(w, selected, precision, &s.display)
	s.displayStop = m.config.Binary.DisplayInvocationSourceCause(&s.display)
	return e
}

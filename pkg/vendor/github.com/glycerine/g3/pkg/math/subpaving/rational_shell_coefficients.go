package subpaving

// Source: src/math/subpaving/subpaving.cpp:59-81, concrete context_mpq_wrapper.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// RationalShellCoefficients owns the MPQ wrapper's persistent constant and
// coefficient numerals. The context and its numeral manager are borrowed.
type RationalShellCoefficients struct {
	self         *RationalShellCoefficients
	context      *RationalContext
	numbers      *numeral.RationalManager
	constant     numeral.Rational
	coefficients []numeral.Rational
	closeIndex   int
	closing      bool
	closed       bool
}

func NewRationalShellCoefficients(context *RationalContext) (*RationalShellCoefficients, error) {
	if context == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err := context.open(); err != nil {
		return nil, err
	}
	w := &RationalShellCoefficients{context: context, numbers: context.numbers, constant: numeral.NewRational()}
	w.self = w
	return w, nil
}

func (w *RationalShellCoefficients) open() error {
	if w == nil || w.self != w || w.context == nil || w.numbers == nil || w.context.numbers != w.numbers {
		return arena.ErrInvalidSlot
	}
	if w.closed || w.closing {
		return arena.ErrClosed
	}
	return w.context.open()
}

// MkSum follows the source's reserve, indexed coefficient conversion, constant
// conversion, and context call. Scratch values remain owned across invocations.
func (w *RationalShellCoefficients) MkSum(constant *numeral.Int, coefficients []*numeral.Int, variables []uint32) (uint32, error) {
	if err := w.open(); err != nil {
		return 0, err
	}
	if constant == nil || len(coefficients) != len(variables) {
		return 0, arena.ErrInvalidSlot
	}
	for len(w.coefficients) < len(coefficients) {
		w.coefficients = append(w.coefficients, numeral.NewRational())
	}
	for i, coefficient := range coefficients {
		if coefficient == nil {
			return 0, arena.ErrInvalidSlot
		}
		if err := w.numbers.SetInteger(&w.coefficients[i], coefficient); err != nil {
			return 0, err
		}
	}
	if err := w.numbers.SetInteger(&w.constant, constant); err != nil {
		return 0, err
	}
	return w.context.DefinePolynomial(&w.constant, w.coefficients[:len(coefficients)], variables)
}

// MkInequality is the MPQ wrapper's exact rational forwarding path. The
// context takes its own copy of the borrowed threshold.
func (w *RationalShellCoefficients) MkInequality(variable uint32, value *numeral.Rational, lower, open bool) (*RationalInequality, error) {
	if err := w.open(); err != nil {
		return nil, err
	}
	return w.context.NewInequality(variable, value, lower, open)
}

// Close destroys the coefficient vector before the scalar, matching reverse
// member order. A refused numeral release resumes at the same element.
func (w *RationalShellCoefficients) Close() error {
	if w == nil || w.self != w || w.context == nil || w.numbers == nil {
		return arena.ErrInvalidSlot
	}
	if w.closed {
		return nil
	}
	w.closing = true
	for w.closeIndex < len(w.coefficients) {
		if err := w.numbers.Del(&w.coefficients[w.closeIndex]); err != nil {
			return err
		}
		w.closeIndex++
	}
	if err := w.numbers.Del(&w.constant); err != nil {
		return err
	}
	w.coefficients = nil
	w.closed = true
	return nil
}

package subpaving

// Source: src/math/subpaving/subpaving.cpp:85-136, concrete context_mpf_wrapper
// coefficient conversion. The enclosing context_mpf graph remains separate.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrArbitraryShellInexactInteger error = &sourceFailure{message: "subpaving: integer coefficient cannot be represented exactly as arbitrary float"}

// ArbitraryShellCoefficients owns the source wrapper's constant, coefficient
// numerals and rational comparison scratch. Its managers are borrowed.
type ArbitraryShellCoefficients struct {
	self         *ArbitraryShellCoefficients
	context      *ArbitraryContext
	core         *numeral.ArbitraryFloatManager
	numbers      *numeral.ArbitraryFloatNumeralManager
	rational     *numeral.RationalManager
	constant     numeral.ArbitraryFloat
	coefficients []numeral.ArbitraryFloat
	used         int
	before, back numeral.Rational
	closeStage   uint8
	closeIndex   int
	ownsContext  bool
	closed       bool
}

// NewArbitraryShellFromParams constructs the retained context_mpf_wrapper as
// one owner. A refused parameter pass returns its partial shell for Close.
func NewArbitraryShellFromParams(m *numeral.ArbitraryFloatNumeralManager, limit *control.Reslimit, pc *params.ParamsContext, p *params.ParamsRef) (*ArbitraryShellCoefficients, error) {
	c, cause := NewArbitraryContextFromParams(m, limit, pc, p)
	if c == nil {
		return nil, cause
	}
	w, err := NewArbitraryShellCoefficients(c)
	if err != nil {
		return nil, errors.Join(cause, err, c.Close())
	}
	w.ownsContext = true
	return w, cause
}

func NewArbitraryShellCoefficients(context *ArbitraryContext) (*ArbitraryShellCoefficients, error) {
	if context == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err := context.open(); err != nil {
		return nil, err
	}
	numbers := context.numbers
	core, err := numbers.BorrowCore()
	if err != nil {
		return nil, err
	}
	q, err := core.BorrowRationalManager()
	if err != nil {
		return nil, err
	}
	w := &ArbitraryShellCoefficients{context: context, core: core, numbers: numbers, rational: q}
	w.self = w
	return w, nil
}

func (w *ArbitraryShellCoefficients) open() error {
	if w == nil || w.self != w || w.context == nil || w.core == nil || w.numbers == nil || w.rational == nil {
		return arena.ErrInvalidSlot
	}
	if w.closed {
		return arena.ErrClosed
	}
	if err := w.context.open(); err != nil {
		return err
	}
	if w.context.numbers != w.numbers {
		return arena.ErrInvalidSlot
	}
	actual, err := w.numbers.BorrowCore()
	if err != nil {
		return err
	}
	if actual != w.core {
		return arena.ErrInvalidSlot
	}
	q, err := w.core.BorrowRationalManager()
	if err != nil {
		return err
	}
	if q != w.rational {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (w *ArbitraryShellCoefficients) MkVar(integer bool) (uint32, error) {
	if err := w.open(); err != nil {
		return 0, err
	}
	return w.context.NewVariable(integer)
}

func (w *ArbitraryShellCoefficients) MkMonomial(powers []ArbitraryPower) (uint32, error) {
	if err := w.open(); err != nil {
		return 0, err
	}
	return w.context.DefineMonomial(powers)
}

func (w *ArbitraryShellCoefficients) AddClause(atoms []*ArbitraryInequality) (*ArbitraryClause, error) {
	if err := w.open(); err != nil {
		return nil, err
	}
	return w.context.AddClause(atoms)
}

func (w *ArbitraryShellCoefficients) SetVariableDisplay(display ArbitraryVariableDisplay) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.SetVariableDisplay(display)
}

func (w *ArbitraryShellCoefficients) DisplayBounds(out io.Writer) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DisplayBounds(out)
}

func (w *ArbitraryShellCoefficients) DisplayConstraints(out io.Writer, useStar bool) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DisplayConstraints(out, useStar)
}

// BindCooperation forwards the borrowed process owner to search checkpoints.
func (w *ArbitraryShellCoefficients) BindCooperation(owner *cooperate.Cooperation) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.BindCooperation(owner)
}

func (w *ArbitraryShellCoefficients) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.UpdateParamsRef(pc, p)
}

func (w *ArbitraryShellCoefficients) CollectParamDescrs(d *params.ParamDescrs) error {
	if err := w.open(); err != nil {
		return err
	}
	return CollectArbitraryParamDescrs(d)
}

func (w *ArbitraryShellCoefficients) Search() error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.Search()
}

func (w *ArbitraryShellCoefficients) ResetStatistics() error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.ResetStatistics()
}

func (w *ArbitraryShellCoefficients) CollectStatistics(st *statistics.Statistics) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.CollectStatistics(st)
}

// MkSum converts exact integer inputs in source order and transfers copies to
// the context's polynomial definition and watcher registry.
func (w *ArbitraryShellCoefficients) MkSum(constant *numeral.Int, coefficients []*numeral.Int, variables []uint32) (uint32, error) {
	if len(coefficients) != len(variables) {
		return 0, arena.ErrInvalidSlot
	}
	c, as, err := w.PrepareSum(constant, coefficients)
	if err != nil {
		return 0, err
	}
	variable, err := w.context.DefinePolynomial(c, as, variables)
	return variable, sourceArbitraryFailure(err)
}

func (w *ArbitraryShellCoefficients) MkInequality(variable uint32, value *numeral.Rational, lower, open bool) (*ArbitraryInequality, error) {
	bound, err := w.PrepareInequality(value, lower)
	if err != nil {
		return nil, err
	}
	atom, err := w.context.NewInequality(variable, bound, lower, open)
	return atom, sourceArbitraryFailure(err)
}

func (w *ArbitraryShellCoefficients) intToArbitrary(value *numeral.Int, out *numeral.ArbitraryFloat) error {
	if err := w.rational.SetInteger(&w.before, value); err != nil {
		return err
	}
	if err := w.numbers.SetRational(out, &w.before); err != nil {
		return sourceArbitraryFailure(err)
	}
	if err := w.core.ToRational(out, w.rational, &w.back); err != nil {
		return err
	}
	exact, err := w.rational.Equal(&w.before, &w.back)
	if err != nil {
		return err
	}
	if !exact {
		return ErrArbitraryShellInexactInteger
	}
	return nil
}

// PrepareSum preserves the source's logical reserve and coefficient-first
// conversion order. The caller borrows the returned values until the next
// mutation or Close; the eventual context_mpf sum owns its own copies.
func (w *ArbitraryShellCoefficients) PrepareSum(constant *numeral.Int, coefficients []*numeral.Int) (*numeral.ArbitraryFloat, []numeral.ArbitraryFloat, error) {
	if err := w.open(); err != nil {
		return nil, nil, err
	}
	if constant == nil {
		return nil, nil, arena.ErrInvalidSlot
	}
	if len(coefficients) > len(w.coefficients) {
		w.coefficients = append(w.coefficients, make([]numeral.ArbitraryFloat, len(coefficients)-len(w.coefficients))...)
	}
	w.used = len(coefficients)
	for i, coefficient := range coefficients {
		if err := w.intToArbitrary(coefficient, &w.coefficients[i]); err != nil {
			return nil, nil, err
		}
	}
	if err := w.intToArbitrary(constant, &w.constant); err != nil {
		return nil, nil, err
	}
	return &w.constant, w.coefficients[:w.used], nil
}

// PrepareInequality chooses directed rounding before converting the bound.
// The returned scalar is borrowed until the next operation or Close.
func (w *ArbitraryShellCoefficients) PrepareInequality(value *numeral.Rational, lower bool) (*numeral.ArbitraryFloat, error) {
	if err := w.open(); err != nil {
		return nil, err
	}
	w.numbers.SetRounding(!lower)
	if err := w.numbers.SetRational(&w.constant, value); err != nil {
		return nil, sourceArbitraryFailure(err)
	}
	return &w.constant, nil
}

// Close follows reverse member order: back, before, coefficient vector,
// constant, then the owning context base when present. A stopped cleanup
// resumes at its failing member.
func (w *ArbitraryShellCoefficients) Close() error {
	if w == nil || w.self != w || w.context == nil || w.core == nil || w.numbers == nil || w.rational == nil {
		return arena.ErrInvalidSlot
	}
	if w.closed {
		return nil
	}
	if w.closeStage == 0 {
		if err := w.rational.Del(&w.back); err != nil {
			return err
		}
		w.closeStage++
	}
	if w.closeStage == 1 {
		if err := w.rational.Del(&w.before); err != nil {
			return err
		}
		w.closeStage++
	}
	if w.closeStage == 2 {
		for w.closeIndex < len(w.coefficients) {
			if err := w.core.Del(&w.coefficients[w.closeIndex]); err != nil {
				return err
			}
			w.closeIndex++
		}
		w.closeStage++
	}
	if w.closeStage == 3 {
		if err := w.core.Del(&w.constant); err != nil {
			return err
		}
		w.closeStage++
	}
	if w.ownsContext {
		if err := w.context.Close(); err != nil {
			return err
		}
	}
	w.coefficients = nil
	w.closed = true
	return nil
}

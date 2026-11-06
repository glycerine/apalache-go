package subpaving

// Source: src/math/subpaving/subpaving.cpp:198-260, concrete
// context_fpoint_wrapper<context_mpfx> coefficient and bound conversion.

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

var ErrFixedShellInexactInteger error = &sourceFailure{message: "subpaving: integer coefficient cannot be represented exactly as fixed point"}

// FixedShellCoefficients owns the source wrapper's scalar, scoped coefficient
// vector and two integer round-trip scratch values. Its context and managers
// are borrowed. The coefficient vector grows but never shrinks on reserve.
type FixedShellCoefficients struct {
	self         *FixedShellCoefficients
	context      *FixedContext
	numbers      *numeral.FixedPointManager
	rational     *numeral.RationalManager
	constant     *numeral.FixedPoint
	coefficients []*numeral.FixedPoint
	z1, z2       numeral.Int
	closeStage   uint8
	closeIndex   int
	ownsContext  bool
	closed       bool
}

// NewFixedShellFromParams constructs the MPFX wrapper with its context base.
// A failed base constructor unwinds before wrapper members are constructed.
func NewFixedShellFromParams(m *numeral.FixedPointManager, limit *control.Reslimit, rational *numeral.RationalManager, pc *params.ParamsContext, p *params.ParamsRef) (*FixedShellCoefficients, error) {
	c, err := NewFixedContextFromParams(m, limit, pc, p)
	if err != nil {
		if c != nil {
			err = errors.Join(err, c.Close())
		}
		return nil, err
	}
	w, err := NewFixedShellCoefficients(c, rational)
	if err != nil {
		return nil, errors.Join(err, c.Close())
	}
	w.ownsContext = true
	return w, nil
}

func NewFixedShellCoefficients(context *FixedContext, rational *numeral.RationalManager) (*FixedShellCoefficients, error) {
	if context == nil || rational == nil || rational.Z == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err := context.open(); err != nil {
		return nil, err
	}
	constant, err := context.numbers.New()
	if err != nil {
		return nil, err
	}
	w := &FixedShellCoefficients{context: context, numbers: context.numbers, rational: rational, constant: constant}
	w.self = w
	return w, nil
}

func (w *FixedShellCoefficients) open() error {
	if w == nil || w.self != w || w.context == nil || w.numbers == nil || w.rational == nil || w.rational.Z == nil || w.constant == nil {
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
	return nil
}

func (w *FixedShellCoefficients) intToFixed(value *numeral.Int, out *numeral.FixedPoint) error {
	if value == nil || out == nil {
		return arena.ErrInvalidSlot
	}
	if err := w.rational.Z.Copy(&w.z1, value); err != nil {
		return err
	}
	if err := w.numbers.SetFromInteger(out, w.rational.Z, &w.z1); err != nil {
		return sourceFixedFailure(err)
	}
	if err := w.numbers.ToInteger(out, w.rational.Z, &w.z2); err != nil {
		return sourceFixedFailure(err)
	}
	exact, err := w.rational.Z.Equal(&w.z1, &w.z2)
	if err != nil {
		return err
	}
	if !exact {
		return ErrFixedShellInexactInteger
	}
	return nil
}

// PrepareSum follows _scoped_numeral_vector::reserve's grow-only logical size,
// converting each coefficient before the constant. Returned values are
// borrowed until the next wrapper mutation or Close.
func (w *FixedShellCoefficients) PrepareSum(constant *numeral.Int, coefficients []*numeral.Int) (*numeral.FixedPoint, []*numeral.FixedPoint, error) {
	if err := w.open(); err != nil {
		return nil, nil, err
	}
	if constant == nil {
		return nil, nil, arena.ErrInvalidSlot
	}
	for len(w.coefficients) < len(coefficients) {
		value, err := w.numbers.New()
		if err != nil {
			return nil, nil, err
		}
		w.coefficients = append(w.coefficients, value)
	}
	for i, coefficient := range coefficients {
		if err := w.intToFixed(coefficient, w.coefficients[i]); err != nil {
			return nil, nil, err
		}
	}
	if err := w.intToFixed(constant, w.constant); err != nil {
		return nil, nil, err
	}
	return w.constant, w.coefficients[:len(coefficients)], nil
}

func (w *FixedShellCoefficients) MkVar(integer bool) (uint32, error) {
	if err := w.open(); err != nil {
		return 0, err
	}
	return w.context.NewVariable(integer)
}

func (w *FixedShellCoefficients) MkMonomial(powers []FixedPower) (uint32, error) {
	if err := w.open(); err != nil {
		return 0, err
	}
	return w.context.DefineMonomial(powers)
}

func (w *FixedShellCoefficients) MkSum(constant *numeral.Int, coefficients []*numeral.Int, variables []uint32) (uint32, error) {
	if len(coefficients) != len(variables) {
		return 0, arena.ErrInvalidSlot
	}
	c, values, err := w.PrepareSum(constant, coefficients)
	if err != nil {
		return 0, err
	}
	terms := make([]FixedPolynomialTerm, len(variables))
	for i, variable := range variables {
		terms[i] = FixedPolynomialTerm{Variable: variable, Coefficient: values[i]}
	}
	x, err := w.context.DefinePolynomial(c, terms)
	return x, sourceFixedFailure(err)
}

func (w *FixedShellCoefficients) PrepareInequality(value *numeral.Rational, lower bool) (*numeral.FixedPoint, error) {
	if err := w.open(); err != nil {
		return nil, err
	}
	if err := w.numbers.SetRounding(!lower); err != nil {
		return nil, sourceFixedFailure(err)
	}
	if err := w.numbers.SetFromRational(w.constant, w.rational, value); err != nil {
		return nil, sourceFixedFailure(err)
	}
	return w.constant, nil
}

func (w *FixedShellCoefficients) MkInequality(variable uint32, value *numeral.Rational, lower, open bool) (*FixedInequality, error) {
	bound, err := w.PrepareInequality(value, lower)
	if err != nil {
		return nil, err
	}
	a, err := w.context.NewInequality(variable, bound, lower, open)
	return a, sourceFixedFailure(err)
}

func (w *FixedShellCoefficients) AddClause(atoms []*FixedInequality) (*FixedClause, error) {
	if err := w.open(); err != nil {
		return nil, err
	}
	return w.context.AddClause(atoms, false, true)
}

func (w *FixedShellCoefficients) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.UpdateParamsRef(pc, p)
}

func (w *FixedShellCoefficients) Search() error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.Search()
}

func (w *FixedShellCoefficients) ResetStatistics() error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.ResetStatistics()
}

func (w *FixedShellCoefficients) CollectStatistics(st *statistics.Statistics) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.CollectStatistics(st)
}

func (w *FixedShellCoefficients) DisplayConstraints(out io.Writer, useStar bool) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DisplayConstraints(out, useStar)
}

func (w *FixedShellCoefficients) DisplayBounds(out io.Writer) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DisplayBounds(out)
}

func (w *FixedShellCoefficients) SetVariableDisplay(display FixedVariableDisplay) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.SetVariableDisplay(display)
}

func (w *FixedShellCoefficients) BindCooperation(owner *cooperate.Cooperation) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.BindCooperation(owner)
}

// Close retires z2, z1, the vector in ascending element order, and the
// scalar, matching reverse source member destruction and resumable cleanup.
func (w *FixedShellCoefficients) Close() error {
	if w == nil || w.self != w || w.rational == nil || w.rational.Z == nil {
		return arena.ErrInvalidSlot
	}
	if w.closed {
		return nil
	}
	if w.closeStage == 0 {
		if err := w.rational.Z.Del(&w.z2); err != nil {
			return err
		}
		w.closeStage++
	}
	if w.closeStage == 1 {
		if err := w.rational.Z.Del(&w.z1); err != nil {
			return err
		}
		w.closeStage++
	}
	if w.closeStage == 2 {
		for w.closeIndex < len(w.coefficients) {
			if err := w.coefficients[w.closeIndex].Close(); err != nil {
				return err
			}
			w.closeIndex++
		}
		w.closeStage++
	}
	if w.closeStage == 3 {
		if err := w.constant.Close(); err != nil {
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

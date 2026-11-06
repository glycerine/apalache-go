package subpaving

// Source: src/math/subpaving/subpaving.cpp:198-250, concrete context_mpff_wrapper.

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

var ErrFastShellInexactInteger error = &sourceFailure{message: "subpaving: integer coefficient cannot be represented exactly as fast float"}

// FastShellCoefficients owns the MPFF wrapper's scalar, coefficient vector,
// and exact integer comparison scratch. It owns the context only when created
// from named parameters; both numeral managers remain borrowed.
type FastShellCoefficients struct {
	self         *FastShellCoefficients
	context      *FastContext
	numbers      *numeral.FastFloatManager
	rational     *numeral.RationalManager
	constant     numeral.FastFloat
	coefficients []numeral.FastFloat
	z1, z2       numeral.Int
	closeStage   uint8
	closeIndex   int
	ownsContext  bool
	closed       bool
}

// NewFastShellFromParams constructs the selected MPFF wrapper and context as
// one owner. A refused parameter pass retains a partial shell for Close.
func NewFastShellFromParams(m *numeral.FastFloatManager, limit *control.Reslimit, rational *numeral.RationalManager, pc *params.ParamsContext, p *params.ParamsRef) (*FastShellCoefficients, error) {
	c, cause := NewFastContextFromParams(m, limit, pc, p)
	if c == nil {
		return nil, cause
	}
	w, err := NewFastShellCoefficients(c, rational)
	if err != nil {
		return nil, errors.Join(cause, err, c.Close())
	}
	w.ownsContext = true
	return w, cause
}

func NewFastShellCoefficients(context *FastContext, rational *numeral.RationalManager) (*FastShellCoefficients, error) {
	if context == nil || rational == nil || rational.Z == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err := context.open(); err != nil {
		return nil, err
	}
	w := &FastShellCoefficients{context: context, numbers: context.numbers, rational: rational}
	w.self = w
	return w, nil
}

func (w *FastShellCoefficients) open() error {
	if w == nil || w.self != w || w.context == nil || w.numbers == nil || w.rational == nil || w.rational.Z == nil {
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

// intToFast retains the source's copy, conversion, integer round trip and
// exact equality order. z1 and z2 remain owned scratch across calls.
func (w *FastShellCoefficients) intToFast(value *numeral.Int, out *numeral.FastFloat) error {
	if err := w.rational.Z.Copy(&w.z1, value); err != nil {
		return err
	}
	if err := w.numbers.SetInteger(out, w.rational.Z, &w.z1); err != nil {
		return sourceFastFailure(err)
	}
	if err := w.numbers.ToInteger(out, w.rational.Z, &w.z2); err != nil {
		return sourceFastFailure(err)
	}
	exact, err := w.rational.Z.Equal(&w.z1, &w.z2)
	if err != nil {
		return err
	}
	if !exact {
		return ErrFastShellInexactInteger
	}
	return nil
}

// PrepareSum converts every coefficient before the constant. The returned
// values are borrowed until the next wrapper mutation or Close.
func (w *FastShellCoefficients) PrepareSum(constant *numeral.Int, coefficients []*numeral.Int) (*numeral.FastFloat, []numeral.FastFloat, error) {
	if err := w.open(); err != nil {
		return nil, nil, err
	}
	if constant == nil {
		return nil, nil, arena.ErrInvalidSlot
	}
	if len(coefficients) > len(w.coefficients) {
		w.coefficients = append(w.coefficients, make([]numeral.FastFloat, len(coefficients)-len(w.coefficients))...)
	}
	for i, coefficient := range coefficients {
		if err := w.intToFast(coefficient, &w.coefficients[i]); err != nil {
			return nil, nil, err
		}
	}
	if err := w.intToFast(constant, &w.constant); err != nil {
		return nil, nil, err
	}
	return &w.constant, w.coefficients[:len(coefficients)], nil
}

func (w *FastShellCoefficients) MkVar(integer bool) (uint32, error) {
	if err := w.open(); err != nil {
		return 0, err
	}
	return w.context.NewVariable(integer)
}

func (w *FastShellCoefficients) MkMonomial(powers []FastPower) (uint32, error) {
	if err := w.open(); err != nil {
		return 0, err
	}
	return w.context.DefineMonomial(powers)
}

func (w *FastShellCoefficients) MkSum(constant *numeral.Int, coefficients []*numeral.Int, variables []uint32) (uint32, error) {
	if len(coefficients) != len(variables) {
		return 0, arena.ErrInvalidSlot
	}
	_, _, err := w.PrepareSum(constant, coefficients)
	if err != nil {
		return 0, err
	}
	terms := make([]FastPolynomialTerm, len(variables))
	for i, variable := range variables {
		terms[i] = FastPolynomialTerm{Variable: variable, Coefficient: &w.coefficients[i]}
	}
	variable, err := w.context.DefinePolynomial(&w.constant, terms)
	return variable, sourceFastFailure(err)
}

func (w *FastShellCoefficients) PrepareInequality(value *numeral.Rational, lower bool) (*numeral.FastFloat, error) {
	if err := w.open(); err != nil {
		return nil, err
	}
	if err := w.numbers.SetRounding(!lower); err != nil {
		return nil, err
	}
	if err := w.numbers.SetRational(&w.constant, w.rational, value); err != nil {
		return nil, sourceFastFailure(err)
	}
	return &w.constant, nil
}

func (w *FastShellCoefficients) MkInequality(variable uint32, value *numeral.Rational, lower, open bool) (*FastInequality, error) {
	bound, err := w.PrepareInequality(value, lower)
	if err != nil {
		return nil, err
	}
	atom, err := w.context.NewInequality(variable, bound, lower, open)
	return atom, sourceFastFailure(err)
}

func (w *FastShellCoefficients) AddClause(atoms []*FastInequality) (*FastClause, error) {
	if err := w.open(); err != nil {
		return nil, err
	}
	return w.context.AddClause(atoms, false, true)
}

func (w *FastShellCoefficients) SetVariableDisplay(display FastVariableDisplay) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.SetVariableDisplay(display)
}

func (w *FastShellCoefficients) DisplayConstraints(out io.Writer, useStar bool) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DisplayConstraints(out, useStar)
}

func (w *FastShellCoefficients) DisplayBounds(out io.Writer) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DisplayBounds(out)
}

func (w *FastShellCoefficients) Search() error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.Search()
}

// BindCooperation passes the borrowed process checkpoint owner to the context.
func (w *FastShellCoefficients) BindCooperation(owner *cooperate.Cooperation) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.BindCooperation(owner)
}

func (w *FastShellCoefficients) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.UpdateParamsRef(pc, p)
}

func (w *FastShellCoefficients) ResetStatistics() error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.ResetStatistics()
}

func (w *FastShellCoefficients) CollectStatistics(st *statistics.Statistics) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.CollectStatistics(st)
}

// Close unwinds the source members in reverse order. A refused release keeps
// its member for a later retry; an owned context closes after wrapper members.
func (w *FastShellCoefficients) Close() error {
	if w == nil || w.self != w || w.numbers == nil || w.rational == nil {
		return arena.ErrInvalidSlot
	}
	if w.closed {
		return nil
	}
	if w.closeStage == 0 {
		if err := w.rational.Z.Del(&w.z2); err != nil {
			return err
		}
		w.closeStage = 1
	}
	if w.closeStage == 1 {
		if err := w.rational.Z.Del(&w.z1); err != nil {
			return err
		}
		w.closeStage = 2
	}
	if w.closeStage == 2 {
		for w.closeIndex < len(w.coefficients) {
			if err := w.numbers.Del(&w.coefficients[w.closeIndex]); err != nil {
				return err
			}
			w.closeIndex++
		}
		w.coefficients = nil
		w.closeStage = 3
	}
	if w.closeStage == 3 {
		if err := w.numbers.Del(&w.constant); err != nil {
			return err
		}
		w.closeStage = 4
	}
	if w.ownsContext {
		if err := w.context.Close(); err != nil {
			return err
		}
	}
	w.closed = true
	return nil
}

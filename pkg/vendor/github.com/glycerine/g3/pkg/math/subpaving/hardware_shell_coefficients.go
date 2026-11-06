package subpaving

// Source: src/math/subpaving/subpaving.cpp:144-206, concrete context_hwf_wrapper.

import (
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrHardwareShellInexactInteger error = &sourceFailure{message: "subpaving: integer coefficient cannot be represented exactly as hardware float"}

// HardwareShellCoefficients owns the scalar coefficient scratch of the
// retained context_hwf_wrapper. The context and rational manager are borrowed.
type HardwareShellCoefficients struct {
	context      *HardwareContext
	rational     *numeral.RationalManager
	constant     numeral.HardwareFloat
	coefficients []numeral.HardwareFloat
}

func NewHardwareShellCoefficients(context *HardwareContext, rational *numeral.RationalManager) (*HardwareShellCoefficients, error) {
	if context == nil || rational == nil || rational.Z == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err := context.open(); err != nil {
		return nil, err
	}
	return &HardwareShellCoefficients{context: context, rational: rational}, nil
}

func (w *HardwareShellCoefficients) open() error {
	if w == nil || w.context == nil || w.rational == nil || w.rational.Z == nil {
		return arena.ErrInvalidSlot
	}
	return w.context.open()
}

// intToHardware follows the source's is_int64, get_int64, double set and
// round-trip check. Exact rational comparison avoids C++'s undefined cast
// when a rounded binary64 value lies beyond the signed int64 range.
func (w *HardwareShellCoefficients) intToHardware(value *numeral.Int, out *numeral.HardwareFloat) error {
	if err := w.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	z := w.rational.Z
	fits, err := z.IsInt64(value)
	if err != nil {
		return err
	}
	if !fits {
		return ErrHardwareShellInexactInteger
	}
	integer, err := z.Int64(value)
	if err != nil {
		return err
	}
	if err := w.context.numbers.SetFloat64(out, float64(integer)); err != nil {
		return err
	}
	exact := new(big.Rat).SetInt64(integer)
	actual := new(big.Rat).SetFloat64(w.context.numbers.ToFloat64(out))
	if actual.Cmp(exact) != 0 {
		return ErrHardwareShellInexactInteger
	}
	return nil
}

// PrepareSum converts coefficients in source order, then the constant. Its
// typed slice is the native replacement for the wrapper's svector<hwf>.
func (w *HardwareShellCoefficients) PrepareSum(constant *numeral.Int, coefficients []*numeral.Int) error {
	if err := w.open(); err != nil {
		return err
	}
	if len(coefficients) > len(w.coefficients) {
		w.coefficients = append(w.coefficients, make([]numeral.HardwareFloat, len(coefficients)-len(w.coefficients))...)
	}
	for i, coefficient := range coefficients {
		if err := w.intToHardware(coefficient, &w.coefficients[i]); err != nil {
			return err
		}
	}
	return w.intToHardware(constant, &w.constant)
}

// PrepareInequality keeps the source's direction-dependent rounding mode
// before converting the borrowed rational into the wrapper's scalar scratch.
// The source catch names f2n<mpf_manager>, so a hardware f2n exception passes
// through unchanged.
func (w *HardwareShellCoefficients) PrepareInequality(value *numeral.Rational, lower bool) error {
	if err := w.open(); err != nil {
		return err
	}
	w.context.numbers.SetRounding(!lower)
	return w.context.numbers.SetRational(&w.constant, w.rational, value)
}

// MkSum completes the source wrapper's coefficient conversion and delegates
// the scalar copies and variable registration to the hardware context.
func (w *HardwareShellCoefficients) MkSum(constant *numeral.Int, coefficients []*numeral.Int, variables []uint32) (uint32, error) {
	if len(coefficients) != len(variables) {
		return 0, arena.ErrInvalidSlot
	}
	if err := w.PrepareSum(constant, coefficients); err != nil {
		return 0, err
	}
	terms := make([]HardwarePolynomialTerm, len(variables))
	for i, variable := range variables {
		terms[i] = HardwarePolynomialTerm{Variable: variable, Coefficient: &w.coefficients[i]}
	}
	return w.context.DefinePolynomial(&w.constant, terms)
}

// MkInequality rounds its rational bound before copying it to the context.
func (w *HardwareShellCoefficients) MkInequality(variable uint32, value *numeral.Rational, lower, open bool) (*HardwareInequality, error) {
	if err := w.PrepareInequality(value, lower); err != nil {
		return nil, err
	}
	return w.context.NewInequality(variable, &w.constant, lower, open)
}

// These concrete wrapper entries are reached by expr2subpaving and the
// subpaving tactic after the rational-to-hardware conversions above.
func (w *HardwareShellCoefficients) MkVar(integer bool) (uint32, error) {
	if err := w.open(); err != nil {
		return 0, err
	}
	return w.context.NewVariable(integer)
}

func (w *HardwareShellCoefficients) MkMonomial(powers []HardwarePower) (uint32, error) {
	if err := w.open(); err != nil {
		return 0, err
	}
	return w.context.DefineMonomial(powers)
}

func (w *HardwareShellCoefficients) AddClause(atoms []*HardwareInequality) error {
	if err := w.open(); err != nil {
		return err
	}
	_, err := w.context.AddClause(atoms, false, true)
	return err
}

func (w *HardwareShellCoefficients) Search() error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.Search()
}

func (w *HardwareShellCoefficients) ResetStatistics() error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.ResetStatistics()
}

func (w *HardwareShellCoefficients) CollectStatistics(st *statistics.Statistics) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.CollectStatistics(st)
}

func (w *HardwareShellCoefficients) SetVariableDisplay(display HardwareVariableDisplay) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.SetVariableDisplay(display)
}

func (w *HardwareShellCoefficients) DisplayConstraints(out io.Writer, useStar bool) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DisplayConstraints(out, useStar)
}

func (w *HardwareShellCoefficients) DisplayBounds(out io.Writer) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DisplayBounds(out)
}

func (w *HardwareShellCoefficients) CollectParamDescrs(d *params.ParamDescrs) error {
	if err := w.open(); err != nil {
		return err
	}
	return CollectHardwareParamDescrs(d)
}

// BindCooperation forwards the borrowed process owner to search checkpoints.
func (w *HardwareShellCoefficients) BindCooperation(owner *cooperate.Cooperation) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.BindCooperation(owner)
}

func (w *HardwareShellCoefficients) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.UpdateParamsRef(pc, p)
}

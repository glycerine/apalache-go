package polynomial

// Source: algebraic_numbers.cpp:2987-3017; mpbq.h::set.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"strconv"
)

// A selected bound reference reads no polynomial or opposite-bound fields.
type algebraicBinaryBoundView struct {
	core  *AlgebraicRootCore
	cell  *algebraicLocation
	upper bool
}

func (v algebraicBinaryBoundView) copyTo(out *unumeral.BinaryRational, receipt *unumeral.IntegerCopyAllocationReceipt) error {
	b := v.core.config.Binary
	if e := b.Check(out); e != nil {
		return e
	}
	offset := uint64(24)
	if v.upper {
		offset = 40
	}
	var words [3]uint32
	for i := range words {
		n, e := v.core.config.Allocator.ReadUint32(v.cell.raw, offset+4*uint64(i))
		if e != nil {
			return e
		}
		words[i] = n
	}
	n := algebraicWordsInt(words[:])
	if e := b.Z.CopyFromOwnerWithReceipt(out.BorrowNumerator(), b.Z, &n, receipt); e != nil {
		return e
	}
	k, e := v.core.config.Allocator.ReadUint32(v.cell.raw, offset+12)
	if e != nil {
		return e
	}
	out.KField = k
	return nil
}

// Source get_interval copies both bounds before passing polynomial arguments
// to refinement; packed root flags are not an operand of this helper.
func (m *AlgebraicRootCore) copyIntervalAtPrecisionFromCell(l *algebraicLocation, lower, upper *unumeral.BinaryRational, precision uint32, receipt *unumeral.IntegerCopyAllocationReceipt) (bool, error) {
	return m.copyIntervalAtPrecisionFromCellWithRefinement(l, lower, upper, precision, receipt, &m.bounds.refinement)
}
func (m *AlgebraicRootCore) copyIntervalAtPrecisionFromCellWithRefinement(l *algebraicLocation, lower, upper *unumeral.BinaryRational, precision uint32, receipt *unumeral.IntegerCopyAllocationReceipt, refinement *algebraicRefinementScope) (bool, error) {
	if _, _, e := algebraicAssertionSwitch(); e != nil {
		return false, e
	}
	if e := algebraicQueryTagAssertionSwitch(); e != nil {
		return false, e
	}
	if e := (algebraicBinaryBoundView{core: m, cell: l}).copyTo(lower, receipt); e != nil {
		return false, e
	}
	if e := (algebraicBinaryBoundView{core: m, cell: l, upper: true}).copyTo(upper, receipt); e != nil {
		return false, e
	}
	r, e := m.copySourcePolynomialFields(l)
	if e != nil {
		return false, e
	}
	v, e := m.sourcePolynomial(&r)
	if e != nil {
		return false, e
	}
	if e := refinement.begin(m); e != nil {
		return false, e
	}
	defer func() { refinement.executing = false }()
	return m.config.Polynomials.refineToPrecisionWithEvaluationReceipt(v, m.config.Binary, lower, upper, precision*4, &refinement.evaluation, refinement)
}

// sourceString follows mpbq.cpp:384-392: numerator formatting completes
// before the exponent is reached. The borrowed fields acquire no ownership.
func (v algebraicBinaryBoundView) sourceString(receipt *unumeral.NumeralInsertionReceipt) (string, error) {
	offset := uint64(24)
	if v.upper {
		offset = 40
	}
	var words [3]uint32
	for i := range words {
		n, e := v.core.config.Allocator.ReadUint32(v.cell.raw, offset+4*uint64(i))
		if e != nil {
			return "", e
		}
		words[i] = n
	}
	n := algebraicWordsInt(words[:])
	text, e := v.core.predicateIntegerString(&n, receipt)
	if e != nil {
		return "", e
	}
	k, e := v.core.config.Allocator.ReadUint32(v.cell.raw, offset+12)
	if e != nil {
		return "", e
	}
	if k == 1 {
		text += "/2"
	} else if k > 1 {
		text += "/2^" + strconv.FormatUint(uint64(k), 10)
	}
	return text, nil
}

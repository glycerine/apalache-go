package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_params.h:22-40.

import (
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Actual inline scalar base. Disabled registration has no active update/getter.
type SMTBitBlasterParams struct{ ExtGates, Quantifiers bool }

func NewSMTBitBlasterParams() SMTBitBlasterParams { return SMTBitBlasterParams{false, false} }
func (p SMTBitBlasterParams) Display(out *utext.NumericTextStream) error {
	if e := out.AppendCString("m_bb_ext_gates="); e != nil {
		return e
	}
	bExtGates := int64(0)
	if p.ExtGates {
		bExtGates = 1
	}
	if e := out.AppendInt64(bExtGates); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	if e := out.AppendCString("m_bb_quantifiers="); e != nil {
		return e
	}
	bQuantifiers := int64(0)
	if p.Quantifiers {
		bQuantifiers = 1
	}
	if e := out.AppendInt64(bQuantifiers); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	if e := out.Flush(); e != nil {
		return e
	}
	return nil
}

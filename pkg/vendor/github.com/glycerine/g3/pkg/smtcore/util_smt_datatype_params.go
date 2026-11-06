package smtcore

// Source: src/smt/params/theory_datatype_params.h:22-35.

import (
	utext "github.com/glycerine/g3/pkg/util/text"
)

// SMTDatatypeParams is the concrete scalar base embedded by source smt_params.
// Its disabled registration block supplies no active validation or parameters.

type SMTDatatypeParams struct {
	LazySplits uint32
}

func NewSMTDatatypeParams() SMTDatatypeParams {
	return SMTDatatypeParams{LazySplits: 1}
}

func (p SMTDatatypeParams) Display(out *utext.NumericTextStream) error {
	if e := out.AppendCString("m_dt_lazy_splits="); e != nil {
		return e
	}
	if e := out.AppendUint32(p.LazySplits); e != nil {
		return e
	}
	if e := out.AppendByte('\n'); e != nil {
		return e
	}
	return out.Flush()
}

package polynomial

import "github.com/glycerine/g3/pkg/util/diagnostic"

// Source: upolynomial.cpp:3113-3117; upolynomial.h:33. The retained profile
// has _EXTERNAL_RELEASE unset. Report after normal factor_core return only.
func (s *UPolynomialFactorizationService) verboseFactorCount(fs *UPolynomialFactors) error {
	enabled, e := diagnostic.ProcessVerboseEnabled(1000)
	if e != nil || !enabled {
		return e
	}
	out, e := diagnostic.BorrowProcessVerboseStream()
	if e != nil {
		return e
	}
	var borrow upolyFactorTraceBorrow
	if e = borrow.bindManagers([3]*UPolynomialManager{s.manager}, s.manager.modular.BorrowTraceTags()); e != nil {
		return e
	}
	defer borrow.unbind()
	if e = out.AppendCString("(polynomial-factorization :distinct-factors "); e != nil {
		return e
	}
	if e = out.AppendUint32(fs.count); e != nil {
		return e
	}
	if e = out.AppendCString(")"); e != nil {
		return e
	}
	if e = out.AppendByte('\n'); e != nil {
		return e
	}
	return out.Flush()
}

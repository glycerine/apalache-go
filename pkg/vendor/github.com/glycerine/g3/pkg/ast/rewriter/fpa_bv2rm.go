package rewriter

// Source: src/ast/rewriter/fpa_rewriter.cpp:663-680.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// fpaBV2RM maps the source's three-bit numeral encoding to an FP rounding
// constant. Values beyond the five named modes take the source default.
func (r *FPARewriter) fpaBV2RM(arg ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, result error) {
	s := r.manager.NumeralService()
	v, err := s.NewValue()
	if err != nil {
		return RewriteFailed, err
	}
	defer func() {
		if e := s.Del(&v); e != nil {
			result = errors.Join(result, e)
			status = RewriteFailed
		}
	}()
	var width uint32
	found, err := r.manager.BitVectorNumeralValue(arg, &v, &width)
	if err != nil || !found {
		return RewriteFailed, err
	}
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(debug)
	if err != nil {
		return RewriteFailed, err
	}
	if enabled {
		ok, e := s.IsUint64(&v)
		if e != nil {
			return RewriteFailed, e
		}
		if !ok {
			if e = diagnostic.DebugAssertionFailure(debug, "src/ast/rewriter/fpa_rewriter.cpp", 668, "bv_val.is_uint64()"); e != nil {
				return RewriteFailed, e
			}
		}
	}
	n, err := s.Uint64(&v)
	if err != nil {
		return RewriteFailed, err
	}
	kind := int32(4) // BV_RM_TO_ZERO, also the default.
	switch n {
	case 0, 1, 2, 3:
		kind = int32(n)
	}
	answer, err := r.manager.MkFamilyApp(r.family, kind, nil, nil)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, answer)
}

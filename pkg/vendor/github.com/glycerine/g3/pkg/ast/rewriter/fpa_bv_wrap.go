package rewriter

// Source: src/ast/rewriter/fpa_rewriter.cpp:815-847.

import "github.com/glycerine/g3/pkg/ast"

// rewriteFPABVWrap recognizes a concrete fp constructor whose three fields
// are extracts of the same BV. Source checks only their total width and base
// identity; it does not require nonoverlapping or ordered extract ranges.
func (r *FPARewriter) rewriteFPABVWrap(value ast.ASTHandle) (ast.ASTHandle, bool, error) {
	m := r.manager
	kind, err := m.GetKind(value)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if kind != ast.ASTApp {
		return ast.ASTHandle{}, false, nil
	}
	family, err := m.GetFamilyID(value)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	op, err := m.GetDeclKind(value)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if family != r.family || op != ast.FPAOpFP {
		return ast.ASTHandle{}, false, nil
	}
	args, err := m.AppArgumentView(value)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if args.Count() != 3 {
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	var bases [3]ast.ASTHandle
	var widths [3]uint32
	for i := 0; i < 3; i++ {
		arg, e := args.At(uint32(i))
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		var low, high uint32
		found, e := m.BitVectorExtractValue(arg, &low, &high, &bases[i])
		if e != nil || !found {
			return ast.ASTHandle{}, false, e
		}
		widths[i] = high - low + 1
	}
	width := widths[0] + widths[1] + widths[2]
	// A valid fp declaration derives its exponent/significand format from these
	// very arguments, so the source cw == ebits + sbits check is guaranteed.
	if bases[0] != bases[1] || bases[1] != bases[2] {
		return ast.ASTHandle{}, false, nil
	}
	baseWidth, err := m.BitVectorSize(bases[0])
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if baseWidth != width {
		return ast.ASTHandle{}, false, nil
	}
	return bases[0], true, nil
}

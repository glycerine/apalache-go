package rewriter

// Source: src/ast/rewriter/fpa_rewriter.cpp:308-370.

import "github.com/glycerine/g3/pkg/ast"

func (r *FPARewriter) fpaUnary(arg ast.ASTHandle, kind int32, out ast.ASTRef) (RewriteStatus, error) {
	nan, err := r.fpaIsNaN(arg)
	if err != nil {
		return RewriteFailed, err
	}
	if nan {
		return RewriteDone, r.manager.SetRef(out, arg)
	}
	if kind == ast.FPAOpNeg {
		positive, err := r.manager.IsFPAPositiveInfinity(arg)
		if err != nil {
			return RewriteFailed, err
		}
		if positive {
			return r.fpaNegInfinity(arg, true, out)
		}
		negative, err := r.manager.IsFPANegativeInfinity(arg)
		if err != nil {
			return RewriteFailed, err
		}
		if negative {
			return r.fpaNegInfinity(arg, false, out)
		}
		isApp, err := r.manager.GetKind(arg)
		if err != nil {
			return RewriteFailed, err
		}
		if isApp == ast.ASTApp {
			family, err := r.manager.GetFamilyID(arg)
			if err != nil {
				return RewriteFailed, err
			}
			if family == r.family {
				operation, err := r.manager.GetDeclKind(arg)
				if err != nil {
					return RewriteFailed, err
				}
				if operation == ast.FPAOpNeg {
					inner, err := r.manager.GetArg(arg, 0)
					if err != nil {
						return RewriteFailed, err
					}
					return RewriteDone, r.manager.SetRef(out, inner)
				}
			}
		}
	}
	numeral, found, err := r.manager.RewriteFPAUnaryNumeral(arg, kind)
	if err != nil || !found {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, numeral)
}

func (r *FPARewriter) fpaNegInfinity(arg ast.ASTHandle, negative bool, out ast.ASTRef) (RewriteStatus, error) {
	sort, err := r.manager.GetSort(arg)
	if err != nil {
		return RewriteFailed, err
	}
	value, err := r.manager.MkFPAInfinityOfSort(sort, negative)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, value)
}

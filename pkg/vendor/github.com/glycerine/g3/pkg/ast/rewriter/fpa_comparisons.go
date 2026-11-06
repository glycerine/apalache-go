package rewriter

// Source: src/ast/rewriter/fpa_rewriter.cpp:476-545.

import "github.com/glycerine/g3/pkg/ast"

func (r *FPARewriter) fpaBoolean(value bool, out ast.ASTRef) (RewriteStatus, error) {
	var h ast.ASTHandle
	var err error
	if value {
		h, err = r.manager.MkTrue()
	} else {
		h, err = r.manager.MkFalse()
	}
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, h)
}

func (r *FPARewriter) fpaIsNaN(h ast.ASTHandle) (bool, error) {
	_, nan, err := r.manager.ClassifyFPANumeral(h, ast.FPAIsNaN)
	return nan, err
}

func (r *FPARewriter) fpaNotEqNaN(h ast.ASTHandle) (ast.ASTHandle, error) {
	sort, err := r.manager.GetSort(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	nan, err := r.manager.MkFPANaNOfSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	eq, err := r.manager.MkEq(h, nan)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.manager.MkNot(eq)
}

func (r *FPARewriter) fpaInfinityLessRewrite(other, infinity ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	eq, err := r.manager.MkEq(other, infinity)
	if err != nil {
		return RewriteFailed, err
	}
	neqInf, err := r.manager.MkNot(eq)
	if err != nil {
		return RewriteFailed, err
	}
	neqNaN, err := r.fpaNotEqNaN(other)
	if err != nil {
		return RewriteFailed, err
	}
	and, err := r.manager.MkAnd(neqInf, neqNaN)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite3, r.manager.SetRef(out, and)
}

func (r *FPARewriter) fpaLess(left, right ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	nan, err := r.fpaIsNaN(left)
	if err != nil {
		return RewriteFailed, err
	}
	if !nan {
		nan, err = r.fpaIsNaN(right)
		if err != nil {
			return RewriteFailed, err
		}
	}
	if nan {
		return r.fpaBoolean(false, out)
	}
	ninf, err := r.manager.IsFPANegativeInfinity(left)
	if err != nil {
		return RewriteFailed, err
	}
	if ninf {
		return r.fpaInfinityLessRewrite(right, left, out)
	}
	ninf, err = r.manager.IsFPANegativeInfinity(right)
	if err != nil {
		return RewriteFailed, err
	}
	if ninf {
		return r.fpaBoolean(false, out)
	}
	pinf, err := r.manager.IsFPAPositiveInfinity(left)
	if err != nil {
		return RewriteFailed, err
	}
	if pinf {
		return r.fpaBoolean(false, out)
	}
	pinf, err = r.manager.IsFPAPositiveInfinity(right)
	if err != nil {
		return RewriteFailed, err
	}
	if pinf {
		return r.fpaInfinityLessRewrite(left, right, out)
	}
	found, less, err := r.manager.CompareFPANumerals(left, right, ast.FPAOpLt)
	if err != nil || !found {
		return RewriteFailed, err
	}
	return r.fpaBoolean(less, out)
}

func (r *FPARewriter) fpaLessEqual(left, right ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	nan, err := r.fpaIsNaN(left)
	if err != nil {
		return RewriteFailed, err
	}
	if !nan {
		nan, err = r.fpaIsNaN(right)
		if err != nil {
			return RewriteFailed, err
		}
	}
	if nan {
		return r.fpaBoolean(false, out)
	}
	found, lessEqual, err := r.manager.CompareFPANumerals(left, right, ast.FPAOpLe)
	if err != nil || !found {
		return RewriteFailed, err
	}
	return r.fpaBoolean(lessEqual, out)
}

func (r *FPARewriter) fpaSwappedComparison(left, right ast.ASTHandle, target int32, out ast.ASTRef) (RewriteStatus, error) {
	app, err := r.manager.MkFamilyApp(r.family, target, nil, []ast.ASTHandle{right, left})
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite1, r.manager.SetRef(out, app)
}

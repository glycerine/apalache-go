package rewriter

// Source: src/ast/rewriter/array_rewriter.cpp:340-388;
// src/ast/rewriter/rewriter_types.h:38-45.

import "github.com/glycerine/g3/pkg/ast"

func (rw *ArrayRewriter) setUnionIntersection(kind int32, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	n := args.Count()
	if n == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	if n == 1 {
		h, e := args.At(0)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, rw.manager.SetRef(out, h)
	}
	status := RewriteFull
	if n-2 < 3 {
		status = RewriteStatus(n - 2)
	}
	var f ast.ASTHandle
	var e error
	if kind == ast.ASTArrayOpSetUnion {
		f, e = rw.manager.MkOrDecl()
	} else {
		f, e = rw.manager.MkAndDecl()
	}
	if e != nil {
		return RewriteFailed, e
	}
	h, e := rw.manager.MkArrayMap(f, args)
	if e != nil {
		return RewriteFailed, e
	}
	return status, rw.manager.SetRef(out, h)
}
func (rw *ArrayRewriter) setComplement(a ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	f, e := rw.manager.MkNotDecl()
	if e != nil {
		return RewriteFailed, e
	}
	return rw.mapCore(f, ast.SliceArguments([]ast.ASTHandle{a}), out)
}
func (rw *ArrayRewriter) setDifference(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	m := rw.manager
	not, e := m.MkNotDecl()
	if e != nil {
		return RewriteFailed, e
	}
	complement, e := m.MkArrayMap(not, ast.SliceArguments([]ast.ASTHandle{b}))
	if e != nil {
		return RewriteFailed, e
	}
	and, e := m.MkAndDecl()
	if e != nil {
		return RewriteFailed, e
	}
	difference, e := m.MkArrayMap(and, ast.SliceArguments([]ast.ASTHandle{a, complement}))
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite2, m.SetRef(out, difference)
}
func (rw *ArrayRewriter) setSubset(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	m := rw.manager
	if _, e := rw.setDifference(a, b, out); e != nil {
		return RewriteFailed, e
	}
	difference, e := m.RefGet(out)
	if e != nil {
		return RewriteFailed, e
	}
	sort, e := m.GetSort(a)
	if e != nil {
		return RewriteFailed, e
	}
	empty, e := m.MkEmptySet(sort)
	if e != nil {
		return RewriteFailed, e
	}
	equality, e := m.MkEq(difference, empty)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite3, m.SetRef(out, equality)
}

// Source: src/ast/rewriter/array_rewriter.h:63-67.
func (rw *ArrayRewriter) MkSetUnion(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if e := rw.open(); e != nil {
		return RewriteFailed, e
	}
	return rw.setUnionIntersection(ast.ASTArrayOpSetUnion, args, out)
}
func (rw *ArrayRewriter) MkSetIntersect(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if e := rw.open(); e != nil {
		return RewriteFailed, e
	}
	return rw.setUnionIntersection(ast.ASTArrayOpSetIntersect, args, out)
}
func (rw *ArrayRewriter) MkSetComplement(a ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if e := rw.open(); e != nil {
		return RewriteFailed, e
	}
	return rw.setComplement(a, out)
}
func (rw *ArrayRewriter) MkSetDifference(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if e := rw.open(); e != nil {
		return RewriteFailed, e
	}
	return rw.setDifference(a, b, out)
}
func (rw *ArrayRewriter) MkSetSubset(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if e := rw.open(); e != nil {
		return RewriteFailed, e
	}
	return rw.setSubset(a, b, out)
}

package bv

// Source: src/ast/simplifiers/bv_slice.cpp process_eq/process_eqs.

import "github.com/glycerine/g3/pkg/ast"

func (f *bvSliceFrame) processEquality(h ast.ASTHandle) error {
	k, e := f.owner.manager.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return e
	}
	d, e := f.owner.manager.GetDecl(h)
	if e != nil {
		return e
	}
	family, e := f.owner.manager.GetFamilyID(d)
	if e != nil {
		return e
	}
	op, e := f.owner.manager.GetDeclKind(d)
	if e != nil || family != f.owner.manager.BasicFamilyID() || op != ast.ASTOpEq {
		return e
	}
	x, e := f.owner.manager.GetArg(h, 0)
	if e != nil {
		return e
	}
	y, e := f.owner.manager.GetArg(h, 1)
	if e != nil {
		return e
	}
	if _, e = f.owner.manager.BitVectorSize(x); e != nil {
		return nil
	}
	xs, e := f.getConcats(x)
	if e != nil {
		return e
	}
	ys, e := f.getConcats(y)
	if e != nil {
		return e
	}
	return f.align(xs, ys)
}

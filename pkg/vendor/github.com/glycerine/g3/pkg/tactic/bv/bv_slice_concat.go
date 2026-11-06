package bv

// Source: src/ast/simplifiers/bv_slice.cpp get_concats/mk_extract.

import "github.com/glycerine/g3/pkg/ast"

func (f *bvSliceFrame) isConcat(h ast.ASTHandle) (bool, error) {
	k, e := f.owner.manager.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	d, e := f.owner.manager.GetDecl(h)
	if e != nil {
		return false, e
	}
	family, e := f.owner.manager.GetFamilyID(d)
	if e != nil {
		return false, e
	}
	op, e := f.owner.manager.GetDeclKind(d)
	if e != nil {
		return false, e
	}
	bv, e := f.owner.manager.RegisterBitVectorPlugin()
	return family == bv && op == 37, e
}
func (f *bvSliceFrame) getConcats(root ast.ASTHandle) ([]ast.ASTHandle, error) {
	out := []ast.ASTHandle{}
	var walk func(ast.ASTHandle) error
	walk = func(h ast.ASTHandle) error {
		yes, e := f.isConcat(h)
		if e != nil {
			return e
		}
		if !yes {
			out = append(out, h)
			return nil
		}
		n, e := f.owner.manager.GetNumArgs(h)
		if e != nil {
			return e
		}
		for i := uint32(0); i < n; i++ {
			a, e := f.owner.manager.GetArg(h, i)
			if e != nil {
				return e
			}
			if e = walk(a); e != nil {
				return e
			}
		}
		return nil
	}
	return out, walk(root)
}
func (f *bvSliceFrame) extract(high, low uint32, h ast.ASTHandle) (ast.ASTHandle, error) {
	var base ast.ASTHandle
	var lo, hi uint32
	for {
		yes, e := f.owner.manager.BitVectorExtractValue(h, &lo, &hi, &base)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !yes {
			break
		}
		low += lo
		high += lo
		h = base
	}
	width, e := f.owner.manager.BitVectorSize(h)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if low == 0 && high+1 == width {
		return h, nil
	}
	return f.owner.manager.MkBitVectorExtract(high, low, h)
}

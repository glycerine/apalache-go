package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (r *bvArrayRewriter) isBVSort(s ast.ASTHandle) bool {
	_, e := r.owner.manager.BitVectorSize(s)
	return e == nil
}
func (r *bvArrayRewriter) isBVArraySort(s ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	f, e := m.GetFamilyID(s)
	if e != nil {
		return false, e
	}
	k, e := m.GetDeclKind(s)
	if e != nil {
		return false, e
	}
	if f != r.owner.arrayFamily || k != 0 {
		return false, nil
	}
	n, e := m.GetSortNumParameters(s)
	if e != nil || n < 2 {
		return false, e
	}
	for i := uint32(0); i+1 < n; i++ {
		v, e := m.ArrayDomain(s, i)
		if e != nil || !r.isBVSort(v) {
			return false, e
		}
	}
	v, e := m.ArrayRange(s)
	return e == nil && r.isBVSort(v), e
}
func (r *bvArrayRewriter) isBVArray(h ast.ASTHandle) (bool, error) {
	s, e := r.owner.manager.GetSort(h)
	if e != nil {
		return false, e
	}
	return r.isBVArraySort(s)
}
func (r *bvArrayRewriter) indexSort(s ast.ASTHandle) (ast.ASTHandle, error) {
	m := r.owner.manager
	n, e := m.GetSortNumParameters(s)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	var width uint32
	for i := uint32(0); i+1 < n; i++ {
		d, e := m.ArrayDomain(s, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		w, e := m.BitVectorSize(d)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		width += w
	}
	return m.BitVectorSort(width)
}
func (r *bvArrayRewriter) packedIndex(args []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	return r.owner.manager.MkBitVectorConcat(args)
}

func (r *bvArrayRewriter) functionFor(h ast.ASTHandle) (ast.ASTHandle, error) {
	m := r.owner.manager
	if f, e := m.AsArrayFunction(h); e == nil {
		return f, nil
	}
	if f, ok := r.arrays[h]; ok {
		return f, nil
	}
	s, e := m.GetSort(h)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	domain, e := r.indexSort(s)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	value, e := m.ArrayRange(s)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	prefix, e := m.Symbols.Intern("f_t")
	if e != nil {
		return ast.ASTHandle{}, e
	}
	f, e := m.MkFreshFuncDecl(prefix, usymbol.Symbol{}, []ast.ASTHandle{domain}, value)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if e = m.IncRef(h); e != nil {
		return ast.ASTHandle{}, e
	}
	if e = m.IncRef(f); e != nil {
		_ = m.DecRef(h)
		return ast.ASTHandle{}, e
	}
	r.arrays[h] = f
	r.keys = append(r.keys, h)
	r.declarations = append(r.declarations, f)
	if constant, yes, x := bvArrayConstant(m, h); x != nil {
		return ast.ASTHandle{}, x
	} else if yes && r.extension != nil {
		array, x := m.MkAsArray(f)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if x = r.extension.Insert(constant, array); x != nil {
			return ast.ASTHandle{}, x
		}
	} else if !yes && r.filter != nil {
		if x = r.filter.Insert(f); x != nil {
			return ast.ASTHandle{}, x
		}
	}
	r.owner.tracePair("for ", h, " new func_decl is ", f, 116)
	return f, nil
}

func bvArrayConstant(m *ast.ASTManager, h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return ast.ASTHandle{}, false, e
	}
	n, e := m.GetNumArgs(h)
	if e != nil || n != 0 {
		return ast.ASTHandle{}, false, e
	}
	f, e := m.GetFamilyID(h)
	if e != nil {
		return ast.ASTHandle{}, false, e
	}
	if f != ast.NullFamilyID {
		return ast.ASTHandle{}, false, nil
	}
	d, e := m.GetDecl(h)
	return d, e == nil, e
}

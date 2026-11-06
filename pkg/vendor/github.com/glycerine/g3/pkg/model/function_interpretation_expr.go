package model

// Source: src/model/func_interp.cpp:108-149,287-326.

import "github.com/glycerine/g3/pkg/ast"

func functionIsOp(m *ast.ASTManager, h ast.ASTHandle, op int32) (bool, error) {
	k, e := m.GetKind(h)
	if e != nil {
		return false, e
	}
	if k != ast.ASTApp {
		return false, nil
	}
	fid, e := m.GetFamilyID(h)
	if e != nil || fid != m.BasicFamilyID() {
		return false, e
	}
	kind, e := m.GetDeclKind(h)
	return kind == op, e
}
func (f *FunctionInterpretation) entryExpression(term ast.ASTHandle, args *[]ast.ASTHandle) (bool, error) {
	clear(*args)
	*args = (*args)[:0]
	yes, e := functionIsOp(f.manager, term, ast.ASTOpIte)
	if e != nil || !yes {
		return false, e
	}
	then, e := f.manager.GetArg(term, 1)
	if e != nil {
		return false, e
	}
	ground, e := f.manager.IsGround(then)
	if e != nil || !ground || f.arity == 0 {
		return false, e
	}
	cond, e := f.manager.GetArg(term, 0)
	if e != nil {
		return false, e
	}
	op := ast.ASTOpEq
	if f.arity > 1 {
		op = ast.ASTOpAnd
	}
	yes, e = functionIsOp(f.manager, cond, op)
	if e != nil || !yes {
		return false, e
	}
	if f.arity > 1 {
		n, e := f.manager.GetNumArgs(cond)
		if e != nil || n != f.arity {
			return false, e
		}
	}
	if uint64(f.arity) > uint64(int(^uint(0)>>1)) {
		return false, ast.ErrASTDomain
	}
	if int(f.arity) > cap(*args) {
		*args = make([]ast.ASTHandle, int(f.arity))
	} else {
		*args = (*args)[:int(f.arity)]
		clear(*args)
	}
	for i := uint32(0); i < f.arity; i++ {
		ci := cond
		if f.arity > 1 {
			ci, e = f.manager.GetArg(cond, i)
			if e != nil {
				return false, e
			}
		}
		yes, e = functionIsOp(f.manager, ci, ast.ASTOpEq)
		if e != nil || !yes {
			return false, e
		}
		a, e := f.manager.GetArg(ci, 0)
		if e != nil {
			return false, e
		}
		b, e := f.manager.GetArg(ci, 1)
		if e != nil {
			return false, e
		}
		found := false
		for _, pair := range [2][2]ast.ASTHandle{{a, b}, {b, a}} {
			kind, e := f.manager.GetKind(pair[0])
			if e != nil {
				return false, e
			}
			if kind != ast.ASTVar {
				continue
			}
			index, e := f.manager.GetVarIndex(pair[0])
			if e != nil {
				return false, e
			}
			if index == i {
				(*args)[i] = pair[1]
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}
func (f *FunctionInterpretation) SetElse(term ast.ASTHandle) error {
	if e := f.open(); e != nil {
		return e
	}
	if term == f.fallback {
		return nil
	}
	if e := f.resetCache(); e != nil {
		return e
	}
	var args []ast.ASTHandle
	for !term.IsNull() {
		yes, e := f.entryExpression(term, &args)
		if e != nil {
			return e
		}
		if !yes {
			break
		}
		if e := f.tracePeeledEntry(term); e != nil {
			return e
		}
		result, e := f.manager.GetArg(term, 1)
		if e != nil {
			return e
		}
		if e = f.InsertEntry(args, result); e != nil {
			return e
		}
		term, e = f.manager.GetArg(term, 2)
		if e != nil {
			return e
		}
	}
	if e := f.manager.IncRef(term); e != nil {
		return e
	}
	f.replacement = term
	consumed, e := f.manager.DecRefConsumed(f.fallback)
	if consumed {
		f.fallback = ast.ASTHandle{}
	}
	if e != nil {
		return e
	}
	if !consumed {
		return ast.ErrASTPending
	}
	f.fallback = term
	f.replacement = ast.ASTHandle{}
	return nil
}

// Interpretation builds source-ordered ITEs. Later entries become outer ITEs;
// the expression is cached with one owning reference until graph mutation.
func (f *FunctionInterpretation) Interpretation() (ast.ASTHandle, error) {
	if e := f.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if !f.cached.IsNull() {
		return f.cached, nil
	}
	r, e := f.interpretationCore()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if !r.IsNull() {
		if e = f.manager.IncRef(r); e != nil {
			return ast.ASTHandle{}, e
		}
		f.cached = r
	}
	return r, nil
}
func (f *FunctionInterpretation) interpretationCore() (ast.ASTHandle, error) {
	if f.fallback.IsNull() {
		return ast.ASTHandle{}, nil
	}
	m := f.manager
	r := f.fallback
	var vars []ast.ASTHandle
	for _, entry := range f.entries {
		if len(vars) == 0 {
			for i := uint32(0); i < f.arity; i++ {
				sort, e := m.GetSort(entry.args[i])
				if e != nil {
					return ast.ASTHandle{}, e
				}
				v, e := m.MkVar(i, sort)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				vars = append(vars, v)
			}
		}
		var eqs []ast.ASTHandle
		next, e := func() (ast.ASTHandle, error) {
			for i := uint32(0); i < f.arity; i++ {
				eq, e := m.MkEq(vars[i], entry.args[i])
				if e != nil {
					return ast.ASTHandle{}, e
				}
				eqs = append(eqs, eq)
			}
			var cond ast.ASTHandle
			var e error
			if f.arity == 1 {
				cond = eqs[0]
			} else {
				cond, e = m.MkAndArgs(eqs)
				if e != nil {
					return ast.ASTHandle{}, e
				}
			}
			return m.MkIte(cond, entry.result, r)
		}()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		r = next
	}
	return r, nil
}

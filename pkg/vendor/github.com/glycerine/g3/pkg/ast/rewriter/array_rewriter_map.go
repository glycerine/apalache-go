package rewriter

// Source: src/ast/rewriter/array_rewriter.cpp:220-323,335-338;
// src/ast/rewriter/array_rewriter.h:44-45,59.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (rw *ArrayRewriter) MkMapCore(f ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if e := rw.open(); e != nil {
		return RewriteFailed, e
	}
	return rw.mapCore(f, args, out)
}
func (rw *ArrayRewriter) MkMap(f ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) error {
	st, e := rw.MkMapCore(f, args, out)
	if e != nil || st != RewriteFailed {
		return e
	}
	h, e := rw.manager.MkArrayMap(f, args)
	if e != nil {
		return e
	}
	return rw.manager.SetRef(out, h)
}
func (rw *ArrayRewriter) mapConst(sort, value ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	m := rw.manager
	p, e := m.NewParameterAST(sort)
	if e != nil {
		return RewriteFailed, e
	}
	defer func() { err = errors.Join(err, m.DelParameter(&p)) }()
	h, e := m.MkFamilyApp(rw.family, ast.ASTArrayOpConst, []ast.ASTParameter{p}, []ast.ASTHandle{value})
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite2, m.SetRef(out, h)
}
func (rw *ArrayRewriter) mapStore(f ast.ASTHandle, arrays, values []ast.ASTHandle, store ast.ASTHandle, indices uint32, out ast.ASTRef) (RewriteStatus, error) {
	m := rw.manager
	var next []ast.ASTHandle
	h, e := m.MkArrayMap(f, ast.SliceArguments(arrays))
	if e != nil {
		return RewriteFailed, e
	}
	next = append(next, h)
	old, e := m.AppArgumentView(store)
	if e != nil {
		return RewriteFailed, e
	}
	if e = rw.appendArgs(&next, old, 1, indices); e != nil {
		return RewriteFailed, e
	}
	h, e = m.MkApp(f, values)
	if e != nil {
		return RewriteFailed, e
	}
	next = append(next, h)
	h, e = m.MkFamilyApp(rw.family, ast.ASTArrayOpStore, nil, next)
	if e != nil {
		return RewriteFailed, e
	}
	return Rewrite2, m.SetRef(out, h)
}
func (rw *ArrayRewriter) mapCore(f ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (status RewriteStatus, err error) {
	m := rw.manager
	n := args.Count()
	if n == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	first, e := args.At(0)
	if e != nil {
		return RewriteFailed, e
	}
	stored, e := rw.isOp(first, ast.ASTArrayOpStore)
	if e != nil {
		return RewriteFailed, e
	}
	constant, e := rw.isOp(first, ast.ASTArrayOpConst)
	if e != nil {
		return RewriteFailed, e
	}
	if n == 1 {
		if stored {
			count, e := m.GetNumArgs(first)
			if e != nil {
				return RewriteFailed, e
			}
			if count < 3 {
				return RewriteFailed, ast.ErrASTDomain
			}
			base, e := m.GetArg(first, 0)
			if e != nil {
				return RewriteFailed, e
			}
			value, e := m.GetArg(first, count-1)
			if e != nil {
				return RewriteFailed, e
			}
			return rw.mapStore(f, []ast.ASTHandle{base}, []ast.ASTHandle{value}, first, count-2, out)
		}
		if constant {
			value, e := m.GetArg(first, 0)
			if e != nil {
				return RewriteFailed, e
			}
			h, e := m.MkApp(f, []ast.ASTHandle{value})
			if e != nil {
				return RewriteFailed, e
			}
			s, e := m.GetSort(first)
			if e != nil {
				return RewriteFailed, e
			}
			return rw.mapConst(s, h, out)
		}
		return RewriteFailed, nil
	}
	if stored {
		count, e := m.GetNumArgs(first)
		if e != nil {
			return RewriteFailed, e
		}
		if count < 3 {
			return RewriteFailed, ast.ErrASTDomain
		}
		indices := count - 2
		for i := uint32(1); i < n; i++ {
			h, e := args.At(i)
			if e != nil {
				return RewriteFailed, e
			}
			isStore, e := rw.isOp(h, ast.ASTArrayOpStore)
			if e != nil {
				return RewriteFailed, e
			}
			if !isStore {
				return RewriteFailed, nil
			}
			for j := uint32(1); j <= indices; j++ {
				a, e := m.GetArg(first, j)
				if e != nil {
					return RewriteFailed, e
				}
				b, e := m.GetArg(h, j)
				if e != nil {
					return RewriteFailed, e
				}
				if a != b {
					return RewriteFailed, nil
				}
			}
		}
		var arrays, values []ast.ASTHandle
		for i := uint32(0); i < n; i++ {
			h, e := args.At(i)
			if e != nil {
				return RewriteFailed, e
			}
			base, e := m.GetArg(h, 0)
			if e != nil {
				return RewriteFailed, e
			}
			arrays = append(arrays, base)
			value, e := m.GetArg(h, indices+1)
			if e != nil {
				return RewriteFailed, e
			}
			values = append(values, value)
		}
		return rw.mapStore(f, arrays, values, first, indices, out)
	}
	if constant {
		for i := uint32(1); i < n; i++ {
			h, e := args.At(i)
			if e != nil {
				return RewriteFailed, e
			}
			ok, e := rw.isOp(h, ast.ASTArrayOpConst)
			if e != nil {
				return RewriteFailed, e
			}
			if !ok {
				return RewriteFailed, nil
			}
		}
		var values []ast.ASTHandle
		for i := uint32(0); i < n; i++ {
			h, e := args.At(i)
			if e != nil {
				return RewriteFailed, e
			}
			v, e := m.GetArg(h, 0)
			if e != nil {
				return RewriteFailed, e
			}
			values = append(values, v)
		}
		value, e := m.MkApp(f, values)
		if e != nil {
			return RewriteFailed, e
		}
		input, e := m.GetSort(first)
		if e != nil {
			return RewriteFailed, e
		}
		var domain []ast.ASTHandle
		arity, e := m.ArrayArity(input)
		if e != nil {
			return RewriteFailed, e
		}
		for i := uint32(0); i < arity; i++ {
			s, e := m.ArrayDomain(input, i)
			if e != nil {
				return RewriteFailed, e
			}
			domain = append(domain, s)
		}
		output, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return RewriteFailed, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&output)) }()
		rangeSort, e := m.GetRange(f)
		if e != nil {
			return RewriteFailed, e
		}
		sort, e := m.MkArraySort(ast.SliceArguments(domain), rangeSort)
		if e != nil {
			return RewriteFailed, e
		}
		if e = m.SetRef(output, sort); e != nil {
			return RewriteFailed, e
		}
		return rw.mapConst(sort, value, out)
	}
	return RewriteFailed, nil
}

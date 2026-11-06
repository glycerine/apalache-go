package rel

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
)

func (x *AAGExporter) collectArguments(app ast.ASTHandle, useNext bool, bindings map[uint32]ast.ASTHandle, terms *[]ast.ASTHandle) error {
	n, e := x.manager.GetNumArgs(app)
	if e != nil {
		return e
	}
	if e = x.ensureArgumentLatches(int(n)); e != nil {
		return e
	}
	latches := x.argCurrent
	if useNext {
		latches = x.argNext
	}
	for i := uint32(0); i < n; i++ {
		arg, e := x.manager.GetArg(app, i)
		if e != nil {
			return e
		}
		kind, e := x.manager.GetKind(arg)
		if e != nil {
			return e
		}
		latch := latches[i]
		if kind == ast.ASTVar {
			index, e := x.manager.GetVarIndex(arg)
			if e != nil {
				return e
			}
			if prior, ok := bindings[index]; ok {
				eq, e := x.manager.MkEq(latch, prior)
				if e != nil {
					return e
				}
				*terms = append(*terms, eq)
			} else {
				bindings[index] = latch
			}
		} else {
			eq, e := x.manager.MkEq(latch, arg)
			if e != nil {
				return e
			}
			*terms = append(*terms, eq)
		}
	}
	return nil
}
func (x *AAGExporter) substitute(root ast.ASTHandle, bindings map[uint32]ast.ASTHandle) (ast.ASTHandle, error) {
	kind, e := x.manager.GetKind(root)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if kind == ast.ASTVar {
		i, e := x.manager.GetVarIndex(root)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		v, ok := bindings[i]
		if !ok {
			return ast.ASTHandle{}, fmt.Errorf("%w: unbound interpreted-tail variable", ErrAAG)
		}
		return v, nil
	}
	if kind != ast.ASTApp {
		return ast.ASTHandle{}, fmt.Errorf("%w: quantified interpreted tail", ErrAAG)
	}
	n, e := x.manager.GetNumArgs(root)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if n == 0 {
		return root, nil
	}
	args := make([]ast.ASTHandle, n)
	changed := false
	for i := uint32(0); i < n; i++ {
		a, e := x.manager.GetArg(root, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args[i], e = x.substitute(a, bindings)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || args[i] != a
	}
	if !changed {
		return root, nil
	}
	d, e := x.manager.GetDecl(root)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return x.manager.MkApp(d, args)
}

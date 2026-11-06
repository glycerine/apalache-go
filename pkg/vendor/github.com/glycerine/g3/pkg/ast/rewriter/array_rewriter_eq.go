package rewriter

// Source: src/ast/rewriter/array_rewriter.cpp:381-417,32-34;
// src/ast/rewriter/array_rewriter.h:50,68.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Concrete owning continuation for the lexical source buffers/references.
// Raw constraint handles are borrowed; their child INCs belong to the manager.
type arrayStoreEquality struct {
	active         bool
	formulas, args []ast.ASTHandle
	left, right    ast.ASTRef
}

func (f *arrayStoreEquality) close(m *ast.ASTManager) error {
	if !f.active {
		return nil
	}
	if f.right != (ast.ASTRef{}) {
		if e := m.DelRef(&f.right); e != nil {
			return e
		}
	}
	if f.left != (ast.ASTRef{}) {
		if e := m.DelRef(&f.left); e != nil {
			return e
		}
	}
	f.args = nil
	f.formulas = nil
	*f = arrayStoreEquality{}
	return nil
}
func (rw *ArrayRewriter) storeBase(h ast.ASTHandle) (ast.ASTHandle, error) {
	for {
		stored, e := rw.isOp(h, ast.ASTArrayOpStore)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !stored {
			return h, nil
		}
		h, e = rw.manager.GetArg(h, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
}
func (rw *ArrayRewriter) MkEqCore(lhs, rhs ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if e := rw.open(); e != nil {
		return RewriteFailed, e
	}
	if !rw.expandStoreEq {
		return RewriteFailed, nil
	}
	left, e := rw.storeBase(lhs)
	if e != nil {
		return RewriteFailed, e
	}
	right, e := rw.storeBase(rhs)
	if e != nil {
		return RewriteFailed, e
	}
	if left != right {
		return RewriteFailed, nil
	}
	m := rw.manager
	f := &rw.equality
	f.active = true
	defer func() { err = errors.Join(err, f.close(m)) }()
	f.left, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return RewriteFailed, e
	}
	f.right, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return RewriteFailed, e
	}
	for _, root := range []ast.ASTHandle{lhs, rhs} {
		current := root
		for {
			stored, e := rw.isOp(current, ast.ASTArrayOpStore)
			if e != nil {
				return RewriteFailed, e
			}
			if !stored {
				break
			}
			old, e := m.AppArgumentView(current)
			if e != nil {
				return RewriteFailed, e
			}
			if old.Count() < 3 {
				return RewriteFailed, ast.ErrASTDomain
			}
			f.args = append(f.args, lhs)
			if e = rw.appendArgs(&f.args, old, 1, old.Count()-2); e != nil {
				return RewriteFailed, e
			}
			// Call the wrapper's actual core/fallback actions directly: the owner is
			// active and a public re-entry must reject it, including accidental copies.
			status, e := rw.selectCore(ast.SliceArguments(f.args), f.left)
			if e != nil {
				return RewriteFailed, e
			}
			if status == RewriteFailed {
				if e = rw.makeFallback(ast.ASTArrayOpSelect, ast.SliceArguments(f.args), f.left); e != nil {
					return RewriteFailed, e
				}
			}
			// Source args[0] = rhs retains the native slice and its capacity.
			f.args[0] = rhs
			status, e = rw.selectCore(ast.SliceArguments(f.args), f.right)
			if e != nil {
				return RewriteFailed, e
			}
			if status == RewriteFailed {
				if e = rw.makeFallback(ast.ASTArrayOpSelect, ast.SliceArguments(f.args), f.right); e != nil {
					return RewriteFailed, e
				}
			}
			a, e := m.RefGet(f.left)
			if e != nil {
				return RewriteFailed, e
			}
			b, e := m.RefGet(f.right)
			if e != nil {
				return RewriteFailed, e
			}
			equality, e := m.MkEq(a, b)
			if e != nil {
				return RewriteFailed, e
			}
			f.formulas = append(f.formulas, equality)
			current, e = old.At(0)
			if e != nil {
				return RewriteFailed, e
			}
			clear(f.args)
			f.args = f.args[:0]
		}
	}
	h, e := m.MkAndArgs(f.formulas)
	if e != nil {
		return RewriteFailed, e
	}
	return RewriteFull, m.SetRef(out, h)
}

// Source static get_param_descrs forwards only the array module collector.
func (rw *ArrayRewriter) GetParamDescrs(d *uparams.ParamDescrs) error {
	return catalogCollectArrayRewriterParams(d)
}

// Source: src/ast/rewriter/array_rewriter.h:44.
func (rw *ArrayRewriter) Manager() (*ast.ASTManager, error) {
	if e := rw.open(); e != nil {
		return nil, e
	}
	return rw.manager, nil
}

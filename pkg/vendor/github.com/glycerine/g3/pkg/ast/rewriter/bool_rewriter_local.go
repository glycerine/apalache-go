package rewriter

// Source: src/ast/rewriter/bool_rewriter.cpp:268-579;
// src/ast/rewriter/bool_rewriter.h:70-84.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (rw *boolRewriter) simpNestedNotOr(app ast.ASTHandle, neg, pos map[ast.ASTHandle]bool, out ast.ASTRef) (simplified bool, err error) {
	m := rw.manager
	var newArgs []ast.ASTHandle
	n, e := m.GetNumArgs(app)
	if e != nil {
		return false, e
	}
	simp := false
	rw.localContextCost += n
	for i := uint32(0); i < n; i++ {
		arg, e := m.GetArg(app, i)
		if e != nil {
			return false, e
		}
		marked := neg[arg]
		if marked {
			h, e := m.MkFalse()
			if e != nil {
				return false, e
			}
			return true, m.SetRef(out, h)
		}
		marked = pos[arg]
		if marked {
			simp = true
			continue
		}
		negative, atom, e := m.IsNot(arg)
		if e != nil {
			return false, e
		}
		if negative {
			marked = neg[atom]
			if marked {
				simp = true
				continue
			}
			marked = pos[atom]
			if marked {
				h, e := m.MkFalse()
				if e != nil {
					return false, e
				}
				return true, m.SetRef(out, h)
			}
		}
		newArgs = append(newArgs, arg)
	}
	if !simp {
		return false, nil
	}
	args := newArgs
	switch len(args) {
	case 0:
		h, e := m.MkTrue()
		if e != nil {
			return false, e
		}
		return true, m.SetRef(out, h)
	case 1:
		return true, rw.mkNot(args[0], out)
	default:
		h, e := m.MkOrArgs(args)
		if e != nil {
			return false, e
		}
		h, e = m.MkNot(h)
		if e != nil {
			return false, e
		}
		return true, m.SetRef(out, h)
	}
}

func (rw *boolRewriter) simpLocalArg(arg ast.ASTHandle, neg, pos map[ast.ASTHandle]bool, modified *bool) (ast.ASTHandle, error) {
	m := rw.manager
	negative, atom, e := m.IsNot(arg)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if !negative {
		atom = arg
	}
	marked := neg[atom]
	if marked {
		*modified = true
		return m.MkBoolVal(!negative)
	}
	marked = pos[atom]
	if marked {
		*modified = true
		return m.MkBoolVal(negative)
	}
	return arg, nil
}

// This source helper deliberately uses raw manager OR/NOT/ITE construction to
// avoid recursively entering local context simplification through mk_ite.
func (rw *boolRewriter) mkNestedITE(c, t, f ast.ASTHandle, out ast.ASTRef) (err error) {
	m := rw.manager
	truth, e := m.MkTrue()
	if e != nil {
		return e
	}
	falsity, e := m.MkFalse()
	if e != nil {
		return e
	}
	if c == truth {
		return m.SetRef(out, t)
	}
	if c == falsity {
		return m.SetRef(out, f)
	}
	if t == f {
		return m.SetRef(out, t)
	}
	boolean, e := rw.isBool(t)
	if e != nil {
		return e
	}
	var temps [2]ast.ASTRef
	size := 0
	newTemp := func() (ast.ASTRef, error) {
		v, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return ast.ASTRef{}, e
		}
		temps[size] = v
		size++
		return v, nil
	}
	defer func() {
		for size > 0 {
			size--
			err = errors.Join(err, m.DelRef(&temps[size]))
		}
	}()
	rawOr := func(a, b ast.ASTHandle) error {
		h, e := m.MkOrArgs([]ast.ASTHandle{a, b})
		if e != nil {
			return e
		}
		return m.SetRef(out, h)
	}
	if boolean {
		if t == truth {
			if f == falsity {
				return m.SetRef(out, c)
			}
			return rawOr(c, f)
		}
		if t == falsity {
			if f == truth {
				return rw.mkNot(c, out)
			}
			tmp, e := newTemp()
			if e != nil {
				return e
			}
			if e = rw.mkNot(f, tmp); e != nil {
				return e
			}
			h, e := m.RefGet(tmp)
			if e != nil {
				return e
			}
			h, e = m.MkOrArgs([]ast.ASTHandle{c, h})
			if e != nil {
				return e
			}
			h, e = m.MkNot(h)
			if e != nil {
				return e
			}
			return m.SetRef(out, h)
		}
		if f == truth {
			tmp, e := newTemp()
			if e != nil {
				return e
			}
			if e = rw.mkNot(c, tmp); e != nil {
				return e
			}
			h, e := m.RefGet(tmp)
			if e != nil {
				return e
			}
			return rawOr(h, t)
		}
		if f == falsity || c == f {
			tmp1, e := newTemp()
			if e != nil {
				return e
			}
			tmp2, e := newTemp()
			if e != nil {
				return e
			}
			if e = rw.mkNot(c, tmp1); e != nil {
				return e
			}
			if e = rw.mkNot(t, tmp2); e != nil {
				return e
			}
			a, e := m.RefGet(tmp1)
			if e != nil {
				return e
			}
			b, e := m.RefGet(tmp2)
			if e != nil {
				return e
			}
			h, e := m.MkOrArgs([]ast.ASTHandle{a, b})
			if e != nil {
				return e
			}
			h, e = m.MkNot(h)
			if e != nil {
				return e
			}
			return m.SetRef(out, h)
		}
		if c == t {
			return rawOr(c, f)
		}
		negative, atom, e := m.IsNot(t)
		if e != nil {
			return e
		}
		if negative && atom == f {
			return rw.mkEq(c, t, out)
		}
		negative, atom, e = m.IsNot(f)
		if e != nil {
			return e
		}
		if negative && atom == t {
			return rw.mkEq(c, t, out)
		}
	}
	h, e := m.MkIte(c, t, f)
	if e != nil {
		return e
	}
	return m.SetRef(out, h)
}

func (rw *boolRewriter) simpNestedEqITE(arg ast.ASTHandle, neg, pos map[ast.ASTHandle]bool, out ast.ASTRef) (bool, error) {
	rw.localContextCost += 3
	negative, atom, e := rw.manager.IsNot(arg)
	if e != nil {
		return false, e
	}
	if negative {
		arg = atom
	}
	eq, e := rw.isOperator(arg, ast.ASTOpIff)
	if e != nil {
		return false, e
	}
	if !eq {
		eq, e = rw.isOperator(arg, ast.ASTOpEq)
		if e != nil {
			return false, e
		}
	}
	modified := false
	if eq {
		a, e := rw.manager.GetArg(arg, 0)
		if e != nil {
			return false, e
		}
		b, e := rw.manager.GetArg(arg, 1)
		if e != nil {
			return false, e
		}
		a, e = rw.simpLocalArg(a, neg, pos, &modified)
		if e != nil {
			return false, e
		}
		b, e = rw.simpLocalArg(b, neg, pos, &modified)
		if e != nil {
			return false, e
		}
		if !modified {
			return false, nil
		}
		if e = rw.mkEq(a, b, out); e != nil {
			return false, e
		}
		if negative {
			h, e := rw.manager.RefGet(out)
			if e != nil {
				return false, e
			}
			if e = rw.mkNot(h, out); e != nil {
				return false, e
			}
		}
		return true, nil
	}
	ite, c, t, f, e := rw.ite(arg)
	if e != nil || !ite {
		return false, e
	}
	c, e = rw.simpLocalArg(c, neg, pos, &modified)
	if e != nil {
		return false, e
	}
	t, e = rw.simpLocalArg(t, neg, pos, &modified)
	if e != nil {
		return false, e
	}
	f, e = rw.simpLocalArg(f, neg, pos, &modified)
	if e != nil {
		return false, e
	}
	if !modified {
		return false, nil
	}
	if e = rw.mkNestedITE(c, t, f, out); e != nil {
		return false, e
	}
	if negative {
		h, e := rw.manager.RefGet(out)
		if e != nil {
			return false, e
		}
		if e = rw.mkNot(h, out); e != nil {
			return false, e
		}
	}
	return true, nil
}

func (rw *boolRewriter) pushLocalArg(arg ast.ASTHandle, v *[]ast.ASTHandle, neg, pos map[ast.ASTHandle]bool) error {
	negative, atom, e := rw.manager.IsNot(arg)
	if e != nil {
		return e
	}
	if negative {
		marked := neg[atom]
		if marked {
			return nil
		}
		neg[atom] = true
	} else {
		marked := pos[arg]
		if marked {
			return nil
		}
		pos[arg] = true
	}
	return appendRewriterRoot(rw.manager, v, arg)
}

func (rw *boolRewriter) localContextSimplify(args []ast.ASTHandle, out ast.ASTRef) (simplified bool, err error) {
	m := rw.manager
	if rw.localActive {
		return false, ast.ErrASTPending
	}
	rw.localActive = true
	defer func() { err = errors.Join(err, rw.closeLocalContext()) }()
	oldArgs, newArgs := &rw.localOld, &rw.localNew
	var e error
	rw.localArg, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return false, e
	}
	newArg := rw.localArg
	neg := make(map[ast.ASTHandle]bool)
	pos := make(map[ast.ASTHandle]bool)
	simp, modified, forward := false, false, true
	fromOwned := false
	n := uint32(len(args))
	truth, e := m.MkTrue()
	if e != nil {
		return false, e
	}
	falsity, e := m.MkFalse()
	if e != nil {
		return false, e
	}
	process := func(i uint32) (done bool, err error) {
		var arg ast.ASTHandle
		if fromOwned {
			arg = (*oldArgs)[i]
		} else {
			arg = args[i]
		}
		negative, atom, e := m.IsNot(arg)
		if e != nil {
			return false, e
		}
		if negative {
			or, e := rw.isOperator(atom, ast.ASTOpOr)
			if e != nil {
				return false, e
			}
			if or {
				changed, e := rw.simpNestedNotOr(atom, neg, pos, newArg)
				if e != nil {
					return false, e
				}
				if changed {
					modified = true
					simp = true
					arg, e = m.RefGet(newArg)
					if e != nil {
						return false, e
					}
				}
			}
		}
		changed, e := rw.simpNestedEqITE(arg, neg, pos, newArg)
		if e != nil {
			return false, e
		}
		if changed {
			modified = true
			simp = true
			arg, e = m.RefGet(newArg)
			if e != nil {
				return false, e
			}
		}
		if arg == falsity {
			return false, nil
		}
		if arg == truth {
			return true, m.SetRef(out, arg)
		}
		or, e := rw.isOperator(arg, ast.ASTOpOr)
		if e != nil {
			return false, e
		}
		if rw.flat && or {
			size, e := m.GetNumArgs(arg)
			if e != nil {
				return false, e
			}
			for j := uint32(0); j < size; j++ {
				h, e := m.GetArg(arg, j)
				if e != nil {
					return false, e
				}
				if e = rw.pushLocalArg(h, newArgs, neg, pos); e != nil {
					return false, e
				}
			}
			return false, nil
		}
		return false, rw.pushLocalArg(arg, newArgs, neg, pos)
	}
	for {
		rw.localContextCost += 2 * n
		if forward {
			for i := uint32(0); i < n; i++ {
				done, e := process(i)
				if e != nil {
					return false, e
				}
				if done {
					return true, nil
				}
			}
			forward = false
		} else {
			for i := n; i > 0; {
				i--
				done, e := process(i)
				if e != nil {
					return false, e
				}
				if done {
					return true, nil
				}
			}
			if !modified {
				if !simp {
					return false, nil
				}
				var h ast.ASTHandle
				remaining := args
				if fromOwned {
					remaining = *oldArgs
				}
				switch len(remaining) {
				case 0:
					h, e = m.MkFalse()
				case 1:
					h = remaining[0]
				default:
					h, e = m.MkOrArgs(remaining)
				}
				if e != nil {
					return false, e
				}
				return true, m.SetRef(out, h)
			}
			for i, j := 0, len(*newArgs)-1; i < j; i, j = i+1, j-1 {
				(*newArgs)[i], (*newArgs)[j] = (*newArgs)[j], (*newArgs)[i]
			}
			modified = false
			forward = true
		}
		clear(pos)
		clear(neg)
		if e = shrinkRewriterRoots(m, oldArgs, 0); e != nil {
			return false, e
		}
		*oldArgs, *newArgs = *newArgs, *oldArgs
		fromOwned = true
		n = uint32(len(*oldArgs))
	}
}

// Keep stopped numerical/AST destruction reachable on the actual rewriter.
func (rw *boolRewriter) closeLocalContext() error {
	m := rw.manager
	if rw.localArg != (ast.ASTRef{}) {
		if e := m.DelRef(&rw.localArg); e != nil {
			return e
		}
	}
	if e := shrinkRewriterRoots(m, &rw.localNew, 0); e != nil {
		return e
	}
	rw.localNew = nil
	if e := shrinkRewriterRoots(m, &rw.localOld, 0); e != nil {
		return e
	}
	rw.localOld = nil
	rw.localActive = false
	return nil
}

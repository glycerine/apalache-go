package core

// Source: src/tactic/core/dom_simplify_tactic.cpp dom_simplifier path.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *DomSimplifyTactic) domRewrite(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := t.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("dom callback ref: %w", err)
	}
	defer t.manager.DelRef(&r)
	if err = t.domSimp.Rewrite(r); err != nil {
		return ast.ASTHandle{}, fmt.Errorf("dom callback rewrite: %w", err)
	}
	h, err = t.manager.RefGet(r)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("dom callback result: %w", err)
	}
	if err = t.pinDom(h); err != nil {
		return ast.ASTHandle{}, fmt.Errorf("dom callback result pin: %w", err)
	}
	return h, nil
}

func (t *DomSimplifyTactic) applyDomSimplifier(g *tactic.Goal, out *tactic.TacticResult) (retErr error) {
	defer func() { retErr = errors.Join(retErr, t.clearDomPins()) }()
	changed := true
	for changed {
		bad, err := g.Inconsistent()
		if err != nil {
			return err
		}
		if bad {
			break
		}
		changed = false
		for _, forward := range []bool{true, false} {
			t.forward = forward
			n, err := g.Size()
			if err != nil {
				return err
			}
			forms := make([]ast.ASTHandle, n)
			for i := range forms {
				forms[i], err = g.Form(uint32(i))
				if err != nil {
					return fmt.Errorf("dom goal form %d: %w", i, err)
				}
				if err = t.pinDom(forms[i]); err != nil {
					return err
				}
			}
			root, err := domMkAndArgs(t.manager, forms)
			if err != nil {
				return fmt.Errorf("dom goal conjunction: %w", err)
			}
			if err = t.pinDom(root); err != nil {
				return err
			}
			compiled, compileErr := t.dominators.compile(root)
			if compileErr != nil {
				return fmt.Errorf("dom compile: %w", compileErr)
			}
			if !compiled {
				t.domCache = nil
				if e := t.clearDomPins(); e != nil {
					return e
				}
				if e := g.IncDepth(); e != nil {
					return e
				}
				return out.AddGoal(g)
			}
			cache := make(map[ast.ASTHandle]ast.ASTHandle)
			t.domCache = cache
			order := make([]uint32, n)
			for i := range order {
				if forward {
					order[i] = uint32(i)
				} else {
					order[i] = n - 1 - uint32(i)
				}
			}
			proofs, e := g.ProofsEnabled()
			if e != nil {
				return e
			}
			for _, i := range order {
				inconsistent, x := g.Inconsistent()
				if x != nil {
					return x
				}
				if inconsistent {
					break
				}
				old := forms[i]
				next, e := t.domRec(old, 0)
				if e != nil {
					return fmt.Errorf("dom recurse goal form %d: %w", i, e)
				}
				pr, e := g.Proof(i)
				if e != nil {
					return e
				}
				dep, e := g.Dependency(i)
				if e != nil {
					return e
				}
				if (forward && i+1 < n || !forward && i > 0) && !t.manager.IsTrue(next) && !t.manager.IsFalse(next) && dep.IsNull() && !proofs {
					ok, x := t.domSimp.AssertExpression(next, false)
					if x != nil {
						return x
					}
					if !ok {
						next, e = t.manager.MkFalse()
						if e != nil {
							return e
						}
					}
				}
				if proofs {
					rw, x := t.manager.MkProofRewrite(old, next)
					if x != nil {
						return x
					}
					pr, x = t.manager.MkProofModusPonens(pr, rw)
					if x != nil {
						return x
					}
				}
				if e = g.Update(i, next, pr, dep); e != nil {
					return fmt.Errorf("dom update goal form %d: %w", i, e)
				}
				changed = changed || next != old
			}
			if e := t.domSimp.Pop(t.domSimp.ScopeLevel()); e != nil {
				return e
			}
			t.domCache = nil
			if e := t.clearDomPins(); e != nil {
				return e
			}
		}
	}
	if err := g.IncDepth(); err != nil {
		return err
	}
	return out.AddGoal(g)
}

func (t *DomSimplifyTactic) pinDom(h ast.ASTHandle) error {
	r, err := t.manager.NewRef(h)
	if err == nil {
		t.domPins = append(t.domPins, r)
	}
	return err
}

func domMkAndArgs(m *ast.ASTManager, args []ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return m.MkTrue()
	case 1:
		return args[0], nil
	default:
		return m.MkAndArgs(args)
	}
}

func domMkOrArgs(m *ast.ASTManager, args []ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return m.MkFalse()
	case 1:
		return args[0], nil
	default:
		return m.MkOrArgs(args)
	}
}

func (t *DomSimplifyTactic) clearDomPins() error {
	var err error
	kept := make([]ast.ASTRef, 0, len(t.domPins))
	for i := len(t.domPins) - 1; i >= 0; i-- {
		err = errors.Join(err, t.manager.DelRef(&t.domPins[i]))
		if t.domPins[i] != (ast.ASTRef{}) {
			kept = append(kept, t.domPins[i])
		}
	}
	t.domPins = kept
	return err
}

func (t *DomSimplifyTactic) cacheDom(k, v ast.ASTHandle) error {
	if err := t.pinDom(k); err != nil {
		return err
	}
	if err := t.pinDom(v); err != nil {
		return err
	}
	t.domCache[k] = v
	return nil
}

func (t *DomSimplifyTactic) domCached(h ast.ASTHandle) ast.ASTHandle {
	if r, ok := t.domCache[h]; ok {
		return r
	}
	return h
}

func (t *DomSimplifyTactic) domArg(h ast.ASTHandle) (ast.ASTHandle, error) {
	return t.domRewrite(t.domCached(h))
}

func (t *DomSimplifyTactic) domRec(h ast.ASTHandle, depth uint32) (ast.ASTHandle, error) {
	original := h
	h = t.domCached(h)
	if depth >= t.maxDepth {
		r, e := t.domRewrite(h)
		if e == nil {
			e = t.cacheDom(original, r)
		}
		return r, e
	}
	k, e := t.manager.GetKind(h)
	if e != nil {
		return h, e
	}
	if k == ast.ASTApp {
		if yes, e := domOp(t.manager, h, ast.ASTOpIte); e != nil {
			return h, e
		} else if yes {
			h, e = t.domITE(h, depth)
			if e != nil {
				return h, e
			}
			goto finish
		}
		if yes, e := domOp(t.manager, h, ast.ASTOpAnd); e != nil {
			return h, e
		} else if yes {
			h, e = t.domAndOr(h, depth, true)
			if e != nil {
				return h, e
			}
			goto finish
		}
		if yes, e := domOp(t.manager, h, ast.ASTOpOr); e != nil {
			return h, e
		} else if yes {
			h, e = t.domAndOr(h, depth, false)
			if e != nil {
				return h, e
			}
			goto finish
		}
		for _, child := range t.dominators.tree[h] {
			if child != h {
				if _, e = t.domRec(child, depth+1); e != nil {
					return h, e
				}
			}
		}
		n, e := t.manager.GetNumArgs(h)
		if e != nil {
			return h, e
		}
		args := make([]ast.ASTHandle, n)
		for i := uint32(0); i < n; i++ {
			a, x := t.manager.GetArg(h, i)
			if x != nil {
				return h, x
			}
			args[i], x = t.domArg(a)
			if x != nil {
				return h, x
			}
		}
		decl, e := t.manager.GetDecl(h)
		if e != nil {
			return h, e
		}
		h, e = t.manager.MkApp(decl, args)
		if e != nil {
			return h, e
		}
	}
finish:
	h, e = t.domRewrite(h)
	if e == nil {
		e = t.cacheDom(original, h)
	}
	return h, e
}

func (t *DomSimplifyTactic) domSubexpr(a, b ast.ASTHandle, seen map[ast.ASTHandle]bool) (bool, error) {
	if a == b {
		return true, nil
	}
	if seen[a] {
		return false, nil
	}
	seen[a] = true
	ad, e := t.manager.GetDepth(a)
	if e != nil {
		return false, e
	}
	bd, e := t.manager.GetDepth(b)
	if e != nil {
		return false, e
	}
	if ad >= bd {
		return false, nil
	}
	p, ok := t.dominators.idom[a]
	if !ok || p == a {
		return false, ast.ErrASTDomain
	}
	return t.domSubexpr(p, b, seen)
}
func (t *DomSimplifyTactic) domExclusive(child, arg ast.ASTHandle, args []ast.ASTHandle) (bool, error) {
	ok, e := t.domSubexpr(child, arg, make(map[ast.ASTHandle]bool))
	if e != nil || !ok {
		return ok, e
	}
	for _, other := range args {
		if other == arg {
			continue
		}
		yes, x := t.domSubexpr(child, other, make(map[ast.ASTHandle]bool))
		if x != nil {
			return false, x
		}
		if yes {
			return false, nil
		}
	}
	return true, nil
}
func (t *DomSimplifyTactic) domAndOr(h ast.ASTHandle, depth uint32, and bool) (ast.ASTHandle, error) {
	n, e := t.manager.GetNumArgs(h)
	if e != nil {
		return h, e
	}
	args := make([]ast.ASTHandle, n)
	orig := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		orig[i], e = t.manager.GetArg(h, i)
		if e != nil {
			return h, e
		}
	}
	old := t.domSimp.ScopeLevel()
	order := make([]uint32, n)
	for i := range order {
		if t.forward {
			order[i] = uint32(i)
		} else {
			order[i] = n - 1 - uint32(i)
		}
	}
	for _, i := range order {
		arg := orig[i]
		for _, child := range t.dominators.tree[arg] {
			yes, x := t.domExclusive(child, arg, orig)
			if x != nil {
				return h, x
			}
			if yes {
				if _, x = t.domRec(child, depth+1); x != nil {
					return h, x
				}
			}
		}
		r, x := t.domArg(arg)
		if x != nil {
			return h, x
		}
		args[i] = r
		ok, x := t.domSimp.AssertExpression(r, !and)
		if x != nil {
			return h, x
		}
		if !ok {
			if and {
				return t.manager.MkFalse()
			}
			return t.manager.MkTrue()
		}
	}
	if e = t.domSimp.Pop(t.domSimp.ScopeLevel() - old); e != nil {
		return h, e
	}
	if and {
		return domMkAndArgs(t.manager, args)
	}
	return domMkOrArgs(t.manager, args)
}
func (t *DomSimplifyTactic) domITE(h ast.ASTHandle, depth uint32) (ast.ASTHandle, error) {
	c, e := t.manager.GetArg(h, 0)
	if e != nil {
		return h, e
	}
	th, e := t.manager.GetArg(h, 1)
	if e != nil {
		return h, e
	}
	el, e := t.manager.GetArg(h, 2)
	if e != nil {
		return h, e
	}
	nc, e := t.domArg(c)
	if e != nil {
		return h, e
	}
	yes, e := t.manager.MkTrue()
	if e != nil {
		return h, e
	}
	no, e := t.manager.MkFalse()
	if e != nil {
		return h, e
	}
	if nc == yes {
		return t.domArg(th)
	}
	old := t.domSimp.ScopeLevel()
	ok, e := t.domSimp.AssertExpression(nc, false)
	if e != nil {
		return h, e
	}
	if nc == no || !ok {
		return t.domArg(el)
	}
	for _, child := range t.dominators.tree[h] {
		a, x := t.domSubexpr(child, th, make(map[ast.ASTHandle]bool))
		if x != nil {
			return h, x
		}
		b, x := t.domSubexpr(child, el, make(map[ast.ASTHandle]bool))
		if x != nil {
			return h, x
		}
		if a && !b {
			if _, x = t.domRec(child, depth+1); x != nil {
				return h, x
			}
		}
	}
	if e = t.domSimp.Pop(t.domSimp.ScopeLevel() - old); e != nil {
		return h, e
	}
	nt, e := t.domArg(th)
	if e != nil {
		return h, e
	}
	ok, e = t.domSimp.AssertExpression(nc, true)
	if e != nil {
		return h, e
	}
	if !ok {
		return nt, nil
	}
	for _, child := range t.dominators.tree[h] {
		a, x := t.domSubexpr(child, el, make(map[ast.ASTHandle]bool))
		if x != nil {
			return h, x
		}
		b, x := t.domSubexpr(child, th, make(map[ast.ASTHandle]bool))
		if x != nil {
			return h, x
		}
		if a && !b {
			if _, x = t.domRec(child, depth+1); x != nil {
				return h, x
			}
		}
	}
	if e = t.domSimp.Pop(t.domSimp.ScopeLevel() - old); e != nil {
		return h, e
	}
	ne, e := t.domArg(el)
	if e != nil {
		return h, e
	}
	if nt == ne {
		return nt, nil
	}
	if nc == c && nt == th && ne == el {
		return h, nil
	}
	return t.manager.MkIte(nc, nt, ne)
}

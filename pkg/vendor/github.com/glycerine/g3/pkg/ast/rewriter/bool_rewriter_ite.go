package rewriter

// Source: src/ast/rewriter/bool_rewriter.cpp:724-938;
// src/ast/rewriter/bool_rewriter.h:159-163.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (rw *boolRewriter) mkDistinctCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	return rw.mkDistinctCoreArguments(astSliceArguments(args), out)
}
func (rw *boolRewriter) mkDistinctCoreArguments(args ast.AppArguments, out ast.ASTRef) (status brStatus, err error) {
	m := rw.manager
	if args.Count() <= 1 {
		h, e := m.MkTrue()
		if e != nil {
			return brFailed, e
		}
		return brDone, m.SetRef(out, h)
	}
	if args.Count() == 2 {
		// The source's unused local expr_ref stays null throughout this branch.
		eq, e := func() (ast.ASTHandle, error) {
			a, e := args.At(0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			b, e := args.At(1)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return m.MkEq(a, b)
		}()
		if e != nil {
			return brFailed, e
		}
		h, e := m.MkNot(eq)
		if e != nil {
			return brFailed, e
		}
		return brRewrite2, m.SetRef(out, h)
	}
	visited := make(map[ast.ASTHandle]bool)
	allValue := true
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return brFailed, e
		}
		if _, e := m.GetKind(arg); e != nil {
			return brFailed, e
		}
		marked := visited[arg]
		if marked {
			h, e := m.MkFalse()
			if e != nil {
				return brFailed, e
			}
			return brDone, m.SetRef(out, h)
		}
		visited[arg] = true
		value, e := m.IsUniqueValue(arg)
		if e != nil {
			return brFailed, e
		}
		if !value {
			allValue = false
		}
	}
	if allValue {
		h, e := m.MkTrue()
		if e != nil {
			return brFailed, e
		}
		return brDone, m.SetRef(out, h)
	}
	first, e := args.At(0)
	if e != nil {
		return brFailed, e
	}
	boolean, e := rw.isBool(first)
	if e != nil {
		return brFailed, e
	}
	if boolean {
		h, e := m.MkFalse()
		if e != nil {
			return brFailed, e
		}
		return brDone, m.SetRef(out, h)
	}
	if rw.blastDistinct && uint64(args.Count()) < uint64(rw.blastDistinctThreshold) {
		var diseqs []ast.ASTHandle
		for i := uint32(0); i < args.Count(); i++ {
			for j := i + 1; j < args.Count(); j++ {
				a, e := args.At(i)
				if e != nil {
					return brFailed, e
				}
				b, e := args.At(j)
				if e != nil {
					return brFailed, e
				}
				eq, e := m.MkEq(a, b)
				if e != nil {
					return brFailed, e
				}
				h, e := m.MkNot(eq)
				if e != nil {
					return brFailed, e
				}
				diseqs = append(diseqs, h)
			}
		}
		h, e := m.MkAndArgs(diseqs)
		if e != nil {
			return brFailed, e
		}
		return brRewrite3, m.SetRef(out, h)
	}
	return brFailed, nil
}

func (rw *boolRewriter) mkITECore(c, t, f ast.ASTHandle, out ast.ASTRef) (status brStatus, err error) {
	m := rw.manager
	// Lexical source references, constructed only at the reached declaration
	// and retired in reverse order. Five is the largest source branch.
	var temps [5]ast.ASTRef
	size := 0
	newTemp := func() (ast.ASTRef, error) {
		if size == len(temps) {
			return ast.ASTRef{}, ast.ErrASTDomain
		}
		r, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return ast.ASTRef{}, e
		}
		temps[size] = r
		size++
		return r, nil
	}
	defer func() {
		for size > 0 {
			size--
			err = errors.Join(err, m.DelRef(&temps[size]))
		}
	}()
	changed := false
	negative, inner, e := m.IsNot(c)
	if e != nil {
		return brFailed, e
	}
	if negative {
		c = inner
		t, f = f, t
		changed = true
	}
	nested, cond, a, b, e := rw.ite(t)
	if e != nil {
		return brFailed, e
	}
	if nested && cond == c {
		t = a
		changed = true
	}
	nested, cond, a, b, e = rw.ite(f)
	if e != nil {
		return brFailed, e
	}
	if nested && cond == c {
		f = b
		changed = true
	}
	truth, e := m.MkTrue()
	if e != nil {
		return brFailed, e
	}
	falsity, e := m.MkFalse()
	if e != nil {
		return brFailed, e
	}
	if c == truth {
		return brDone, m.SetRef(out, t)
	}
	if c == falsity {
		return brDone, m.SetRef(out, f)
	}
	if t == f {
		return brDone, m.SetRef(out, t)
	}
	boolean, e := rw.isBool(t)
	if e != nil {
		return brFailed, e
	}
	if boolean {
		if t == truth {
			if f == falsity {
				return brDone, m.SetRef(out, c)
			}
			return brDone, rw.mkOr([]ast.ASTHandle{c, f}, out)
		}
		if t == falsity {
			if f == truth {
				return brDone, rw.mkNot(c, out)
			}
			tmp, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			if e = rw.mkNot(c, tmp); e != nil {
				return brFailed, e
			}
			nc, e := m.RefGet(tmp)
			if e != nil {
				return brFailed, e
			}
			return brDone, rw.mkAnd([]ast.ASTHandle{nc, f}, out)
		}
		if f == truth {
			tmp, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			if e = rw.mkNot(c, tmp); e != nil {
				return brFailed, e
			}
			nc, e := m.RefGet(tmp)
			if e != nil {
				return brFailed, e
			}
			return brDone, rw.mkOr([]ast.ASTHandle{nc, t}, out)
		}
		if f == falsity || c == f {
			return brDone, rw.mkAnd([]ast.ASTHandle{c, t}, out)
		}
		if c == t {
			return brDone, rw.mkOr([]ast.ASTHandle{c, f}, out)
		}
		comp, e := rw.complement(t, f)
		if e != nil {
			return brFailed, e
		}
		if comp {
			return brDone, rw.mkEq(c, t, out)
		}
	}
	tITE, c2, t1, t2, e := rw.ite(t)
	if e != nil {
		return brFailed, e
	}
	if tITE && rw.iteExtraRules {
		if f == t1 {
			notc2, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			if e = rw.mkNot(c2, notc2); e != nil {
				return brFailed, e
			}
			newc, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			nc2, e := m.RefGet(notc2)
			if e != nil {
				return brFailed, e
			}
			if e = rw.mkAnd([]ast.ASTHandle{c, nc2}, newc); e != nil {
				return brFailed, e
			}
			h, e := m.RefGet(newc)
			if e != nil {
				return brFailed, e
			}
			result, e := m.MkIte(h, t2, f)
			if e != nil {
				return brFailed, e
			}
			return brRewrite1, m.SetRef(out, result)
		}
		if f == t2 {
			newc, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			if e = rw.mkAnd([]ast.ASTHandle{c, c2}, newc); e != nil {
				return brFailed, e
			}
			h, e := m.RefGet(newc)
			if e != nil {
				return brFailed, e
			}
			result, e := m.MkIte(h, t1, f)
			if e != nil {
				return brFailed, e
			}
			return brRewrite1, m.SetRef(out, result)
		}
		eITE, c3, e1, e2, e := rw.ite(f)
		if e != nil {
			return brFailed, e
		}
		same, swapped := t1 == e1 && t2 == e2, t1 == e2 && t2 == e1
		if eITE && (same || swapped) {
			and1, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			and2, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			notc, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			if e = rw.mkAnd([]ast.ASTHandle{c, c2}, and1); e != nil {
				return brFailed, e
			}
			if e = rw.mkNot(c, notc); e != nil {
				return brFailed, e
			}
			nc, e := m.RefGet(notc)
			if e != nil {
				return brFailed, e
			}
			if !same {
				notc3, e := newTemp()
				if e != nil {
					return brFailed, e
				}
				if e = rw.mkNot(c3, notc3); e != nil {
					return brFailed, e
				}
				c3, e = m.RefGet(notc3)
				if e != nil {
					return brFailed, e
				}
			}
			if e = rw.mkAnd([]ast.ASTHandle{nc, c3}, and2); e != nil {
				return brFailed, e
			}
			newc, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			h1, e := m.RefGet(and1)
			if e != nil {
				return brFailed, e
			}
			h2, e := m.RefGet(and2)
			if e != nil {
				return brFailed, e
			}
			if e = rw.mkOr([]ast.ASTHandle{h1, h2}, newc); e != nil {
				return brFailed, e
			}
			h, e := m.RefGet(newc)
			if e != nil {
				return brFailed, e
			}
			result, e := m.MkIte(h, t1, t2)
			if e != nil {
				return brFailed, e
			}
			return brRewrite1, m.SetRef(out, result)
		}
	}
	eITE, c2, e1, e2, e := rw.ite(f)
	if e != nil {
		return brFailed, e
	}
	if eITE && rw.iteExtraRules {
		if t == e1 {
			newc, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			if e = rw.mkOr([]ast.ASTHandle{c, c2}, newc); e != nil {
				return brFailed, e
			}
			h, e := m.RefGet(newc)
			if e != nil {
				return brFailed, e
			}
			result, e := m.MkIte(h, t, e2)
			if e != nil {
				return brFailed, e
			}
			return brRewrite1, m.SetRef(out, result)
		}
		if t == e2 {
			notc2, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			if e = rw.mkNot(c2, notc2); e != nil {
				return brFailed, e
			}
			newc, e := newTemp()
			if e != nil {
				return brFailed, e
			}
			nc2, e := m.RefGet(notc2)
			if e != nil {
				return brFailed, e
			}
			if e = rw.mkOr([]ast.ASTHandle{c, nc2}, newc); e != nil {
				return brFailed, e
			}
			h, e := m.RefGet(newc)
			if e != nil {
				return brFailed, e
			}
			result, e := m.MkIte(h, t, e1)
			if e != nil {
				return brFailed, e
			}
			return brRewrite1, m.SetRef(out, result)
		}
	}
	if changed {
		h, e := m.MkIte(c, t, f)
		if e != nil {
			return brFailed, e
		}
		return brDone, m.SetRef(out, h)
	}
	return brFailed, nil
}

func (rw *boolRewriter) mkITE(c, t, f ast.ASTHandle, out ast.ASTRef) error {
	status, e := rw.mkITECore(c, t, f, out)
	if e != nil || status != brFailed {
		return e
	}
	h, e := rw.manager.MkIte(c, t, f)
	if e != nil {
		return e
	}
	return rw.manager.SetRef(out, h)
}
func (rw *boolRewriter) mkDistinct(args []ast.ASTHandle, out ast.ASTRef) error {
	status, e := rw.mkDistinctCore(args, out)
	if e != nil || status != brFailed {
		return e
	}
	h, e := rw.manager.MkDistinct(args)
	if e != nil {
		return e
	}
	return rw.manager.SetRef(out, h)
}

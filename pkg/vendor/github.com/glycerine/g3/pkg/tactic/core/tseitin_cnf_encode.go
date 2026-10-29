package core

// Source: src/tactic/core/tseitin_cnf_tactic.cpp:48-748.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

type tseitinFrame struct {
	term  ast.ASTHandle
	first bool
}
type tseitinMatch uint8

const (
	tseitinNo tseitinMatch = iota
	tseitinContinue
	tseitinDone
)

type tseitinClause struct {
	ref ast.ASTRef
	dep ast.ASTDependency
}

type tseitinInvocation struct {
	owner         *TseitinCNFTactic
	rw            *rewriter.BoolRewriter
	occs          *tactic.GoalSharedOccurrences
	cache         map[ast.ASTHandle]ast.ASTHandle
	frames        []tseitinFrame
	owned         []ast.ASTRef
	clauses       []tseitinClause
	fresh         []ast.ASTHandle
	currDep       ast.ASTDependency
	models, cores bool
	numAux        uint32
}

func newTseitinInvocation(t *TseitinCNFTactic) (_ *tseitinInvocation, err error) {
	rw, err := rewriter.NewBoolRewriter(t.manager, t.context, t.globals, &t.params)
	if err != nil {
		return nil, err
	}
	if err = rw.SetFlat(false); err != nil {
		return nil, errors.Join(err, rw.Close())
	}
	occs, err := tactic.NewGoalSharedOccurrences(t.manager, false, false, false)
	if err != nil {
		return nil, errors.Join(err, rw.Close())
	}
	return &tseitinInvocation{owner: t, rw: rw, occs: occs, cache: make(map[ast.ASTHandle]ast.ASTHandle)}, nil
}
func (c *tseitinInvocation) close() (err error) {
	for i := len(c.clauses) - 1; i >= 0; i-- {
		if !c.clauses[i].dep.IsNull() {
			err = errors.Join(err, c.owner.manager.DependencyDec(c.clauses[i].dep))
		}
		err = errors.Join(err, c.owner.manager.DelRef(&c.clauses[i].ref))
	}
	for i := len(c.owned) - 1; i >= 0; i-- {
		err = errors.Join(err, c.owner.manager.DelRef(&c.owned[i]))
	}
	if c.occs != nil {
		err = errors.Join(err, c.occs.Close())
	}
	if c.rw != nil {
		err = errors.Join(err, c.rw.Close())
	}
	return err
}
func (c *tseitinInvocation) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.owned = append(c.owned, r)
	}
	return h, err
}
func (c *tseitinInvocation) op(h ast.ASTHandle) (int32, []ast.ASTHandle, bool, error) {
	m := c.owner.manager
	k, err := m.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return 0, nil, false, err
	}
	f, err := m.GetFamilyID(h)
	if err != nil || f != m.BasicFamilyID() {
		return 0, nil, false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return 0, nil, false, err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return 0, nil, false, err
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		if args[i], err = m.GetArg(h, uint32(i)); err != nil {
			return 0, nil, false, err
		}
	}
	return op, args, true, nil
}
func (c *tseitinInvocation) isBool(h ast.ASTHandle) (bool, error) {
	s, err := c.owner.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	b, err := c.owner.manager.MkBoolSort()
	return s == b, err
}
func (c *tseitinInvocation) iff(h ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	op, a, app, err := c.op(h)
	if err != nil || !app || len(a) != 2 {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if op == ast.ASTOpIff {
		return a[0], a[1], true, nil
	}
	if op == ast.ASTOpEq {
		b, e := c.isBool(a[0])
		return a[0], a[1], b, e
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}
func (c *tseitinInvocation) notArg(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	y, a, e := c.owner.manager.IsNot(h)
	return a, y, e
}
func (c *tseitinInvocation) isOp(h ast.ASTHandle, wanted int32) ([]ast.ASTHandle, bool, error) {
	op, a, app, e := c.op(h)
	return a, app && op == wanted, e
}
func (c *tseitinInvocation) shared(h ast.ASTHandle) (bool, error) { return c.occs.IsShared(h) }

func (c *tseitinInvocation) visit(h ast.ASTHandle, visited *bool, root bool) error {
	for {
		if _, ok := c.cache[h]; ok {
			return nil
		}
		op, args, app, err := c.op(h)
		if err != nil || !app || len(args) == 0 {
			return err
		}
		switch op {
		case ast.ASTOpNot:
			if !root {
				h = args[0]
				continue
			}
		case ast.ASTOpOr, ast.ASTOpIff:
		case ast.ASTOpIte, ast.ASTOpEq:
			b, e := c.isBool(args[1])
			if e != nil {
				return e
			}
			if !b {
				return nil
			}
		case ast.ASTOpAnd, ast.ASTOpXor, ast.ASTOpImplies, ast.ASTOpDistinct:
			return tactic.NewTacticException("operator not supported, apply simplifier before invoking this strategy")
		default:
			return nil
		}
		*visited = false
		c.frames = append(c.frames, tseitinFrame{h, true})
		return nil
	}
}
func (c *tseitinInvocation) mkNot(h ast.ASTHandle) (ast.ASTHandle, error) {
	m := c.owner.manager
	if m.IsTrue(h) {
		x, e := m.MkFalse()
		return x, e
	}
	if m.IsFalse(h) {
		x, e := m.MkTrue()
		return x, e
	}
	if a, yes, e := c.notArg(h); e != nil || yes {
		return a, e
	}
	x, e := m.MkNot(h)
	if e != nil {
		return x, e
	}
	return c.hold(x)
}
func (c *tseitinInvocation) lit(h ast.ASTHandle, sign bool) (ast.ASTHandle, error) {
	for {
		op, args, app, e := c.op(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !app || len(args) == 0 {
			if sign {
				return c.mkNot(h)
			}
			return h, nil
		}
		if op == ast.ASTOpNot {
			h = args[0]
			sign = !sign
			continue
		}
		if op == ast.ASTOpOr || op == ast.ASTOpIff || op == ast.ASTOpIte || op == ast.ASTOpEq {
			if op == ast.ASTOpIte || op == ast.ASTOpEq {
				b, x := c.isBool(args[1])
				if x != nil {
					return ast.ASTHandle{}, x
				}
				if !b {
					if sign {
						return c.mkNot(h)
					}
					return h, nil
				}
			}
			v, ok := c.cache[h]
			if !ok {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
			if sign {
				return c.mkNot(v)
			}
			return v, nil
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
}
func (c *tseitinInvocation) clause(lits ...ast.ASTHandle) error {
	r, err := c.owner.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = c.rw.MkOr(lits, r); err != nil {
		return errors.Join(err, c.owner.manager.DelRef(&r))
	}
	d := ast.ASTDependency{}
	if c.cores {
		d = c.currDep
		if err = c.owner.manager.DependencyInc(d); err != nil {
			return errors.Join(err, c.owner.manager.DelRef(&r))
		}
	}
	c.clauses = append(c.clauses, tseitinClause{r, d})
	return nil
}
func (c *tseitinInvocation) freshVar() (ast.ASTHandle, error) {
	s, e := c.owner.manager.MkBoolSort()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	v, e := c.owner.manager.MkFreshConstNullable(nil, s)
	if e != nil {
		return v, e
	}
	v, e = c.hold(v)
	if e == nil {
		c.fresh = append(c.fresh, v)
		c.numAux++
	}
	return v, e
}
func (c *tseitinInvocation) cacheResult(term, value ast.ASTHandle) error {
	if _, e := c.hold(term); e != nil {
		return e
	}
	c.cache[term] = value
	return nil
}
func (c *tseitinInvocation) checkpoint() error {
	if e := c.owner.services.Cooperation.Cooperate("tseitin cnf"); e != nil {
		return e
	}
	n, e := c.owner.services.Memory.AllocationSize()
	if e != nil {
		return e
	}
	if n > c.owner.maxMemory {
		return tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
	}
	y, e := c.owner.manager.Canceled()
	if e != nil {
		return e
	}
	if y {
		return tactic.NewTacticException(c.owner.manager.ResourceLimit().CancelMessage())
	}
	return nil
}

func (c *tseitinInvocation) or3and(t ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error) {
	if !c.owner.commonPatterns {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	x, ok, e := c.isOp(t, ast.ASTOpOr)
	if e != nil || !ok || len(x) != 3 {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	pairs := make([][2]ast.ASTHandle, 3)
	for i := range x {
		n, yes, e := c.notArg(x[i])
		if e != nil || !yes {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		shared, e := c.shared(n)
		if e != nil || shared {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		a, yes, e := c.isOp(n, ast.ASTOpOr)
		if e != nil || !yes || len(a) != 2 {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		pairs[i] = [2]ast.ASTHandle{a[0], a[1]}
		sort.Slice(pairs[i][:], func(j, k int) bool {
			aj, _ := c.owner.manager.GetASTID(pairs[i][j])
			ak, _ := c.owner.manager.GetASTID(pairs[i][k])
			return aj < ak
		})
	}
	a1, a2, b1, b2, d1, d2 := pairs[0][0], pairs[0][1], pairs[1][0], pairs[1][1], pairs[2][0], pairs[2][1]
	if (a1 == b1 && a2 == d1 && b2 == d2) || (a1 == b1 && a2 == d2 && b2 == d1) || (a1 == d1 && a2 == b1 && b2 == d2) {
		return a1, a2, b2, true, nil
	}
	if (a1 == b2 && a2 == d2 && b1 == d1) || (a1 == d1 && a2 == b2 && b1 == d2) || (a1 == d2 && a2 == b2 && b1 == d1) {
		return a1, a2, b1, true, nil
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}
func (c *tseitinInvocation) iff3(t ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error) {
	l, r, yes, e := c.iff(t)
	if e != nil || !yes {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	sh, e := c.shared(l)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	if !sh {
		a, b, y, x := c.iff(l)
		if x != nil {
			return a, b, r, false, x
		}
		if y {
			return a, b, r, true, nil
		}
	}
	sh, e = c.shared(r)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	if !sh {
		b, d, y, x := c.iff(r)
		if x != nil {
			return l, b, d, false, x
		}
		if y {
			return l, b, d, true, nil
		}
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}

func (c *tseitinInvocation) visitAll(first bool, terms ...ast.ASTHandle) (tseitinMatch, error) {
	if !first {
		return tseitinDone, nil
	}
	visited := true
	for _, h := range terms {
		if e := c.visit(h, &visited, false); e != nil {
			return tseitinNo, e
		}
	}
	if !visited {
		return tseitinContinue, nil
	}
	return tseitinDone, nil
}
func (c *tseitinInvocation) matchNot(t ast.ASTHandle, first, root bool) (tseitinMatch, error) {
	a, yes, e := c.notArg(t)
	if e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		r, e := c.visitAll(true, a)
		if e != nil || r == tseitinContinue {
			return r, e
		}
	}
	if root {
		l, e := c.lit(a, true)
		if e != nil {
			return tseitinNo, e
		}
		if e = c.clause(l); e != nil {
			return tseitinNo, e
		}
	}
	return tseitinDone, nil
}
func (c *tseitinInvocation) matchOr3And(t ast.ASTHandle, first, root bool) (tseitinMatch, error) {
	a, b, d, yes, e := c.or3and(t)
	if e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		r, e := c.visitAll(true, a, b, d)
		if e != nil || r == tseitinContinue {
			return r, e
		}
	}
	na, e := c.lit(a, true)
	if e != nil {
		return tseitinNo, e
	}
	nb, e := c.lit(b, true)
	if e != nil {
		return tseitinNo, e
	}
	nd, e := c.lit(d, true)
	if e != nil {
		return tseitinNo, e
	}
	if root {
		return tseitinDone, errors.Join(c.clause(na, nb), c.clause(na, nd), c.clause(nb, nd))
	}
	k, e := c.freshVar()
	if e != nil {
		return tseitinNo, e
	}
	nk, e := c.mkNot(k)
	if e != nil {
		return tseitinNo, e
	}
	la, e := c.mkNot(na)
	if e != nil {
		return tseitinNo, e
	}
	lb, e := c.mkNot(nb)
	if e != nil {
		return tseitinNo, e
	}
	ld, e := c.mkNot(nd)
	if e != nil {
		return tseitinNo, e
	}
	e = errors.Join(c.clause(nk, na, nb), c.clause(nk, na, nd), c.clause(nk, nb, nd), c.clause(k, la, lb), c.clause(k, la, ld), c.clause(k, lb, ld))
	if e == nil {
		e = c.cacheResult(t, k)
	}
	return tseitinDone, e
}
func (c *tseitinInvocation) matchIff3(t ast.ASTHandle, first, root bool) (tseitinMatch, error) {
	a, b, d, yes, e := c.iff3(t)
	if e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		r, e := c.visitAll(true, a, b, d)
		if e != nil || r == tseitinContinue {
			return r, e
		}
	}
	la, e := c.lit(a, false)
	if e != nil {
		return tseitinNo, e
	}
	lb, e := c.lit(b, false)
	if e != nil {
		return tseitinNo, e
	}
	ld, e := c.lit(d, false)
	if e != nil {
		return tseitinNo, e
	}
	na, e := c.mkNot(la)
	if e != nil {
		return tseitinNo, e
	}
	nb, e := c.mkNot(lb)
	if e != nil {
		return tseitinNo, e
	}
	nd, e := c.mkNot(ld)
	if e != nil {
		return tseitinNo, e
	}
	if root {
		return tseitinDone, errors.Join(c.clause(la, lb, ld), c.clause(la, nb, nd), c.clause(na, lb, nd), c.clause(na, nb, ld))
	}
	k, e := c.freshVar()
	if e != nil {
		return tseitinNo, e
	}
	nk, e := c.mkNot(k)
	if e != nil {
		return tseitinNo, e
	}
	e = errors.Join(c.clause(nk, la, lb, ld), c.clause(nk, la, nb, nd), c.clause(nk, na, lb, nd), c.clause(nk, na, nb, ld), c.clause(k, na, nb, nd), c.clause(k, na, lb, ld), c.clause(k, la, nb, ld), c.clause(k, la, lb, nd))
	if e == nil {
		e = c.cacheResult(t, k)
	}
	return tseitinDone, e
}
func (c *tseitinInvocation) matchIff(t ast.ASTHandle, first, root bool) (tseitinMatch, error) {
	a, b, yes, e := c.iff(t)
	if e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		r, e := c.visitAll(true, a, b)
		if e != nil || r == tseitinContinue {
			return r, e
		}
	}
	la, e := c.lit(a, false)
	if e != nil {
		return tseitinNo, e
	}
	lb, e := c.lit(b, false)
	if e != nil {
		return tseitinNo, e
	}
	na, e := c.mkNot(la)
	if e != nil {
		return tseitinNo, e
	}
	nb, e := c.mkNot(lb)
	if e != nil {
		return tseitinNo, e
	}
	if root {
		return tseitinDone, errors.Join(c.clause(la, nb), c.clause(na, lb))
	}
	k, e := c.freshVar()
	if e != nil {
		return tseitinNo, e
	}
	nk, e := c.mkNot(k)
	if e != nil {
		return tseitinNo, e
	}
	e = errors.Join(c.clause(nk, la, nb), c.clause(nk, na, lb), c.clause(k, na, nb), c.clause(k, la, lb))
	if e == nil {
		e = c.cacheResult(t, k)
	}
	return tseitinDone, e
}

func (c *tseitinInvocation) iteArgs(h ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	return c.isOp(h, ast.ASTOpIte)
}
func (c *tseitinInvocation) unsharedIte(h ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	a, y, e := c.iteArgs(h)
	if e != nil || !y {
		return nil, false, e
	}
	sh, e := c.shared(h)
	return a, !sh, e
}
func (c *tseitinInvocation) visitIteChain(t ast.ASTHandle) (bool, error) {
	visited := true
	ite := t
	for {
		a, yes, e := c.iteArgs(ite)
		if e != nil || !yes {
			return visited, e
		}
		if e = c.visit(a[0], &visited, false); e != nil {
			return visited, e
		}
		if c.owner.iteChains {
			if _, yes, e := c.unsharedIte(a[1]); e != nil {
				return visited, e
			} else if yes {
				if e = c.visit(a[2], &visited, false); e != nil {
					return visited, e
				}
				ite = a[1]
				continue
			}
			if _, yes, e := c.unsharedIte(a[2]); e != nil {
				return visited, e
			} else if yes {
				if e = c.visit(a[1], &visited, false); e != nil {
					return visited, e
				}
				ite = a[2]
				continue
			}
		}
		if e = c.visit(a[1], &visited, false); e != nil {
			return visited, e
		}
		if e = c.visit(a[2], &visited, false); e != nil {
			return visited, e
		}
		return visited, nil
	}
}
func (c *tseitinInvocation) matchIte(t ast.ASTHandle, first, root bool) (tseitinMatch, error) {
	if _, yes, e := c.iteArgs(t); e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		visited, e := c.visitIteChain(t)
		if e != nil {
			return tseitinNo, e
		}
		if !visited {
			return tseitinContinue, nil
		}
	}
	var k, nk ast.ASTHandle
	var e error
	if !root {
		k, e = c.freshVar()
		if e != nil {
			return tseitinNo, e
		}
		nk, e = c.mkNot(k)
		if e != nil {
			return tseitinNo, e
		}
		if e = c.cacheResult(t, k); e != nil {
			return tseitinNo, e
		}
	}
	ctx, exPos, exNeg := []ast.ASTHandle{}, []ast.ASTHandle{}, []ast.ASTHandle{}
	ite := t
	add := func(extra []ast.ASTHandle, ls ...ast.ASTHandle) error {
		x := append(append([]ast.ASTHandle{}, extra...), ls...)
		return c.clause(x...)
	}
	for {
		a, _, x := c.iteArgs(ite)
		if x != nil {
			return tseitinNo, x
		}
		la, x := c.lit(a[0], false)
		if x != nil {
			return tseitinNo, x
		}
		na, x := c.mkNot(la)
		if x != nil {
			return tseitinNo, x
		}
		if c.owner.iteChains {
			if _, yes, x := c.unsharedIte(a[1]); x != nil {
				return tseitinNo, x
			} else if yes {
				lc, x := c.lit(a[2], false)
				if x != nil {
					return tseitinNo, x
				}
				if root {
					x = add(ctx, la, lc)
				} else {
					nc, y := c.mkNot(lc)
					if y != nil {
						return tseitinNo, y
					}
					x = errors.Join(add(ctx, la, lc, nk), add(ctx, la, nc, k))
					if c.owner.iteExtra {
						exNeg = append(exNeg, lc)
						exPos = append(exPos, nc)
					}
				}
				if x != nil {
					return tseitinNo, x
				}
				ctx = append(ctx, na)
				ite = a[1]
				continue
			}
			if _, yes, x := c.unsharedIte(a[2]); x != nil {
				return tseitinNo, x
			} else if yes {
				lb, x := c.lit(a[1], false)
				if x != nil {
					return tseitinNo, x
				}
				if root {
					x = add(ctx, na, lb)
				} else {
					nb, y := c.mkNot(lb)
					if y != nil {
						return tseitinNo, y
					}
					x = errors.Join(add(ctx, na, lb, nk), add(ctx, na, nb, k))
					if c.owner.iteExtra {
						exNeg = append(exNeg, lb)
						exPos = append(exPos, nb)
					}
				}
				if x != nil {
					return tseitinNo, x
				}
				ctx = append(ctx, la)
				ite = a[2]
				continue
			}
		}
		lb, x := c.lit(a[1], false)
		if x != nil {
			return tseitinNo, x
		}
		lc, x := c.lit(a[2], false)
		if x != nil {
			return tseitinNo, x
		}
		if root {
			x = errors.Join(add(ctx, na, lb), add(ctx, la, lc))
		} else {
			nb, y := c.mkNot(lb)
			if y != nil {
				return tseitinNo, y
			}
			nc, y := c.mkNot(lc)
			if y != nil {
				return tseitinNo, y
			}
			x = errors.Join(add(ctx, na, lb, nk), add(ctx, la, lc, nk), add(ctx, na, nb, k), add(ctx, la, nc, k))
			if c.owner.iteExtra {
				x = errors.Join(x, add(nil, lb, lc, nk), add(nil, nb, nc, k), add(exNeg, lb, lc, nk), add(exPos, nb, nc, k))
			}
		}
		return tseitinDone, x
	}
}

func (c *tseitinInvocation) distributiveArg(h ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	n, yes, e := c.notArg(h)
	if e != nil || !yes {
		return nil, false, e
	}
	a, yes, e := c.isOp(n, ast.ASTOpOr)
	if e != nil || !yes {
		return nil, false, e
	}
	sh, e := c.shared(n)
	return a, !sh, e
}
func (c *tseitinInvocation) distributiveChoices(args []ast.ASTHandle, visit bool) ([][]ast.ASTHandle, bool, error) {
	choices := make([][]ast.ASTHandle, len(args))
	blowup := uint32(1)
	used := false
	for i, a := range args {
		inner, yes, e := c.distributiveArg(a)
		if e != nil {
			return nil, false, e
		}
		n := uint32(len(inner))
		accept := c.owner.distributivity && yes && n < c.owner.distributivityBlowup && blowup*n < c.owner.distributivityBlowup && blowup < blowup*n
		if accept {
			used = true
			blowup *= n
			choices[i] = append([]ast.ASTHandle{}, inner...)
			if visit {
				continue
			}
		}
		if !accept {
			choices[i] = []ast.ASTHandle{a}
		}
	}
	return choices, used, nil
}
func (c *tseitinInvocation) matchOr(t ast.ASTHandle, first, root bool) (tseitinMatch, error) {
	args, yes, e := c.isOp(t, ast.ASTOpOr)
	if e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		choices, _, e := c.distributiveChoices(args, true)
		if e != nil {
			return tseitinNo, e
		}
		visited := true
		for i, a := range args {
			if len(choices[i]) > 1 {
				for _, x := range choices[i] {
					if e = c.visit(x, &visited, false); e != nil {
						return tseitinNo, e
					}
				}
			} else if e = c.visit(a, &visited, false); e != nil {
				return tseitinNo, e
			}
		}
		if !visited {
			return tseitinContinue, nil
		}
	}
	var k, nk ast.ASTHandle
	if !root {
		k, e = c.freshVar()
		if e != nil {
			return tseitinNo, e
		}
		nk, e = c.mkNot(k)
		if e != nil {
			return tseitinNo, e
		}
		if e = c.cacheResult(t, k); e != nil {
			return tseitinNo, e
		}
	}
	choices, distributed, e := c.distributiveChoices(args, false)
	if e != nil {
		return tseitinNo, e
	}
	if !distributed {
		lits := make([]ast.ASTHandle, len(args))
		for i, a := range args {
			if lits[i], e = c.lit(a, false); e != nil {
				return tseitinNo, e
			}
		}
		if root {
			return tseitinDone, c.clause(lits...)
		}
		for _, l := range lits {
			nl, x := c.mkNot(l)
			if x != nil {
				return tseitinNo, x
			}
			if x = c.clause(nl, k); x != nil {
				return tseitinNo, x
			}
		}
		return tseitinDone, c.clause(append(lits, nk)...)
	}
	converted := make([][]ast.ASTHandle, len(choices))
	indices := make([]int, len(choices))
	for i, group := range choices {
		inner, expanded, x := c.distributiveArg(args[i])
		if x != nil {
			return tseitinNo, x
		}
		if expanded && len(group) > 1 {
			converted[i] = make([]ast.ASTHandle, len(inner))
			back := make([]ast.ASTHandle, 0, len(inner)+1)
			for j, a := range inner {
				l, x := c.lit(a, true)
				if x != nil {
					return tseitinNo, x
				}
				converted[i][j] = l
				if !root {
					nl, y := c.mkNot(l)
					if y != nil {
						return tseitinNo, y
					}
					back = append(back, nl)
				}
			}
			if !root {
				back = append(back, k)
				if x = c.clause(back...); x != nil {
					return tseitinNo, x
				}
			}
		} else {
			l, x := c.lit(args[i], false)
			if x != nil {
				return tseitinNo, x
			}
			converted[i] = []ast.ASTHandle{l}
			if !root {
				nl, y := c.mkNot(l)
				if y != nil {
					return tseitinNo, y
				}
				if x = c.clause(nl, k); x != nil {
					return tseitinNo, x
				}
			}
		}
	}
	for {
		clause := make([]ast.ASTHandle, 0, len(converted)+1)
		for i := range converted {
			clause = append(clause, converted[i][indices[i]])
		}
		if !root {
			clause = append(clause, nk)
		}
		if e = c.clause(clause...); e != nil {
			return tseitinNo, e
		}
		pos := 0
		for ; pos < len(indices); pos++ {
			indices[pos]++
			if indices[pos] < len(converted[pos]) {
				break
			}
			indices[pos] = 0
		}
		if pos == len(indices) {
			break
		}
	}
	return tseitinDone, nil
}

func (c *tseitinInvocation) process(root ast.ASTHandle, dep ast.ASTDependency) error {
	c.currDep = dep
	visited := true
	if e := c.visit(root, &visited, true); e != nil {
		return e
	}
	if visited {
		l, e := c.lit(root, false)
		if e != nil {
			return e
		}
		return c.clause(l)
	}
	for len(c.frames) > 0 {
		if e := c.checkpoint(); e != nil {
			return e
		}
		i := len(c.frames) - 1
		f := c.frames[i]
		c.frames[i].first = false
		matchers := []func(ast.ASTHandle, bool, bool) (tseitinMatch, error){c.matchOr3And, c.matchOr, c.matchIff3, c.matchIff, c.matchIte, c.matchNot}
		matched := false
		for _, matcher := range matchers {
			r, e := matcher(f.term, f.first, f.term == root)
			if e != nil {
				return e
			}
			if r == tseitinContinue {
				matched = true
				break
			}
			if r == tseitinDone {
				c.frames = c.frames[:i]
				matched = true
				break
			}
		}
		if !matched {
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (t *TseitinCNFTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if sorted, e := goal.IsWellSorted(); e != nil {
		return e
	} else if !sorted {
		return ast.ErrASTDomain
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("tseitin-cnf", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = tactic.FailIfProofGeneration("tseitin-cnf", goal); err != nil {
		return err
	}
	inv, e := newTseitinInvocation(t)
	if e != nil {
		return e
	}
	defer func() { t.numAux += inv.numAux; err = errors.Join(err, inv.close()) }()
	if inv.models, e = goal.ModelsEnabled(); e != nil {
		return e
	}
	if inv.cores, e = goal.UnsatCoresEnabled(); e != nil {
		return e
	}
	if e = inv.occs.Collect(goal); e != nil {
		return e
	}
	size, e := goal.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		form, x := goal.Form(i)
		if x != nil {
			return x
		}
		dep, x := goal.Dependency(i)
		if x != nil {
			return x
		}
		if x = inv.process(form, dep); x != nil {
			return x
		}
	}
	if e = goal.Reset(); e != nil {
		return e
	}
	added := make(map[ast.ASTHandle]bool, len(inv.clauses))
	for _, row := range inv.clauses {
		h, x := t.manager.RefGet(row.ref)
		if x != nil {
			return x
		}
		if added[h] {
			continue
		}
		added[h] = true
		if x = goal.AssertWithProof(h, ast.ASTHandle{}, row.dep); x != nil {
			return x
		}
	}
	if inv.models && len(inv.fresh) > 0 {
		filter, x := tactic.NewFilterModel(t.manager)
		if x != nil {
			return x
		}
		if x = filter.IncRef(); x != nil {
			return errors.Join(x, filter.Close())
		}
		defer func() { err = errors.Join(err, filter.DecRef()) }()
		for _, v := range inv.fresh {
			d, y := t.manager.GetDecl(v)
			if y != nil {
				return y
			}
			if y = filter.Insert(d); y != nil {
				return y
			}
		}
		if x = result.SetModelConverter(filter); x != nil {
			return x
		}
	}
	if e = goal.IncDepth(); e != nil {
		return e
	}
	if e = result.AddGoal(goal); e != nil {
		return e
	}
	return tactic.ReportTacticProgress(":cnf-aux-vars", t.numAux+inv.numAux)
}

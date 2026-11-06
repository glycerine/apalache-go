package substitution

// Source: src/ast/substitution/unifier.cpp;
// src/ast/substitution/unifier.h.

import "github.com/glycerine/g3/pkg/ast"

type unifyPair struct{ left, right ExprOffset }

// Unifier implements the source quasi-linear union/find algorithm over one or
// more variable banks.
type Unifier struct {
	self    *Unifier
	manager *ast.ASTManager
	find    exprOffsetOffsetMap
	sizes   exprOffsetUintMap
	todo    []unifyPair
	scratch []ast.ASTRef
	closed  bool
}

func NewUnifier(m *ast.ASTManager) (*Unifier, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	u := &Unifier{manager: m, find: make(exprOffsetOffsetMap), sizes: make(exprOffsetUintMap)}
	u.self = u
	return u, nil
}

func (u *Unifier) open() error {
	if u == nil || u.self != u || u.manager == nil || u.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (u *Unifier) reset() error {
	u.todo = u.todo[:0]
	u.find.reset()
	u.sizes.reset()
	for len(u.scratch) != 0 {
		i := len(u.scratch) - 1
		if err := u.manager.DelRef(&u.scratch[i]); err != nil {
			return err
		}
		u.scratch = u.scratch[:i]
	}
	return nil
}

func (u *Unifier) root(p ExprOffset) ExprOffset {
	path := make([]ExprOffset, 0, 4)
	for {
		next, ok := u.find.find(p)
		if !ok {
			break
		}
		path = append(path, p)
		p = next
	}
	for _, q := range path {
		u.find.insert(q, p)
	}
	return p
}

func (u *Unifier) saveVariable(s *Substitution, p, target ExprOffset) error {
	k, err := u.manager.GetKind(p.expr)
	if err != nil {
		return err
	}
	if k != ast.ASTVar {
		return nil
	}
	i, err := u.manager.GetVarIndex(p.expr)
	if err != nil {
		return err
	}
	return s.Insert(i, p.offset, target)
}

func (u *Unifier) unionOne(s *Substitution, left, right ExprOffset) error {
	a, b := uint32(1), uint32(1)
	if v, ok := u.sizes.find(left); ok {
		a = v
	}
	if v, ok := u.sizes.find(right); ok {
		b = v
	}
	u.find.insert(left, right)
	u.sizes.insert(right, a+b)
	return u.saveVariable(s, left, right)
}

func (u *Unifier) unionTwo(s *Substitution, left, right ExprOffset) error {
	a, b := uint32(1), uint32(1)
	if v, ok := u.sizes.find(left); ok {
		a = v
	}
	if v, ok := u.sizes.find(right); ok {
		b = v
	}
	if a > b {
		left, right = right, left
	}
	u.find.insert(left, right)
	u.sizes.insert(right, a+b)
	return u.saveVariable(s, left, right)
}

func (u *Unifier) core(s *Substitution, left, right ExprOffset) (bool, error) {
	u.todo = append(u.todo, unifyPair{left, right})
	for len(u.todo) != 0 {
		i := len(u.todo) - 1
		p := u.todo[i]
		u.todo = u.todo[:i]
		left, right = u.root(p.left), u.root(p.right)
		if left == right {
			continue
		}
		lk, e := u.manager.GetKind(left.expr)
		if e != nil {
			return false, e
		}
		rk, e := u.manager.GetKind(right.expr)
		if e != nil {
			return false, e
		}
		if lk == ast.ASTQuantifier || rk == ast.ASTQuantifier {
			return false, ast.ErrASTDomain
		}
		lv, rv := lk == ast.ASTVar, rk == ast.ASTVar
		switch {
		case lv && rv:
			if e = u.unionTwo(s, left, right); e != nil {
				return false, e
			}
		case lv:
			if e = u.unionOne(s, left, right); e != nil {
				return false, e
			}
		case rv:
			if e = u.unionOne(s, right, left); e != nil {
				return false, e
			}
		default:
			if lk != ast.ASTApp || rk != ast.ASTApp {
				return false, ast.ErrASTDomain
			}
			ld, e := u.manager.GetDecl(left.expr)
			if e != nil {
				return false, e
			}
			rd, e := u.manager.GetDecl(right.expr)
			if e != nil {
				return false, e
			}
			ln, e := u.manager.GetNumArgs(left.expr)
			if e != nil {
				return false, e
			}
			rn, e := u.manager.GetNumArgs(right.expr)
			if e != nil {
				return false, e
			}
			if ld != rd || ln != rn {
				return false, nil
			}
			if e = u.unionTwo(s, left, right); e != nil {
				return false, e
			}
			for j := ln; j > 0; j-- {
				la, e := u.manager.GetArg(left.expr, j-1)
				if e != nil {
					return false, e
				}
				ra, e := u.manager.GetArg(right.expr, j-1)
				if e != nil {
					return false, e
				}
				u.todo = append(u.todo, unifyPair{NewExprOffset(la, left.offset), NewExprOffset(ra, right.offset)})
			}
		}
	}
	return true, nil
}

// Unify unifies expressions into s. With useOffsets, each expression uses a
// distinct variable bank, matching the source array overload.
func (u *Unifier) Unify(expressions []ast.ASTHandle, s *Substitution, useOffsets bool) (bool, error) {
	if err := u.open(); err != nil {
		return false, err
	}
	if s == nil || len(expressions) == 0 {
		return false, ast.ErrASTDomain
	}
	if err := s.open(); err != nil {
		return false, err
	}
	if s.manager != u.manager {
		return false, ast.ErrASTHandle
	}
	if err := u.reset(); err != nil {
		return false, err
	}
	n, err := s.NumBindings()
	if err != nil {
		return false, err
	}
	for i := n; i > 0; i-- {
		bound, target, e := s.Binding(i - 1)
		if e != nil {
			return false, e
		}
		kind, e := u.manager.GetKind(target.expr)
		if e != nil {
			return false, e
		}
		if kind != ast.ASTVar {
			continue
		}
		sort, e := u.manager.GetSort(target.expr)
		if e != nil {
			return false, e
		}
		v, e := u.manager.MkVar(bound.Variable, sort)
		if e != nil {
			return false, e
		}
		ref, e := u.manager.NewRef(v)
		if e != nil {
			return false, e
		}
		u.scratch = append(u.scratch, ref)
		child := NewExprOffset(v, bound.Offset)
		a, b := uint32(1), uint32(1)
		if q, ok := u.sizes.find(child); ok {
			a = q
		}
		if q, ok := u.sizes.find(target); ok {
			b = q
		}
		u.find.insert(child, target)
		u.sizes.insert(target, a+b)
	}
	for i := 0; i+1 < len(expressions); i++ {
		lo, ro := uint32(0), uint32(0)
		if useOffsets {
			lo, ro = uint32(i), uint32(i+1)
		}
		ok, e := u.core(s, NewExprOffset(expressions[i], lo), NewExprOffset(expressions[i+1], ro))
		if e != nil || !ok {
			return ok, e
		}
	}
	return s.Acyclic()
}

func (u *Unifier) UnifyPair(left, right ast.ASTHandle, s *Substitution, useOffsets bool) (bool, error) {
	return u.Unify([]ast.ASTHandle{left, right}, s, useOffsets)
}

func (u *Unifier) Close() error {
	if u == nil || u.self != u || u.manager == nil {
		return ast.ErrASTHandle
	}
	if u.closed {
		return nil
	}
	if err := u.reset(); err != nil {
		return err
	}
	u.closed = true
	return nil
}

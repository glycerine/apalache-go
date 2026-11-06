package qe

// Source: src/qe/qe_arrays.cpp indices/solve_eq/solve/compare.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type projectionIndices struct{ expressions, values []ast.ASTHandle }

func (c *arrayProjectionCall) indices(expressions []ast.ASTHandle) (projectionIndices, error) {
	r := projectionIndices{expressions: append([]ast.ASTHandle(nil), expressions...)}
	for _, h := range expressions {
		v, err := c.eval(h)
		if err != nil {
			return r, err
		}
		r.values = append(r.values, v)
	}
	return r, nil
}
func (p *ArrayProjection) compareValues(a, b ast.ASTHandle) (lbool.LBool, error) {
	equal, err := p.manager.AreEqual(a, b)
	if err != nil {
		return lbool.Undef, err
	}
	if equal {
		return lbool.True, nil
	}
	distinct, err := p.manager.AreDistinct(a, b)
	if err != nil {
		return lbool.Undef, err
	}
	if distinct {
		return lbool.False, nil
	}
	return lbool.Undef, nil
}
func (p *ArrayProjection) compareIndices(a, b projectionIndices) (lbool.LBool, error) {
	if len(a.values) != len(b.values) {
		return lbool.Undef, ast.ErrASTDomain
	}
	for i, h := range a.values {
		r, err := p.compareValues(h, b.values[i])
		if err != nil {
			return lbool.Undef, err
		}
		if r != lbool.True {
			return r, nil
		}
	}
	return lbool.True, nil
}
func (p *ArrayProjection) containsIndices(a projectionIndices, previous []projectionIndices) (lbool.LBool, int, error) {
	for i, b := range previous {
		r, err := p.compareIndices(a, b)
		if err != nil {
			return lbool.Undef, 0, err
		}
		if r == lbool.True {
			return r, i, nil
		}
		if r == lbool.Undef {
			return r, 0, nil
		}
	}
	return lbool.False, 0, nil
}
func (c *arrayProjectionCall) equalIndices(a, b projectionIndices) ([]ast.ASTHandle, error) {
	if len(a.expressions) != len(b.expressions) {
		return nil, ast.ErrASTDomain
	}
	result := make([]ast.ASTHandle, 0, len(a.expressions))
	for i, h := range a.expressions {
		eq, err := c.hold(c.owner.manager.MkEq(h, b.expressions[i]))
		if err != nil {
			return nil, err
		}
		result = append(result, eq)
	}
	return result, nil
}
func (c *arrayProjectionCall) solveEquality(additional, lits *[]ast.ASTRef) (bool, error) {
	p := c.owner
	m := p.manager
	for i := 0; i < len(*lits); i++ {
		save, err := m.RefGet((*lits)[i])
		if err != nil {
			return false, err
		}
		args, eq, err := p.operation(save, m.BasicFamilyID(), ast.ASTOpEq)
		if err != nil {
			return false, err
		}
		if !eq || len(args) != 2 {
			continue
		}
		back, err := m.RefGet((*lits)[len(*lits)-1])
		if err != nil {
			return false, err
		}
		if _, err = c.ref(save); err != nil {
			return false, err
		}
		if _, err = c.ref(back); err != nil {
			return false, err
		}
		if err = m.SetRef((*lits)[i], back); err != nil {
			return false, err
		}
		if err = p.shrink(lits, len(*lits)-1); err != nil {
			return false, err
		}
		size := len(*lits)
		left, err := p.variable.Check(args[0])
		if err != nil {
			return false, err
		}
		right, err := p.variable.Check(args[1])
		if err != nil {
			return false, err
		}
		var indices []projectionIndices
		if left && !right {
			kind, err := m.GetKind(args[0])
			if err != nil {
				return false, err
			}
			if kind == ast.ASTApp {
				ok, e := c.solveStore(args[0], args[1], &indices, additional, lits)
				if e != nil {
					return false, e
				}
				if ok {
					return true, nil
				}
			}
		} else if right && !left {
			kind, err := m.GetKind(args[1])
			if err != nil {
				return false, err
			}
			if kind == ast.ASTApp {
				ok, e := c.solveStore(args[1], args[0], &indices, additional, lits)
				if e != nil {
					return false, e
				}
				if ok {
					return true, nil
				}
			}
		}
		// Source rollback removes only the new side conditions and restores
		// the saved equality at its original slot, including the last slot.
		if err = p.shrink(lits, size); err != nil {
			return false, err
		}
		if err = p.appendRef(lits, back); err != nil {
			return false, err
		}
		if err = m.SetRef((*lits)[i], save); err != nil {
			return false, err
		}
	}
	return false, nil
}
func (c *arrayProjectionCall) solveStore(s, t ast.ASTHandle, indices *[]projectionIndices, additional, lits *[]ast.ASTRef) (bool, error) {
	p := c.owner
	m := p.manager
	target, err := p.variable.Target()
	if err != nil {
		return false, err
	}
	if s == target {
		result := t
		sort, err := m.GetSort(s)
		if err != nil {
			return false, err
		}
		rangeSort, err := m.ArrayRange(sort)
		if err != nil {
			return false, err
		}
		for _, index := range *indices {
			variable, err := c.hold(m.MkFreshConst("value", rangeSort))
			if err != nil {
				return false, err
			}
			if err = p.appendRef(additional, variable); err != nil {
				return false, err
			}
			args := append([]ast.ASTHandle{s}, index.expressions...)
			sel, err := c.hold(m.MkArrayAppArguments(p.family, ast.ASTArrayOpSelect, ast.SliceArguments(args)))
			if err != nil {
				return false, err
			}
			value, err := c.eval(sel)
			if err != nil {
				return false, err
			}
			decl, err := m.GetDecl(variable)
			if err != nil {
				return false, err
			}
			if err = c.model.RegisterConstant(decl, value); err != nil {
				return false, err
			}
			args[0] = result
			args = append(args, variable)
			result, err = c.hold(m.MkArrayAppArguments(p.family, ast.ASTArrayOpStore, ast.SliceArguments(args)))
			if err != nil {
				return false, err
			}
		}
		sub, err := c.replacement()
		if err != nil {
			return false, err
		}
		if err = sub.Insert(s, result); err != nil {
			return false, err
		}
		for _, r := range *lits {
			if err = sub.ApplyRef(r); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	args, store, err := p.operation(s, p.family, ast.ASTArrayOpStore)
	if err != nil || !store {
		return false, err
	}
	if len(args) < 3 {
		return false, ast.ErrASTDomain
	}
	n := len(args) - 2
	index, err := c.indices(args[1 : n+1])
	if err != nil {
		return false, err
	}
	// Preserve source's loop bound, which checks all but the final index.
	for i := 1; i < n; i++ {
		contains, err := p.variable.Check(args[i])
		if err != nil {
			return false, err
		}
		if contains {
			return false, nil
		}
	}
	comparison, previous, err := p.containsIndices(index, *indices)
	if err != nil {
		return false, err
	}
	switch comparison {
	case lbool.True:
		eqs, err := c.equalIndices(index, (*indices)[previous])
		if err != nil {
			return false, err
		}
		for _, eq := range eqs {
			if err = p.appendRef(lits, eq); err != nil {
				return false, err
			}
		}
	case lbool.False:
		for _, earlier := range *indices {
			eqs, err := c.equalIndices(index, earlier)
			if err != nil {
				return false, err
			}
			conjunction, err := c.hold(m.MkAndArray(eqs))
			if err != nil {
				return false, err
			}
			negative, err := c.hold(m.MkNot(conjunction))
			if err != nil {
				return false, err
			}
			if err = p.appendRef(lits, negative); err != nil {
				return false, err
			}
		}
		selectArgs := append([]ast.ASTHandle{t}, args[1:n+1]...)
		sel, err := c.hold(m.MkArrayAppArguments(p.family, ast.ASTArrayOpSelect, ast.SliceArguments(selectArgs)))
		if err != nil {
			return false, err
		}
		eq, err := c.hold(m.MkEq(sel, args[n+1]))
		if err != nil {
			return false, err
		}
		if err = p.appendRef(lits, eq); err != nil {
			return false, err
		}
		*indices = append(*indices, index)
	case lbool.Undef:
		return false, nil
	}
	kind, err := m.GetKind(args[0])
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	return c.solveStore(args[0], t, indices, additional, lits)
}

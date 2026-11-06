package qe

// Source: src/qe/qe_bool_plugin.cpp bool_plugin::solve, solve_units and
// solve_polarized(expr*). These callbacks choose a variable over the complete
// context, then publish through i_solver_context::elim_var.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// SolveContext performs one source Boolean elimination. Unit conjunct order
// takes priority over context variable order. Without a unit, the first
// eligible Boolean variable in the context is eliminated by polarity.
func (p *BoolPlugin) SolveContext(conjs *ConjunctionEnum, formula ast.ASTHandle, atoms *BoolAtomIndex) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if conjs == nil || conjs.manager != p.manager || atoms == nil || atoms.manager != p.manager {
		return false, ast.ErrASTHandle
	}
	if err := atoms.open(); err != nil {
		return false, err
	}
	count, err := conjs.Len()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		expression, err := conjs.At(i)
		if err != nil {
			return false, err
		}
		kind, err := p.manager.GetKind(expression)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		target := expression
		truth := true
		index, found, err := p.context.IsVariable(target)
		if err != nil {
			return false, err
		}
		if !found {
			family, err := p.manager.GetFamilyID(expression)
			if err != nil {
				return false, err
			}
			op, err := p.manager.GetDeclKind(expression)
			if err != nil {
				return false, err
			}
			if family != p.manager.BasicFamilyID() || op != ast.ASTOpNot {
				continue
			}
			target, err = p.manager.GetArg(expression, 0)
			if err != nil {
				return false, err
			}
			index, found, err = p.context.IsVariable(target)
			if err != nil {
				return false, err
			}
			if !found {
				continue
			}
			truth = false
		}
		value, err := p.manager.MkBoolVal(truth)
		if err != nil {
			return false, err
		}
		result, err := p.replaceAndOwn(target, value, formula)
		if err != nil {
			return false, err
		}
		err = errors.Join(p.publishContextElimination(index, result), result.Close())
		return err == nil, err
	}
	count, err = p.context.NumVariables()
	if err != nil {
		return false, err
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		target, err := p.context.Variable(i)
		if err != nil {
			return false, err
		}
		sort, err := p.manager.GetSort(target)
		if err != nil {
			return false, err
		}
		if sort != boolean {
			continue
		}
		contains, err := p.context.Contains(i)
		if err != nil {
			return false, err
		}
		result, handled, err := p.solvePolarized(contains, formula, atoms)
		if err != nil {
			return false, err
		}
		if !handled {
			continue
		}
		err = errors.Join(p.publishContextElimination(i, result), result.Close())
		return err == nil, err
	}
	return false, nil
}

func (p *BoolPlugin) publishContextElimination(index uint32, result *BoolElimination) error {
	formula, err := p.manager.RefGet(result.Formula)
	if err != nil {
		return err
	}
	definition, err := p.manager.RefGet(result.Definition)
	if err != nil {
		return err
	}
	return p.context.EliminateVariable(index, formula, definition)
}

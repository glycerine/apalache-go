package qe

// Source: src/qe/qe.cpp, simplify_solver_context::solve and simplify_exists.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type qeSimplifyResult struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables []ast.ASTRef
}

func (r *qeSimplifyResult) Close() error {
	if r == nil || r.manager == nil {
		return nil
	}
	for len(r.variables) != 0 {
		i := len(r.variables) - 1
		if err := r.manager.DelRef(&r.variables[i]); err != nil {
			return err
		}
		r.variables = r.variables[:i]
	}
	if r.formula != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&r.formula); err != nil {
			return err
		}
	}
	r.manager = nil
	return nil
}

// simplifyExists solves source Boolean units and arithmetic equations to a
// fixed point. Inputs are borrowed; the result owns its formula and variables.
func simplifyExists(m *ast.ASTManager, variables []ast.ASTHandle, formula ast.ASTHandle) (result *qeSimplifyResult, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	result = &qeSimplifyResult{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	context, err := NewSolverContext(m, variables)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, context.Close()) }()
	boolean, err := NewBoolPlugin(context)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, boolean.Close()) }()
	arithmetic, err := NewArithmeticPlugin(context)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, arithmetic.Close()) }()
	atoms, err := NewBoolAtomIndex(m, nil, nil)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, atoms.Close()) }()
	current, err := m.NewRef(formula)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&current)) }()
	for {
		canceled, e := m.Canceled()
		if e != nil {
			return result, e
		}
		if canceled {
			return result, tactic.NewTacticException(m.ResourceLimit().CancelMessage())
		}
		f, e := m.RefGet(current)
		if e != nil {
			return result, e
		}
		conjs, e := NewConjunctionEnum(m, f)
		if e != nil {
			return result, e
		}
		solved, e := boolean.SolveContext(conjs, f, atoms)
		if e == nil && !solved {
			solved, e = arithmetic.Solve(conjs, f)
			if e == nil && solved {
				var equation *ArithmeticEquationElimination
				equation, e = arithmetic.TakeEquationElimination()
				if e == nil && equation == nil {
					e = ast.ErrASTDomain
				}
				if e == nil {
					e = publishSimplifyArithmetic(m, context, equation)
				}
				if equation != nil {
					e = errors.Join(e, equation.Close())
				}
			}
		}
		e = errors.Join(e, conjs.Close())
		if e != nil {
			return result, e
		}
		if !solved {
			break
		}
		f, ok, e := context.CurrentFormula()
		if e != nil {
			return result, e
		}
		if !ok {
			return result, ast.ErrASTDomain
		}
		if err = m.SetRef(current, f); err != nil {
			return result, err
		}
	}
	result.formula, err = m.CopyRef(current)
	if err != nil {
		return result, err
	}
	remaining, err := context.Variables()
	if err != nil {
		return result, err
	}
	for _, v := range remaining {
		ref, e := m.NewRef(v)
		if e != nil {
			return result, e
		}
		result.variables = append(result.variables, ref)
	}
	return result, nil
}

func publishSimplifyArithmetic(m *ast.ASTManager, context *SolverContext, equation *ArithmeticEquationElimination) error {
	for _, fresh := range equation.Fresh {
		ref, err := m.CopyRef(fresh)
		if err != nil {
			return err
		}
		h, err := m.RefGet(ref)
		if err == nil {
			err = context.adoptVariable(h, &ref)
		}
		if ref != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&ref))
		}
		if err != nil {
			return err
		}
	}
	formula, err := m.RefGet(equation.Formula)
	if err != nil {
		return err
	}
	definition, err := m.RefGet(equation.Definition)
	if err != nil {
		return err
	}
	return context.EliminateVariable(equation.VariableIndex, formula, definition)
}

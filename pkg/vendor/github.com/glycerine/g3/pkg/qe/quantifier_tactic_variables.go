package qe

// Source: src/qe/qe.cpp, quant_elim_new::eliminate_exists when
// eliminate_variables_as_block is false.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// rewriteQuantifierVariablesIndividually eliminates declarations in source
// order. Declarations a theory plugin cannot eliminate remain bound together
// after all later declarations have had their turn.
func rewriteQuantifierVariablesIndividually(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	if m == nil || config.Cooperation == nil {
		return result, false, ast.ErrASTHandle
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	extracted, err := extractExprQuantifier(m, quantifier)
	if err != nil {
		return result, false, closeFailedExprQuantifierExtract(extracted, err)
	}
	defer func() { err = errors.Join(err, extracted.Close()) }()
	forall, err := m.IsForall(quantifier)
	if err != nil {
		return result, false, err
	}
	body, err := m.RefGet(extracted.body)
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNot(body)
		if err != nil {
			return result, false, err
		}
	}
	current, err := m.NewRef(body)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&current)) }()
	remaining := make([]ast.ASTHandle, 0, len(extracted.variables))
	for variableIndex, variableRef := range extracted.variables {
		variable, e := m.RefGet(variableRef)
		if e != nil {
			return result, false, e
		}
		formula, e := m.RefGet(current)
		if e != nil {
			return result, false, e
		}
		bound, e := bindExprQuantifierVariables(m, []ast.ASTHandle{variable}, formula)
		if e != nil {
			return result, false, e
		}
		boundFormula, e := m.RefGet(bound)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		kind, e := m.GetKind(boundFormula)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		if kind != ast.ASTQuantifier {
			// The declaration is absent from the current formula, so the
			// binder correctly omitted it.
			if e = m.SetRef(current, boundFormula); e != nil {
				return result, false, errors.Join(e, m.DelRef(&bound))
			}
			if e = m.DelRef(&bound); e != nil {
				return result, false, e
			}
			continue
		}
		part, done, e := rewriteQuantifierScalar(m, boundFormula, config)
		e = errors.Join(e, m.DelRef(&bound))
		if e != nil {
			if part != (ast.ASTRef{}) {
				e = errors.Join(e, m.DelRef(&part))
			}
			return result, false, e
		}
		if !done {
			remaining = append(remaining, variable)
			for _, later := range extracted.variables[variableIndex+1:] {
				value, e := m.RefGet(later)
				if e != nil {
					return result, false, e
				}
				remaining = append(remaining, value)
			}
			break
		}
		value, e := m.RefGet(part)
		if e == nil {
			e = m.SetRef(current, value)
		}
		e = errors.Join(e, m.DelRef(&part))
		if e != nil {
			return result, false, e
		}
	}
	formula, err := m.RefGet(current)
	if err != nil {
		return result, false, err
	}
	if len(remaining) > 0 {
		bound, e := bindExprQuantifierVariables(m, remaining, formula)
		if e != nil {
			return result, false, e
		}
		formula, e = m.RefGet(bound)
		err = errors.Join(e, m.DelRef(&bound))
		if err != nil {
			return result, false, err
		}
	}
	if forall {
		formula, err = m.MkNotSimplified(formula)
		if err != nil {
			return result, false, err
		}
	}
	result, err = m.NewRef(formula)
	return result, err == nil, err
}

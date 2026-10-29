package interp

// Source checkpoint: src/interp/iz3mgr.cpp solve_arith, cont_eq, apply_quant,
// and close_universally.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func (m *Manager) solveArithmetic(variable, left, right ast.ASTHandle) (ast.ASTRef, bool, error) {
	found, err := m.Occurs(variable, right)
	if err != nil || found {
		return ast.ASTRef{}, false, err
	}
	op, err := m.Operator(left)
	if err != nil {
		return ast.ASTRef{}, false, err
	}
	if op != OperatorPlus {
		return ast.ASTRef{}, false, nil
	}
	count, err := m.NumArgs(left)
	if err != nil {
		return ast.ASTRef{}, false, err
	}
	for i := uint32(0); i < count; i++ {
		arg, e := m.Arg(left, i)
		if e != nil {
			return ast.ASTRef{}, false, e
		}
		if arg != variable {
			continue
		}
		remainder, e := m.Make(OperatorSub, left, variable)
		if e != nil {
			return ast.ASTRef{}, false, e
		}
		solution, e := m.Make(OperatorSub, right, remainder)
		if e != nil {
			return ast.ASTRef{}, false, e
		}
		result, e := m.Simplify(solution)
		return result, e == nil, e
	}
	return ast.ASTRef{}, false, nil
}

func (m *Manager) controllingEquality(memo map[ast.ASTHandle]bool, truth bool, variable, expression ast.ASTHandle) (ast.ASTRef, bool, error) {
	isNot, body, err := m.ast.IsNot(expression)
	if err != nil {
		return ast.ASTRef{}, false, err
	}
	if isNot {
		return m.controllingEquality(memo, !truth, variable, body)
	}
	if memo[expression] {
		return ast.ASTRef{}, false, nil
	}
	memo[expression] = true
	op, err := m.Operator(expression)
	if err != nil {
		return ast.ASTRef{}, false, err
	}
	if !truth && op == OperatorEqual {
		left, e := m.Arg(expression, 0)
		if e != nil {
			return ast.ASTRef{}, false, e
		}
		right, e := m.Arg(expression, 1)
		if e != nil {
			return ast.ASTRef{}, false, e
		}
		if left == variable {
			occurs, e := m.Occurs(variable, right)
			if e != nil {
				return ast.ASTRef{}, false, e
			}
			if !occurs {
				result, e := m.ast.NewRef(right)
				return result, e == nil, e
			}
		}
		if right == variable {
			occurs, e := m.Occurs(variable, left)
			if e != nil {
				return ast.ASTRef{}, false, e
			}
			if !occurs {
				result, e := m.ast.NewRef(left)
				return result, e == nil, e
			}
		}
		if result, solved, e := m.solveArithmetic(variable, left, right); e != nil || solved {
			return result, solved, e
		}
		if result, solved, e := m.solveArithmetic(variable, right, left); e != nil || solved {
			return result, solved, e
		}
	}
	if (!truth && op == OperatorAnd) || (truth && op == OperatorOr) {
		count, e := m.NumArgs(expression)
		if e != nil {
			return ast.ASTRef{}, false, e
		}
		for i := uint32(0); i < count; i++ {
			arg, e := m.Arg(expression, i)
			if e != nil {
				return ast.ASTRef{}, false, e
			}
			if result, found, e := m.controllingEquality(memo, truth, variable, arg); e != nil || found {
				return result, found, e
			}
		}
	}
	if truth && op == OperatorImplies {
		antecedent, e := m.Arg(expression, 0)
		if e != nil {
			return ast.ASTRef{}, false, e
		}
		if result, found, e := m.controllingEquality(memo, false, variable, antecedent); e != nil || found {
			return result, found, e
		}
		consequent, e := m.Arg(expression, 1)
		if e != nil {
			return ast.ASTRef{}, false, e
		}
		return m.controllingEquality(memo, true, variable, consequent)
	}
	return ast.ASTRef{}, false, nil
}

func (m *Manager) ApplyQuantifier(quantifier Operator, variable, expression ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if quantifier != OperatorForall && quantifier != OperatorExists {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	op, err := m.Operator(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if quantifier == OperatorForall && op == OperatorAnd || quantifier == OperatorExists && op == OperatorOr {
		count, e := m.NumArgs(expression)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args := make([]ast.ASTHandle, count)
		for i := range args {
			child, e := m.Arg(expression, uint32(i))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			args[i], e = m.ApplyQuantifier(quantifier, variable, child)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		return m.Make(op, args...)
	}
	occurs, err := m.Occurs(variable, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !occurs {
		return expression, nil
	}
	term, found, err := m.controllingEquality(make(map[ast.ASTHandle]bool), quantifier == OperatorForall, variable, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if found {
		defer func() { resultErr = errors.Join(resultErr, m.ast.DelRef(&term)) }()
		replacement, e := m.ast.RefGet(term)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return m.SubstituteOne(variable, replacement, expression)
	}
	return m.MakeQuantifier(quantifier, []ast.ASTHandle{variable}, expression)
}

func (m *Manager) CloseUniversally(expression ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	used, err := ast.NewASTUsedVars(m.ast)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = used.Collect(expression); err != nil {
		return ast.ASTHandle{}, err
	}
	var owned []ast.ASTRef
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.ast.DelRef(&owned[i]))
		}
	}()
	bound := make([]ast.ASTHandle, 0, used.NumVars())
	replacements := make(map[ast.ASTHandle]ast.ASTHandle)
	for i := uint32(0); i < used.MaxFoundVarIndexPlusOne(); i++ {
		sort := used.Contains(i)
		if sort.IsNull() {
			continue
		}
		constant, e := m.MakeVariable(fmt.Sprintf("%%%%%d", i), sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		constantRef, e := m.ast.NewRef(constant)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		owned = append(owned, constantRef)
		variable, e := m.ast.MkVar(i, sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		variableRef, e := m.ast.NewRef(variable)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		owned = append(owned, variableRef)
		replacements[variable] = constant
		bound = append(bound, constant)
	}
	result, err = m.Substitute(replacements, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, variable := range bound {
		result, err = m.ApplyQuantifier(OperatorForall, variable, result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

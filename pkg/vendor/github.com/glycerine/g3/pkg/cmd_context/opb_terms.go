package cmd_context

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
)

func (c *Context) makeOPBTerm(term *opt.OPBTerm, declarations map[int32]ast.ASTHandle, zero ast.ASTHandle) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	literals := make([]ast.ASTHandle, len(term.Literals))
	for i, literal := range term.Literals {
		value, e := c.makeOPBVariable(literal.Variable, declarations)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if literal.Negative {
			value, e = manager.MkNot(value)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		literals[i] = value
	}
	condition, err := manager.MkAndArray(literals)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	coefficient := new(big.Rat).SetInt(&term.Coefficient)
	value, err := manager.ArithmeticNumeralRat(coefficient, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkIte(condition, value, zero)
}

func (c *Context) makeOPBSum(terms []opt.OPBTerm, minimizeSyntax bool, declarations map[int32]ast.ASTHandle, zero ast.ASTHandle) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := c.makeOPBTerm(&terms[0], declarations, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := 1; i < len(terms); i++ {
		next, e := c.makeOPBTerm(&terms[i], declarations, zero)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if minimizeSyntax {
			result, e = manager.MkArithmeticAdd(result, next)
		} else {
			// The retained parser represents max: by subtracting every term
			// after the first and registering the resulting minimization.
			result, e = manager.MkArithmeticSub(result, next)
		}
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return result, nil
}

// A single indicator term can only evaluate to zero or its coefficient. This
// Boolean reduction avoids sending a one-term ITE bound to the optimizer.
func (c *Context) makeSingleOPBConstraint(constraint *opt.OPBConstraint, declarations map[int32]ast.ASTHandle) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	term := &constraint.Terms[0]
	literals := make([]ast.ASTHandle, len(term.Literals))
	for i, literal := range term.Literals {
		value, e := c.makeOPBVariable(literal.Variable, declarations)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if literal.Negative {
			value, e = m.MkNot(value)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		literals[i] = value
	}
	condition, err := m.MkAndArray(literals)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	compare := func(sign int) (bool, error) {
		switch constraint.Relation {
		case opt.OPBGreaterEqual:
			return sign >= 0, nil
		case opt.OPBLessEqual:
			return sign <= 0, nil
		case opt.OPBEqual:
			return sign == 0, nil
		default:
			return false, ast.ErrASTDomain
		}
	}
	whenFalse, err := compare(-constraint.Bound.Sign())
	if err != nil {
		return ast.ASTHandle{}, err
	}
	whenTrue, err := compare(term.Coefficient.Cmp(&constraint.Bound))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if whenFalse && whenTrue {
		return m.MkTrue()
	}
	if !whenFalse && !whenTrue {
		return m.MkFalse()
	}
	if whenTrue {
		return condition, nil
	}
	return m.MkNot(condition)
}

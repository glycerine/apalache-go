package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util::mk_atom.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (u *ArithmeticQETermUtil) relation(expression ast.ASTHandle) (kind int32, left, right ast.ASTHandle, arithmetic bool, err error) {
	family, err := u.manager.GetFamilyID(expression)
	if err != nil {
		return 0, left, right, false, err
	}
	kind, err = u.manager.GetDeclKind(expression)
	if err != nil {
		return 0, left, right, false, err
	}
	if family != u.family || kind < 2 || kind > 5 {
		return kind, left, right, false, nil
	}
	left, err = u.manager.GetArg(expression, 0)
	if err == nil {
		right, err = u.manager.GetArg(expression, 1)
	}
	return kind, left, right, err == nil, err
}

func (u *ArithmeticQETermUtil) strictZero(expression ast.ASTHandle) (ast.ASTRef, error) {
	integer, err := u.isInteger(expression)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !integer {
		return u.comparisonZero(expression, true)
	}
	minusOne, err := u.manager.ArithmeticIntegerNumeralText("-1")
	if err != nil {
		return ast.ASTRef{}, err
	}
	comparison, err := u.manager.MkArithmeticLE(expression, minusOne)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return u.manager.NewRef(comparison)
}

func (u *ArithmeticQETermUtil) pairBoolean(left, right ast.ASTRef, conjunction bool) (result ast.ASTRef, resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&right), u.manager.DelRef(&left)) }()
	lh, err := u.manager.RefGet(left)
	if err != nil {
		return result, err
	}
	rh, err := u.manager.RefGet(right)
	if err != nil {
		return result, err
	}
	var combined ast.ASTHandle
	if conjunction {
		combined, err = u.manager.MkAnd(lh, rh)
	} else {
		combined, err = u.manager.MkOrArray([]ast.ASTHandle{lh, rh})
	}
	if err != nil {
		return result, err
	}
	return u.manager.NewRef(combined)
}

// NormalizeAtom retains equalities and converts arithmetic atoms to the
// source zero-comparison normal forms for the requested polarity.
func (u *ArithmeticQETermUtil) NormalizeAtom(expression ast.ASTHandle, positive bool) (result ast.ASTRef, handled bool, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, false, resultErr
	}
	if _, _, divides, err := u.MatchDivides(expression); err != nil {
		return result, false, err
	} else if divides {
		if positive {
			result, err = u.manager.NewRef(expression)
			return result, err == nil, err
		}
		negative, err := u.manager.MkNot(expression)
		if err != nil {
			return result, false, err
		}
		result, err = u.manager.NewRef(negative)
		return result, err == nil, err
	}
	args, equality, err := u.operation(expression, u.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil {
		return result, false, err
	}
	if equality && len(args) == 2 {
		sort, cause := u.manager.GetSort(args[0])
		if cause != nil {
			return result, false, cause
		}
		family, cause := u.manager.GetFamilyID(sort)
		if cause != nil || family != u.family {
			return result, false, cause
		}
		forward, cause := u.manager.MkArithmeticSub(args[0], args[1])
		if cause != nil {
			return result, false, cause
		}
		reverse, cause := u.manager.MkArithmeticSub(args[1], args[0])
		if cause != nil {
			return result, false, cause
		}
		if positive {
			left, cause := u.comparisonZero(forward, false)
			if cause != nil {
				return result, false, cause
			}
			right, cause := u.comparisonZero(reverse, false)
			if cause != nil {
				return result, false, errors.Join(cause, u.manager.DelRef(&left))
			}
			result, cause = u.pairBoolean(left, right, true)
			return result, cause == nil, cause
		}
		left, cause := u.strictZero(forward)
		if cause != nil {
			return result, false, cause
		}
		right, cause := u.strictZero(reverse)
		if cause != nil {
			return result, false, errors.Join(cause, u.manager.DelRef(&left))
		}
		result, cause = u.pairBoolean(left, right, false)
		return result, cause == nil, cause
	}
	kind, left, right, relation, err := u.relation(expression)
	if err != nil || !relation {
		return result, false, err
	}
	if positive && (kind == 2 || kind == 3) {
		result, err = u.manager.NewRef(expression)
		return result, err == nil, err
	}
	if kind == 3 || kind == 5 {
		left, right = right, left
	}
	if !positive {
		left, right = right, left
	}
	difference, err := u.manager.MkArithmeticSub(left, right)
	if err != nil {
		return result, false, err
	}
	strict := positive == (kind == 4 || kind == 5)
	if strict {
		result, err = u.strictZero(difference)
	} else {
		result, err = u.comparisonZero(difference, false)
	}
	return result, err == nil, err
}

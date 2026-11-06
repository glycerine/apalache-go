package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util term decomposition.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ArithmeticQETermUtil is the dependency-closed arithmetic term and bound
// foundation used by the retained arithmetic QE plugin.
type ArithmeticQETermUtil struct {
	self     *ArithmeticQETermUtil
	manager  *ast.ASTManager
	family   ast.FamilyID
	theory   *rewriter.TheoryRewriter
	qeConfig *Config
	closed   bool
}

func NewArithmeticQETermUtil(manager *ast.ASTManager) (*ArithmeticQETermUtil, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	u := &ArithmeticQETermUtil{manager: manager, family: family}
	u.self = u
	return u, nil
}

func (u *ArithmeticQETermUtil) open() error {
	if u == nil || u.self != u || u.manager == nil {
		return ast.ErrASTHandle
	}
	if u.closed {
		return arena.ErrClosed
	}
	return nil
}

func (u *ArithmeticQETermUtil) Close() error {
	if u == nil || u.self != u {
		return ast.ErrASTHandle
	}
	if u.closed {
		return nil
	}
	u.theory = nil
	u.qeConfig = nil
	u.closed = true
	return nil
}

func (u *ArithmeticQETermUtil) operation(expression ast.ASTHandle, family ast.FamilyID, kind int32) ([]ast.ASTHandle, bool, error) {
	actualFamily, err := u.manager.GetFamilyID(expression)
	if err != nil {
		return nil, false, err
	}
	actualKind, err := u.manager.GetDeclKind(expression)
	if err != nil || actualFamily != family || actualKind != kind {
		return nil, false, err
	}
	count, err := u.manager.GetNumArgs(expression)
	if err != nil {
		return nil, false, err
	}
	args := make([]ast.ASTHandle, count)
	for i := range args {
		args[i], err = u.manager.GetArg(expression, uint32(i))
		if err != nil {
			return nil, false, err
		}
	}
	return args, true, nil
}

func (u *ArithmeticQETermUtil) numeral(expression ast.ASTHandle) (*big.Rat, bool, error) {
	text, _, matched, err := u.manager.ArithmeticNumeralText(expression)
	if err != nil || !matched {
		return nil, matched, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return value, true, nil
}

func (u *ArithmeticQETermUtil) isInteger(expression ast.ASTHandle) (bool, error) {
	sort, err := u.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	family, err := u.manager.GetFamilyID(sort)
	if err != nil || family != u.family {
		return false, errors.Join(ast.ErrASTDomain, err)
	}
	kind, err := u.manager.GetDeclKind(sort)
	return kind == 1, err
}

func (u *ArithmeticQETermUtil) zero(expression ast.ASTHandle) (ast.ASTHandle, error) {
	integer, err := u.isInteger(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if integer {
		return u.manager.ArithmeticIntegerNumeralText("0")
	}
	return u.manager.ArithmeticRealNumeralText("0")
}

func (u *ArithmeticQETermUtil) numeralFor(value *big.Rat, expression ast.ASTHandle) (ast.ASTHandle, error) {
	if value == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	integer, err := u.isInteger(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if integer {
		if !value.IsInt() {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return u.manager.ArithmeticIntegerNumeralText(value.RatString())
	}
	return u.manager.ArithmeticRealNumeralText(value.RatString())
}

func (u *ArithmeticQETermUtil) sum(terms []ast.ASTHandle, sortWitness ast.ASTHandle) (result ast.ASTRef, err error) {
	if len(terms) == 0 {
		zero, cause := u.zero(sortWitness)
		if cause != nil {
			return result, cause
		}
		return u.manager.NewRef(zero)
	}
	result, err = u.manager.NewRef(terms[0])
	if err != nil {
		return result, err
	}
	for _, term := range terms[1:] {
		current, cause := u.manager.RefGet(result)
		if cause != nil {
			return result, errors.Join(cause, u.manager.DelRef(&result))
		}
		combined, cause := u.manager.MkArithmeticAdd(current, term)
		if cause != nil {
			return result, errors.Join(cause, u.manager.DelRef(&result))
		}
		if cause = u.manager.SetRef(result, combined); cause != nil {
			return result, errors.Join(cause, u.manager.DelRef(&result))
		}
	}
	return result, nil
}

// Coefficient matches p = k*x + rest using the source LIFO flattening order.
// Rest is an owning reference when matched.
func (u *ArithmeticQETermUtil) Coefficient(variable *ast.ContainsApp, polynomial ast.ASTHandle) (coefficient *big.Rat, rest ast.ASTRef, matched bool, err error) {
	if err = u.open(); err != nil {
		return nil, rest, false, err
	}
	if variable == nil {
		return nil, rest, false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return nil, rest, false, err
	}
	todo := []ast.ASTHandle{polynomial}
	remainder := make([]ast.ASTHandle, 0, 4)
	for len(todo) != 0 {
		expression := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if args, add, cause := u.operation(expression, u.family, 6); cause != nil {
			return nil, rest, false, cause
		} else if add {
			todo = append(todo, args...)
			continue
		}
		if expression == target {
			coefficient = big.NewRat(1, 1)
			matched = true
			break
		}
		args, multiply, cause := u.operation(expression, u.family, 9)
		if cause != nil {
			return nil, rest, false, cause
		}
		if multiply && len(args) == 2 {
			if args[0] == target {
				if value, numeral, cause := u.numeral(args[1]); cause != nil {
					return nil, rest, false, cause
				} else if numeral {
					coefficient, matched = value, true
					break
				}
			}
			if args[1] == target {
				if value, numeral, cause := u.numeral(args[0]); cause != nil {
					return nil, rest, false, cause
				} else if numeral {
					coefficient, matched = value, true
					break
				}
			}
		}
		remainder = append(remainder, expression)
	}
	if !matched {
		return nil, rest, false, nil
	}
	for len(todo) != 0 {
		remainder = append(remainder, todo[len(todo)-1])
		todo = todo[:len(todo)-1]
	}
	rest, err = u.sum(remainder, target)
	if err != nil {
		return nil, rest, false, err
	}
	restHandle, err := u.manager.RefGet(rest)
	if err != nil {
		return nil, rest, false, errors.Join(err, u.manager.DelRef(&rest))
	}
	contains, err := variable.Check(restHandle)
	if err != nil || contains {
		return nil, rest, false, errors.Join(err, u.manager.DelRef(&rest))
	}
	return coefficient, rest, true, nil
}

// Constant extracts the first numeral in source LIFO order and owns rest.
func (u *ArithmeticQETermUtil) Constant(polynomial ast.ASTHandle) (constant *big.Rat, rest ast.ASTRef, err error) {
	if err = u.open(); err != nil {
		return nil, rest, err
	}
	todo := []ast.ASTHandle{polynomial}
	remainder := make([]ast.ASTHandle, 0, 4)
	constant = new(big.Rat)
	for len(todo) != 0 {
		expression := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if args, add, cause := u.operation(expression, u.family, 6); cause != nil {
			return nil, rest, cause
		} else if add {
			todo = append(todo, args...)
			continue
		}
		value, numeral, cause := u.numeral(expression)
		if cause != nil {
			return nil, rest, cause
		}
		if numeral {
			constant.Set(value)
			break
		}
		remainder = append(remainder, expression)
	}
	for len(todo) != 0 {
		remainder = append(remainder, todo[len(todo)-1])
		todo = todo[:len(todo)-1]
	}
	rest, err = u.sum(remainder, polynomial)
	return constant, rest, err
}

package rel

// Source: src/muz/rel/dl_bound_relation.cpp filter_interpreted_fn.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// FilterIdentical merges the source's requested column equivalence classes.
// The first column is equated with every later column in input order.
func (r *BoundRelation) FilterIdentical(columns []uint32) error {
	if r == nil {
		return ast.ErrASTDomain
	}
	for _, column := range columns {
		if uint64(column) >= uint64(len(r.columns)) {
			return ast.ErrASTDomain
		}
	}
	if len(columns) < 2 {
		return nil
	}
	for _, column := range columns[1:] {
		if err := r.Equate(columns[0], column); err != nil {
			return err
		}
	}
	return nil
}

// FilterEqual preserves the source bound relation's coarse abstraction: its
// filter_equal mutator accepts the request but does not refine any bounds.
func (r *BoundRelation) FilterEqual(_ Element, _ uint32) {}

// FilterInterpreted applies source strict and non-strict comparisons between
// column variables. Unsupported formula shapes return false without mutation.
func (r *BoundRelation) FilterInterpreted(m *ast.ASTManager, condition ast.ASTHandle) (bool, error) {
	if r == nil || m == nil {
		return false, ast.ErrASTDomain
	}
	if m.IsFalse(condition) {
		r.reset()
		return true, nil
	}
	negated, arg, err := m.IsNot(condition)
	if err != nil {
		return false, err
	}
	if negated {
		condition = arg
	}
	arithmetic, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return false, err
	}
	family, op, left, right, binary, err := intervalParts(m, condition)
	if err != nil || !binary {
		return false, err
	}
	if !negated && family == m.BasicFamilyID() && op == ast.ASTOpEq {
		leftKind, e := m.GetKind(left)
		if e != nil {
			return false, e
		}
		rightKind, e := m.GetKind(right)
		if e != nil {
			return false, e
		}
		if leftKind == ast.ASTVar && rightKind == ast.ASTVar {
			i, e := m.GetVarIndex(left)
			if e != nil {
				return false, e
			}
			j, e := m.GetVarIndex(right)
			if e != nil {
				return false, e
			}
			return true, r.Equate(i, j)
		}
		// The source recognizes x = z - y (in either orientation), but its
		// EQ_SUB branch is explicitly a TODO and does not refine the relation.
		for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
			variable, difference := pair[0], pair[1]
			variableKind, e := m.GetKind(variable)
			if e != nil {
				return false, e
			}
			if variableKind != ast.ASTVar {
				continue
			}
			subFamily, subOp, minuend, subtrahend, sub, e := intervalParts(m, difference)
			if e != nil {
				return false, e
			}
			if !sub || subFamily != arithmetic || subOp != 7 {
				continue
			}
			minuendKind, e := m.GetKind(minuend)
			if e != nil {
				return false, e
			}
			subtrahendKind, e := m.GetKind(subtrahend)
			if e != nil {
				return false, e
			}
			if minuendKind == ast.ASTVar && subtrahendKind == ast.ASTVar {
				return true, nil
			}
		}
		if leftKind != ast.ASTVar {
			return false, nil
		}
		addFamily, addOp, a, b, add, e := intervalParts(m, right)
		if e != nil || !add {
			return false, e
		}
		if addFamily != arithmetic || addOp != 6 {
			return false, nil
		}
		for _, pair := range [][2]ast.ASTHandle{{a, b}, {b, a}} {
			text, _, numeral, e := m.ArithmeticNumeralText(pair[0])
			if e != nil {
				return false, e
			}
			if !numeral {
				continue
			}
			constant, ok := new(big.Rat).SetString(text)
			if !ok {
				return false, ast.ErrASTDomain
			}
			if constant.Sign() <= 0 {
				continue
			}
			kind, e := m.GetKind(pair[1])
			if e != nil {
				return false, e
			}
			if kind != ast.ASTVar {
				continue
			}
			smaller, e := m.GetVarIndex(pair[1])
			if e != nil {
				return false, e
			}
			larger, e := m.GetVarIndex(left)
			if e != nil {
				return false, e
			}
			return true, r.AddLess(smaller, larger)
		}
		return false, nil
	}
	if family != arithmetic || op < 2 || op > 5 {
		return false, nil
	}
	leftKind, err := m.GetKind(left)
	if err != nil {
		return false, err
	}
	rightKind, err := m.GetKind(right)
	if err != nil {
		return false, err
	}
	if leftKind != ast.ASTVar || rightKind != ast.ASTVar {
		return false, nil
	}
	i, err := m.GetVarIndex(left)
	if err != nil {
		return false, err
	}
	j, err := m.GetVarIndex(right)
	if err != nil {
		return false, err
	}
	strict := op == 4 || op == 5
	if op == 3 || op == 5 {
		i, j = j, i
	}
	if negated {
		i, j, strict = j, i, !strict
	}
	if strict {
		err = r.AddLess(i, j)
	} else {
		err = r.AddLessEqual(i, j)
	}
	return true, err
}

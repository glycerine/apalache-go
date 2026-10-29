package arith

// Source: the retained bounds_arith_subsumption instantiation in
// src/tactic/arith/arith_bounds_tactic.cpp.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type arithmeticBoundInfo struct {
	value  big.Rat
	index  int
	strict bool
}

func arithmeticBoundsApp(manager *ast.ASTManager, expression ast.ASTHandle) (operation int32, left, right ast.ASTHandle, matched bool, err error) {
	kind, err := manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, left, right, false, err
	}
	family, err := manager.GetFamilyID(expression)
	if err != nil {
		return 0, left, right, false, err
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil || family != arithmetic {
		return 0, left, right, false, err
	}
	operation, err = manager.GetDeclKind(expression)
	if err != nil || operation < 2 || operation > 5 {
		return 0, left, right, false, err
	}
	count, err := manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return 0, left, right, false, err
	}
	left, err = manager.GetArg(expression, 0)
	if err != nil {
		return 0, left, right, false, err
	}
	right, err = manager.GetArg(expression, 1)
	return operation, left, right, err == nil, err
}

func arithmeticBoundsRelation(manager *ast.ASTManager, expression ast.ASTHandle) (left, right ast.ASTHandle, strict, matched bool, err error) {
	negated, argument, err := manager.IsNot(expression)
	if err != nil {
		return left, right, false, false, err
	}
	if negated {
		expression = argument
	}
	operation, first, second, matched, err := arithmeticBoundsApp(manager, expression)
	if err != nil || !matched {
		return left, right, false, false, err
	}
	switch operation {
	case 2: // <=
		left, right, strict = first, second, negated
	case 3: // >=
		left, right, strict = second, first, negated
	case 4: // <
		left, right, strict = first, second, !negated
	case 5: // >
		left, right, strict = second, first, !negated
	}
	if negated {
		left, right = right, left
	}
	return left, right, strict, true, nil
}

func arithmeticBoundsNumeral(manager *ast.ASTManager, expression ast.ASTHandle) (value big.Rat, matched bool, err error) {
	text, _, matched, err := manager.ArithmeticNumeralText(expression)
	if err != nil || !matched {
		return value, matched, err
	}
	if _, ok := value.SetString(text); !ok {
		return value, false, ast.ErrASTDomain
	}
	return value, true, nil
}

// ArithmeticBoundsKeep returns the source arithmetic-bound subsumption mask.
// It is the concrete, proof-disabled instantiation used to simplify Farkas
// lemmas: for each term it retains only the strongest lower and upper bound.
func ArithmeticBoundsKeep(manager *ast.ASTManager, formulas []ast.ASTHandle) ([]bool, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	keep := make([]bool, len(formulas))
	for i := range keep {
		keep[i] = true
	}
	lower := make(map[ast.ASTHandle]arithmeticBoundInfo)
	upper := make(map[ast.ASTHandle]arithmeticBoundInfo)
	for index, formula := range formulas {
		canceled, err := manager.Canceled()
		if err != nil {
			return nil, err
		}
		if canceled {
			return nil, z3exception.NewDefaultException(messages.Canceled)
		}
		left, right, strict, matched, err := arithmeticBoundsRelation(manager, formula)
		if err != nil || !matched {
			if err != nil {
				return nil, err
			}
			continue
		}
		if value, numeral, err := arithmeticBoundsNumeral(manager, right); err != nil {
			return nil, err
		} else if numeral {
			previous, found := upper[left]
			if !found || previous.value.Cmp(&value) > 0 || previous.value.Cmp(&value) == 0 && strict && !previous.strict {
				if found {
					keep[previous.index] = false
				}
				upper[left] = arithmeticBoundInfo{value: value, index: index, strict: strict}
			} else {
				keep[index] = false
			}
			continue
		}
		value, numeral, err := arithmeticBoundsNumeral(manager, left)
		if err != nil {
			return nil, err
		}
		if !numeral {
			continue
		}
		previous, found := lower[right]
		if !found || previous.value.Cmp(&value) < 0 || previous.value.Cmp(&value) == 0 && strict && !previous.strict {
			if found {
				keep[previous.index] = false
			}
			lower[right] = arithmeticBoundInfo{value: value, index: index, strict: strict}
		} else {
			keep[index] = false
		}
	}
	return keep, nil
}

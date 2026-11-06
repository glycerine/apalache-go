package pdr

// Source: test_diff_logic, is_difference_logic and is_utvpi_logic in
// src/muz/pdr/pdr_util.cpp.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type arithmeticFragmentClassifier struct {
	manager    *ast.ASTManager
	arithmetic ast.FamilyID
	bitVector  ast.FamilyID
	boolean    ast.ASTHandle
	utvpi      bool
}

func newArithmeticFragmentClassifier(manager *ast.ASTManager, utvpi bool) (*arithmeticFragmentClassifier, error) {
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	bitVector, err := manager.BitVectorUtilityFamilyID()
	if err != nil {
		return nil, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	return &arithmeticFragmentClassifier{manager: manager, arithmetic: arithmetic, bitVector: bitVector, boolean: boolean, utvpi: utvpi}, nil
}

func (c *arithmeticFragmentClassifier) operation(expression ast.ASTHandle, family ast.FamilyID, operation int32) (bool, error) {
	kind, err := c.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	foundFamily, err := c.manager.GetFamilyID(expression)
	if err != nil || foundFamily != family {
		return false, err
	}
	foundOperation, err := c.manager.GetDeclKind(expression)
	return foundOperation == operation, err
}

func (c *arithmeticFragmentClassifier) binaryOperation(expression ast.ASTHandle, family ast.FamilyID, operation int32) (left, right ast.ASTHandle, matched bool, err error) {
	matched, err = c.operation(expression, family, operation)
	if err != nil || !matched {
		return
	}
	count, err := c.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return left, right, false, err
	}
	left, err = c.manager.GetArg(expression, 0)
	if err != nil {
		return left, right, false, err
	}
	right, err = c.manager.GetArg(expression, 1)
	return left, right, err == nil, err
}

func (c *arithmeticFragmentClassifier) numeral(expression ast.ASTHandle) (bool, error) {
	_, _, matched, err := c.manager.ArithmeticNumeralText(expression)
	return matched, err
}

func (c *arithmeticFragmentClassifier) minusOne(expression ast.ASTHandle) (bool, error) {
	text, _, matched, err := c.manager.ArithmeticNumeralText(expression)
	if err != nil || !matched {
		return false, err
	}
	var value big.Rat
	if _, ok := value.SetString(text); !ok {
		return false, ast.ErrASTDomain
	}
	return value.Num().Cmp(big.NewInt(-1)) == 0 && value.Denom().Cmp(big.NewInt(1)) == 0, nil
}

func (c *arithmeticFragmentClassifier) arithmeticExpression(expression ast.ASTHandle) (bool, error) {
	kind, err := c.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := c.manager.GetFamilyID(expression)
	return family == c.arithmetic, err
}

func (c *arithmeticFragmentClassifier) numeric(expression ast.ASTHandle) (bool, error) {
	if numeral, err := c.numeral(expression); err != nil || numeral {
		return numeral, err
	}
	_, thenBranch, elseBranch, ite, err := c.ternaryITE(expression)
	if err != nil || !ite {
		return false, err
	}
	thenNumeric, err := c.numeric(thenBranch)
	if err != nil || !thenNumeric {
		return false, err
	}
	return c.numeric(elseBranch)
}

func (c *arithmeticFragmentClassifier) ternaryITE(expression ast.ASTHandle) (condition, thenBranch, elseBranch ast.ASTHandle, matched bool, err error) {
	matched, err = c.operation(expression, c.manager.BasicFamilyID(), ast.ASTOpIte)
	if err != nil || !matched {
		return
	}
	count, err := c.manager.GetNumArgs(expression)
	if err != nil || count != 3 {
		return condition, thenBranch, elseBranch, false, err
	}
	condition, err = c.manager.GetArg(expression, 0)
	if err == nil {
		thenBranch, err = c.manager.GetArg(expression, 1)
	}
	if err == nil {
		elseBranch, err = c.manager.GetArg(expression, 2)
	}
	return condition, thenBranch, elseBranch, err == nil, err
}

func (c *arithmeticFragmentClassifier) offset(expression ast.ASTHandle) (bool, error) {
	if numeral, err := c.numeral(expression); err != nil || numeral {
		return numeral, err
	}
	if _, thenBranch, elseBranch, ite, err := c.ternaryITE(expression); err != nil {
		return false, err
	} else if ite {
		thenOffset, err := c.offset(thenBranch)
		if err != nil || !thenOffset {
			return false, err
		}
		return c.offset(elseBranch)
	}
	if left, right, add, err := c.binaryOperation(expression, c.arithmetic, 6); err != nil {
		return false, err
	} else if add {
		if numeric, err := c.numeric(left); err != nil {
			return false, err
		} else if numeric {
			return c.offset(right)
		}
		if numeric, err := c.numeric(right); err != nil {
			return false, err
		} else if numeric {
			return c.offset(left)
		}
		return false, nil
	}
	if c.utvpi {
		if left, right, multiply, err := c.binaryOperation(expression, c.arithmetic, 9); err != nil {
			return false, err
		} else if multiply {
			if minusOne, err := c.minusOne(left); err != nil {
				return false, err
			} else if minusOne {
				return c.offset(right)
			}
			if minusOne, err := c.minusOne(right); err != nil {
				return false, err
			} else if minusOne {
				return c.offset(left)
			}
		}
	}
	arithmetic, err := c.arithmeticExpression(expression)
	return !arithmetic, err
}

func (c *arithmeticFragmentClassifier) inequality(expression ast.ASTHandle) (bool, error) {
	left, err := c.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	right, err := c.manager.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	leftOffset, err := c.offset(left)
	if err != nil {
		return false, err
	}
	rightOffset, err := c.offset(right)
	if err != nil {
		return false, err
	}
	if leftOffset && rightOffset {
		return true, nil
	}
	rightNumeric, err := c.numeric(right)
	if err != nil {
		return false, err
	}
	if !rightNumeric {
		left, right = right, left
		rightNumeric, err = c.numeric(right)
		if err != nil {
			return false, err
		}
	}
	if !rightNumeric {
		return false, nil
	}
	if leftOffset, err = c.offset(left); err != nil || leftOffset {
		return leftOffset, err
	}
	first, second, add, err := c.binaryOperation(left, c.arithmetic, 6)
	if err != nil || !add {
		return false, err
	}
	if c.utvpi {
		firstOffset, err := c.offset(first)
		if err != nil || !firstOffset {
			return false, err
		}
		return c.offset(second)
	}
	firstArithmetic, err := c.arithmeticExpression(first)
	if err != nil {
		return false, err
	}
	if firstArithmetic {
		first, second = second, first
		firstArithmetic, err = c.arithmeticExpression(first)
		if err != nil {
			return false, err
		}
	}
	if firstArithmetic {
		return false, nil
	}
	minusOne, term, multiply, err := c.binaryOperation(second, c.arithmetic, 9)
	if err != nil || !multiply {
		return false, err
	}
	negative, err := c.minusOne(minusOne)
	if err != nil || !negative {
		return false, err
	}
	return c.offset(term)
}

func (c *arithmeticFragmentClassifier) term(expression ast.ASTHandle) (bool, error) {
	sort, err := c.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	if sort == c.boolean {
		return true, nil
	}
	if numeral, err := c.numeral(expression); err != nil || numeral {
		return numeral, err
	}
	if offset, err := c.offset(expression); err != nil || offset {
		return offset, err
	}
	if left, right, add, err := c.binaryOperation(expression, c.arithmetic, 6); err != nil {
		return false, err
	} else if add {
		leftNumeral, err := c.numeral(left)
		if err != nil {
			return false, err
		}
		if !leftNumeral {
			left, right = right, left
			leftNumeral, err = c.numeral(left)
			if err != nil {
				return false, err
			}
		}
		if !leftNumeral {
			return false, nil
		}
		return c.offset(right)
	}
	if left, right, multiply, err := c.binaryOperation(expression, c.arithmetic, 9); err != nil {
		return false, err
	} else if multiply {
		if negative, err := c.minusOne(left); err != nil || negative {
			return negative, err
		}
		return c.minusOne(right)
	}
	return false, nil
}

func (c *arithmeticFragmentClassifier) intOrReal(expression ast.ASTHandle) (bool, error) {
	sort, err := c.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	family, err := c.manager.GetFamilyID(sort)
	if err != nil || family != c.arithmetic {
		return false, err
	}
	operation, err := c.manager.GetDeclKind(sort)
	return operation == 0 || operation == 1, err
}

func (c *arithmeticFragmentClassifier) equality(expression ast.ASTHandle) (bool, error) {
	left, err := c.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	right, err := c.manager.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	arithmetic, err := c.intOrReal(left)
	if err != nil || !arithmetic {
		return !arithmetic, err
	}
	leftNumeral, err := c.numeral(left)
	if err != nil {
		return false, err
	}
	rightNumeral, err := c.numeral(right)
	if err != nil {
		return false, err
	}
	if leftNumeral || rightNumeral {
		return c.inequality(expression)
	}
	leftTerm, err := c.term(left)
	if err != nil || !leftTerm {
		return false, err
	}
	rightTerm, err := c.term(right)
	if err != nil || !rightTerm {
		return false, err
	}
	leftProduct, err := c.operation(left, c.arithmetic, 9)
	if err != nil || leftProduct {
		return false, err
	}
	rightProduct, err := c.operation(right, c.arithmetic, 9)
	return !rightProduct, err
}

func (c *arithmeticFragmentClassifier) visit(expression ast.ASTHandle) (bool, error) {
	if le, err := c.operation(expression, c.arithmetic, 2); err != nil {
		return false, err
	} else if le {
		return c.inequality(expression)
	}
	if ge, err := c.operation(expression, c.arithmetic, 3); err != nil {
		return false, err
	} else if ge {
		return c.inequality(expression)
	}
	if equality, err := c.operation(expression, c.manager.BasicFamilyID(), ast.ASTOpEq); err != nil {
		return false, err
	} else if equality {
		return c.equality(expression)
	}
	kind, err := c.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return true, err
	}
	family, err := c.manager.GetFamilyID(expression)
	if err != nil {
		return false, err
	}
	count, err := c.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	if family == ast.NullFamilyID && count != 0 {
		sort, err := c.manager.GetSort(expression)
		if err != nil {
			return false, err
		}
		if sort != c.boolean {
			return false, nil
		}
	} else if family != c.manager.BasicFamilyID() && family != ast.NullFamilyID && family != c.arithmetic && family != c.bitVector {
		return false, nil
	}
	for index := uint32(0); index < count; index++ {
		argument, err := c.manager.GetArg(expression, index)
		if err != nil {
			return false, err
		}
		term, err := c.term(argument)
		if err != nil || !term {
			return false, err
		}
	}
	return true, nil
}

func classifyArithmeticFragment(manager *ast.ASTManager, formulas []ast.ASTHandle, utvpi bool) (bool, error) {
	if manager == nil {
		return false, ast.ErrASTHandle
	}
	classifier, err := newArithmeticFragmentClassifier(manager, utvpi)
	if err != nil {
		return false, err
	}
	seen := make(map[ast.ASTHandle]bool)
	type frame struct {
		expression ast.ASTHandle
		ready      bool
	}
	stack := make([]frame, 0, len(formulas))
	for i := len(formulas) - 1; i >= 0; i-- {
		stack = append(stack, frame{expression: formulas[i]})
	}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		expression := current.expression
		if current.ready {
			accepted, err := classifier.visit(expression)
			if err != nil {
				return false, err
			}
			if !accepted {
				return false, classifier.verboseRejection(expression)
			}
			continue
		}
		if seen[expression] {
			continue
		}
		seen[expression] = true
		stack = append(stack, frame{expression: expression, ready: true})
		var children []ast.ASTHandle
		kind, err := manager.GetKind(expression)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTApp:
			count, err := manager.GetNumArgs(expression)
			if err != nil {
				return false, err
			}
			for index := uint32(0); index < count; index++ {
				argument, err := manager.GetArg(expression, index)
				if err != nil {
					return false, err
				}
				children = append(children, argument)
			}
		case ast.ASTQuantifier:
			body, err := manager.GetQuantifierBody(expression)
			if err != nil {
				return false, err
			}
			children = append(children, body)
			patterns, err := manager.GetNumPatterns(expression)
			if err != nil {
				return false, err
			}
			for index := uint32(0); index < patterns; index++ {
				pattern, err := manager.GetPattern(expression, index)
				if err != nil {
					return false, err
				}
				children = append(children, pattern)
			}
			noPatterns, err := manager.GetNumNoPatterns(expression)
			if err != nil {
				return false, err
			}
			for index := uint32(0); index < noPatterns; index++ {
				pattern, err := manager.GetNoPattern(expression, index)
				if err != nil {
					return false, err
				}
				children = append(children, pattern)
			}
		}
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, frame{expression: children[i]})
		}
	}
	return true, nil
}

// IsDifferenceLogic reports whether all formulas belong to the source PDR
// difference-logic fragment.
func IsDifferenceLogic(manager *ast.ASTManager, formulas []ast.ASTHandle) (bool, error) {
	return classifyArithmeticFragment(manager, formulas, false)
}

// IsUTVPILogic reports whether all formulas belong to the source PDR UTVPI
// fragment.
func IsUTVPILogic(manager *ast.ASTManager, formulas []ast.ASTHandle) (bool, error) {
	return classifyArithmeticFragment(manager, formulas, true)
}

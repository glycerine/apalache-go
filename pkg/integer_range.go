package apalache

import (
	"fmt"
	"math/big"
)

func constantInteger(expression Expr) (*big.Int, bool) {
	switch typed := expression.(type) {
	case *ValueExpr:
		value, ok := typed.Value().(IntValue)
		if !ok {
			return nil, false
		}
		return value.BigInt(), true
	case *OperExpr:
		arguments := typed.Arguments()
		if typed.Operator().Name() == "UNARY_MINUS" && len(arguments) == 1 {
			value, ok := constantInteger(arguments[0])
			if !ok {
				return nil, false
			}
			return new(big.Int).Neg(value), true
		}
		if len(arguments) != 2 {
			return nil, false
		}
		left, leftOK := constantInteger(arguments[0])
		right, rightOK := constantInteger(arguments[1])
		if !leftOK || !rightOK {
			return nil, false
		}
		switch typed.Operator().Name() {
		case "PLUS":
			return new(big.Int).Add(left, right), true
		case "MINUS":
			return new(big.Int).Sub(left, right), true
		case "MULT":
			return new(big.Int).Mul(left, right), true
		}
	}
	return nil, false
}

func (rewriter *StratifiedRewriter) rewriteIntegerRange(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("integer range requires two bounds")
	}
	resultType, typed := expression.Tag().Type()
	if !typed || !EqualTypes(resultType, mustSet(IntT1)) {
		return scope, ArenaCell{}, fmt.Errorf("integer range must have type Set(Int), found %v", resultType)
	}
	for _, argument := range arguments {
		argumentType, argumentTyped := argument.Tag().Type()
		if !argumentTyped || !EqualTypes(argumentType, IntT1) {
			return scope, ArenaCell{}, fmt.Errorf("integer range bounds must have type Int, found %v", argumentType)
		}
	}
	left, leftOK := constantInteger(arguments[0])
	right, rightOK := constantInteger(arguments[1])
	if !leftOK || !rightOK {
		var lower, upper ArenaCell
		var err error
		scope, lower, err = rewriter.Rewrite(arguments[0], scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, upper, err = rewriter.Rewrite(arguments[1], scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		if !EqualTypes(lower.Type.TLAType(), IntT1) || !EqualTypes(upper.Type.TLAType(), IntT1) {
			return scope, ArenaCell{}, fmt.Errorf("integer range bounds must be integers, found %s and %s", lower.Type, upper.Type)
		}
		newArena, result := scope.Arena.AppendCellType(IntegerRangeCellType{})
		scope.Arena = newArena.SetDomain(result, lower).SetCodomain(result, upper)
		return scope, result, nil
	}
	return rewriter.materializeIntegerRange(scope, left, right)
}

func (rewriter *StratifiedRewriter) materializeIntegerRange(scope RewriterScope, left, right *big.Int) (RewriterScope, ArenaCell, error) {
	if !left.IsInt64() || !right.IsInt64() || int64(int(left.Int64())) != left.Int64() || int64(int(right.Int64())) != right.Int64() {
		return scope, ArenaCell{}, fmt.Errorf("integer range bounds must fit in an int")
	}
	from, to := int(left.Int64()), int(right.Int64())
	newArena, value, err := rewriter.ranges.GetOrCreate(scope.Arena, from, to)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope.Arena = newArena
	for _, element := range value.Elements {
		integer, found := rewriter.integers.FindValue(element)
		if !found {
			return scope, ArenaCell{}, fmt.Errorf("integer range element %s is absent from the integer cache", element)
		}
		rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), element.NameExpr(),
			NewValueExpr(NewIntValue(integer), Typed(IntT1))))
	}
	rewriter.addMembershipConstraints(scope.Arena, value.Set)
	return scope, value.Set, nil
}

func (rewriter *StratifiedRewriter) materializeArenaIntegerRange(scope RewriterScope, integerRange ArenaCell) (RewriterScope, ArenaCell, error) {
	lower, err := scope.Arena.Domain(integerRange)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	upper, err := scope.Arena.Codomain(integerRange)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	left, leftOK := rewriter.integers.FindValue(lower)
	right, rightOK := rewriter.integers.FindValue(upper)
	if !leftOK || !rightOK {
		return scope, ArenaCell{}, fmt.Errorf("integer range quantifier requires concretely known bounds")
	}
	return rewriter.materializeIntegerRange(scope, left, right)
}

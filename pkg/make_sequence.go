package apalache

import (
	"fmt"
	"math/big"
)

func instantiateEmbeddedOperator(operator Expr, resultType Type, arguments ...Expr) (Expr, error) {
	if _, ok := operator.(*LetInExpr); !ok {
		return nil, fmt.Errorf("embedded operator must be represented as LET/IN")
	}
	applicationArguments := append([]Expr{operator}, arguments...)
	application := MustOperExpr(Op("OPER_APP"), Typed(resultType), applicationArguments...)
	return NewInliner(nil, nil, false, nil).Transform(application)
}

func (rewriter *StratifiedRewriter) sequenceCapacity(scope RewriterScope, expression Expr) (RewriterScope, int, error) {
	if operation, ok := expression.(*OperExpr); ok {
		if operation.Operator().Name() == "ApalacheInternal!__ApalacheSeqCapacity" && len(operation.Arguments()) == 1 {
			scope, sequence, err := rewriter.Rewrite(operation.Arguments()[0], scope)
			if err != nil {
				return scope, 0, err
			}
			_, _, elements, err := unpackSequence(scope.Arena, sequence)
			return scope, len(elements), err
		}
		arguments := operation.Arguments()
		if len(arguments) == 2 {
			var left, right int
			var err error
			scope, left, err = rewriter.sequenceCapacity(scope, arguments[0])
			if err != nil {
				return scope, 0, err
			}
			scope, right, err = rewriter.sequenceCapacity(scope, arguments[1])
			if err != nil {
				return scope, 0, err
			}
			leftBig, rightBig := big.NewInt(int64(left)), big.NewInt(int64(right))
			var result *big.Int
			switch operation.Operator().Name() {
			case "PLUS":
				result = new(big.Int).Add(leftBig, rightBig)
			case "MINUS":
				result = new(big.Int).Sub(leftBig, rightBig)
			case "MULT":
				result = new(big.Int).Mul(leftBig, rightBig)
			case "DIV":
				if right == 0 {
					return scope, 0, fmt.Errorf("division by zero in sequence capacity")
				}
				result = new(big.Int).Quo(leftBig, rightBig)
			default:
				result = big.NewInt(-1)
			}
			if result.Sign() >= 0 && result.IsInt64() && int64(int(result.Int64())) == result.Int64() {
				return scope, int(result.Int64()), nil
			}
		}
	}
	value, ok := constantInteger(expression)
	if !ok || !value.IsInt64() || value.Sign() < 0 || int64(int(value.Int64())) != value.Int64() {
		return scope, 0, fmt.Errorf("sequence capacity requires a non-negative constant integer expression")
	}
	return scope, int(value.Int64()), nil
}

func (rewriter *StratifiedRewriter) rewriteMakeSequence(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("MkSeq requires a capacity and unary operator")
	}
	typ, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("MkSeq must be typed")
	}
	sequenceType, ok := typ.(SeqType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("MkSeq has non-sequence type %s", typ)
	}
	operator, ok := arguments[1].(*LetInExpr)
	if !ok || !isPassByNameLet(operator) || len(operator.Declarations()[0].FormalParams()) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("MkSeq requires an embedded unary operator")
	}
	var capacity int
	var err error
	scope, capacity, err = rewriter.sequenceCapacity(scope, arguments[0])
	if err != nil {
		return scope, ArenaCell{}, err
	}
	elements := make([]ArenaCell, capacity)
	for index := 0; index < capacity; index++ {
		var indexCell ArenaCell
		scope, indexCell, err = rewriter.Rewrite(
			NewValueExpr(Int64Value(int64(index+1)), Typed(IntT1)), scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		body, inlineErr := instantiateEmbeddedOperator(operator, sequenceType.Element(), indexCell.NameExpr())
		if inlineErr != nil {
			return scope, ArenaCell{}, inlineErr
		}
		scope, elements[index], err = rewriter.Rewrite(body, scope)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("MkSeq element %d: %w", index+1, err)
		}
	}
	var length ArenaCell
	scope, length, err = rewriter.Rewrite(NewValueExpr(Int64Value(int64(capacity)), Typed(IntT1)), scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newArena, result := makeSequenceWithLength(scope.Arena, sequenceType, elements, length)
	scope.Arena = newArena
	return scope, result, nil
}

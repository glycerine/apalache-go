package apalache

import "fmt"

func (rewriter *StratifiedRewriter) rewriteRepeat(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("Repeat requires an operator, bound, and base value")
	}
	operator, ok := arguments[0].(*LetInExpr)
	if !ok || !isPassByNameLet(operator) || len(operator.Declarations()[0].FormalParams()) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("Repeat requires an embedded binary operator")
	}
	bound, ok := constantInteger(arguments[1])
	if !ok || !bound.IsInt64() || bound.Sign() < 0 || int64(int(bound.Int64())) != bound.Int64() {
		return scope, ArenaCell{}, fmt.Errorf("Repeat requires a constant non-negative machine-sized integer bound")
	}
	scope, accumulator, err := rewriter.Rewrite(arguments[2], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	resultType, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("Repeat must be typed")
	}
	scope, accumulator, err = rewriter.coerceCell(scope, accumulator, resultType)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("Repeat base cannot be widened to %s: %w", resultType, err)
	}
	declaration := operator.Declarations()[0]
	expectedOperatorType := mustOperator([]Type{resultType, IntT1}, resultType)
	actualOperatorType, operatorTyped := declaration.Tag().Type()
	if !operatorTyped || !EqualTypes(actualOperatorType, expectedOperatorType) {
		foundType := declaration.Tag().String()
		if operatorTyped {
			foundType = fmt.Sprint(actualOperatorType)
		}
		return scope, ArenaCell{}, NewTypingException(fmt.Sprintf(
			"FoldSet argument %s should have the tag %s, found %s.",
			declaration.Name(), expectedOperatorType, foundType), declaration.ID())
	}
	for index := 1; index <= int(bound.Int64()); index++ {
		var indexCell ArenaCell
		scope, indexCell, err = rewriter.Rewrite(NewValueExpr(Int64Value(int64(index)), Typed(IntT1)), scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		body, inlineErr := instantiateEmbeddedOperator(operator, resultType,
			accumulator.NameExpr(), indexCell.NameExpr())
		if inlineErr != nil {
			return scope, ArenaCell{}, inlineErr
		}
		scope, accumulator, err = rewriter.Rewrite(body, scope)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("Repeat iteration %d: %w", index, err)
		}
		scope, accumulator, err = rewriter.coerceCell(scope, accumulator, resultType)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("Repeat operator result cannot be widened to %s: %w", resultType, err)
		}
	}
	return scope, accumulator, nil
}

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
	resultType := accumulator.Type.TLAType()
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
		if !EqualTypes(accumulator.Type.TLAType(), resultType) {
			return scope, ArenaCell{}, fmt.Errorf("Repeat operator result has type %s, expected %s", accumulator.Type, resultType)
		}
	}
	return scope, accumulator, nil
}

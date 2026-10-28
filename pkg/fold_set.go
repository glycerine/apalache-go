package apalache

import "fmt"

func (rewriter *StratifiedRewriter) rewriteFoldSet(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("FoldSet requires an operator, base value, and set")
	}
	operator, ok := arguments[0].(*LetInExpr)
	if !ok || !isPassByNameLet(operator) || len(operator.Declarations()[0].FormalParams()) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("FoldSet requires an embedded binary operator")
	}
	resultType, err := validateFoldExpression(expression)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, accumulator, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, accumulator, err = rewriter.coerceCell(scope, accumulator, resultType)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("FoldSet base cannot be widened to %s: %w", resultType, err)
	}
	scope, set, err := rewriter.Rewrite(arguments[2], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !isFiniteSetCellType(set.Type) {
		return scope, ArenaCell{}, fmt.Errorf("FoldSet requires an ordinary finite set, found %s", set.Type)
	}
	pointers := scope.Arena.GetHas(set)
	scope, nonDuplicates, err := rewriter.setDeduplicationPredicates(scope, set)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	for index, pointer := range pointers {
		body, inlineErr := instantiateEmbeddedOperator(operator, resultType,
			accumulator.NameExpr(), pointer.Element().NameExpr())
		if inlineErr != nil {
			return scope, ArenaCell{}, inlineErr
		}
		var next ArenaCell
		scope, next, err = rewriter.Rewrite(body, scope)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("fold set element %d: %w", index+1, err)
		}
		scope, next, err = rewriter.coerceCell(scope, next, resultType)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("fold operator result cannot be widened to %s: %w", resultType, err)
		}
		scope, accumulator, err = rewriter.chooseConditional(scope, nonDuplicates[index], next, accumulator)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("select fold set element %d: %w", index+1, err)
		}
	}
	return scope, accumulator, nil
}

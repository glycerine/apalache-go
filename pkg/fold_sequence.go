package apalache

import "fmt"

func (rewriter *StratifiedRewriter) rewriteFoldSequence(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("FoldSeqLeft requires an operator, base value, and sequence")
	}
	operator, ok := arguments[0].(*LetInExpr)
	if !ok || !isPassByNameLet(operator) || len(operator.Declarations()[0].FormalParams()) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("FoldSeqLeft requires an embedded binary operator")
	}
	scope, accumulator, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	resultType, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("FoldSeqLeft must be typed")
	}
	scope, accumulator, err = rewriter.coerceCell(scope, accumulator, resultType)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("FoldSeqLeft base cannot be widened to %s: %w", resultType, err)
	}
	scope, sequence, err := rewriter.Rewrite(arguments[2], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	sequenceType, ok := sequence.Type.TLAType().(SeqType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("FoldSeqLeft requires a sequence, found %s", sequence.Type)
	}
	length, _, elements, err := unpackSequence(scope.Arena, sequence)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	for index, pointer := range elements {
		body, inlineErr := instantiateEmbeddedOperator(operator, resultType,
			accumulator.NameExpr(), pointer.Element().NameExpr())
		if inlineErr != nil {
			return scope, ArenaCell{}, inlineErr
		}
		var next ArenaCell
		scope, next, err = rewriter.Rewrite(body, scope)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("fold sequence element %d: %w", index+1, err)
		}
		scope, next, err = rewriter.coerceCell(scope, next, resultType)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("fold operator result cannot be widened to %s: %w", resultType, err)
		}
		inRange := MustOperExpr(Op("LE"), Typed(BoolT1),
			NewValueExpr(Int64Value(int64(index+1)), Typed(IntT1)), length.NameExpr())
		var predicate ArenaCell
		scope, predicate = rewriter.bindBoolean(scope, inRange)
		scope, accumulator, err = rewriter.chooseConditional(scope, predicate, next, accumulator)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("select fold sequence element %d: %w", index+1, err)
		}
	}
	_ = sequenceType
	return scope, accumulator, nil
}

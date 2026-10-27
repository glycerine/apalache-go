package apalache

import "fmt"

func (rewriter *StratifiedRewriter) rewriteSetUnaryUnion(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("unary union requires one operand")
	}
	scope, outer, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	outerType, ok := outer.Type.TLAType().(SetType)
	if !ok || !isFiniteSetCellType(outer.Type) {
		return scope, ArenaCell{}, fmt.Errorf("unary union requires a finite set of sets, found %s", outer.Type)
	}
	if _, ok := outerType.Element().(SetType); !ok {
		return scope, ArenaCell{}, fmt.Errorf("unary union requires a set of sets, found %s", outer.Type)
	}
	resultType, err := CellTypeFromTag(expression.Tag())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if _, ok := resultType.TLAType().(SetType); !ok {
		return scope, ArenaCell{}, fmt.Errorf("unary union has non-set result type %s", resultType)
	}
	pointers := make([]ElementPointer, 0)
	for _, outerPointer := range scope.Arena.GetHas(outer) {
		inner := outerPointer.Element()
		if !isFiniteSetCellType(inner.Type) {
			return scope, ArenaCell{}, fmt.Errorf("unary union inner value %s is not a finite set", inner)
		}
		for _, innerPointer := range scope.Arena.GetHas(inner) {
			pointers = append(pointers, SMTElementPointer{
				Cell:       innerPointer.Element(),
				Expression: booleanAnd(outerPointer.ToSMT(), innerPointer.ToSMT()),
			})
		}
	}
	if len(pointers) > 0 {
		pointers, err = MergeElementPointersByCell(pointers)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	newArena, result := scope.Arena.AppendCellType(resultType)
	scope.Arena = newArena.AppendHas(result, pointers...)
	rewriter.addMembershipConstraints(scope.Arena, result)
	return scope, result, nil
}

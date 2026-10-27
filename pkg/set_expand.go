package apalache

import "fmt"

const powerSetMaxBaseSize = 20

func (rewriter *StratifiedRewriter) rewriteSetExpansion(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("Expand requires one powerset or function-set operand")
	}
	constructor, ok := arguments[0].(*OperExpr)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("Expand requires an explicit powerset or function-set constructor")
	}
	if constructor.Operator().Name() == "FUN_SET" {
		// Function sets remain compact. Skolemized existential consumers can
		// draw a symbolic function from this representation without enumerating
		// the exponentially large collection.
		return rewriter.Rewrite(constructor, scope)
	}
	if constructor.Operator().Name() != "SET_POWERSET" || len(constructor.Arguments()) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("Expand requires an explicit powerset constructor")
	}
	scope, base, err := rewriter.Rewrite(constructor.Arguments()[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !isFiniteSetCellType(base.Type) {
		return scope, ArenaCell{}, fmt.Errorf("powerset expansion requires a finite base set, found %s", base.Type)
	}
	basePointers := scope.Arena.GetHas(base)
	if len(basePointers) > powerSetMaxBaseSize {
		return scope, ArenaCell{}, fmt.Errorf("powerset expansion size 2^%d exceeds limit 2^%d", len(basePointers), powerSetMaxBaseSize)
	}
	subsetCount := 1 << uint(len(basePointers))
	subsets := make([]ArenaCell, subsetCount)
	for mask := 0; mask < subsetCount; mask++ {
		pointers := make([]ElementPointer, 0, len(basePointers))
		for index, pointer := range basePointers {
			if mask&(1<<uint(index)) != 0 {
				pointers = append(pointers, pointer)
			}
		}
		newArena, subset := scope.Arena.AppendCellType(base.Type)
		scope.Arena = newArena.AppendHas(subset, pointers...)
		rewriter.addMembershipConstraints(scope.Arena, subset)
		subsets[mask] = subset
	}
	resultType, err := CellTypeFromTag(expression.Tag())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	pointers := make([]ElementPointer, len(subsets))
	for index, subset := range subsets {
		pointers[index] = FixedElementPointer{Cell: subset}
	}
	newArena, result := scope.Arena.AppendCellType(resultType)
	scope.Arena = newArena.AppendHas(result, pointers...)
	rewriter.addMembershipConstraints(scope.Arena, result)
	return scope, result, nil
}

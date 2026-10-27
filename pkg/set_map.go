package apalache

import "fmt"

// rewriteSetMap evaluates a finite set map over the Cartesian product of its
// domains. Each output edge carries the conjunction of its source membership
// guards; equal output cells are merged by disjoining their preimages.
func (rewriter *StratifiedRewriter) rewriteSetMap(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) < 3 || len(arguments)%2 == 0 {
		return scope, ArenaCell{}, fmt.Errorf("set map requires an expression and one or more name/set pairs")
	}
	resultType, err := CellTypeFromTag(expression.Tag())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	setType, ok := resultType.TLAType().(SetType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("set map has non-set type %s", resultType)
	}
	binderCount := (len(arguments) - 1) / 2
	binders := make([]*NameExpr, binderCount)
	domains := make([]ArenaCell, binderCount)
	pointerLists := make([][]ElementPointer, binderCount)
	for index := 0; index < binderCount; index++ {
		binder, nameOK := arguments[1+2*index].(*NameExpr)
		if !nameOK {
			return scope, ArenaCell{}, fmt.Errorf("set map binder %d must be a name", index+1)
		}
		binders[index] = binder
		scope, domains[index], err = rewriter.Rewrite(arguments[2+2*index], scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		if !isFiniteSetCellType(domains[index].Type) {
			return scope, ArenaCell{}, fmt.Errorf("set map domain %d must be finite, found %s", index+1, domains[index].Type)
		}
		domainType := domains[index].Type.TLAType().(SetType)
		binderType, typed := binder.Tag().Type()
		if !typed || !EqualTypes(binderType, domainType.Element()) {
			return scope, ArenaCell{}, fmt.Errorf("set map binder %q has type %v, expected %s", binder.Name(), binderType, domainType.Element())
		}
		pointerLists[index] = scope.Arena.GetHas(domains[index])
	}

	baseBinding := scope.Binding
	selected := make([]ElementPointer, binderCount)
	outputPointers := make([]ElementPointer, 0)
	var enumerate func(int) error
	enumerate = func(depth int) error {
		if depth < binderCount {
			for _, pointer := range pointerLists[depth] {
				selected[depth] = pointer
				if recursionErr := enumerate(depth + 1); recursionErr != nil {
					return recursionErr
				}
			}
			return nil
		}
		binding := baseBinding
		for index, binder := range binders {
			binding = binding.With(binder.Name(), selected[index].Element())
		}
		mappedScope := RewriterScope{Arena: scope.Arena, Binding: binding}
		mappedScope, result, rewriteErr := rewriter.Rewrite(arguments[0], mappedScope)
		if rewriteErr != nil {
			return rewriteErr
		}
		if !EqualTypes(result.Type.TLAType(), setType.Element()) {
			return fmt.Errorf("set map result has type %s, expected %s", result.Type, setType.Element())
		}
		scope.Arena = mappedScope.Arena
		pointer, pointerErr := TupleElementPointer(selected, result)
		if pointerErr != nil {
			return pointerErr
		}
		outputPointers = append(outputPointers, pointer)
		return nil
	}
	if err := enumerate(0); err != nil {
		return scope, ArenaCell{}, err
	}
	scope.Binding = baseBinding
	if len(outputPointers) > 0 {
		outputPointers, err = MergeElementPointersByCell(outputPointers)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	newArena, result := scope.Arena.AppendCellType(resultType)
	scope.Arena = newArena.AppendHas(result, outputPointers...)
	rewriter.addMembershipConstraints(scope.Arena, result)
	return scope, result, nil
}

package apalache

import "fmt"

func (rewriter *StratifiedRewriter) rewriteSetAsFunction(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("SetAsFun requires one relation set")
	}
	scope, source, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if !isFiniteSetCellType(source.Type) {
		return scope, ArenaCell{}, fmt.Errorf("SetAsFun requires a finite relation set, found %s", source.Type)
	}
	typ, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("SetAsFun must be typed")
	}
	functionType, ok := typ.(FunType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("SetAsFun has non-function type %s", typ)
	}
	pairType, ok := source.Type.TLAType().(SetType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("SetAsFun source has non-set type %s", source.Type)
	}
	pairTuple, ok := pairType.Element().(TupleType)
	if !ok || len(pairTuple.Elements()) != 2 ||
		!EqualTypes(pairTuple.Elements()[0], functionType.Argument()) ||
		!EqualTypes(pairTuple.Elements()[1], functionType.Result()) {
		return scope, ArenaCell{}, fmt.Errorf("SetAsFun relation element type %s does not match %s", pairType.Element(), functionType)
	}

	sourcePointers := scope.Arena.GetHas(source)
	domainPointers := make([]ElementPointer, len(sourcePointers))
	relationPointers := make([]ElementPointer, len(sourcePointers))
	keys := make([]ArenaCell, len(sourcePointers))
	for index, pointer := range sourcePointers {
		key, _, pairErr := unpackFunctionPair(scope.Arena, pointer.Element())
		if pairErr != nil {
			return scope, ArenaCell{}, pairErr
		}
		keys[index] = key
		conditions := []Expr{pointer.ToSMT()}
		for previous := 0; previous < index; previous++ {
			equal, equalErr := rewriter.equality.Equal(scope.Arena, key, keys[previous])
			if equalErr != nil {
				return scope, ArenaCell{}, equalErr
			}
			conditions = append(conditions, booleanOr(
				MustOperExpr(Op("NOT"), Typed(BoolT1), sourcePointers[previous].ToSMT()),
				MustOperExpr(Op("NOT"), Typed(BoolT1), equal)))
		}
		guard := booleanAnd(conditions...)
		domainPointers[index] = SMTElementPointer{Cell: key, Expression: guard}
		relationPointers[index] = SMTElementPointer{Cell: pointer.Element(), Expression: guard}
	}
	domainPointers, err = MergeElementPointersByCell(domainPointers)
	if err != nil && len(domainPointers) > 0 {
		return scope, ArenaCell{}, err
	}
	newArena, domain := scope.Arena.AppendCellType(BasicCellType{Type: mustSet(functionType.Argument())})
	scope.Arena = newArena.AppendHas(domain, domainPointers...)
	rewriter.addMembershipConstraints(scope.Arena, domain)
	newArena, relation := scope.Arena.AppendCellType(BasicCellType{Type: functionRelationType(functionType)})
	scope.Arena = newArena.AppendHas(relation, relationPointers...)
	rewriter.addMembershipConstraints(scope.Arena, relation)
	newArena, function := scope.Arena.AppendCellType(BasicCellType{Type: functionType})
	scope.Arena = newArena.SetDomain(function, domain).SetCodomain(function, relation)
	return scope, function, nil
}

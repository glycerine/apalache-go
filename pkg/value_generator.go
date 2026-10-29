package apalache

import "fmt"

func (rewriter *StratifiedRewriter) rewriteGenerate(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("Gen requires one positive integer bound")
	}
	typ, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("Gen must be typed")
	}
	boundType, boundTyped := arguments[0].Tag().Type()
	if !boundTyped || !EqualTypes(boundType, IntT1) {
		return scope, ArenaCell{}, fmt.Errorf("Gen bound must have type Int, found %v", boundType)
	}
	boundValue, ok := constantInteger(arguments[0])
	if !ok || !boundValue.IsInt64() || boundValue.Sign() <= 0 || int64(int(boundValue.Int64())) != boundValue.Int64() {
		return scope, ArenaCell{}, fmt.Errorf("Gen requires a constant positive machine-sized integer bound")
	}
	return rewriter.generateBoundedValue(scope, typ, int(boundValue.Int64()))
}

func (rewriter *StratifiedRewriter) generateBoundedValue(scope RewriterScope, typ Type, bound int) (RewriterScope, ArenaCell, error) {
	cellType, err := CellTypeFromType(typ)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if constantType, constant := typ.(ConstType); constant {
		cellType = generatedConstCellType{Type: constantType}
	}
	if isDirectSMTCellType(cellType) {
		newArena, cell := scope.Arena.AppendCellType(cellType, true)
		scope.Arena = newArena
		return scope, cell, nil
	}
	switch typed := typ.(type) {
	case SetType:
		newArena, set := scope.Arena.AppendCellType(cellType)
		scope.Arena = newArena
		pointers := make([]ElementPointer, bound)
		for index := 0; index < bound; index++ {
			var element ArenaCell
			scope, element, err = rewriter.generateBoundedValue(scope, typed.Element(), bound)
			if err != nil {
				return scope, ArenaCell{}, err
			}
			newArena, guard := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1}, true)
			scope.Arena = newArena
			pointers[index] = SMTElementPointer{Cell: element, Expression: guard.NameExpr()}
		}
		scope.Arena = scope.Arena.AppendHas(set, pointers...)
		rewriter.addMembershipConstraints(scope.Arena, set)
		return scope, set, nil
	case SeqType:
		elements := make([]ArenaCell, bound)
		for index := range elements {
			scope, elements[index], err = rewriter.generateBoundedValue(scope, typed.Element(), bound)
			if err != nil {
				return scope, ArenaCell{}, err
			}
		}
		newArena, length := scope.Arena.AppendCellType(BasicCellType{Type: IntT1}, true)
		scope.Arena = newArena
		rewriter.assert(booleanAnd(
			MustOperExpr(Op("LE"), Typed(BoolT1), NewValueExpr(Int64Value(0), Typed(IntT1)), length.NameExpr()),
			MustOperExpr(Op("LE"), Typed(BoolT1), length.NameExpr(), NewValueExpr(Int64Value(int64(bound)), Typed(IntT1)))))
		newArena, sequence := makeSequenceWithLength(scope.Arena, typed, elements, length)
		scope.Arena = newArena
		return scope, sequence, nil
	case TupleType:
		return rewriter.generateProduct(scope, cellType, typed.Elements(), bound)
	case RecordType:
		fields := typed.Fields()
		keys := sortedStringKeys(fields)
		types := make([]Type, len(keys))
		for index, key := range keys {
			types[index] = fields[key]
		}
		return rewriter.generateProduct(scope, cellType, types, bound)
	case RecordRowType:
		if _, open := typed.Row().Other(); open {
			return scope, ArenaCell{}, fmt.Errorf("Gen does not support open record type %s", typed)
		}
		fields := typed.Row().Fields()
		keys := sortedStringKeys(fields)
		types := make([]Type, len(keys))
		for index, key := range keys {
			types[index] = fields[key]
		}
		return rewriter.generateProduct(scope, cellType, types, bound)
	case VariantType:
		if _, open := typed.Row().Other(); open {
			return scope, ArenaCell{}, fmt.Errorf("Gen does not support open variant type %s", typed)
		}
		options := typed.Row().Fields()
		if len(options) == 0 {
			return scope, ArenaCell{}, fmt.Errorf("Gen does not support empty variant type %s", typed)
		}
		newArena, tag := scope.Arena.AppendCellType(BasicCellType{Type: StrT1}, true)
		scope.Arena = newArena
		tagCases := make([]Expr, 0, len(options))
		values := make(map[string]ArenaCell, len(options))
		for _, name := range sortedStringKeys(options) {
			var literal ArenaCell
			scope, literal, err = rewriter.materializeVariantTag(scope, name)
			if err != nil {
				return scope, ArenaCell{}, err
			}
			tagCases = append(tagCases, directEquality(tag, literal))
			scope, values[name], err = rewriter.generateBoundedValue(scope, options[name], bound)
			if err != nil {
				return scope, ArenaCell{}, err
			}
		}
		rewriter.assert(booleanOr(tagCases...))
		keys := variantFieldKeys(options)
		pointers := make([]ElementPointer, len(keys))
		for index, key := range keys {
			field := values[key]
			if key == variantTagField {
				field = tag
			}
			pointers[index] = FixedElementPointer{Cell: field}
		}
		newArena, variant := scope.Arena.AppendCellType(cellType)
		scope.Arena = newArena.AppendHas(variant, pointers...)
		return scope, variant, nil
	case FunType:
		return rewriter.generateFunction(scope, typed, bound)
	default:
		return scope, ArenaCell{}, fmt.Errorf("Gen does not support type %s", typ)
	}
}

func (rewriter *StratifiedRewriter) generateProduct(
	scope RewriterScope, cellType CellType, fieldTypes []Type, bound int,
) (RewriterScope, ArenaCell, error) {
	fields := make([]ArenaCell, len(fieldTypes))
	var err error
	for index, fieldType := range fieldTypes {
		scope, fields[index], err = rewriter.generateBoundedValue(scope, fieldType, bound)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	newArena, product := scope.Arena.AppendCellType(cellType)
	pointers := make([]ElementPointer, len(fields))
	for index, field := range fields {
		pointers[index] = FixedElementPointer{Cell: field}
	}
	scope.Arena = newArena.AppendHas(product, pointers...)
	return scope, product, nil
}

func (rewriter *StratifiedRewriter) generateFunction(scope RewriterScope, typ FunType, bound int) (RewriterScope, ArenaCell, error) {
	pairType, _ := NewTupleType(typ.Argument(), typ.Result())
	relationPointers := make([]ElementPointer, bound)
	domainPointers := make([]ElementPointer, bound)
	keys := make([]ArenaCell, bound)
	for index := 0; index < bound; index++ {
		var key, value ArenaCell
		var err error
		scope, key, err = rewriter.generateBoundedValue(scope, typ.Argument(), bound)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, value, err = rewriter.generateBoundedValue(scope, typ.Result(), bound)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		newArena, pair := scope.Arena.AppendCellType(BasicCellType{Type: pairType})
		scope.Arena = newArena.AppendHas(pair, FixedElementPointer{Cell: key}, FixedElementPointer{Cell: value})
		newArena, guard := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1}, true)
		scope.Arena = newArena
		keys[index] = key
		for previous := 0; previous < index; previous++ {
			equal, equalErr := rewriter.equality.Equal(scope.Arena, key, keys[previous])
			if equalErr != nil {
				return scope, ArenaCell{}, equalErr
			}
			rewriter.assert(booleanOr(
				MustOperExpr(Op("NOT"), Typed(BoolT1), guard.NameExpr()),
				MustOperExpr(Op("NOT"), Typed(BoolT1), relationPointers[previous].ToSMT()),
				MustOperExpr(Op("NOT"), Typed(BoolT1), equal)))
		}
		relationPointers[index] = SMTElementPointer{Cell: pair, Expression: guard.NameExpr()}
		domainPointers[index] = SMTElementPointer{Cell: key, Expression: guard.NameExpr()}
	}
	newArena, domain := scope.Arena.AppendCellType(BasicCellType{Type: mustSet(typ.Argument())})
	scope.Arena = newArena.AppendHas(domain, domainPointers...)
	rewriter.addMembershipConstraints(scope.Arena, domain)
	newArena, relation := scope.Arena.AppendCellType(BasicCellType{Type: functionRelationType(typ)})
	scope.Arena = newArena.AppendHas(relation, relationPointers...)
	rewriter.addMembershipConstraints(scope.Arena, relation)
	newArena, function := scope.Arena.AppendCellType(BasicCellType{Type: typ})
	scope.Arena = newArena.SetDomain(function, domain).SetCodomain(function, relation)
	return scope, function, nil
}

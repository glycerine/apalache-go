package apalache

import "fmt"

func exceptSingleKey(expression *OperExpr) (Expr, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return nil, fmt.Errorf("simple EXCEPT requires a function, one accessor, and a value")
	}
	accessor, ok := arguments[1].(*OperExpr)
	if !ok || accessor.Operator().Name() != "TUPLE" || len(accessor.Arguments()) != 1 {
		return nil, fmt.Errorf("simple EXCEPT accessor must be a singleton tuple")
	}
	return accessor.Arguments()[0], nil
}

func validateExceptExpression(expression *OperExpr, key Expr) error {
	arguments := expression.Arguments()
	resultType, resultTyped := expression.Tag().Type()
	targetType, targetTyped := arguments[0].Tag().Type()
	keyType, keyTyped := key.Tag().Type()
	valueType, valueTyped := arguments[2].Tag().Type()
	if !resultTyped || !targetTyped || !keyTyped || !valueTyped {
		return fmt.Errorf("EXCEPT target, key, value, and result must be typed")
	}
	if !CompatibleTypes(targetType, resultType) {
		return fmt.Errorf("EXCEPT result type %s is incompatible with target type %s", resultType, targetType)
	}
	switch target := targetType.(type) {
	case FunType:
		if !CompatibleTypes(keyType, target.Argument()) || !CompatibleTypes(valueType, target.Result()) {
			return fmt.Errorf("EXCEPT key/value types %s/%s do not match function type %s", keyType, valueType, target)
		}
	case TupleType:
		index, err := literalExceptIndex(key)
		if err != nil {
			return err
		}
		elements := target.Elements()
		if index > len(elements) {
			return fmt.Errorf("tuple EXCEPT index %d is out of bounds for arity %d", index, len(elements))
		}
		if !CompatibleTypes(valueType, elements[index-1]) {
			return fmt.Errorf("tuple EXCEPT value has type %s, expected %s", valueType, elements[index-1])
		}
	case RecordType:
		field, err := literalExceptField(key)
		if err != nil {
			return err
		}
		expected, found := target.Fields()[field]
		if !found || !CompatibleTypes(valueType, expected) {
			return fmt.Errorf("record EXCEPT field %q has incompatible or missing type", field)
		}
	case RecordRowType:
		if _, open := target.Row().Other(); open {
			return fmt.Errorf("EXCEPT cannot update open record type %s", target)
		}
		field, err := literalExceptField(key)
		if err != nil {
			return err
		}
		expected, found := target.Row().Fields()[field]
		if !found || !CompatibleTypes(valueType, expected) {
			return fmt.Errorf("record EXCEPT field %q has incompatible or missing type", field)
		}
	case SeqType:
		if !CompatibleTypes(keyType, IntT1) || !CompatibleTypes(valueType, target.Element()) {
			return fmt.Errorf("sequence EXCEPT key/value types %s/%s do not match %s", keyType, valueType, target)
		}
	default:
		return fmt.Errorf("EXCEPT is not implemented for %s", targetType)
	}
	return nil
}

func (rewriter *StratifiedRewriter) rewriteExcept(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	keyExpression, err := exceptSingleKey(expression)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if err := validateExceptExpression(expression, keyExpression); err != nil {
		return scope, ArenaCell{}, err
	}
	arguments := expression.Arguments()
	scope, oldValue, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, key, err := rewriter.Rewrite(keyExpression, scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, newValue, err := rewriter.Rewrite(arguments[2], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	switch typ := oldValue.Type.TLAType().(type) {
	case FunType:
		return rewriter.rewriteFunctionExcept(scope, oldValue, key, newValue, typ)
	case TupleType:
		return rewriter.rewriteTupleExcept(scope, oldValue, keyExpression, newValue, typ)
	case RecordType:
		return rewriter.rewriteRecordExcept(scope, oldValue, keyExpression, newValue, typ.Fields())
	case RecordRowType:
		if _, open := typ.Row().Other(); open {
			return scope, ArenaCell{}, fmt.Errorf("EXCEPT cannot update open record type %s", typ)
		}
		return rewriter.rewriteRecordExcept(scope, oldValue, keyExpression, newValue, typ.Row().Fields())
	case SeqType:
		return rewriter.rewriteSequenceExcept(scope, oldValue, key, newValue, typ)
	default:
		return scope, ArenaCell{}, fmt.Errorf("EXCEPT is not implemented for %s", oldValue.Type)
	}
}

func (rewriter *StratifiedRewriter) rewriteFunctionExcept(
	scope RewriterScope,
	oldFunction, key, newValue ArenaCell,
	functionType FunType,
) (RewriterScope, ArenaCell, error) {
	var err error
	scope, key, err = rewriter.coerceCell(scope, key, functionType.Argument())
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("function EXCEPT key cannot be widened to %s: %w", functionType.Argument(), err)
	}
	scope, newValue, err = rewriter.coerceCell(scope, newValue, functionType.Result())
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("function EXCEPT value cannot be widened to %s: %w", functionType.Result(), err)
	}
	relation, err := scope.Arena.Codomain(oldFunction)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	domain, err := scope.Arena.Domain(oldFunction)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	pairType, _ := NewTupleType(functionType.Argument(), functionType.Result())
	newArena, replacement := scope.Arena.AppendCellType(BasicCellType{Type: pairType})
	scope.Arena = newArena.AppendHas(replacement,
		FixedElementPointer{Cell: key}, FixedElementPointer{Cell: newValue})

	oldPointers := scope.Arena.GetHas(relation)
	updatedPointers := make([]ElementPointer, len(oldPointers))
	for index, pointer := range oldPointers {
		pairFields := scope.Arena.GetHas(pointer.Element())
		if len(pairFields) != 2 {
			return scope, ArenaCell{}, fmt.Errorf("function relation pair %s has %d fields", pointer.Element(), len(pairFields))
		}
		equal, equalErr := rewriter.equality.Equal(scope.Arena, pairFields[0].Element(), key)
		if equalErr != nil {
			return scope, ArenaCell{}, equalErr
		}
		var predicate ArenaCell
		scope, predicate = rewriter.bindBoolean(scope, equal)
		var updated ArenaCell
		scope, updated, err = rewriter.chooseConditional(scope, predicate, replacement, pointer.Element())
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("update function relation row %d: %w", index+1, err)
		}
		updatedPointers[index] = PointerWithSameCondition(pointer, updated)
	}
	newArena, updatedRelation := scope.Arena.AppendCellType(relation.Type)
	scope.Arena = newArena.AppendHas(updatedRelation, updatedPointers...)
	rewriter.addMembershipConstraints(scope.Arena, updatedRelation)
	newArena, updatedFunction := scope.Arena.AppendCellType(oldFunction.Type)
	scope.Arena = newArena.SetDomain(updatedFunction, domain).SetCodomain(updatedFunction, updatedRelation)
	return scope, updatedFunction, nil
}

func literalExceptIndex(expression Expr) (int, error) {
	valueExpression, ok := expression.(*ValueExpr)
	if !ok {
		return 0, fmt.Errorf("tuple EXCEPT index must be an integer literal")
	}
	value, ok := valueExpression.Value().(IntValue)
	if !ok || !value.BigInt().IsInt64() {
		return 0, fmt.Errorf("tuple EXCEPT index must be an integer literal")
	}
	index := value.BigInt().Int64()
	if index <= 0 || int64(int(index)) != index {
		return 0, fmt.Errorf("tuple EXCEPT index %d is out of bounds", index)
	}
	return int(index), nil
}

func (rewriter *StratifiedRewriter) rewriteTupleExcept(
	scope RewriterScope,
	oldTuple ArenaCell,
	keyExpression Expr,
	newValue ArenaCell,
	tupleType TupleType,
) (RewriterScope, ArenaCell, error) {
	index, err := literalExceptIndex(keyExpression)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	elementTypes := tupleType.Elements()
	if index > len(elementTypes) {
		return scope, ArenaCell{}, fmt.Errorf("tuple EXCEPT index %d is out of bounds for arity %d", index, len(elementTypes))
	}
	scope, newValue, err = rewriter.coerceCell(scope, newValue, elementTypes[index-1])
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("tuple EXCEPT value cannot be widened to %s: %w", elementTypes[index-1], err)
	}
	pointers := scope.Arena.GetHas(oldTuple)
	if len(pointers) != len(elementTypes) {
		return scope, ArenaCell{}, fmt.Errorf("tuple %s contains %d of %d fields", oldTuple, len(pointers), len(elementTypes))
	}
	updated := append([]ElementPointer(nil), pointers...)
	updated[index-1] = PointerWithSameCondition(pointers[index-1], newValue)
	newArena, result := scope.Arena.AppendCellType(oldTuple.Type)
	scope.Arena = newArena.AppendHas(result, updated...)
	return scope, result, nil
}

func literalExceptField(expression Expr) (string, error) {
	valueExpression, ok := expression.(*ValueExpr)
	if !ok {
		return "", fmt.Errorf("record EXCEPT key must be a string literal")
	}
	value, ok := valueExpression.Value().(StringValue)
	if !ok {
		return "", fmt.Errorf("record EXCEPT key must be a string literal")
	}
	return string(value), nil
}

func (rewriter *StratifiedRewriter) rewriteRecordExcept(
	scope RewriterScope,
	oldRecord ArenaCell,
	keyExpression Expr,
	newValue ArenaCell,
	fieldTypes map[string]Type,
) (RewriterScope, ArenaCell, error) {
	field, err := literalExceptField(keyExpression)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	expectedType, found := fieldTypes[field]
	if !found {
		return scope, ArenaCell{}, fmt.Errorf("record EXCEPT field %q is absent from %s", field, oldRecord.Type)
	}
	scope, newValue, err = rewriter.coerceCell(scope, newValue, expectedType)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("record EXCEPT value cannot be widened to %s: %w", expectedType, err)
	}
	pointers := scope.Arena.GetHas(oldRecord)
	keys := sortedStringKeys(fieldTypes)
	if len(pointers) != len(keys) {
		return scope, ArenaCell{}, fmt.Errorf("record %s contains %d of %d fields", oldRecord, len(pointers), len(keys))
	}
	updated := append([]ElementPointer(nil), pointers...)
	for index, name := range keys {
		if name == field {
			updated[index] = PointerWithSameCondition(pointers[index], newValue)
			break
		}
	}
	newArena, result := scope.Arena.AppendCellType(oldRecord.Type)
	scope.Arena = newArena.AppendHas(result, updated...)
	if _, legacy := oldRecord.Type.TLAType().(RecordType); legacy {
		domain, domainErr := scope.Arena.Domain(oldRecord)
		if domainErr != nil {
			return scope, ArenaCell{}, domainErr
		}
		scope.Arena = scope.Arena.SetDomain(result, domain)
	}
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteSequenceExcept(
	scope RewriterScope,
	oldSequence, key, newValue ArenaCell,
	sequenceType SeqType,
) (RewriterScope, ArenaCell, error) {
	if !EqualTypes(key.Type.TLAType(), IntT1) {
		return scope, ArenaCell{}, fmt.Errorf("sequence EXCEPT received key type %s for %s", key.Type, sequenceType)
	}
	var err error
	scope, newValue, err = rewriter.coerceCell(scope, newValue, sequenceType.Element())
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("sequence EXCEPT value cannot be widened to %s: %w", sequenceType.Element(), err)
	}
	length, _, elements, err := unpackSequence(scope.Arena, oldSequence)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	updated := make([]ArenaCell, len(elements))
	for index, pointer := range elements {
		var position ArenaCell
		scope, position, err = rewriter.Rewrite(NewValueExpr(Int64Value(int64(index+1)), Typed(IntT1)), scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		equal, equalErr := rewriter.equality.Equal(scope.Arena, key, position)
		if equalErr != nil {
			return scope, ArenaCell{}, equalErr
		}
		var predicate ArenaCell
		scope, predicate = rewriter.bindBoolean(scope, equal)
		scope, updated[index], err = rewriter.chooseConditional(scope, predicate, newValue, pointer.Element())
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("update sequence position %d: %w", index+1, err)
		}
	}
	newArena, result := makeSequenceWithLength(scope.Arena, sequenceType, updated, length)
	scope.Arena = newArena
	return scope, result, nil
}

package apalache

import "fmt"

// coerceCell widens legacy record values, and finite collections containing
// them, to the static type selected by the type checker. Scala's RecT1 arena
// representation totalizes missing fields and keeps the actual key set in a
// separate domain edge; this operation mirrors that representation.
func (rewriter *StratifiedRewriter) coerceCell(
	scope RewriterScope,
	cell ArenaCell,
	expected Type,
) (RewriterScope, ArenaCell, error) {
	actual := cell.Type.TLAType()
	if EqualTypes(actual, expected) {
		return scope, cell, nil
	}
	switch expectedType := expected.(type) {
	case RecordType:
		switch actualType := actual.(type) {
		case RecordType:
			return rewriter.coerceLegacyRecord(scope, cell, actualType, expectedType)
		case RecordRowType:
			if _, open := actualType.Row().Other(); open {
				return scope, ArenaCell{}, fmt.Errorf("cannot materialize open row-record type %s", actual)
			}
			return rewriter.coerceRowRecordToLegacy(scope, cell, actualType, expectedType)
		default:
			return scope, ArenaCell{}, fmt.Errorf("cannot widen %s to %s", actual, expected)
		}
	case RecordRowType:
		if _, open := expectedType.Row().Other(); open {
			return scope, ArenaCell{}, fmt.Errorf("cannot materialize open row-record type %s", expected)
		}
		actualFields, ok := recordTypeFields(cell.Type)
		if !ok {
			return scope, ArenaCell{}, fmt.Errorf("cannot widen %s to %s", actual, expected)
		}
		return rewriter.coerceClosedRowRecord(scope, cell, actualFields, expectedType)
	case SetType:
		actualType, ok := actual.(SetType)
		if !ok {
			return scope, ArenaCell{}, fmt.Errorf("cannot widen %s to %s", actual, expected)
		}
		if !CompatibleTypes(actualType.Element(), expectedType.Element()) {
			return scope, ArenaCell{}, fmt.Errorf("incompatible set element types %s and %s", actualType.Element(), expectedType.Element())
		}
		basic, ok := cell.Type.(BasicCellType)
		if !ok || !isFiniteSetCellType(basic) {
			return scope, ArenaCell{}, fmt.Errorf("cannot widen compact set cell %s", cell.Type)
		}
		pointers := scope.Arena.GetHas(cell)
		widened := make([]ElementPointer, len(pointers))
		for index, pointer := range pointers {
			var element ArenaCell
			var err error
			scope, element, err = rewriter.coerceCell(scope, pointer.Element(), expectedType.Element())
			if err != nil {
				return scope, ArenaCell{}, fmt.Errorf("widen set element %d: %w", index+1, err)
			}
			widened[index] = PointerWithSameCondition(pointer, element)
		}
		newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: expectedType})
		scope.Arena = newArena.AppendHas(result, widened...)
		rewriter.addMembershipConstraints(scope.Arena, result)
		return scope, result, nil
	case SeqType:
		actualType, ok := actual.(SeqType)
		if !ok || !CompatibleTypes(actualType.Element(), expectedType.Element()) {
			return scope, ArenaCell{}, fmt.Errorf("cannot widen %s to %s", actual, expected)
		}
		length, _, pointers, err := unpackSequence(scope.Arena, cell)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		elements := make([]ArenaCell, len(pointers))
		for index, pointer := range pointers {
			scope, elements[index], err = rewriter.coerceCell(scope, pointer.Element(), expectedType.Element())
			if err != nil {
				return scope, ArenaCell{}, fmt.Errorf("widen sequence element %d: %w", index+1, err)
			}
		}
		newArena, result := makeSequenceWithLength(scope.Arena, expectedType, elements, length)
		scope.Arena = newArena
		return scope, result, nil
	case TupleType:
		actualType, ok := actual.(TupleType)
		if !ok || len(actualType.Elements()) != len(expectedType.Elements()) {
			return scope, ArenaCell{}, fmt.Errorf("cannot widen %s to %s", actual, expected)
		}
		actualElements := scope.Arena.GetHas(cell)
		expectedElements := expectedType.Elements()
		if len(actualElements) != len(expectedElements) {
			return scope, ArenaCell{}, fmt.Errorf("tuple %s has %d of %d fields", cell, len(actualElements), len(expectedElements))
		}
		pointers := make([]ElementPointer, len(expectedElements))
		for index, expectedElement := range expectedElements {
			var element ArenaCell
			var err error
			scope, element, err = rewriter.coerceCell(scope, actualElements[index].Element(), expectedElement)
			if err != nil {
				return scope, ArenaCell{}, fmt.Errorf("widen tuple element %d: %w", index+1, err)
			}
			pointers[index] = PointerWithSameCondition(actualElements[index], element)
		}
		newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: expectedType})
		scope.Arena = newArena.AppendHas(result, pointers...)
		return scope, result, nil
	default:
		return scope, ArenaCell{}, fmt.Errorf("cannot coerce %s to %s", actual, expected)
	}
}

func (rewriter *StratifiedRewriter) coerceRowRecordToLegacy(
	scope RewriterScope,
	record ArenaCell,
	actual RecordRowType,
	expected RecordType,
) (RewriterScope, ArenaCell, error) {
	actualFields := actual.Row().Fields()
	expectedFields := expected.Fields()
	actualKeys := sortedStringKeys(actualFields)
	actualValues := scope.Arena.GetHas(record)
	if len(actualValues) != len(actualKeys) {
		return scope, ArenaCell{}, fmt.Errorf("row record %s has %d of %d fields", record, len(actualValues), len(actualKeys))
	}
	byName := make(map[string]ArenaCell, len(actualKeys))
	for index, key := range actualKeys {
		expectedField, found := expectedFields[key]
		if !found || !CompatibleTypes(actualFields[key], expectedField) {
			return scope, ArenaCell{}, fmt.Errorf("record field %q cannot be widened from %s to %s", key, actualFields[key], expectedField)
		}
		var value ArenaCell
		var err error
		scope, value, err = rewriter.coerceCell(scope, actualValues[index].Element(), expectedField)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("widen record field %q: %w", key, err)
		}
		byName[key] = value
	}
	expectedKeys := sortedStringKeys(expectedFields)
	pointers := make([]ElementPointer, len(expectedKeys))
	for index, key := range expectedKeys {
		value, found := byName[key]
		if !found {
			var err error
			scope, value, err = rewriter.defaults.GetOrCreate(scope, expectedFields[key])
			if err != nil {
				return scope, ArenaCell{}, fmt.Errorf("default widened record field %q: %w", key, err)
			}
		}
		pointers[index] = FixedElementPointer{Cell: value}
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: expected})
	scope.Arena = newArena.AppendHas(result, pointers...)
	domainElements := make([]Expr, len(actualKeys))
	for index, key := range actualKeys {
		domainElements[index] = NewValueExpr(StringValue(key), Typed(StrT1))
	}
	domainExpression := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(StrT1)), domainElements...)
	var domain ArenaCell
	var err error
	scope, domain, err = rewriter.Rewrite(domainExpression, scope)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("materialize widened record domain: %w", err)
	}
	scope.Arena = scope.Arena.SetDomain(result, domain)
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) coerceClosedRowRecord(
	scope RewriterScope,
	record ArenaCell,
	actualFields map[string]Type,
	expected RecordRowType,
) (RewriterScope, ArenaCell, error) {
	expectedFields := expected.Row().Fields()
	actualKeys := sortedStringKeys(actualFields)
	expectedKeys := sortedStringKeys(expectedFields)
	if len(actualKeys) != len(expectedKeys) {
		return scope, ArenaCell{}, fmt.Errorf("record fields %v cannot be widened to %v", actualKeys, expectedKeys)
	}
	actualValues := scope.Arena.GetHas(record)
	if len(actualValues) != len(actualKeys) {
		return scope, ArenaCell{}, fmt.Errorf("record %s has %d of %d fields", record, len(actualValues), len(actualKeys))
	}
	pointers := make([]ElementPointer, len(expectedKeys))
	for index, key := range expectedKeys {
		if actualKeys[index] != key || !CompatibleTypes(actualFields[key], expectedFields[key]) {
			return scope, ArenaCell{}, fmt.Errorf("record field %q cannot be widened from %s to %s", key, actualFields[key], expectedFields[key])
		}
		var value ArenaCell
		var err error
		scope, value, err = rewriter.coerceCell(scope, actualValues[index].Element(), expectedFields[key])
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("widen record field %q: %w", key, err)
		}
		pointers[index] = PointerWithSameCondition(actualValues[index], value)
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: expected})
	scope.Arena = newArena.AppendHas(result, pointers...)
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) coerceLegacyRecord(
	scope RewriterScope,
	record ArenaCell,
	actual, expected RecordType,
) (RewriterScope, ArenaCell, error) {
	actualFields := actual.Fields()
	expectedFields := expected.Fields()
	actualKeys := sortedStringKeys(actualFields)
	actualValues := scope.Arena.GetHas(record)
	if len(actualValues) != len(actualKeys) {
		return scope, ArenaCell{}, fmt.Errorf("legacy record %s has %d of %d totalized fields", record, len(actualValues), len(actualKeys))
	}
	byName := make(map[string]ArenaCell, len(actualKeys))
	for index, key := range actualKeys {
		expectedField, found := expectedFields[key]
		if !found || !CompatibleTypes(actualFields[key], expectedField) {
			return scope, ArenaCell{}, fmt.Errorf("record field %q cannot be widened from %s to %s", key, actualFields[key], expectedField)
		}
		var value ArenaCell
		var err error
		scope, value, err = rewriter.coerceCell(scope, actualValues[index].Element(), expectedField)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("widen record field %q: %w", key, err)
		}
		byName[key] = value
	}
	expectedKeys := sortedStringKeys(expectedFields)
	pointers := make([]ElementPointer, len(expectedKeys))
	for index, key := range expectedKeys {
		value, found := byName[key]
		if !found {
			var err error
			scope, value, err = rewriter.defaults.GetOrCreate(scope, expectedFields[key])
			if err != nil {
				return scope, ArenaCell{}, fmt.Errorf("default widened record field %q: %w", key, err)
			}
		}
		pointers[index] = FixedElementPointer{Cell: value}
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: expected})
	scope.Arena = newArena.AppendHas(result, pointers...)
	domain, err := scope.Arena.Domain(record)
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("legacy record %s has no domain: %w", record, err)
	}
	scope.Arena = scope.Arena.SetDomain(result, domain)
	return scope, result, nil
}

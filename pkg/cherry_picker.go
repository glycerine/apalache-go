package apalache

import "fmt"

// CherryPicker selects one of several arena values using a single oracle.
// Reusing that oracle recursively preserves correlations between every field
// of a structural value.
type CherryPicker struct{ rewriter *StratifiedRewriter }

func NewCherryPicker(rewriter *StratifiedRewriter) *CherryPicker {
	return &CherryPicker{rewriter: rewriter}
}

func (picker *CherryPicker) Pick(scope RewriterScope, oracle ChoiceOracle, cells []ArenaCell) (RewriterScope, ArenaCell, error) {
	if picker == nil || picker.rewriter == nil || oracle == nil {
		return scope, ArenaCell{}, fmt.Errorf("cherry picking requires a rewriter and oracle")
	}
	if len(cells) == 0 || oracle.Size() < len(cells) {
		return scope, ArenaCell{}, fmt.Errorf("oracle size %d is below %d candidate cells", oracle.Size(), len(cells))
	}
	for _, cell := range cells[1:] {
		if !EqualCellTypes(cells[0].Type, cell.Type) {
			return scope, ArenaCell{}, fmt.Errorf("cherry-pick candidates have types %s and %s", cells[0].Type, cell.Type)
		}
	}
	allSame := true
	for _, cell := range cells[1:] {
		allSame = allSame && cell.Equal(cells[0])
	}
	if allSame && oracle.Size() == len(cells) {
		return scope, cells[0], nil
	}
	typ := cells[0].Type
	switch {
	case isDirectSMTCellType(typ):
		return picker.pickBasic(scope, oracle, cells)
	case isTupleCellType(typ) || isRecordCellType(typ):
		return picker.pickProduct(scope, oracle, cells)
	case isFiniteSetCellType(typ):
		return picker.pickSet(scope, oracle, cells)
	case isSequenceCellType(typ):
		return picker.pickSequence(scope, oracle, cells)
	default:
		return scope, ArenaCell{}, fmt.Errorf("cherry picking is not yet implemented for %s", typ)
	}
}

func (picker *CherryPicker) pickBasic(scope RewriterScope, oracle ChoiceOracle, cells []ArenaCell) (RewriterScope, ArenaCell, error) {
	newArena, result := scope.Arena.AppendCellType(cells[0].Type, true)
	scope.Arena = newArena
	assertions := make([]Expr, oracle.Size())
	for index := range assertions {
		assertions[index] = boolExpr(true)
	}
	for index, cell := range cells {
		equal, err := picker.rewriter.equality.Equal(scope.Arena, result, cell)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		assertions[index] = equal
	}
	constraint, err := oracle.Cases(scope, assertions, nil)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	picker.rewriter.assert(constraint)
	return scope, result, nil
}

func (picker *CherryPicker) pickProduct(scope RewriterScope, oracle ChoiceOracle, cells []ArenaCell) (RewriterScope, ArenaCell, error) {
	fieldLists := make([][]ElementPointer, len(cells))
	fieldCount := -1
	for index, cell := range cells {
		fieldLists[index] = scope.Arena.GetHas(cell)
		if fieldCount < 0 {
			fieldCount = len(fieldLists[index])
		} else if len(fieldLists[index]) != fieldCount {
			return scope, ArenaCell{}, fmt.Errorf("product candidates have different field counts")
		}
	}
	fields := make([]ArenaCell, fieldCount)
	for fieldIndex := 0; fieldIndex < fieldCount; fieldIndex++ {
		candidates := make([]ArenaCell, len(cells))
		for candidateIndex := range cells {
			candidates[candidateIndex] = fieldLists[candidateIndex][fieldIndex].Element()
		}
		var err error
		scope, fields[fieldIndex], err = picker.Pick(scope, oracle, candidates)
		if err != nil {
			return scope, ArenaCell{}, fmt.Errorf("pick product field %d: %w", fieldIndex+1, err)
		}
	}
	newArena, result := scope.Arena.AppendCellType(cells[0].Type)
	pointers := make([]ElementPointer, len(fields))
	for index, field := range fields {
		pointers[index] = FixedElementPointer{Cell: field}
	}
	scope.Arena = newArena.AppendHas(result, pointers...)
	return scope, result, nil
}

func (picker *CherryPicker) pickSet(scope RewriterScope, oracle ChoiceOracle, cells []ArenaCell) (RewriterScope, ArenaCell, error) {
	pointers := make([]ElementPointer, 0)
	for candidateIndex, setCell := range cells {
		chosen, err := oracle.Chosen(scope, candidateIndex)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		for _, pointer := range scope.Arena.GetHas(setCell) {
			pointers = append(pointers, SMTElementPointer{
				Cell: pointer.Element(), Expression: booleanAnd(chosen, pointer.ToSMT()),
			})
		}
	}
	merged, err := MergeElementPointersByCell(pointers)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if oracle.Size() > len(cells) && len(merged) > 0 {
		extraChoices := make([]Expr, 0, oracle.Size()-len(cells))
		for index := len(cells); index < oracle.Size(); index++ {
			chosen, chosenErr := oracle.Chosen(scope, index)
			if chosenErr != nil {
				return scope, ArenaCell{}, chosenErr
			}
			extraChoices = append(extraChoices, chosen)
		}
		extraChosen := booleanOr(extraChoices...)
		for index, pointer := range merged {
			newArena, arbitraryMembership := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1}, true)
			scope.Arena = newArena
			merged[index] = SMTElementPointer{Cell: pointer.Element(), Expression: booleanOr(
				pointer.ToSMT(), booleanAnd(extraChosen, arbitraryMembership.NameExpr()),
			)}
		}
	}
	newArena, result := scope.Arena.AppendCellType(cells[0].Type)
	scope.Arena = newArena.AppendHas(result, merged...)
	picker.rewriter.addMembershipConstraints(scope.Arena, result)
	return scope, result, nil
}

func (picker *CherryPicker) pickSequence(scope RewriterScope, oracle ChoiceOracle, cells []ArenaCell) (RewriterScope, ArenaCell, error) {
	lengths := make([]ArenaCell, len(cells))
	elementLists := make([][]ElementPointer, len(cells))
	capacity := 0
	for index, cell := range cells {
		length, _, elements, err := unpackSequence(scope.Arena, cell)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		lengths[index], elementLists[index] = length, elements
		if len(elements) > capacity {
			capacity = len(elements)
		}
	}
	scope, length, err := picker.Pick(scope, oracle, lengths)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	sequenceType := cells[0].Type.TLAType().(SeqType)
	scope, defaultElement, err := picker.rewriter.defaults.GetOrCreate(scope, sequenceType.Element())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	elements := make([]ArenaCell, capacity)
	for elementIndex := 0; elementIndex < capacity; elementIndex++ {
		candidates := make([]ArenaCell, len(cells))
		for candidateIndex := range cells {
			candidates[candidateIndex] = defaultElement
			if elementIndex < len(elementLists[candidateIndex]) {
				candidates[candidateIndex] = elementLists[candidateIndex][elementIndex].Element()
			}
		}
		scope, elements[elementIndex], err = picker.Pick(scope, oracle, candidates)
		if err != nil {
			return scope, ArenaCell{}, err
		}
	}
	newArena, result := makeSequenceWithLength(scope.Arena, sequenceType, elements, length)
	scope.Arena = newArena
	return scope, result, nil
}

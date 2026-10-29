package ast

// Source: src/ast/datatype_decl_plugin.cpp:112-147,215-229,557-673.

import "math"

func datatypeSizeAdd(left, right SortSize) SortSize {
	if left.IsInfinite() || right.IsInfinite() {
		return InfiniteSortSize()
	}
	if left.IsVeryBig() || right.IsVeryBig() {
		return VeryBigSortSize()
	}
	a, _ := left.Size()
	b, _ := right.Size()
	if math.MaxUint64-a < b {
		return VeryBigSortSize()
	}
	return FiniteSortSize(a + b)
}

func datatypeSizeMultiply(left, right SortSize) SortSize {
	if left.IsInfinite() || right.IsInfinite() {
		return InfiniteSortSize()
	}
	if left.IsVeryBig() || right.IsVeryBig() {
		return VeryBigSortSize()
	}
	a, _ := left.Size()
	b, _ := right.Size()
	if a != 0 && b > math.MaxUint64/a {
		return VeryBigSortSize()
	}
	return FiniteSortSize(a * b)
}

func (m *ASTManager) datatypeStoredSortSize(sort astNodeRef) (SortSize, error) {
	info := m.sortAt(sort.slot).info
	if info == (ASTSortInfo{}) {
		return InfiniteSortSize(), nil
	}
	record, err := m.sortInfo(info)
	if err != nil {
		return SortSize{}, err
	}
	return record.size, nil
}

func (m *ASTManager) datatypeCardinality(sort astNodeRef, visiting map[astNodeRef]bool, memo map[astNodeRef]SortSize) (SortSize, error) {
	if size, found := memo[sort]; found {
		return size, nil
	}
	definition, err := m.datatypeDefinitionForSort(sort)
	if err != nil {
		return m.datatypeStoredSortSize(sort)
	}
	if visiting[sort] {
		return InfiniteSortSize(), nil
	}
	visiting[sort] = true
	result := FiniteSortSize(0)
	for i := range definition.constructors {
		ranges, e := m.datatypeInstantiatedRanges(sort, definition, &definition.constructors[i])
		if e != nil {
			delete(visiting, sort)
			return SortSize{}, e
		}
		product := FiniteSortSize(1)
		for _, field := range ranges {
			fieldSize, e := m.datatypeCardinality(field, visiting, memo)
			if e != nil {
				delete(visiting, sort)
				return SortSize{}, e
			}
			product = datatypeSizeMultiply(product, fieldSize)
		}
		result = datatypeSizeAdd(result, product)
	}
	delete(visiting, sort)
	memo[sort] = result
	return result, nil
}

func (m *ASTManager) datatypeSetCardinality(sort astNodeRef) error {
	size, err := m.datatypeCardinality(sort, make(map[astNodeRef]bool), make(map[astNodeRef]SortSize))
	if err != nil {
		return err
	}
	info, err := m.sortInfo(m.sortAt(sort.slot).info)
	if err != nil {
		return err
	}
	info.size = size
	return nil
}

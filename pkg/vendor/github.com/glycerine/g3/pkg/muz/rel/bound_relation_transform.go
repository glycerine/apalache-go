package rel

// Source: src/muz/rel/dl_vector_relation.h mk_join/mk_project and
// dl_bound_relation.cpp bound_relation_helper::mk_project_t.

import "github.com/glycerine/g3/pkg/ast"

// JoinBoundRelations copies the source column bounds and equates the selected
// cross-relation columns, following the concrete vector relation join.
func JoinBoundRelations(left, right *BoundRelation, leftColumns, rightColumns []uint32) (*BoundRelation, error) {
	if left == nil || right == nil || len(leftColumns) != len(rightColumns) {
		return nil, ast.ErrASTDomain
	}
	for i := range leftColumns {
		if uint64(leftColumns[i]) >= uint64(len(left.columns)) || uint64(rightColumns[i]) >= uint64(len(right.columns)) {
			return nil, ast.ErrASTDomain
		}
	}
	result := NewBoundRelation(uint32(len(left.columns)+len(right.columns)), false)
	if left.empty || right.empty {
		result.empty = true
		return result, nil
	}
	for i := range left.columns {
		result.columns[i] = cloneBoundColumn(left.columns[left.find(uint32(i))])
	}
	for i := range right.columns {
		column := right.columns[right.find(uint32(i))]
		// vector_relation::mk_join copies the right payload unchanged into the
		// concatenated vector; bound indices inside uint_set2 are not offset.
		result.columns[len(left.columns)+i] = cloneBoundColumn(column)
	}
	for i := range leftColumns {
		if err := result.Equate(leftColumns[i], uint32(len(left.columns))+rightColumns[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Project removes sorted, unique columns and maps bounds through the source
// renaming vector. Bounds naming removed columns are mapped to MaxUint32, as
// bound_relation_helper::mk_project_t does in C++.
func (r *BoundRelation) Project(removed []uint32) (*BoundRelation, error) {
	if r == nil || len(removed) > len(r.columns) {
		return nil, ast.ErrASTDomain
	}
	for i, col := range removed {
		if uint64(col) >= uint64(len(r.columns)) || i > 0 && removed[i-1] >= col {
			return nil, ast.ErrASTDomain
		}
	}
	result := NewBoundRelation(uint32(len(r.columns)-len(removed)), false)
	kept := make([]uint32, 0, len(result.columns))
	classes := make(map[uint32]uint32)
	removeIndex := 0
	for i := range r.columns {
		index := uint32(i)
		if removeIndex < len(removed) && removed[removeIndex] == index {
			removeIndex++
			continue
		}
		dest := uint32(len(kept))
		kept = append(kept, index)
		root := r.find(index)
		result.columns[dest] = cloneBoundColumn(r.columns[root])
		if first, ok := classes[root]; ok {
			result.merge(first, dest)
		} else {
			classes[root] = dest
		}
	}
	renaming := make(map[uint32]uint32, len(r.columns))
	keptIndex := 0
	removeIndex = 0
	for old := range r.columns {
		index := uint32(old)
		if removeIndex < len(removed) && removed[removeIndex] == index {
			renaming[index] = ^uint32(0)
			removeIndex++
			continue
		}
		renaming[index] = result.find(uint32(keptIndex))
		keptIndex++
	}
	// Preserve the source sentinel if an already-mapped representative is
	// visited again through another member of its equality class.
	renaming[^uint32(0)] = ^uint32(0)
	// The source applies the projection helper through operator[] for every
	// column, including repeated representatives.
	for i := range result.columns {
		column := &result.columns[result.find(uint32(i))]
		mapped := boundColumn{lt: make(map[uint32]bool), le: make(map[uint32]bool)}
		for old := range column.lt {
			mapped.lt[renaming[old]] = true
		}
		for old := range column.le {
			mapped.le[renaming[old]] = true
		}
		*column = mapped
	}
	return result, nil
}

// Rename rotates the selected columns and applies the source bound-set rename.
func (r *BoundRelation) Rename(cycle []uint32) (*BoundRelation, error) {
	if r == nil || len(cycle) == 0 {
		return nil, ast.ErrASTDomain
	}
	seen := make(map[uint32]bool, len(cycle))
	for _, column := range cycle {
		if uint64(column) >= uint64(len(r.columns)) || seen[column] {
			return nil, ast.ErrASTDomain
		}
		seen[column] = true
	}
	result := NewBoundRelation(uint32(len(r.columns)), false)
	classes := make([]uint32, len(r.columns))
	for i := range r.columns {
		classes[i] = r.find(uint32(i))
		result.columns[i] = cloneBoundColumn(r.columns[classes[i]])
	}
	for i, column := range cycle {
		dest := cycle[(i+1)%len(cycle)]
		result.columns[dest] = cloneBoundColumn(r.columns[column])
		classes[dest] = r.find(column)
	}
	first := make(map[uint32]uint32)
	for i, class := range classes {
		if representative, ok := first[class]; ok {
			result.merge(representative, uint32(i))
		} else {
			first[class] = uint32(i)
		}
	}
	for i := range result.columns {
		column := &result.columns[i]
		last := result.find(cycle[len(cycle)-1])
		lastLT, lastLE := column.lt[last], column.le[last]
		delete(column.lt, last)
		delete(column.le, last)
		for j := 0; j+1 < len(cycle); j++ {
			from, to := result.find(cycle[j]), result.find(cycle[j+1])
			if column.lt[from] {
				delete(column.lt, from)
				column.lt[to] = true
			}
			if column.le[from] {
				delete(column.le, from)
				column.le[to] = true
			}
		}
		if lastLT {
			column.lt[result.find(cycle[0])] = true
		}
		if lastLE {
			column.le[result.find(cycle[0])] = true
		}
	}
	return result, nil
}

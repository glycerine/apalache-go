package rel

// Source: src/muz/rel/dl_product_relation.cpp transform_fn and
// dl_sieve_relation.cpp project/rename transformer paths.

import "github.com/glycerine/g3/pkg/ast"

func (p *SparseIntervalSieveProduct) Project(removed []int) (*SparseIntervalSieveProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	drop := make([]bool, len(p.sorts))
	for index, column := range removed {
		if column < 0 || column >= len(drop) || index > 0 && removed[index-1] >= column {
			return nil, ErrSparseIntervalProduct
		}
		drop[column] = true
	}
	finiteRemoved := make([]int, 0, len(removed))
	for local, outer := range p.finiteColumns {
		if drop[outer] {
			finiteRemoved = append(finiteRemoved, local)
		}
	}
	finite, err := p.finite.Project(finiteRemoved)
	if err != nil {
		return nil, err
	}
	intervalRemoved := make([]uint32, len(removed))
	for i, column := range removed {
		intervalRemoved[i] = uint32(column)
	}
	interval, err := p.interval.Project(intervalRemoved)
	if err != nil {
		_ = finite.Close()
		return nil, err
	}
	sorts := make([]ast.ASTHandle, 0, len(p.sorts)-len(removed))
	offset := make([]int, len(p.sorts))
	removedBefore := 0
	for outer, sort := range p.sorts {
		offset[outer] = removedBefore
		if drop[outer] {
			removedBefore++
			continue
		}
		sorts = append(sorts, sort)
	}
	finiteColumns := make([]int, 0, len(p.finiteColumns)-len(finiteRemoved))
	finiteSorts := make([]ast.ASTHandle, 0, len(p.finiteSorts)-len(finiteRemoved))
	for local, outer := range p.finiteColumns {
		if drop[outer] {
			continue
		}
		finiteColumns = append(finiteColumns, outer-offset[outer])
		finiteSorts = append(finiteSorts, p.finiteSorts[local])
	}
	return &SparseIntervalSieveProduct{manager: p.manager, sorts: sorts, finiteColumns: finiteColumns, finiteSorts: finiteSorts, finite: finite, interval: interval, intervalShape: p.intervalShape, intervalNullary: p.intervalShape == sparseIntervalShapeRaw && len(sorts) == 0}, nil
}

func sparseIntervalOuterSources(size int, cycle []int) ([]int, error) {
	sources := make([]int, size)
	for i := range sources {
		sources[i] = i
	}
	seen := make([]bool, size)
	for _, column := range cycle {
		if column < 0 || column >= size || seen[column] {
			return nil, ErrSparseIntervalProduct
		}
		seen[column] = true
	}
	if len(cycle) > 1 {
		for i := 1; i < len(cycle); i++ {
			sources[cycle[i-1]] = cycle[i]
		}
		sources[cycle[len(cycle)-1]] = cycle[0]
	}
	return sources, nil
}

func sparseIntervalReorderSparse(source *SparseTable, desired []int) (*SparseTable, error) {
	result, err := source.Clone()
	if err != nil {
		return nil, err
	}
	current := make([]int, len(desired))
	for i := range current {
		current[i] = i
	}
	for destination, wanted := range desired {
		position := destination
		for position < len(current) && current[position] != wanted {
			position++
		}
		if position == len(current) {
			_ = result.Close()
			return nil, ErrSparseIntervalProduct
		}
		if position == destination {
			continue
		}
		next, renameErr := result.Rename([]int{position, destination})
		_ = result.Close()
		if renameErr != nil {
			return nil, renameErr
		}
		result = next
		current[position], current[destination] = current[destination], current[position]
	}
	return result, nil
}

func sparseIntervalReorderInterval(source *IntervalRelation, desired []int) (*IntervalRelation, error) {
	// Projecting no columns is the source identity transformer and preserves
	// equality classes; IntervalRelation.Clone intentionally copies only values.
	result, err := source.Project(nil)
	if err != nil {
		return nil, err
	}
	current := make([]int, len(desired))
	for i := range current {
		current[i] = i
	}
	for destination, wanted := range desired {
		position := destination
		for position < len(current) && current[position] != wanted {
			position++
		}
		if position == len(current) {
			return nil, ErrSparseIntervalProduct
		}
		if position == destination {
			continue
		}
		result, err = result.Rename([]uint32{uint32(position), uint32(destination)})
		if err != nil {
			return nil, err
		}
		current[position], current[destination] = current[destination], current[position]
	}
	return result, nil
}

// Rename rotates outer columns by cycle and derives the corresponding concrete
// sub-permutations for both selected components.
func (p *SparseIntervalSieveProduct) Rename(cycle []int) (*SparseIntervalSieveProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	sources, err := sparseIntervalOuterSources(len(p.sorts), cycle)
	if err != nil {
		return nil, err
	}
	oldFinite := make(map[int]int, len(p.finiteColumns))
	for local, outer := range p.finiteColumns {
		oldFinite[outer] = local
	}
	oldInterval := make(map[int]int, len(p.interval.innerToSig))
	for local, outer := range p.interval.innerToSig {
		oldInterval[outer] = local
	}
	sorts := make([]ast.ASTHandle, len(p.sorts))
	finiteColumns := make([]int, 0, len(p.finiteColumns))
	finiteSorts := make([]ast.ASTHandle, 0, len(p.finiteSorts))
	finiteDesired := make([]int, 0, len(p.finiteColumns))
	intervalMask := make([]bool, len(p.sorts))
	intervalDesired := make([]int, 0, len(p.interval.innerToSig))
	for destination, source := range sources {
		sorts[destination] = p.sorts[source]
		if local, selected := oldFinite[source]; selected {
			finiteColumns = append(finiteColumns, destination)
			finiteSorts = append(finiteSorts, p.finiteSorts[local])
			finiteDesired = append(finiteDesired, local)
		}
		if local, selected := oldInterval[source]; selected {
			intervalMask[destination] = true
			intervalDesired = append(intervalDesired, local)
		}
	}
	finite, err := sparseIntervalReorderSparse(p.finite, finiteDesired)
	if err != nil {
		return nil, err
	}
	inner, err := sparseIntervalReorderInterval(p.interval.inner, intervalDesired)
	if err != nil {
		_ = finite.Close()
		return nil, err
	}
	interval, err := newIntervalSieve(p.manager.manager, sorts, intervalMask, inner)
	if err != nil {
		_ = finite.Close()
		return nil, err
	}
	return &SparseIntervalSieveProduct{manager: p.manager, sorts: sorts, finiteColumns: finiteColumns, finiteSorts: finiteSorts, finite: finite, interval: interval, intervalShape: p.intervalShape, intervalNullary: p.intervalShape == sparseIntervalShapeRaw && len(sorts) == 0}, nil
}

package rel

// Source: sieve_relation_plugin::mk_rename_fn collects the selected-column
// sub-permutation, then relation_manager decomposes it into source cycles.

import "github.com/glycerine/g3/pkg/ast"

func (p *UDocTableProduct) Rename(cycle []uint32) (*UDocTableProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	permutation := make([]uint32, len(p.doc.sorts))
	seen := make([]bool, len(permutation))
	finiteCycle := make([]int, len(cycle))
	for i := range permutation {
		permutation[i] = uint32(i)
	}
	for i, column := range cycle {
		if column >= uint32(len(permutation)) || seen[column] {
			return nil, ErrUDocTableProduct
		}
		seen[column], finiteCycle[i] = true, int(column)
	}
	for i, column := range cycle {
		permutation[column] = cycle[(i+1)%len(cycle)]
	}
	sorts := make([]ast.ASTHandle, len(permutation))
	mask := make([]bool, len(permutation))
	var innerPermutation []uint32
	oldTableMask := productTableMask(p.doc.sorts, p.tableColumns)
	newTableMask := make([]bool, len(permutation))
	oldTableMap := productTableColumnMap(oldTableMask)
	var tablePermutation []uint32
	for destination, source := range permutation {
		sorts[destination], mask[destination] = p.doc.sorts[source], p.doc.innerColumns[source]
		if mask[destination] {
			innerPermutation = append(innerPermutation, uint32(p.doc.sigToInner[source]))
		}
		newTableMask[destination] = oldTableMask[source]
		if newTableMask[destination] {
			tablePermutation = append(tablePermutation, uint32(oldTableMap[source]))
		}
	}
	a := newUDocTableProductAttempt(p.manager)
	inner, err := p.doc.inner.Clone()
	if inner != nil {
		a.inners = append(a.inners, inner)
	}
	if err != nil {
		return a.finish(err)
	}
	for {
		local, found := removePermutationCycle(innerPermutation)
		if !found {
			break
		}
		next, cause := inner.Rename(local)
		if next != nil {
			a.inners = append(a.inners, next)
		}
		if cause != nil {
			return a.finish(cause)
		}
		inner = next
	}
	a.product.doc, err = AdoptUDocSieve(sorts, mask, inner)
	if err != nil {
		return a.finish(err)
	}
	// Only the final inner transfers into the new sieve. Earlier cycle
	// intermediates remain in the attempt until all construction has finished.
	a.inners = a.inners[:len(a.inners)-1]
	if p.table != nil {
		table, cause := p.table.CloneFinite()
		if table != nil {
			a.tables = append(a.tables, table)
		}
		if cause != nil {
			return a.finish(cause)
		}
		for {
			local, found := removePermutationCycle(tablePermutation)
			if !found {
				break
			}
			cycle := make([]int, len(local))
			for i, column := range local {
				cycle[i] = int(column)
			}
			next, renameErr := finiteTableRename(table, cycle)
			if next != nil {
				a.tables = append(a.tables, next)
			}
			if renameErr != nil {
				return a.finish(renameErr)
			}
			table = next
		}
		a.product.table = table
		a.product.tableColumns = newTableMask
		a.tables = a.tables[:len(a.tables)-1]
	}
	return a.finish(err)
}

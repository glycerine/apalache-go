package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp/.h, finite sparse-table
// outer relation with sparse-table inner relations.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

// FiniteSparseProduct maps each finite table key to a sparse relation over the
// remaining finite columns. It is the concrete table/table product used by the
// retained relation tests; it borrows its manager and sort handles.
type FiniteSparseProduct struct {
	manager                 *RelationManager
	sorts                   []ast.ASTHandle
	tableColumns, innerCols []int
	innerSignature          Signature
	table                   *SparseTable
	inner                   []*SparseTable
	available               []int
}

func NewFiniteSparseProduct(manager *RelationManager, sorts []ast.ASTHandle, tableColumns []bool) (*FiniteSparseProduct, error) {
	if manager == nil || manager.open() != nil || tableColumns != nil && len(tableColumns) != len(sorts) {
		return nil, ast.ErrASTDomain
	}
	p := &FiniteSparseProduct{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...)}
	var outerDomains, innerDomains []uint64
	for i, sort := range sorts {
		domain, err := manager.sortDomain(sort)
		if err != nil {
			return nil, err
		}
		isTable := tableColumns == nil || tableColumns[i]
		if isTable {
			p.tableColumns = append(p.tableColumns, i)
			outerDomains = append(outerDomains, domain)
		} else {
			p.innerCols = append(p.innerCols, i)
			innerDomains = append(innerDomains, domain)
		}
	}
	outerDomains = append(outerDomains, math.MaxInt32)
	outerSig, err := NewSignature(outerDomains)
	if err != nil {
		return nil, err
	}
	p.table, err = NewSparseTable(outerSig, 1)
	if err != nil {
		return nil, err
	}
	innerSig, err := NewSignature(innerDomains)
	if err != nil {
		return nil, errors.Join(err, p.table.Close())
	}
	// Keep the signature with a prototype so zero-column inner relations still
	// have a concrete, reusable table layout.
	p.innerSignature = innerSig
	return p, nil
}

func (p *FiniteSparseProduct) open() error {
	if p == nil || p.manager == nil || p.manager.open() != nil || p.table == nil || p.table.open() != nil {
		return ErrTableRelation
	}
	return nil
}

func (p *FiniteSparseProduct) Close() error {
	if p == nil || p.table == nil {
		return nil
	}
	err := p.table.Close()
	for i, inner := range p.inner {
		if inner != nil {
			err = errors.Join(err, inner.Close())
			p.inner[i] = nil
		}
	}
	p.table, p.manager, p.sorts, p.tableColumns, p.innerCols, p.inner, p.available = nil, nil, nil, nil, nil, nil, nil
	return err
}

// Reset clears all rows and recycles each allocated inner-relation index.
func (p *FiniteSparseProduct) Reset() error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.table.Reset(); err != nil {
		return err
	}
	return p.collectGarbage(false)
}

// SizeEstimateRows reports the source's outer-table row estimate; each row
// represents one finite key and its nested sparse relation.
func (p *FiniteSparseProduct) SizeEstimateRows() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.table.SizeEstimateRows()
}

// SizeEstimateBytes reports the source's outer-table storage estimate.
func (p *FiniteSparseProduct) SizeEstimateBytes() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return p.table.SizeEstimateBytes()
}

func (p *FiniteSparseProduct) split(values []ast.ASTHandle) (Fact, Fact, error) {
	if err := p.open(); err != nil {
		return nil, nil, err
	}
	if len(values) != len(p.sorts) {
		return nil, nil, ast.ErrASTDomain
	}
	key := make(Fact, len(p.tableColumns)+1)
	inner := make(Fact, len(p.innerCols))
	for i, column := range p.tableColumns {
		value, err := p.manager.EncodeValue(p.sorts[column], values[column])
		if err != nil {
			return nil, nil, err
		}
		key[i] = value
	}
	for i, column := range p.innerCols {
		value, err := p.manager.EncodeValue(p.sorts[column], values[column])
		if err != nil {
			return nil, nil, err
		}
		inner[i] = value
	}
	return key, inner, nil
}

func (p *FiniteSparseProduct) AddFact(values []ast.ASTHandle) (resultErr error) {
	key, innerFact, err := p.split(values)
	if err != nil {
		return err
	}
	return p.addEncodedFact(key, innerFact)
}

func (p *FiniteSparseProduct) addEncodedFact(key, innerFact Fact) (resultErr error) {
	found, err := p.fetchInnerKey(key)
	if err != nil {
		return err
	}
	var inner *SparseTable
	index := -1
	if found {
		index = int(key[len(key)-1])
		if index < 0 || index >= len(p.inner) || p.inner[index] == nil {
			return ErrTableRelation
		}
		inner, err = p.inner[index].Clone()
	} else {
		inner, err = NewSparseTable(p.innerSignature, 0)
	}
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, inner.Close())
		}
	}()
	if _, err = inner.Add(innerFact); err != nil {
		return err
	}
	if found {
		old := p.inner[index]
		p.inner[index] = inner
		keep = true
		return old.Close()
	}
	if len(p.available) != 0 {
		index = p.available[len(p.available)-1]
		p.available = p.available[:len(p.available)-1]
	} else {
		if uint64(len(p.inner)) >= math.MaxInt32 {
			return ErrTableRelation
		}
		index = len(p.inner)
		p.inner = append(p.inner, nil)
	}
	key[len(key)-1] = Element(index)
	if err = p.table.EnsureFact(key); err != nil {
		if index == len(p.inner)-1 && p.inner[index] == nil {
			p.inner = p.inner[:index]
		} else {
			p.available = append(p.available, index)
		}
		return err
	}
	p.inner[index] = inner
	keep = true
	return nil
}

// Project returns a product with removed columns omitted. Rows that become
// equal are merged by insertion into their resulting inner relations.
func (p *FiniteSparseProduct) Project(removed []int) (*FiniteSparseProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	remove := make([]bool, len(p.sorts))
	last := -1
	for _, column := range removed {
		if column <= last || column < 0 || column >= len(p.sorts) {
			return nil, ErrTableRelation
		}
		remove[column] = true
		last = column
	}
	sorts := make([]ast.ASTHandle, 0, len(p.sorts)-len(removed))
	tableColumns := make([]bool, 0, cap(sorts))
	for column, sort := range p.sorts {
		if !remove[column] {
			sorts = append(sorts, sort)
			tableColumns = append(tableColumns, p.isTableColumn(column))
		}
	}
	result, err := NewFiniteSparseProduct(p.manager, sorts, tableColumns)
	if err != nil {
		return nil, err
	}
	rows, err := p.table.Facts()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	for _, key := range rows {
		index := int(key[len(key)-1])
		if index < 0 || index >= len(p.inner) || p.inner[index] == nil {
			return nil, errors.Join(ErrTableRelation, result.Close())
		}
		innerFacts, err := p.inner[index].Facts()
		if err != nil {
			return nil, errors.Join(err, result.Close())
		}
		for _, innerFact := range innerFacts {
			fact := make(Fact, len(p.sorts))
			for i, column := range p.tableColumns {
				fact[column] = key[i]
			}
			for i, column := range p.innerCols {
				fact[column] = innerFact[i]
			}
			projected := make(Fact, 0, len(sorts))
			for column, value := range fact {
				if !remove[column] {
					projected = append(projected, value)
				}
			}
			outKey, outInner := result.splitEncoded(projected)
			if err = result.addEncodedFact(outKey, outInner); err != nil {
				return nil, errors.Join(err, result.Close())
			}
		}
	}
	return result, nil
}

// Rename returns a product with the selected columns rotated by cycle. The
// finite sort and outer/inner partition move with each column.
func (p *FiniteSparseProduct) Rename(cycle []int) (*FiniteSparseProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if len(cycle) < 2 {
		return nil, ErrTableRelation
	}
	seen := make(map[int]bool, len(cycle))
	for _, column := range cycle {
		if column < 0 || column >= len(p.sorts) || seen[column] {
			return nil, ErrTableRelation
		}
		seen[column] = true
	}
	sorts := append([]ast.ASTHandle(nil), p.sorts...)
	mask := p.tableColumnMask()
	for i := 1; i < len(cycle); i++ {
		sorts[cycle[i-1]] = p.sorts[cycle[i]]
		mask[cycle[i-1]] = p.isTableColumn(cycle[i])
	}
	sorts[cycle[len(cycle)-1]] = p.sorts[cycle[0]]
	mask[cycle[len(cycle)-1]] = p.isTableColumn(cycle[0])
	result, err := NewFiniteSparseProduct(p.manager, sorts, mask)
	if err != nil {
		return nil, err
	}
	flat, err := p.flatten()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	defer flat.Close()
	facts, err := flat.Facts()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	for _, fact := range facts {
		rotated := append(Fact(nil), fact...)
		for i := 1; i < len(cycle); i++ {
			rotated[cycle[i-1]] = fact[cycle[i]]
		}
		rotated[cycle[len(cycle)-1]] = fact[cycle[0]]
		key, inner := result.splitEncoded(rotated)
		if err = result.addEncodedFact(key, inner); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}

func (p *FiniteSparseProduct) isTableColumn(column int) bool {
	for _, tableColumn := range p.tableColumns {
		if tableColumn == column {
			return true
		}
	}
	return false
}

func (p *FiniteSparseProduct) splitEncoded(values Fact) (Fact, Fact) {
	key := make(Fact, len(p.tableColumns)+1)
	inner := make(Fact, len(p.innerCols))
	for i, column := range p.tableColumns {
		key[i] = values[column]
	}
	for i, column := range p.innerCols {
		inner[i] = values[column]
	}
	return key, inner
}

// Join returns the concatenation join of two finite sparse products. It joins
// outer tables and nested sparse relations independently, then enforces any
// equalities that cross those partitions.
func (p *FiniteSparseProduct) Join(right *FiniteSparseProduct, leftColumns, rightColumns []int) (*FiniteSparseProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if err := right.open(); err != nil {
		return nil, err
	}
	if p.manager != right.manager {
		return nil, ErrTableRelation
	}
	for i := range leftColumns {
		if i >= len(rightColumns) || leftColumns[i] < 0 || leftColumns[i] >= len(p.sorts) || rightColumns[i] < 0 || rightColumns[i] >= len(right.sorts) || p.sorts[leftColumns[i]] != right.sorts[rightColumns[i]] {
			return nil, ErrTableRelation
		}
	}
	if len(leftColumns) != len(rightColumns) {
		return nil, ErrTableRelation
	}
	leftTablePosition := make(map[int]int, len(p.tableColumns))
	for position, column := range p.tableColumns {
		leftTablePosition[column] = position
	}
	rightTablePosition := make(map[int]int, len(right.tableColumns))
	for position, column := range right.tableColumns {
		rightTablePosition[column] = position
	}
	leftInnerPosition := make(map[int]int, len(p.innerCols))
	for position, column := range p.innerCols {
		leftInnerPosition[column] = position
	}
	rightInnerPosition := make(map[int]int, len(right.innerCols))
	for position, column := range right.innerCols {
		rightInnerPosition[column] = position
	}
	var tableLeft, tableRight, innerLeft, innerRight []int
	var crossPairs [][2]int
	for i, leftColumn := range leftColumns {
		rightColumn := rightColumns[i]
		leftTable, rightTable := p.isTableColumn(leftColumn), right.isTableColumn(rightColumn)
		switch {
		case leftTable && rightTable:
			tableLeft = append(tableLeft, leftTablePosition[leftColumn])
			tableRight = append(tableRight, rightTablePosition[rightColumn])
		case !leftTable && !rightTable:
			innerLeft = append(innerLeft, leftInnerPosition[leftColumn])
			innerRight = append(innerRight, rightInnerPosition[rightColumn])
		case leftTable:
			crossPairs = append(crossPairs, [2]int{leftColumn, len(p.sorts) + rightColumn})
		default:
			crossPairs = append(crossPairs, [2]int{leftColumn, len(p.sorts) + rightColumn})
		}
	}
	joinedOuter, err := p.table.Join(right.table, tableLeft, tableRight)
	if err != nil {
		return nil, err
	}
	defer joinedOuter.Close()
	sorts := append(append([]ast.ASTHandle(nil), p.sorts...), right.sorts...)
	mask := make([]bool, 0, len(sorts))
	for column := range p.sorts {
		mask = append(mask, p.isTableColumn(column))
	}
	for column := range right.sorts {
		mask = append(mask, right.isTableColumn(column))
	}
	result, err := NewFiniteSparseProduct(p.manager, sorts, mask)
	if err != nil {
		return nil, err
	}
	keepResult := false
	defer func() {
		if !keepResult {
			_ = result.Close()
		}
	}()
	outerFacts, err := joinedOuter.Facts()
	if err != nil {
		return nil, err
	}
	leftIndexColumn := len(p.tableColumns) + len(right.tableColumns)
	rightIndexColumn := leftIndexColumn + 1
	for _, outerFact := range outerFacts {
		leftIndex, rightIndex := int(outerFact[leftIndexColumn]), int(outerFact[rightIndexColumn])
		if leftIndex < 0 || leftIndex >= len(p.inner) || p.inner[leftIndex] == nil || rightIndex < 0 || rightIndex >= len(right.inner) || right.inner[rightIndex] == nil {
			return nil, ErrTableRelation
		}
		joinedInner, joinErr := p.inner[leftIndex].Join(right.inner[rightIndex], innerLeft, innerRight)
		if joinErr != nil {
			return nil, joinErr
		}
		key := make(Fact, len(result.tableColumns)+1)
		for position, column := range result.tableColumns {
			if column < len(p.sorts) {
				key[position] = outerFact[leftTablePosition[column]]
			} else {
				rightColumn := column - len(p.sorts)
				key[position] = outerFact[len(p.tableColumns)+rightTablePosition[rightColumn]]
			}
		}
		if err = result.storeEncodedInner(key, joinedInner); err != nil {
			return nil, errors.Join(err, joinedInner.Close())
		}
	}
	if len(crossPairs) != 0 {
		if err = result.FilterIdenticalPairs(crossPairs); err != nil {
			return nil, err
		}
	}
	keepResult = true
	return result, nil
}

// JoinTableRelation converts a table relation operand to a product with an
// empty inner signature, matching the source plugin's table conversion path.
func (p *FiniteSparseProduct) JoinTableRelation(right *TableRelation, leftColumns, rightColumns []int) (*FiniteSparseProduct, error) {
	converted, err := NewFiniteSparseProductFromTableRelation(right)
	if err != nil {
		return nil, err
	}
	defer converted.Close()
	return p.Join(converted, leftColumns, rightColumns)
}

// JoinFromTableRelation joins a table relation on the left with p on the
// right, preserving the source operand order in the result columns.
func (p *FiniteSparseProduct) JoinFromTableRelation(left *TableRelation, leftColumns, rightColumns []int) (*FiniteSparseProduct, error) {
	converted, err := NewFiniteSparseProductFromTableRelation(left)
	if err != nil {
		return nil, err
	}
	defer converted.Close()
	return converted.Join(p, leftColumns, rightColumns)
}

// JoinInnerSparseTable converts an inner sparse-table relation before joining
// it on the right, matching the source plugin's inner-relation conversion.
func (p *FiniteSparseProduct) JoinInnerSparseTable(right *SparseTable, rightSorts []ast.ASTHandle, leftColumns, rightColumns []int) (*FiniteSparseProduct, error) {
	converted, err := NewFiniteSparseProductFromInnerSparseTable(p.manager, rightSorts, right)
	if err != nil {
		return nil, err
	}
	defer converted.Close()
	return p.Join(converted, leftColumns, rightColumns)
}

// JoinFromInnerSparseTable converts an inner sparse-table relation before
// joining it on the left, preserving the source operand order.
func (p *FiniteSparseProduct) JoinFromInnerSparseTable(left *SparseTable, leftSorts []ast.ASTHandle, leftColumns, rightColumns []int) (*FiniteSparseProduct, error) {
	converted, err := NewFiniteSparseProductFromInnerSparseTable(p.manager, leftSorts, left)
	if err != nil {
		return nil, err
	}
	defer converted.Close()
	return converted.Join(p, leftColumns, rightColumns)
}

func (p *FiniteSparseProduct) flatten() (*SparseTable, error) {
	domains := make([]uint64, len(p.sorts))
	for column, sort := range p.sorts {
		domain, err := p.manager.sortDomain(sort)
		if err != nil {
			return nil, err
		}
		domains[column] = domain
	}
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	flat, err := NewSparseTable(signature, 0)
	if err != nil {
		return nil, err
	}
	keys, err := p.table.Facts()
	if err != nil {
		return nil, errors.Join(err, flat.Close())
	}
	for _, key := range keys {
		index := int(key[len(key)-1])
		if index < 0 || index >= len(p.inner) || p.inner[index] == nil {
			return nil, errors.Join(ErrTableRelation, flat.Close())
		}
		innerFacts, innerErr := p.inner[index].Facts()
		if innerErr != nil {
			return nil, errors.Join(innerErr, flat.Close())
		}
		for _, inner := range innerFacts {
			fact := make(Fact, len(p.sorts))
			for i, column := range p.tableColumns {
				fact[column] = key[i]
			}
			for i, column := range p.innerCols {
				fact[column] = inner[i]
			}
			if _, err = flat.Add(fact); err != nil {
				return nil, errors.Join(err, flat.Close())
			}
		}
	}
	return flat, nil
}

// FilterEqual keeps only rows whose selected column equals value.
func (p *FiniteSparseProduct) FilterEqual(column int, value ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if column < 0 || column >= len(p.sorts) {
		return ErrTableRelation
	}
	wanted, err := p.manager.EncodeValue(p.sorts[column], value)
	if err != nil {
		return err
	}
	flat, err := p.flatten()
	if err != nil {
		return err
	}
	defer flat.Close()
	facts, err := flat.Facts()
	if err != nil {
		return err
	}
	kept := make([]Fact, 0, len(facts))
	for _, fact := range facts {
		if fact[column] == wanted {
			kept = append(kept, fact)
		}
	}
	return p.replaceFacts(kept)
}

// FilterIdentical keeps rows where all selected columns have equal values.
func (p *FiniteSparseProduct) FilterIdentical(columns []int) error {
	if err := p.open(); err != nil {
		return err
	}
	for _, column := range columns {
		if column < 0 || column >= len(p.sorts) {
			return ErrTableRelation
		}
	}
	sameSort := true
	for i := 1; i < len(columns); i++ {
		if p.sorts[columns[i]] != p.sorts[columns[0]] {
			sameSort = false
			break
		}
	}
	flat, err := p.flatten()
	if err != nil {
		return err
	}
	defer flat.Close()
	facts, err := flat.Facts()
	if err != nil {
		return err
	}
	kept := make([]Fact, 0, len(facts))
	for _, fact := range facts {
		match := sameSort
		for i := 1; i < len(columns); i++ {
			if fact[columns[i]] != fact[columns[0]] {
				match = false
				break
			}
		}
		if match {
			kept = append(kept, fact)
		}
	}
	return p.replaceFacts(kept)
}

// FilterIdenticalPairs keeps rows satisfying each selected column equality.
func (p *FiniteSparseProduct) FilterIdenticalPairs(pairs [][2]int) error {
	if err := p.open(); err != nil {
		return err
	}
	for _, pair := range pairs {
		if pair[0] < 0 || pair[0] >= len(p.sorts) || pair[1] < 0 || pair[1] >= len(p.sorts) {
			return ErrTableRelation
		}
	}
	orderedPairs := append([][2]int(nil), pairs...)
	sortPairedColumnIndices(orderedPairs)
	flat, err := p.flatten()
	if err != nil {
		return err
	}
	defer flat.Close()
	facts, err := flat.Facts()
	if err != nil {
		return err
	}
	kept := make([]Fact, 0, len(facts))
	for _, fact := range facts {
		match := true
		for _, pair := range orderedPairs {
			if p.sorts[pair[0]] != p.sorts[pair[1]] || fact[pair[0]] != fact[pair[1]] {
				match = false
				break
			}
		}
		if match {
			kept = append(kept, fact)
		}
	}
	return p.replaceFacts(kept)
}

// FilterNegation removes each row in p that matches a row in negative on the
// selected column pairs, and reports the number of removed rows.
func (p *FiniteSparseProduct) FilterNegation(negative *FiniteSparseProduct, targetColumns, negativeColumns []int) (int, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	if err := negative.open(); err != nil {
		return 0, err
	}
	if p.manager != negative.manager || len(targetColumns) != len(negativeColumns) {
		return 0, ErrTableRelation
	}
	for i, targetColumn := range targetColumns {
		negativeColumn := negativeColumns[i]
		if targetColumn < 0 || targetColumn >= len(p.sorts) || negativeColumn < 0 || negativeColumn >= len(negative.sorts) || p.sorts[targetColumn] != negative.sorts[negativeColumn] {
			return 0, ErrTableRelation
		}
	}
	target, err := p.flatten()
	if err != nil {
		return 0, err
	}
	defer target.Close()
	negated, err := negative.flatten()
	if err != nil {
		return 0, err
	}
	defer negated.Close()
	before, err := target.Len()
	if err != nil {
		return 0, err
	}
	if err = target.FilterNegation(negated, targetColumns, negativeColumns); err != nil {
		return 0, err
	}
	afterFacts, err := target.Facts()
	if err != nil {
		return 0, err
	}
	if err = p.replaceFacts(afterFacts); err != nil {
		return 0, err
	}
	return before - len(afterFacts), nil
}

func (p *FiniteSparseProduct) replaceFacts(facts []Fact) error {
	replacement, err := p.productFromFacts(facts)
	if err != nil {
		return err
	}
	p.swapStorage(replacement)
	return replacement.Close()
}

func (p *FiniteSparseProduct) productFromFacts(facts []Fact) (*FiniteSparseProduct, error) {
	replacement, err := NewFiniteSparseProduct(p.manager, p.sorts, p.tableColumnMask())
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		key, inner := replacement.splitEncoded(fact)
		if err = replacement.addEncodedFact(key, inner); err != nil {
			return nil, errors.Join(err, replacement.Close())
		}
	}
	return replacement, nil
}

func (p *FiniteSparseProduct) swapStorage(replacement *FiniteSparseProduct) {
	p.table, replacement.table = replacement.table, p.table
	p.inner, replacement.inner = replacement.inner, p.inner
	p.available, replacement.available = replacement.available, p.available
}

func (p *FiniteSparseProduct) tableColumnMask() []bool {
	mask := make([]bool, len(p.sorts))
	for _, column := range p.tableColumns {
		mask[column] = true
	}
	return mask
}

func (p *FiniteSparseProduct) ContainsFact(values []ast.ASTHandle) (bool, error) {
	key, innerFact, err := p.split(values)
	if err != nil {
		return false, err
	}
	found, err := p.fetchInnerKey(key)
	if err != nil || !found {
		return false, err
	}
	index := uint64(key[len(key)-1])
	if index >= uint64(len(p.inner)) || p.inner[index] == nil {
		return false, ErrTableRelation
	}
	return p.inner[index].Contains(innerFact)
}

// Empty reports whether the product has any live inner facts. Empty inner
// relations are removed along with their outer keys, matching the native
// finite-product garbage collection performed by empty().
func (p *FiniteSparseProduct) Empty() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if err := p.collectGarbage(true); err != nil {
		return false, err
	}
	n, err := p.table.Len()
	return n == 0, err
}

func (p *FiniteSparseProduct) collectGarbage(removeEmpty bool) error {
	keys, err := p.table.Facts()
	if err != nil {
		return err
	}
	live := make(map[int][]Fact, len(keys))
	for _, key := range keys {
		index := int(key[len(key)-1])
		if index < 0 || index >= len(p.inner) || p.inner[index] == nil {
			return ErrTableRelation
		}
		live[index] = append(live[index], key)
	}
	for index, inner := range p.inner {
		if inner == nil {
			continue
		}
		referencingKeys, referenced := live[index]
		if referenced && removeEmpty {
			n, err := p.inner[index].Len()
			if err != nil {
				return err
			}
			if n != 0 {
				continue
			}
			for _, key := range referencingKeys {
				if _, err = p.table.Remove(key); err != nil {
					return err
				}
			}
		}
		if referenced && !removeEmpty {
			continue
		}
		if err = inner.Close(); err != nil {
			return err
		}
		p.inner[index] = nil
		p.available = append(p.available, index)
	}
	if len(p.inner) != 0 && len(p.available) == len(p.inner) {
		p.inner = nil
		p.available = nil
	}
	return nil
}

// UnionFrom adds source rows into p. The source and destination must have the
// same finite signature and column partition; inner tables are copied before
// updates so source ownership remains independent.
func (p *FiniteSparseProduct) UnionFrom(source *FiniteSparseProduct) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil {
		return err
	}
	if !p.sameSorts(source) {
		return ErrTableRelation
	}
	if !p.sameLayout(source) {
		mask := p.commonTableMask(source)
		convertedTarget, err := p.repartition(mask)
		if err != nil {
			return err
		}
		convertedSource, err := source.repartition(mask)
		if err != nil {
			return errors.Join(err, convertedTarget.Close())
		}
		if err = convertedTarget.UnionFrom(convertedSource); err != nil {
			return errors.Join(err, convertedTarget.Close(), convertedSource.Close())
		}
		p.swapRelation(convertedTarget)
		return errors.Join(convertedTarget.Close(), convertedSource.Close())
	}
	rows, err := source.table.Facts()
	if err != nil {
		return err
	}
	for _, sourceKey := range rows {
		sourceIndex := int(sourceKey[len(sourceKey)-1])
		if sourceIndex < 0 || sourceIndex >= len(source.inner) || source.inner[sourceIndex] == nil {
			return ErrTableRelation
		}
		key := append(Fact(nil), sourceKey...)
		exists, err := p.fetchInnerKey(key)
		if err != nil {
			return err
		}
		var merged *SparseTable
		var oldIndex int = -1
		if exists {
			oldIndex = int(key[len(key)-1])
			if oldIndex < 0 || oldIndex >= len(p.inner) || p.inner[oldIndex] == nil {
				return ErrTableRelation
			}
			merged, err = p.inner[oldIndex].Clone()
		} else {
			merged, err = NewSparseTable(p.innerSignature, 0)
		}
		if err != nil {
			return err
		}
		facts, factsErr := source.inner[sourceIndex].Facts()
		if factsErr != nil {
			return errors.Join(factsErr, merged.Close())
		}
		for _, fact := range facts {
			if _, err = merged.Add(fact); err != nil {
				return errors.Join(err, merged.Close())
			}
		}
		index, err := p.reserveIndex()
		if err != nil {
			return errors.Join(err, merged.Close())
		}
		key[len(key)-1] = Element(index)
		if err = p.table.EnsureFact(key); err != nil {
			p.recycleReservedIndex(index)
			return errors.Join(err, merged.Close())
		}
		p.inner[index] = merged
		if oldIndex >= 0 {
			old := p.inner[oldIndex]
			p.inner[oldIndex] = nil
			p.available = append(p.available, oldIndex)
			if err = old.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// UnionFromWithDelta adds source facts to p and accumulates only newly added
// facts in delta. Existing delta facts are preserved.
func (p *FiniteSparseProduct) UnionFromWithDelta(source, delta *FiniteSparseProduct) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil {
		return err
	}
	if err := delta.open(); err != nil {
		return err
	}
	if !p.sameSorts(source) || !p.sameSorts(delta) || p == delta || source == delta {
		return ErrTableRelation
	}
	if !p.sameLayout(source) || !p.sameLayout(delta) {
		mask := p.commonTableMask(source, delta)
		convertedTarget, err := p.repartition(mask)
		if err != nil {
			return err
		}
		convertedSource, err := source.repartition(mask)
		if err != nil {
			return errors.Join(err, convertedTarget.Close())
		}
		convertedDelta, err := delta.repartition(mask)
		if err != nil {
			return errors.Join(err, convertedTarget.Close(), convertedSource.Close())
		}
		if err = convertedTarget.UnionFromWithDelta(convertedSource, convertedDelta); err != nil {
			return errors.Join(err, convertedTarget.Close(), convertedSource.Close(), convertedDelta.Close())
		}
		p.swapRelation(convertedTarget)
		delta.swapRelation(convertedDelta)
		return errors.Join(convertedTarget.Close(), convertedSource.Close(), convertedDelta.Close())
	}
	targetTable, err := p.flatten()
	if err != nil {
		return err
	}
	defer targetTable.Close()
	sourceTable, err := source.flatten()
	if err != nil {
		return err
	}
	defer sourceTable.Close()
	deltaTable, err := delta.flatten()
	if err != nil {
		return err
	}
	defer deltaTable.Close()
	if err = targetTable.Union(sourceTable, deltaTable); err != nil {
		return err
	}
	targetFacts, err := targetTable.Facts()
	if err != nil {
		return err
	}
	deltaFacts, err := deltaTable.Facts()
	if err != nil {
		return err
	}
	newTarget, err := p.productFromFacts(targetFacts)
	if err != nil {
		return err
	}
	newDelta, err := delta.productFromFacts(deltaFacts)
	if err != nil {
		return errors.Join(err, newTarget.Close())
	}
	p.swapStorage(newTarget)
	delta.swapStorage(newDelta)
	return errors.Join(newTarget.Close(), newDelta.Close())
}

func (p *FiniteSparseProduct) sameLayout(other *FiniteSparseProduct) bool {
	if !p.sameSorts(other) || len(p.tableColumns) != len(other.tableColumns) || !p.innerSignature.equal(other.innerSignature) {
		return false
	}
	for i := range p.tableColumns {
		if p.tableColumns[i] != other.tableColumns[i] {
			return false
		}
	}
	return true
}

func (p *FiniteSparseProduct) sameSorts(other *FiniteSparseProduct) bool {
	if other == nil || p.manager != other.manager || len(p.sorts) != len(other.sorts) {
		return false
	}
	for i := range p.sorts {
		if p.sorts[i] != other.sorts[i] {
			return false
		}
	}
	return true
}

func (p *FiniteSparseProduct) commonTableMask(others ...*FiniteSparseProduct) []bool {
	mask := p.tableColumnMask()
	for _, other := range others {
		for column := range mask {
			mask[column] = mask[column] && other.isTableColumn(column)
		}
	}
	return mask
}

func (p *FiniteSparseProduct) repartition(mask []bool) (*FiniteSparseProduct, error) {
	flat, err := p.flatten()
	if err != nil {
		return nil, err
	}
	defer flat.Close()
	facts, err := flat.Facts()
	if err != nil {
		return nil, err
	}
	result, err := NewFiniteSparseProduct(p.manager, p.sorts, mask)
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		key, inner := result.splitEncoded(fact)
		if err = result.addEncodedFact(key, inner); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}

func (p *FiniteSparseProduct) swapRelation(other *FiniteSparseProduct) {
	p.swapStorage(other)
	p.tableColumns, other.tableColumns = other.tableColumns, p.tableColumns
	p.innerCols, other.innerCols = other.innerCols, p.innerCols
	p.innerSignature, other.innerSignature = other.innerSignature, p.innerSignature
}

func (p *FiniteSparseProduct) reserveIndex() (int, error) {
	if n := len(p.available); n != 0 {
		index := p.available[n-1]
		p.available = p.available[:n-1]
		return index, nil
	}
	if uint64(len(p.inner)) >= math.MaxInt32 {
		return -1, ErrTableRelation
	}
	index := len(p.inner)
	p.inner = append(p.inner, nil)
	return index, nil
}

func (p *FiniteSparseProduct) recycleReservedIndex(index int) {
	if index == len(p.inner)-1 && p.inner[index] == nil {
		p.inner = p.inner[:index]
		return
	}
	p.available = append(p.available, index)
}

func (p *FiniteSparseProduct) Clone() (*FiniteSparseProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	table, err := p.table.Clone()
	if err != nil {
		return nil, err
	}
	copy := &FiniteSparseProduct{
		manager: p.manager, sorts: append([]ast.ASTHandle(nil), p.sorts...),
		tableColumns: append([]int(nil), p.tableColumns...), innerCols: append([]int(nil), p.innerCols...),
		innerSignature: p.innerSignature, table: table, inner: make([]*SparseTable, len(p.inner)),
		available: append([]int(nil), p.available...),
	}
	for i, inner := range p.inner {
		if inner != nil {
			copy.inner[i], err = inner.Clone()
			if err != nil {
				return nil, errors.Join(err, copy.Close())
			}
		}
	}
	return copy, nil
}

// Complement returns the finite-domain complement of p. The source table
// complement contract accepts at most one data column in each product layer.
func (p *FiniteSparseProduct) Complement() (*FiniteSparseProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if len(p.tableColumns) > 1 || len(p.innerCols) > 1 {
		return nil, ErrTableRelation
	}
	flat, err := p.flatten()
	if err != nil {
		return nil, err
	}
	defer flat.Close()
	existingFacts, err := flat.Facts()
	if err != nil {
		return nil, err
	}
	existing := make(map[string]struct{}, len(existingFacts))
	for _, fact := range existingFacts {
		existing[factKey(fact)] = struct{}{}
	}
	result, err := NewFiniteSparseProduct(p.manager, p.sorts, p.tableColumnMask())
	if err != nil {
		return nil, err
	}
	row := make(Fact, len(p.sorts))
	var enumerate func(int) error
	enumerate = func(column int) error {
		if column == len(row) {
			if _, found := existing[factKey(row)]; found {
				return nil
			}
			key, inner := result.splitEncoded(row)
			return result.addEncodedFact(key, inner)
		}
		domain, err := p.manager.sortDomain(p.sorts[column])
		if err != nil {
			return err
		}
		for value := uint64(0); value < domain; value++ {
			row[column] = Element(value)
			if err = enumerate(column + 1); err != nil {
				return err
			}
		}
		return nil
	}
	if err = enumerate(0); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

// ComplementSelf replaces p with its finite-domain complement. The replacement
// is built completely before p's storage is changed, so enumeration or
// allocation failures leave the original relation intact.
func (p *FiniteSparseProduct) ComplementSelf() error {
	if err := p.open(); err != nil {
		return err
	}
	replacement, err := p.Complement()
	if err != nil {
		return err
	}
	p.swapStorage(replacement)
	return replacement.Close()
}

// NewFullFiniteSparseProduct constructs the full finite-domain product using
// the source mk_full sequence of empty construction followed by complement.
func NewFullFiniteSparseProduct(manager *RelationManager, sorts []ast.ASTHandle, tableColumns []bool) (*FiniteSparseProduct, error) {
	empty, err := NewFiniteSparseProduct(manager, sorts, tableColumns)
	if err != nil {
		return nil, err
	}
	defer empty.Close()
	return empty.Complement()
}

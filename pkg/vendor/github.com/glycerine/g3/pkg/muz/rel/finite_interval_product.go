package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp/.h, concrete sparse-table
// and interval-relation instantiation.

import (
	"errors"
	"fmt"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// FiniteIntervalProduct maps each tuple of finite columns to an interval
// abstraction over the remaining arithmetic columns. It borrows the manager
// and sort handles, and owns the table and interval relations.
type FiniteIntervalProduct struct {
	manager                       *RelationManager
	sorts                         []ast.ASTHandle
	tableColumns, intervalColumns []int
	table                         *SparseTable
	intervals                     []*IntervalRelation
	available                     []int
}

// NewFiniteIntervalProduct selects every table-compatible sort when columns is
// nil; an explicit mask may instead leave arithmetic columns in the interval.
func NewFiniteIntervalProduct(manager *RelationManager, sorts []ast.ASTHandle, columns []bool) (*FiniteIntervalProduct, error) {
	if manager == nil || manager.open() != nil || columns != nil && len(columns) != len(sorts) {
		return nil, ast.ErrASTDomain
	}
	m := manager.manager
	if _, err := m.RegisterArithmeticPlugin(); err != nil {
		return nil, err
	}
	p := &FiniteIntervalProduct{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...)}
	var domains []uint64
	for i, sort := range sorts {
		kind, err := m.GetKind(sort)
		if err != nil {
			return nil, err
		}
		if kind != ast.ASTSort {
			return nil, ast.ErrASTDomain
		}
		domain, domainErr := manager.sortDomain(sort)
		asTable := domainErr == nil
		if columns != nil {
			asTable = columns[i]
		}
		if asTable {
			if domainErr != nil {
				return nil, domainErr
			}
			p.tableColumns = append(p.tableColumns, i)
			domains = append(domains, domain)
		} else {
			if !CanHandleIntervalSorts(m, []ast.ASTHandle{sort}) {
				return nil, fmt.Errorf("%w: interval columns must be arithmetic", ErrTableRelation)
			}
			p.intervalColumns = append(p.intervalColumns, i)
		}
	}
	// The source stores inner relation indexes in an INT_MAX-sized functional column.
	domains = append(domains, math.MaxInt32)
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	p.table, err = NewSparseTable(signature, 1)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// NewFullFiniteIntervalProduct implements source mk_full for this concrete
// interval specialization by complementing a newly empty product.
func NewFullFiniteIntervalProduct(manager *RelationManager, sorts []ast.ASTHandle, columns []bool) (_ *FiniteIntervalProduct, resultErr error) {
	p, err := NewFiniteIntervalProduct(manager, sorts, columns)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	fullTable, err := p.table.ComplementFunctional(Fact{0})
	if err != nil {
		return nil, err
	}
	if err = p.table.Close(); err != nil {
		return nil, errors.Join(err, fullTable.Close())
	}
	p.table = fullTable
	p.intervals = append(p.intervals, NewIntervalFull(uint32(len(p.intervalColumns))))
	return p, nil
}

func (p *FiniteIntervalProduct) open() error {
	if p == nil || p.manager == nil || p.manager.open() != nil || p.table == nil || p.table.open() != nil {
		return ErrTableRelation
	}
	return nil
}

func (p *FiniteIntervalProduct) splitFact(values []ast.ASTHandle) (Fact, []ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, nil, err
	}
	if len(values) != len(p.sorts) {
		return nil, nil, ast.ErrASTDomain
	}
	key := make(Fact, len(p.tableColumns)+1)
	for i, col := range p.tableColumns {
		value, err := p.manager.EncodeValue(p.sorts[col], values[col])
		if err != nil {
			return nil, nil, err
		}
		key[i] = value
	}
	other := make([]ast.ASTHandle, len(p.intervalColumns))
	for i, col := range p.intervalColumns {
		kind, err := p.manager.manager.GetKind(values[col])
		if err != nil {
			return nil, nil, err
		}
		if kind != ast.ASTApp {
			return nil, nil, ast.ErrASTDomain
		}
		sort, err := p.manager.manager.GetSort(values[col])
		if err != nil {
			return nil, nil, err
		}
		if sort != p.sorts[col] {
			return nil, nil, ast.ErrASTDomain
		}
		other[i] = values[col]
	}
	return key, other, nil
}

// intervalFact mirrors interval_relation::add_fact's full relation followed
// by interpreted equalities for the supplied expressions.
func (p *FiniteIntervalProduct) intervalFact(values []ast.ASTHandle) (_ *IntervalRelation, resultErr error) {
	sorts := make([]ast.ASTHandle, len(p.intervalColumns))
	for i, column := range p.intervalColumns {
		sorts[i] = p.sorts[column]
	}
	inner := NewIntervalEmpty(uint32(len(values)))
	if err := inner.AddFactExpressions(p.manager.manager, sorts, values); err != nil {
		return nil, err
	}
	return inner, nil
}

func cloneIntervalRelation(source *IntervalRelation) *IntervalRelation {
	values := source.values
	copy := newIntervalColumns(len(values.columns), values.empty)
	if values.empty {
		return &IntervalRelation{values: copy}
	}
	// vector_relation::copy reads each representative's value, but its merge
	// loop uses the fresh destination's representatives, so classes split.
	for i := range values.columns {
		interval := values.columns[values.find(uint32(i))]
		lo, loOpen, hi, hiOpen := interval.Bounds()
		copy.columns[i] = smtcore.NewRelationInterval(lo, loOpen, hi, hiOpen)
	}
	return &IntervalRelation{values: copy}
}

// AddFact creates a fresh inner relation for a new table key, and replaces an
// existing key's inner relation with a clone extended by the new fact.
func (p *FiniteIntervalProduct) AddFact(values []ast.ASTHandle) error {
	key, other, err := p.splitFact(values)
	if err != nil {
		return err
	}
	found, err := p.fetchInnerKey(key)
	if err != nil {
		return err
	}
	inner := NewIntervalEmpty(uint32(len(other)))
	if found {
		index := uint64(key[len(key)-1])
		if index >= uint64(len(p.intervals)) || p.intervals[index] == nil {
			return ErrTableRelation
		}
		inner = cloneIntervalRelation(p.intervals[index])
	}
	fact, err := p.intervalFact(other)
	if err != nil {
		return err
	}
	if _, err = inner.Union(fact, false); err != nil {
		return err
	}
	return p.storeInner(key, inner)
}

func (p *FiniteIntervalProduct) storeInner(key Fact, inner *IntervalRelation) error {
	if len(p.available) == 0 && uint64(len(p.intervals)) >= math.MaxInt32 {
		return ErrTableRelation
	}
	index := len(p.intervals)
	if len(p.available) != 0 {
		index = p.available[len(p.available)-1]
	}
	key[len(key)-1] = Element(index)
	if err := p.table.EnsureFact(key); err != nil {
		return err
	}
	if index == len(p.intervals) {
		p.intervals = append(p.intervals, inner)
	} else {
		p.available = p.available[:len(p.available)-1]
		p.intervals[index] = inner
	}
	return nil
}

// ContainsFact follows the source interval lookup: non-numerals are accepted
// conservatively, and rows with empty inner relations remain visible until a
// collection operation such as Empty removes them.
func (p *FiniteIntervalProduct) ContainsFact(values []ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if len(values) != len(p.sorts) {
		return false, ast.ErrASTDomain
	}
	key := make(Fact, len(p.tableColumns)+1)
	for local, global := range p.tableColumns {
		value, err := p.manager.EncodeValue(p.sorts[global], values[global])
		if err != nil {
			return false, err
		}
		key[local] = value
	}
	found, err := p.fetchInnerKey(key)
	if err != nil || !found {
		return false, err
	}
	index := uint64(key[len(key)-1])
	if index >= uint64(len(p.intervals)) || p.intervals[index] == nil {
		return false, ErrTableRelation
	}
	inner := p.intervals[index]
	m := p.manager.manager
	innerValues := make([]ast.ASTHandle, len(p.intervalColumns))
	for local, global := range p.intervalColumns {
		sort, err := m.GetSort(values[global])
		if err != nil {
			return false, err
		}
		if sort != p.sorts[global] {
			return false, ast.ErrASTDomain
		}
		innerValues[local] = values[global]
	}
	return inner.ContainsFactExpressions(m, innerValues)
}

// FilterEqual restricts a finite column in the table or an arithmetic column
// in each live interval, following the two source filter paths.
func (p *FiniteIntervalProduct) FilterEqual(column uint32, value ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if uint64(column) >= uint64(len(p.sorts)) {
		return ast.ErrASTDomain
	}
	for local, global := range p.tableColumns {
		if global != int(column) {
			continue
		}
		encoded, err := p.manager.EncodeValue(p.sorts[column], value)
		if err != nil {
			return err
		}
		rows, err := p.table.Facts()
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row[local] != encoded {
				if _, err = p.table.Remove(row); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for local, global := range p.intervalColumns {
		if global != int(column) {
			continue
		}
		sort, err := p.manager.manager.GetSort(value)
		if err != nil {
			return err
		}
		if sort != p.sorts[column] {
			return ast.ErrASTDomain
		}
		number, err := intervalEqualityValue(p.manager.manager, value)
		if err != nil {
			return err
		}
		if err = p.garbageCollect(false); err != nil {
			return err
		}
		for _, inner := range p.intervals {
			if inner != nil {
				if err = inner.FilterEqual(uint32(local), number); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return ErrTableRelation
}

// FilterIdentical enforces equality across columns of the same sort.
func (p *FiniteIntervalProduct) FilterIdentical(columns []uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	var tableCols, innerCols []uint32
	for i, column := range columns {
		if uint64(column) >= uint64(len(p.sorts)) || i > 0 && p.sorts[column] != p.sorts[columns[0]] {
			return ast.ErrASTDomain
		}
		for local, global := range p.tableColumns {
			if global == int(column) {
				tableCols = append(tableCols, uint32(local))
			}
		}
		for local, global := range p.intervalColumns {
			if global == int(column) {
				innerCols = append(innerCols, uint32(local))
			}
		}
	}
	if len(tableCols) != 0 && len(innerCols) != 0 {
		return ErrTableRelation
	}
	if len(tableCols) > 1 {
		rows, err := p.table.Facts()
		if err != nil {
			return err
		}
		for _, row := range rows {
			for _, column := range tableCols[1:] {
				if row[column] != row[tableCols[0]] {
					if _, err = p.table.Remove(row); err != nil {
						return err
					}
					break
				}
			}
		}
	}
	if len(innerCols) > 1 {
		if err := p.garbageCollect(true); err != nil {
			return err
		}
		for _, inner := range p.intervals {
			if inner != nil {
				for _, column := range innerCols[1:] {
					if err := inner.FilterIdentical(innerCols[0], column); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (p *FiniteIntervalProduct) sameSpecification(other *FiniteIntervalProduct) bool {
	if other == nil || p.manager != other.manager || len(p.sorts) != len(other.sorts) || len(p.tableColumns) != len(other.tableColumns) {
		return false
	}
	for i := range p.sorts {
		if p.sorts[i] != other.sorts[i] {
			return false
		}
	}
	for i := range p.tableColumns {
		if p.tableColumns[i] != other.tableColumns[i] {
			return false
		}
	}
	return true
}

// Union merges aligned table keys and unions their interval relations. Delta,
// when supplied, accumulates the fresh interval deltas and source-only rows.
func (p *FiniteIntervalProduct) Union(source, delta *FiniteIntervalProduct) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil || !p.sameSpecification(source) {
		return ErrTableRelation
	}
	if delta != nil {
		if delta == p || delta == source || delta.open() != nil || !p.sameSpecification(delta) {
			return ErrTableRelation
		}
		if _, err := delta.Empty(); err != nil {
			return err
		}
	}
	var fresh *FiniteIntervalProduct
	if delta != nil {
		mask := make([]bool, len(p.sorts))
		for _, column := range p.tableColumns {
			mask[column] = true
		}
		var err error
		fresh, err = NewFiniteIntervalProduct(p.manager, p.sorts, mask)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, fresh.Close()) }()
	}
	rows, err := source.table.Facts()
	if err != nil {
		return err
	}
	for _, row := range rows {
		sourceIndex := uint64(row[len(row)-1])
		if sourceIndex >= uint64(len(source.intervals)) || source.intervals[sourceIndex] == nil {
			return ErrTableRelation
		}
		src := source.intervals[sourceIndex]
		key := cloneFact(row)
		key[len(key)-1] = 0
		found, err := p.fetchInnerKey(key)
		if err != nil {
			return err
		}
		var merged, change *IntervalRelation
		if found {
			index := uint64(key[len(key)-1])
			if index >= uint64(len(p.intervals)) || p.intervals[index] == nil {
				return ErrTableRelation
			}
			merged = cloneIntervalRelation(p.intervals[index])
			if fresh != nil {
				change = NewIntervalEmpty(uint32(len(p.intervalColumns)))
				_, err = merged.UnionWithDelta(src, change, false)
			} else {
				_, err = merged.Union(src, false)
			}
			if err != nil {
				return err
			}
		} else {
			merged = cloneIntervalRelation(src)
			if fresh != nil {
				change = cloneIntervalRelation(src)
			}
		}
		if err = p.storeInner(key, merged); err != nil {
			return err
		}
		if fresh != nil {
			if err = fresh.storeInner(key, change); err != nil {
				return err
			}
		}
	}
	if fresh != nil {
		return delta.Union(fresh, nil)
	}
	return nil
}

// Project removes sorted signature columns. Table-key collisions are merged
// before inner interval columns are projected, matching the source order.
func (p *FiniteIntervalProduct) Project(removed []uint32) (_ *FiniteIntervalProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	deleted := make([]bool, len(p.sorts))
	for i, column := range removed {
		if uint64(column) >= uint64(len(p.sorts)) || i > 0 && column <= removed[i-1] {
			return nil, ast.ErrASTDomain
		}
		deleted[column] = true
	}
	if err := p.garbageCollect(false); err != nil {
		return nil, err
	}
	var sorts []ast.ASTHandle
	mask := make([]bool, 0, len(p.sorts)-len(removed))
	isTable := make([]bool, len(p.sorts))
	for _, column := range p.tableColumns {
		isTable[column] = true
	}
	for i, sort := range p.sorts {
		if !deleted[i] {
			sorts = append(sorts, sort)
			mask = append(mask, isTable[i])
		}
	}
	var removedInner []uint32
	for local, global := range p.intervalColumns {
		if deleted[global] {
			removedInner = append(removedInner, uint32(local))
		}
	}
	result, err := NewFiniteIntervalProduct(p.manager, sorts, mask)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	rows, err := p.table.Facts()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		index := uint64(row[len(row)-1])
		if index >= uint64(len(p.intervals)) || p.intervals[index] == nil {
			return nil, ErrTableRelation
		}
		key := make(Fact, 0, len(result.tableColumns)+1)
		for local, global := range p.tableColumns {
			if !deleted[global] {
				key = append(key, row[local])
			}
		}
		key = append(key, 0)
		found, err := result.fetchInnerKey(key)
		if err != nil {
			return nil, err
		}
		inner := cloneIntervalRelation(p.intervals[index])
		if found {
			existing := uint64(key[len(key)-1])
			if existing >= uint64(len(result.intervals)) || result.intervals[existing] == nil {
				return nil, ErrTableRelation
			}
			merged := cloneIntervalRelation(result.intervals[existing])
			if _, err = merged.Union(inner, false); err != nil {
				return nil, err
			}
			inner = merged
		}
		if err = result.storeInner(key, inner); err != nil {
			return nil, err
		}
	}
	if len(removedInner) != 0 {
		for i, inner := range result.intervals {
			if inner != nil {
				result.intervals[i], err = inner.Project(removedInner)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return result, nil
}

// Rename rotates the global signature and applies the corresponding table
// and inner permutations, including cycles that cross the two column kinds.
func (p *FiniteIntervalProduct) Rename(cycle []uint32) (_ *FiniteIntervalProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if len(cycle) == 0 {
		return nil, ast.ErrASTDomain
	}
	permutation := make([]int, len(p.sorts))
	for i := range permutation {
		permutation[i] = i
	}
	seen := make([]bool, len(permutation))
	for _, column := range cycle {
		if uint64(column) >= uint64(len(permutation)) || seen[column] {
			return nil, ast.ErrASTDomain
		}
		seen[column] = true
	}
	for i, column := range cycle {
		permutation[column] = int(cycle[(i+1)%len(cycle)])
	}
	if err := p.garbageCollect(false); err != nil {
		return nil, err
	}
	tableLocal, innerLocal := make([]int, len(p.sorts)), make([]int, len(p.sorts))
	for i := range p.sorts {
		tableLocal[i], innerLocal[i] = -1, -1
	}
	for local, global := range p.tableColumns {
		tableLocal[global] = local
	}
	for local, global := range p.intervalColumns {
		innerLocal[global] = local
	}
	sorts := make([]ast.ASTHandle, len(p.sorts))
	mask := make([]bool, len(p.sorts))
	var tablePermutation []int
	var innerPermutation []uint32
	for i, old := range permutation {
		sorts[i] = p.sorts[old]
		if tableLocal[old] >= 0 {
			mask[i] = true
			tablePermutation = append(tablePermutation, tableLocal[old])
		} else {
			innerPermutation = append(innerPermutation, uint32(innerLocal[old]))
		}
	}
	result, err := NewFiniteIntervalProduct(p.manager, sorts, mask)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	result.intervals = make([]*IntervalRelation, len(p.intervals))
	result.available = append([]int(nil), p.available...)
	for i, inner := range p.intervals {
		if inner == nil {
			continue
		}
		renamed := cloneIntervalRelation(inner)
		remaining := append([]uint32(nil), innerPermutation...)
		for {
			localCycle, found := removePermutationCycle(remaining)
			if !found {
				break
			}
			renamed, err = renamed.Rename(localCycle)
			if err != nil {
				return nil, err
			}
		}
		result.intervals[i] = renamed
	}
	rows, err := p.table.Facts()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		newRow := make(Fact, len(row))
		for i, old := range tablePermutation {
			newRow[i] = row[old]
		}
		newRow[len(newRow)-1] = row[len(row)-1]
		if err = result.table.EnsureFact(newRow); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// garbageCollect drops inner relations no longer referenced by the table and,
// when requested, removes rows referring to empty inner relations.
func (p *FiniteIntervalProduct) garbageCollect(removeEmpty bool) error {
	rows, err := p.table.Facts()
	if err != nil {
		return err
	}
	live := make([]bool, len(p.intervals))
	for _, row := range rows {
		index := uint64(row[len(row)-1])
		if index >= uint64(len(p.intervals)) || p.intervals[index] == nil {
			return ErrTableRelation
		}
		if removeEmpty && p.intervals[index].Empty() {
			if _, err = p.table.Remove(row); err != nil {
				return err
			}
		} else {
			live[index] = true
		}
	}
	for i, interval := range p.intervals {
		if interval != nil && !live[i] {
			p.intervals[i] = nil
			p.available = append(p.available, i)
		}
	}
	if len(p.available) == len(p.intervals) {
		p.available = nil
		p.intervals = nil
	}
	return nil
}

func (p *FiniteIntervalProduct) Empty() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if err := p.garbageCollect(true); err != nil {
		return false, err
	}
	n, err := p.table.Len()
	return n == 0, err
}

// Reset clears the table and collects every no-longer-referenced interval.
func (p *FiniteIntervalProduct) Reset() error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.table.Reset(); err != nil {
		return err
	}
	return p.garbageCollect(false)
}

func (p *FiniteIntervalProduct) Clone() (*FiniteIntervalProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	table, err := p.table.Clone()
	if err != nil {
		return nil, err
	}
	copy := &FiniteIntervalProduct{manager: p.manager, sorts: append([]ast.ASTHandle(nil), p.sorts...), tableColumns: append([]int(nil), p.tableColumns...), intervalColumns: append([]int(nil), p.intervalColumns...), table: table, intervals: make([]*IntervalRelation, len(p.intervals)), available: append([]int(nil), p.available...)}
	for i, inner := range p.intervals {
		if inner != nil {
			copy.intervals[i] = cloneIntervalRelation(inner)
		}
	}
	return copy, nil
}

func (p *FiniteIntervalProduct) Close() error {
	if p == nil || p.table == nil {
		return nil
	}
	err := p.table.Close()
	p.table, p.manager, p.sorts, p.intervals, p.available = nil, nil, nil, nil, nil
	return err
}

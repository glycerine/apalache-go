package rel

// Source: src/muz/rel/udoc_relation.cpp udoc_plugin::filter_equal_fn.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/backtrack"
)

// FilterIdentical retains rows whose selected columns are bitwise equal.
func (r *UDocRelation) FilterIdentical(columns []uint32) error {
	if err := r.check(); err != nil {
		return err
	}
	if len(columns) == 0 {
		return ErrDOC
	}
	width, err := r.ColumnNumBits(columns[0])
	if err != nil {
		return err
	}
	classes := backtrack.NewDefaultUnionFind()
	for i := uint32(0); i < r.NumBits(); i++ {
		classes.MkVar()
	}
	for _, column := range columns {
		if column >= uint32(len(r.widths)) || r.widths[column] != width {
			return ErrDOC
		}
	}
	base := r.offsets[columns[0]]
	for _, column := range columns[1:] {
		other := r.offsets[column]
		for bit := uint32(0); bit < width; bit++ {
			classes.Merge(base+bit, other+bit)
		}
	}
	discarded := make([]bool, r.NumBits())
	for i := 0; i < len(r.elems.values); {
		valid, err := r.manager.MergeRange(r.elems.values[i], base, width, classes, discarded)
		if err != nil {
			return err
		}
		if !valid {
			if err = r.elems.erase(i); err != nil {
				return err
			}
			continue
		}
		i++
	}
	return nil
}

// FilterEqual keeps rows whose selected bit-vector column equals value.
func (r *UDocRelation) FilterEqual(column uint32, value uint64) (resultErr error) {
	return r.FilterEqualBig(column, new(big.Int).SetUint64(value))
}

// FilterEqualBig preserves bit-vector values wider than a machine word.
func (r *UDocRelation) FilterEqualBig(column uint32, value *big.Int) (resultErr error) {
	if err := r.check(); err != nil {
		return err
	}
	if column >= uint32(len(r.widths)) {
		return arena.ErrBounds
	}
	if value == nil || value.Sign() < 0 {
		return ErrDOC
	}
	width := r.widths[column]
	if value.BitLen() > int(width) {
		return ErrDOC
	}
	document, err := r.manager.AllocateX()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.Deallocate(document)) }()
	low, high := r.offsets[column], r.offsets[column+1]-1
	if err = r.manager.vectors.SetBigInt(document.positive, new(big.Int).Set(value), high, low); err != nil {
		return err
	}
	return r.elems.IntersectDoc(document)
}

// FilterByNegation removes target documents that join with a negative row.
// productManager owns intermediate documents for the general join/project path.
func (r *UDocRelation) FilterByNegation(negative *UDocRelation, targetColumns, negativeColumns []uint32, productManager *DocManager) (resultErr error) {
	if err := r.check(); err != nil {
		return err
	}
	if negative == nil || negative.check() != nil || len(targetColumns) != len(negativeColumns) {
		return ErrDOC
	}
	if r.FastEmpty() || negative.FastEmpty() {
		return nil
	}
	if len(negative.widths) == 0 {
		return r.Reset()
	}
	targetBits, err := r.ExpandColumns(targetColumns, nil)
	if err != nil {
		return err
	}
	negativeBits, err := negative.ExpandColumns(negativeColumns, nil)
	if err != nil || len(targetBits) != len(negativeBits) {
		return errors.Join(ErrDOC, err)
	}
	canSubtract := r.manager == negative.manager && len(targetColumns) == len(r.widths) && len(negativeColumns) == len(negative.widths)
	seen := make(map[uint32]bool, len(targetColumns))
	for i := range targetColumns {
		if targetColumns[i] != negativeColumns[i] || seen[targetColumns[i]] {
			canSubtract = false
		}
		seen[targetColumns[i]] = true
	}
	if canSubtract {
		return r.elems.Subtract(negative.elems)
	}
	if productManager == nil || productManager.closed || productManager.NumTBits() != r.NumBits()+negative.NumBits() {
		return ErrDOC
	}
	joined, err := r.Join(negative, targetColumns, negativeColumns, productManager)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, joined.Close()) }()
	removed := make([]uint32, len(negative.widths))
	for i := range removed {
		removed[i] = uint32(len(r.widths) + i)
	}
	projected, err := joined.Project(removed, r.manager)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, projected.Close()) }()
	return r.elems.Subtract(projected.elems)
}

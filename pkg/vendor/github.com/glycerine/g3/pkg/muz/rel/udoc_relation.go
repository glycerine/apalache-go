package rel

// Source: src/muz/rel/udoc_relation.h/.cpp retained concrete relation storage,
// fact conversion, clone/complement, column expansion, join, project, rename,
// and union operations. Plugin factory selection remains in the relation
// manager integration layer.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type UDocRelation struct {
	manager *DocManager // borrowed from the source plugin's width cache
	elems   *DocUnion
	widths  []uint32
	offsets []uint32
	closed  bool
}

func NewUDocRelation(manager *DocManager, widths []uint32) (*UDocRelation, error) {
	if manager == nil || manager.closed {
		return nil, arena.ErrClosed
	}
	offsets := make([]uint32, len(widths)+1)
	var total uint32
	for i, width := range widths {
		if width == 0 || width > ^uint32(0)-total {
			return nil, arena.ErrBounds
		}
		offsets[i] = total
		total += width
	}
	offsets[len(widths)] = total
	if total != manager.NumTBits() {
		return nil, arena.ErrBounds
	}
	elems, err := NewDocUnion(manager)
	if err != nil {
		return nil, err
	}
	return &UDocRelation{manager: manager, elems: elems, widths: append([]uint32(nil), widths...), offsets: offsets}, nil
}

func (r *UDocRelation) check() error {
	if r == nil || r.closed || r.manager == nil || r.manager.closed || r.elems == nil {
		return arena.ErrClosed
	}
	return nil
}

func (r *UDocRelation) Manager() *DocManager {
	if r.check() != nil {
		return nil
	}
	return r.manager
}

func (r *UDocRelation) NumRecords() uint32 {
	if r.check() != nil {
		return 0
	}
	return r.elems.Len()
}

func (r *UDocRelation) NumBits() uint32 {
	if r == nil || len(r.offsets) == 0 {
		return 0
	}
	return r.offsets[len(r.offsets)-1]
}

func (r *UDocRelation) NumColumns() uint32 {
	if r == nil {
		return 0
	}
	return uint32(len(r.widths))
}

func (r *UDocRelation) ColumnIndex(column uint32) (uint32, error) {
	if err := r.check(); err != nil {
		return 0, err
	}
	if column >= uint32(len(r.widths)) {
		return 0, arena.ErrBounds
	}
	return r.offsets[column], nil
}

func (r *UDocRelation) ColumnNumBits(column uint32) (uint32, error) {
	if err := r.check(); err != nil {
		return 0, err
	}
	if column >= uint32(len(r.widths)) {
		return 0, arena.ErrBounds
	}
	return r.widths[column], nil
}

func (r *UDocRelation) Reset() error {
	if err := r.check(); err != nil {
		return err
	}
	return r.elems.Reset()
}

func (r *UDocRelation) Close() error {
	if r == nil || r.closed {
		return nil
	}
	result := r.elems.Reset()
	r.closed = true
	r.elems = nil
	r.manager = nil
	r.widths = nil
	r.offsets = nil
	return result
}

func (r *UDocRelation) factToDoc(values []*big.Int) (*Doc, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if len(values) != len(r.widths) {
		return nil, arena.ErrBounds
	}
	document, err := r.manager.Allocate0()
	if err != nil {
		return nil, err
	}
	for i, value := range values {
		if value == nil || value.Sign() < 0 || value.BitLen() > int(r.widths[i]) {
			_ = r.manager.Deallocate(document)
			return nil, ErrDOC
		}
		low := r.offsets[i]
		high := r.offsets[i+1] - 1
		if err = r.manager.vectors.SetBigInt(document.positive, value, high, low); err != nil {
			_ = r.manager.Deallocate(document)
			return nil, err
		}
	}
	return document, nil
}

func uintFact(values []uint64) []*big.Int {
	result := make([]*big.Int, len(values))
	for i, value := range values {
		result[i] = new(big.Int).SetUint64(value)
	}
	return result
}

func (r *UDocRelation) AddFact(values []uint64) (bool, error) {
	return r.AddBigFact(uintFact(values))
}

func (r *UDocRelation) AddBigFact(values []*big.Int) (bool, error) {
	document, err := r.factToDoc(values)
	if err != nil {
		return false, err
	}
	return r.elems.InsertOwned(document)
}

func (r *UDocRelation) AddNewFact(values []uint64) error {
	document, err := r.factToDoc(uintFact(values))
	if err != nil {
		return err
	}
	return r.elems.PushOwned(document)
}

func (r *UDocRelation) ContainsFact(values []uint64) (bool, error) {
	document, err := r.factToDoc(uintFact(values))
	if err != nil {
		return false, err
	}
	defer r.manager.Deallocate(document)
	return r.elems.Contains(document)
}

func (r *UDocRelation) FastEmpty() bool { return r == nil || r.closed || r.elems.Empty() }

func (r *UDocRelation) Empty(manager *ast.ASTManager) (bool, error) {
	if err := r.check(); err != nil {
		return false, err
	}
	return r.elems.EmptyComplete(manager)
}

func (r *UDocRelation) Clone() (*UDocRelation, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	result, err := NewUDocRelation(r.manager, r.widths)
	if err != nil {
		return nil, err
	}
	if err = result.elems.Copy(r.elems); err != nil {
		_ = result.Close()
		return nil, err
	}
	return result, nil
}

func (r *UDocRelation) Complement() (*UDocRelation, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	result, err := NewUDocRelation(r.manager, r.widths)
	if err != nil {
		return nil, err
	}
	if err = r.elems.Complement(result.elems); err != nil {
		_ = result.Close()
		return nil, err
	}
	return result, nil
}

func (r *UDocRelation) String() (string, error) {
	if err := r.check(); err != nil {
		return "", err
	}
	return r.elems.String()
}

func (r *UDocRelation) SizeEstimateBytes() (uint32, error) {
	if err := r.check(); err != nil {
		return 0, err
	}
	return r.elems.SizeEstimateBytes()
}

func (r *UDocRelation) ExpandColumns(columns []uint32, other *UDocRelation) ([]uint32, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	result := make([]uint32, 0)
	for _, column := range columns {
		var low, limit uint32
		if column < uint32(len(r.widths)) {
			low, limit = r.offsets[column], r.offsets[column+1]
		} else {
			if other == nil || other.check() != nil {
				return nil, arena.ErrBounds
			}
			index := column - uint32(len(r.widths))
			if index >= uint32(len(other.widths)) {
				return nil, arena.ErrBounds
			}
			low = r.NumBits() + other.offsets[index]
			limit = r.NumBits() + other.offsets[index+1]
		}
		for bit := low; bit < limit; bit++ {
			result = append(result, bit)
		}
	}
	return result, nil
}

func (r *UDocRelation) Project(removedColumns []uint32, destination *DocManager) (*UDocRelation, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	removed := make([]bool, len(r.widths))
	deletedBits := make([]bool, r.NumBits())
	previous := ^uint32(0)
	for i, column := range removedColumns {
		if column >= uint32(len(r.widths)) || (i != 0 && column <= previous) {
			return nil, arena.ErrBounds
		}
		previous, removed[column] = column, true
		for bit := r.offsets[column]; bit < r.offsets[column+1]; bit++ {
			deletedBits[bit] = true
		}
	}
	widths := make([]uint32, 0, len(r.widths)-len(removedColumns))
	for i, width := range r.widths {
		if !removed[i] {
			widths = append(widths, width)
		}
	}
	result, err := NewUDocRelation(destination, widths)
	if err != nil {
		return nil, err
	}
	for _, document := range r.elems.values {
		projected, projectErr := r.manager.Project(destination, deletedBits, document)
		if projectErr != nil {
			_ = result.Close()
			return nil, projectErr
		}
		result.elems.values = append(result.elems.values, projected)
	}
	return result, nil
}

func (r *UDocRelation) Rename(cycle []uint32) (*UDocRelation, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	columnPermutation := make([]uint32, len(r.widths))
	for i := range columnPermutation {
		columnPermutation[i] = uint32(i)
	}
	seen := make(map[uint32]bool, len(cycle))
	for i, destination := range cycle {
		if destination >= uint32(len(r.widths)) || seen[destination] {
			return nil, arena.ErrBounds
		}
		seen[destination] = true
		// Source permutate_by_cycle moves the next source column into the
		// current destination, independently of the columns' bit widths.
		source := cycle[(i+1)%len(cycle)]
		columnPermutation[destination] = source
	}
	newWidths := make([]uint32, len(r.widths))
	for destination, source := range columnPermutation {
		newWidths[destination] = r.widths[source]
	}
	newOffsets := make([]uint32, len(newWidths)+1)
	for i, width := range newWidths {
		newOffsets[i+1] = newOffsets[i] + width
	}
	permutation := make([]uint32, r.NumBits())
	for oldColumn := range r.widths {
		var destination int
		for candidate, source := range columnPermutation {
			if source == uint32(oldColumn) {
				destination = candidate
				break
			}
		}
		if r.widths[oldColumn] != newWidths[destination] {
			return nil, ErrDOC
		}
		for bit := uint32(0); bit < r.widths[oldColumn]; bit++ {
			permutation[r.offsets[oldColumn]+bit] = newOffsets[destination] + bit
		}
	}
	result, err := NewUDocRelation(r.manager, newWidths)
	if err != nil {
		return nil, err
	}
	for _, document := range r.elems.values {
		copy, copyErr := r.manager.AllocatePermutation(document, permutation)
		if copyErr != nil {
			_ = result.Close()
			return nil, copyErr
		}
		result.elems.values = append(result.elems.values, copy)
	}
	return result, nil
}

func (r *UDocRelation) UnionFrom(source *UDocRelation, delta *UDocRelation) error {
	if err := r.check(); err != nil {
		return err
	}
	if source == nil || source.check() != nil || source.manager != r.manager || r.ValidateSameShape(source) != nil {
		return ErrDOC
	}
	if delta != nil && (delta.check() != nil || delta.manager != r.manager || r.ValidateSameShape(delta) != nil) {
		return ErrDOC
	}
	deltaEmpty := delta != nil && delta.elems.Empty()
	if r.elems.Empty() {
		for _, document := range source.elems.values {
			copy, err := r.manager.AllocateCopy(document)
			if err != nil {
				return err
			}
			r.elems.values = append(r.elems.values, copy)
			if delta != nil {
				deltaCopy, copyErr := r.manager.AllocateCopy(document)
				if copyErr != nil {
					return copyErr
				}
				if deltaEmpty {
					delta.elems.values = append(delta.elems.values, deltaCopy)
				} else if _, copyErr = delta.elems.InsertOwned(deltaCopy); copyErr != nil {
					return copyErr
				}
			}
		}
		return nil
	}
	for _, document := range source.elems.values {
		copy, err := r.manager.AllocateCopy(document)
		if err != nil {
			return err
		}
		inserted, err := r.elems.InsertOwned(copy)
		if err != nil {
			return err
		}
		if inserted && delta != nil {
			deltaCopy, copyErr := r.manager.AllocateCopy(document)
			if copyErr != nil {
				return copyErr
			}
			if deltaEmpty {
				delta.elems.values = append(delta.elems.values, deltaCopy)
			} else if _, copyErr = delta.elems.InsertOwned(deltaCopy); copyErr != nil {
				return copyErr
			}
		}
	}
	return nil
}

func (r *UDocRelation) Join(other *UDocRelation, leftColumns, rightColumns []uint32, destination *DocManager) (*UDocRelation, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if other == nil || other.check() != nil || len(leftColumns) != len(rightColumns) {
		return nil, ErrDOC
	}
	leftBits, err := r.ExpandColumns(leftColumns, nil)
	if err != nil {
		return nil, err
	}
	rightBits, err := other.ExpandColumns(rightColumns, nil)
	if err != nil {
		return nil, err
	}
	if len(leftBits) != len(rightBits) {
		return nil, ErrDOC
	}
	widths := append(append([]uint32(nil), r.widths...), other.widths...)
	result, err := NewUDocRelation(destination, widths)
	if err != nil {
		return nil, err
	}
	for _, left := range r.elems.values {
		for _, right := range other.elems.values {
			joined, joinErr := destination.Join(left, right, r.manager, leftBits, rightBits)
			if joinErr != nil {
				_ = result.Close()
				return nil, joinErr
			}
			if joined != nil {
				if _, joinErr = result.elems.InsertOwned(joined); joinErr != nil {
					_ = result.Close()
					return nil, joinErr
				}
			}
		}
	}
	return result, nil
}

func (r *UDocRelation) ValidateSameShape(other *UDocRelation) error {
	if r == nil || other == nil || len(r.widths) != len(other.widths) {
		return ErrDOC
	}
	for i := range r.widths {
		if r.widths[i] != other.widths[i] {
			return ErrDOC
		}
	}
	return nil
}

func closeRelations(relations ...*UDocRelation) (result error) {
	for _, relation := range relations {
		if relation != nil {
			result = errors.Join(result, relation.Close())
		}
	}
	return result
}

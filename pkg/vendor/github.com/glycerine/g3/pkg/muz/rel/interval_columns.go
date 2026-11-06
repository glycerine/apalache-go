package rel

// Source: src/muz/rel/dl_interval_relation.cpp:417-502 and the concrete
// interval instantiation of src/muz/rel/dl_vector_relation.h.

import (
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// intervalColumns is the column and equality portion of the interval relation.
type intervalColumns struct {
	columns []smtcore.RelationInterval
	parent  []uint32
	size    []uint32
	empty   bool
}

func newIntervalColumns(arity int, empty bool) *intervalColumns {
	r := &intervalColumns{columns: make([]smtcore.RelationInterval, arity), parent: make([]uint32, arity), size: make([]uint32, arity), empty: empty}
	for i := range r.columns {
		r.columns[i] = smtcore.NewRelationInterval(nil, true, nil, true)
		r.parent[i] = uint32(i)
		r.size[i] = 1
	}
	return r
}

func (r *intervalColumns) find(index uint32) uint32 {
	for r.parent[index] != index {
		index = r.parent[index]
	}
	return index
}

func (r *intervalColumns) merge(i, j uint32) uint32 {
	i, j = r.find(i), r.find(j)
	if i == j {
		return i
	}
	if r.size[i] > r.size[j] {
		i, j = j, i
	}
	r.parent[i] = j
	r.size[j] += r.size[i]
	return j
}

func (r *intervalColumns) setEmpty() {
	r.empty = true
	for i := range r.columns {
		r.parent[i] = uint32(i)
		r.size[i] = 1
		r.columns[i] = smtcore.NewRelationInterval(nil, true, nil, true)
	}
}

func (r *intervalColumns) equate(i, j uint32) error {
	if uint64(i) >= uint64(len(r.columns)) || uint64(j) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	if r.empty {
		return nil
	}
	i, j = r.find(i), r.find(j)
	if i == j {
		return nil
	}
	value, empty := r.columns[i].Meet(r.columns[j])
	if empty || value.EmptyBounds() {
		r.empty = true
		return nil
	}
	// union_find attaches the smaller root to the larger root, choosing the
	// second root on equal sizes.
	r.columns[r.merge(i, j)] = value
	return nil
}

func (r *intervalColumns) intersect(index uint32, bound smtcore.RelationInterval) error {
	if uint64(index) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	index = r.find(index)
	var empty bool
	r.columns[index], empty = r.columns[index].Meet(bound)
	if empty || r.columns[index].EmptyBounds() {
		r.setEmpty()
	}
	return nil
}

func shiftedInterval(i smtcore.RelationInterval, shift *big.Rat) smtcore.RelationInterval {
	lo, loOpen, hi, hiOpen := i.Bounds()
	if lo != nil {
		lo.Add(lo, shift)
	}
	if hi != nil {
		hi.Add(hi, shift)
	}
	return smtcore.NewRelationInterval(lo, loOpen, hi, hiOpen)
}

// filterInterpreted applies recognized linear comparisons; unknown predicates
// leave the relation unchanged, as in the source.
func (r *intervalColumns) filterInterpreted(m *ast.ASTManager, condition ast.ASTHandle) error {
	kind, c, ok, err := intervalComparison(m, condition)
	if err != nil {
		return err
	}
	if !ok {
		if m.IsFalse(condition) {
			r.setEmpty()
		}
		return nil
	}
	x, y, k := c.positive, c.negative, &c.constant
	if x != math.MaxUint32 && uint64(x) >= uint64(len(r.columns)) || y != math.MaxUint32 && uint64(y) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	if kind == 2 {
		if x == math.MaxUint32 {
			return r.intersect(y, smtcore.NewRelationInterval(k, false, k, false))
		}
		minusK := new(big.Rat).Neg(k)
		if y == math.MaxUint32 {
			return r.intersect(x, smtcore.NewRelationInterval(minusK, false, minusK, false))
		}
		xBound, yBound := shiftedInterval(r.columns[r.find(x)], k), shiftedInterval(r.columns[r.find(y)], minusK)
		if err := r.intersect(x, yBound); err != nil {
			return err
		}
		return r.intersect(y, xBound)
	}
	open := kind == 1
	if x == math.MaxUint32 {
		return r.intersect(y, smtcore.NewRelationInterval(nil, true, k, open))
	}
	if y == math.MaxUint32 {
		return r.intersect(x, smtcore.NewRelationInterval(new(big.Rat).Neg(k), open, nil, true))
	}
	_, _, xHigh, _ := r.columns[r.find(x)].Bounds()
	yLow, _, _, _ := r.columns[r.find(y)].Bounds()
	if xHigh != nil {
		if err := r.intersect(y, smtcore.NewRelationInterval(nil, true, new(big.Rat).Add(k, xHigh), open)); err != nil {
			return err
		}
	}
	if yLow != nil {
		return r.intersect(x, smtcore.NewRelationInterval(new(big.Rat).Sub(yLow, k), open, nil, true))
	}
	return nil
}

package rel

// Source: src/muz/rel/dl_bound_relation.cpp/.h strict and weak column bounds.

import (
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type boundColumn struct{ lt, le map[uint32]bool }

// BoundRelation tracks strict and non-strict order constraints between
// columns and their equality classes.
type BoundRelation struct {
	columns []boundColumn
	parent  []uint32
	size    []uint32
	empty   bool
}

func NewBoundRelation(arity uint32, empty bool) *BoundRelation {
	r := &BoundRelation{columns: make([]boundColumn, int(arity)), parent: make([]uint32, int(arity)), size: make([]uint32, int(arity)), empty: empty}
	for i := range r.columns {
		r.columns[i] = boundColumn{lt: make(map[uint32]bool), le: make(map[uint32]bool)}
		r.parent[i], r.size[i] = uint32(i), 1
	}
	return r
}

// Clone follows vector_relation::copy, which copies each representative's
// bound payload into independent columns instead of preserving equivalence
// classes. The source empty state is copied as an empty relation.
func (r *BoundRelation) Clone() *BoundRelation {
	if r == nil {
		return nil
	}
	result := NewBoundRelation(uint32(len(r.columns)), r.empty)
	if r.empty {
		return result
	}
	for i := range r.columns {
		result.columns[i] = cloneBoundColumn(r.columns[r.find(uint32(i))])
	}
	return result
}

func (r *BoundRelation) Empty() bool { return r == nil || r.empty }

// IsPrecise mirrors bound_relation::is_precise; bound relations are always
// abstractions, even when their current formula happens to be exact.
func (r *BoundRelation) IsPrecise() bool { return false }

// AddFact joins the source's unconstrained abstraction of a concrete fact.
// Its bound-relation equal-value filter does not add constraints.
func (r *BoundRelation) AddFact(fact []ast.ASTHandle) error {
	if r == nil || len(fact) != len(r.columns) {
		return ast.ErrASTDomain
	}
	_, err := r.Union(NewBoundRelation(uint32(len(fact)), false))
	return err
}

// ContainsFact follows the source's deliberately coarse membership check.
func (r *BoundRelation) ContainsFact(fact []ast.ASTHandle) bool {
	return r != nil && !r.empty && len(fact) == len(r.columns)
}

func (r *BoundRelation) find(index uint32) uint32 {
	for r.parent[index] != index {
		index = r.parent[index]
	}
	return index
}

func (r *BoundRelation) reset() {
	r.empty = true
	for i := range r.columns {
		r.parent[i], r.size[i] = uint32(i), 1
		clear(r.columns[i].lt)
		clear(r.columns[i].le)
	}
}

// Equate merges columns using the concrete vector relation's intersection
// and union-by-size rules.
func (r *BoundRelation) Equate(left, right uint32) error {
	if r == nil || uint64(left) >= uint64(len(r.columns)) || uint64(right) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	if r.empty {
		return nil
	}
	left, right = r.find(left), r.find(right)
	if left == right {
		return nil
	}
	combined := boundColumn{lt: make(map[uint32]bool), le: make(map[uint32]bool)}
	for i := range r.columns[left].lt {
		combined.lt[i] = true
	}
	for i := range r.columns[right].lt {
		combined.lt[i] = true
	}
	for i := range r.columns[left].le {
		combined.le[i] = true
	}
	for i := range r.columns[right].le {
		combined.le[i] = true
	}
	// vector_relation::equate checks only the pre-merge representative of its
	// first argument, and is_empty treats both strict and weak self-bounds as
	// empty. Keep the source's argument-order-sensitive behavior.
	if combined.lt[left] || combined.le[left] {
		r.empty = true
		return nil
	}
	if r.size[left] > r.size[right] {
		left, right = right, left
	}
	r.parent[left] = right
	r.size[right] += r.size[left]
	r.columns[right] = combined
	return nil
}

// AddLess and AddLessEqual propagate the upper column's known bounds.
func (r *BoundRelation) AddLess(left, right uint32) error      { return r.addBound(left, right, true) }
func (r *BoundRelation) AddLessEqual(left, right uint32) error { return r.addBound(left, right, false) }

func (r *BoundRelation) addBound(left, right uint32, strict bool) error {
	if r == nil || uint64(left) >= uint64(len(r.columns)) || uint64(right) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	left, right = r.find(left), r.find(right)
	type pending struct {
		column uint32
		strict bool
	}
	todo := []pending{{right, strict}}
	seen := make(map[pending]bool)
	for len(todo) != 0 {
		i := len(todo) - 1
		item := todo[i]
		todo = todo[:i]
		// dl_bound_relation.cpp reads (*m_elems)[j] directly while walking
		// stored edges. Edges created before an equality merge can therefore
		// inspect the stale payload of a nonrepresentative column; only the
		// initial left and right arguments are normalized with find().
		if seen[item] {
			continue
		}
		seen[item] = true
		if item.column == left {
			if item.strict {
				r.empty = true
				return nil
			}
			continue
		}
		source := &r.columns[item.column]
		for next := range source.lt {
			todo = append(todo, pending{next, true})
		}
		for next := range source.le {
			todo = append(todo, pending{next, item.strict})
		}
		if item.strict {
			r.columns[left].lt[item.column] = true
		} else {
			r.columns[left].le[item.column] = true
		}
	}
	return nil
}

// Bounds returns detached, sorted column indices for the source lt/le sets.
func (r *BoundRelation) Bounds(column uint32) (strict, weak []uint32, err error) {
	if r == nil || uint64(column) >= uint64(len(r.columns)) {
		return nil, nil, ast.ErrASTDomain
	}
	column = r.find(column)
	for index := range r.columns[column].lt {
		strict = append(strict, index)
	}
	for index := range r.columns[column].le {
		weak = append(weak, index)
	}
	sort.Slice(strict, func(i, j int) bool { return strict[i] < strict[j] })
	sort.Slice(weak, func(i, j int) bool { return weak[i] < weak[j] })
	return strict, weak, nil
}

// UnionInterval drops bounds not established by an interval relation, as in
// bound_relation::mk_union_i. Endpoint openness is ignored by the source.
func (r *BoundRelation) UnionInterval(source *IntervalRelation) error {
	if r == nil || source == nil || source.values == nil || len(r.columns) != len(source.values.columns) {
		return ast.ErrASTDomain
	}
	for i := range r.columns {
		if r.find(uint32(i)) != uint32(i) {
			continue
		}
		_, _, upper, _ := source.values.columns[source.values.find(uint32(i))].Bounds()
		if upper == nil {
			clear(r.columns[i].lt)
			clear(r.columns[i].le)
			continue
		}
		for j := range r.columns[i].lt {
			lower, _, _, _ := source.values.columns[source.values.find(j)].Bounds()
			if lower == nil || upper.Cmp(lower) >= 0 {
				delete(r.columns[i].lt, j)
			}
		}
		for j := range r.columns[i].le {
			lower, _, _, _ := source.values.columns[source.values.find(j)].Bounds()
			if lower == nil || upper.Cmp(lower) > 0 {
				delete(r.columns[i].le, j)
			}
		}
	}
	return nil
}

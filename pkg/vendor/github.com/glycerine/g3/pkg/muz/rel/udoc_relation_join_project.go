package rel

// Source: src/muz/rel/udoc_relation.cpp udoc_plugin::join_project_fn.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// JoinProject joins documents and projects each successful pair directly into
// the result. productManager owns the temporary concatenated bit space.
func (r *UDocRelation) JoinProject(other *UDocRelation, leftColumns, rightColumns, removedColumns []uint32, productManager, resultManager *DocManager) (result *UDocRelation, resultErr error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if other == nil || other.check() != nil || len(leftColumns) != len(rightColumns) {
		return nil, ErrDOC
	}
	for i := range leftColumns {
		if leftColumns[i] >= uint32(len(r.widths)) || rightColumns[i] >= uint32(len(other.widths)) || r.widths[leftColumns[i]] != other.widths[rightColumns[i]] {
			return nil, ErrDOC
		}
	}
	if productManager == nil || productManager.closed || productManager.NumTBits() != r.NumBits()+other.NumBits() {
		return nil, ErrDOC
	}
	leftBits, err := r.ExpandColumns(leftColumns, nil)
	if err != nil {
		return nil, err
	}
	rightBits, err := other.ExpandColumns(rightColumns, nil)
	if err != nil || len(leftBits) != len(rightBits) {
		return nil, errors.Join(ErrDOC, err)
	}

	allWidths := append(append([]uint32(nil), r.widths...), other.widths...)
	removed := make([]bool, len(allWidths))
	deletedBits := make([]bool, productManager.NumTBits())
	previous := ^uint32(0)
	for i, column := range removedColumns {
		if column >= uint32(len(allWidths)) || (i != 0 && column <= previous) {
			return nil, arena.ErrBounds
		}
		previous = column
		removed[column] = true
		var low uint32
		for j := uint32(0); j < column; j++ {
			low += allWidths[j]
		}
		for bit := low; bit < low+allWidths[column]; bit++ {
			deletedBits[bit] = true
		}
	}
	resultWidths := make([]uint32, 0, len(allWidths)-len(removedColumns))
	for i, width := range allWidths {
		if !removed[i] {
			resultWidths = append(resultWidths, width)
		}
	}

	// The source specializes h(X) :- f(X), g(X) to DOC intersection. Require
	// complete distinct coverage as well: its syntactic check accepts repeated
	// join columns even though the intersection shortcut is then unsound.
	intersection := len(leftColumns) == len(removedColumns) && len(leftColumns) == len(r.widths) && len(leftColumns) == len(other.widths)
	covered := make([]bool, len(r.widths))
	for i := range leftColumns {
		intersection = intersection && removedColumns[i] == uint32(i) && leftColumns[i] == rightColumns[i]
		if leftColumns[i] >= uint32(len(covered)) || covered[leftColumns[i]] {
			intersection = false
		} else {
			covered[leftColumns[i]] = true
		}
	}
	if intersection && r.manager == other.manager && r.manager == resultManager && r.ValidateSameShape(other) == nil {
		result, err = r.Clone()
		if err != nil {
			return nil, err
		}
		if err = result.elems.Intersect(other.elems); err != nil {
			return nil, errors.Join(err, result.Close())
		}
		return result, nil
	}

	result, err = NewUDocRelation(resultManager, resultWidths)
	if err != nil {
		return nil, err
	}
	owned := result
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, owned.Close())
			result = nil
		}
	}()
	for _, left := range r.elems.values {
		for _, right := range other.elems.values {
			joined, joinErr := productManager.Join(left, right, r.manager, leftBits, rightBits)
			if joinErr != nil {
				return nil, joinErr
			}
			if joined == nil {
				continue
			}
			projected, projectErr := productManager.Project(resultManager, deletedBits, joined)
			joinedErr := productManager.Deallocate(joined)
			if projectErr != nil || joinedErr != nil {
				if projected != nil {
					projectErr = errors.Join(projectErr, resultManager.Deallocate(projected))
				}
				return nil, errors.Join(projectErr, joinedErr)
			}
			if _, projectErr = result.elems.InsertOwned(projected); projectErr != nil {
				return nil, errors.Join(projectErr, resultManager.Deallocate(projected))
			}
		}
	}
	keep = true
	return result, nil
}

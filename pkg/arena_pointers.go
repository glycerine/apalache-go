package apalache

import (
	"fmt"
	"sort"
)

// SetElementPointers associates a set cell with its outgoing membership edges.
type SetElementPointers struct {
	Set      ArenaCell
	Pointers []ElementPointer
}

// GroupElementPointers groups membership edges by destination cell ID.
func GroupElementPointers(pointers []ElementPointer) map[int][]ElementPointer {
	grouped := make(map[int][]ElementPointer)
	for _, pointer := range pointers {
		id := pointer.Element().ID
		grouped[id] = append(grouped[id], pointer)
	}
	return grouped
}

// GroupPointersAndSets groups membership edges by destination and records all
// source sets that point at each destination.
func GroupPointersAndSets(entries []SetElementPointers) (map[int][]ElementPointer, map[int][]ArenaCell) {
	pointers := make(map[int][]ElementPointer)
	sets := make(map[int]map[int]ArenaCell)
	for _, entry := range entries {
		for _, pointer := range entry.Pointers {
			id := pointer.Element().ID
			pointers[id] = append(pointers[id], pointer)
			if sets[id] == nil {
				sets[id] = make(map[int]ArenaCell)
			}
			sets[id][entry.Set.ID] = entry.Set
		}
	}
	resultSets := make(map[int][]ArenaCell, len(sets))
	for id, byID := range sets {
		resultSets[id] = make([]ArenaCell, 0, len(byID))
		for _, cell := range byID {
			resultSets[id] = append(resultSets[id], cell)
		}
		sort.Slice(resultSets[id], func(left, right int) bool {
			return resultSets[id][left].ID < resultSets[id][right].ID
		})
	}
	return pointers, resultSets
}

// MergeElementPointers collapses multiple pointers to the same cell. A fixed
// edge dominates; otherwise the SMT guards are disjoined.
func MergeElementPointers(cell ArenaCell, pointers []ElementPointer) (ElementPointer, error) {
	if len(pointers) == 0 {
		return nil, fmt.Errorf("cannot merge an empty pointer collection")
	}
	for _, pointer := range pointers {
		if !pointer.Element().Equal(cell) {
			return nil, fmt.Errorf("pointer to %s cannot be merged as a pointer to %s", pointer.Element(), cell)
		}
	}
	if len(pointers) == 1 {
		return pointers[0], nil
	}
	for _, pointer := range pointers {
		if _, fixed := pointer.(FixedElementPointer); fixed {
			return FixedElementPointer{Cell: cell}, nil
		}
	}
	guards := make([]Expr, len(pointers))
	for index, pointer := range pointers {
		guards[index] = pointer.ToSMT()
	}
	return SMTElementPointer{Cell: cell, Expression: MustOperExpr(Op("OR"), Typed(BoolT1), guards...)}, nil
}

// MergeElementPointersByCell groups and merges pointers in ascending cell-ID
// order, making the result deterministic despite Go map iteration order.
func MergeElementPointersByCell(pointers []ElementPointer) ([]ElementPointer, error) {
	grouped := GroupElementPointers(pointers)
	ids := make([]int, 0, len(grouped))
	for id := range grouped {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	result := make([]ElementPointer, len(ids))
	for index, id := range ids {
		merged, err := MergeElementPointers(grouped[id][0].Element(), grouped[id])
		if err != nil {
			return nil, err
		}
		result[index] = merged
	}
	return result, nil
}

// PointerWithSameCondition retargets a pointer while preserving whether it is
// fixed or SMT-guarded and, for the latter, preserving its guard expression.
func PointerWithSameCondition(original ElementPointer, cell ArenaCell) ElementPointer {
	switch pointer := original.(type) {
	case FixedElementPointer:
		return FixedElementPointer{Cell: cell}
	case SMTElementPointer:
		return SMTElementPointer{Cell: cell, Expression: pointer.Expression}
	default:
		panic(fmt.Sprintf("unexpected element pointer %T", original))
	}
}

// TupleElementPointer combines the guards of pointers selected from Cartesian
// product operands. It is fixed exactly when every component pointer is fixed.
func TupleElementPointer(components []ElementPointer, tuple ArenaCell) (ElementPointer, error) {
	if len(components) == 0 {
		return nil, fmt.Errorf("tuple pointer requires at least one component")
	}
	allFixed := true
	guards := make([]Expr, len(components))
	for index, pointer := range components {
		_, fixed := pointer.(FixedElementPointer)
		allFixed = allFixed && fixed
		guards[index] = pointer.ToSMT()
	}
	if allFixed {
		return FixedElementPointer{Cell: tuple}, nil
	}
	return SMTElementPointer{Cell: tuple, Expression: MustOperExpr(Op("AND"), Typed(BoolT1), guards...)}, nil
}

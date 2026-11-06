package rel

// Source: src/muz/rel/doc.cpp merge, project, pick_resolvent, and join.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/backtrack"
)

// MergeRange applies source equality classes to a contiguous bit range.
func (m *DocManager) MergeRange(document *Doc, low, length uint32, equalities *backtrack.UnionFind, discarded []bool) (bool, error) {
	if err := m.check(document); err != nil {
		return false, err
	}
	if equalities == nil || low > m.NumTBits() || length > m.NumTBits()-low || uint32(len(discarded)) < m.NumTBits() || equalities.GetNumVars() < m.NumTBits() {
		return false, arena.ErrBounds
	}
	for offset := uint32(0); offset < length; offset++ {
		valid, err := m.mergeAt(document, low+offset, equalities, discarded)
		if err != nil || !valid {
			return valid, err
		}
	}
	return true, nil
}

func (m *DocManager) mergeAt(document *Doc, index uint32, equalities *backtrack.UnionFind, discarded []bool) (bool, error) {
	root := equalities.Find(index)
	index = root
	var numX uint32
	root1 := root
	value := TBitX
	for {
		bit, err := m.vectors.Get(document.positive, index)
		if err != nil {
			return false, err
		}
		switch bit {
		case TBit0:
			if value == TBit1 {
				return false, nil
			}
			value = TBit0
		case TBit1:
			if value == TBit0 {
				return false, nil
			}
			value = TBit1
		case TBitX:
			numX++
			if !discarded[index] {
				root1 = index
			}
		default:
			return false, ErrDOC
		}
		index = equalities.Next(index)
		if index == root {
			break
		}
	}
	if numX == 0 {
		return true, nil
	}
	if value != TBitX {
		index = root
		for {
			bit, err := m.vectors.Get(document.positive, index)
			if err != nil {
				return false, err
			}
			if bit == TBitX {
				if err = m.Set(document, index, value); err != nil {
					return false, err
				}
			}
			index = equalities.Next(index)
			if index == root {
				break
			}
		}
		return true, nil
	}

	allX := true
	if len(document.negative.values) != 0 {
		index = root
		for {
			for _, negative := range document.negative.values {
				bit, err := m.vectors.Get(negative, index)
				if err != nil {
					return false, err
				}
				if bit != TBitX {
					allX = false
					break
				}
			}
			index = equalities.Next(index)
			if index == root || !allX {
				break
			}
		}
	}
	index = root
	for {
		if (!discarded[index] || !allX) && index != root1 {
			first, err := m.vectors.AllocateCopy(document.positive)
			if err != nil {
				return false, err
			}
			if err = m.vectors.Set(first, index, TBit0); err != nil {
				_ = m.vectors.Deallocate(first)
				return false, err
			}
			if err = m.vectors.Set(first, root1, TBit1); err != nil {
				_ = m.vectors.Deallocate(first)
				return false, err
			}
			if _, err = document.negative.insert(m.vectors, first); err != nil {
				return false, err
			}
			second, err := m.vectors.AllocateCopy(document.positive)
			if err != nil {
				return false, err
			}
			if err = m.vectors.Set(second, index, TBit1); err != nil {
				_ = m.vectors.Deallocate(second)
				return false, err
			}
			if err = m.vectors.Set(second, root1, TBit0); err != nil {
				_ = m.vectors.Deallocate(second)
				return false, err
			}
			if _, err = document.negative.insert(m.vectors, second); err != nil {
				return false, err
			}
		}
		index = equalities.Next(index)
		if index == root {
			break
		}
	}
	return true, nil
}

type projectAction uint8

const (
	projectIsEmpty projectAction = iota
	projectDone
	projectMonolithic
	projectNeg
	projectPos
	projectResolve
)

func (m *DocManager) pickResolvent(positive *TBV, negatives []*TBV, deleted []bool) (projectAction, uint32, error) {
	if len(negatives) == 0 {
		return projectDone, 0, nil
	}
	for _, negative := range negatives {
		equal, err := m.vectors.Equals(positive, negative)
		if err != nil {
			return 0, 0, err
		}
		if equal {
			return projectIsEmpty, 0, nil
		}
	}
	bestPositive, bestNegative, bestIndex := ^uint32(0), ^uint32(0), ^uint32(0)
	for index := uint32(0); index < m.NumTBits(); index++ {
		if !deleted[index] {
			continue
		}
		positiveBit, err := m.vectors.Get(positive, index)
		if err != nil {
			return 0, 0, err
		}
		if positiveBit != TBitX {
			continue
		}
		first, err := m.vectors.Get(negatives[0], index)
		if err != nil {
			return 0, 0, err
		}
		var numPositive, numNegative uint32
		if first == TBit0 {
			numNegative++
		} else if first == TBit1 {
			numPositive++
		}
		monolithic := true
		for _, negative := range negatives[1:] {
			bit, readErr := m.vectors.Get(negative, index)
			if readErr != nil {
				return 0, 0, readErr
			}
			if bit != first {
				monolithic = false
			}
			if bit == TBit0 {
				numNegative++
			} else if bit == TBit1 {
				numPositive++
			}
		}
		if monolithic && first != TBitX {
			return projectMonolithic, index, nil
		}
		if monolithic {
			continue
		}
		if numPositive == 0 {
			return projectNeg, index, nil
		}
		if numNegative == 0 {
			return projectPos, index, nil
		}
		if (bestPositive >= numPositive && bestNegative >= numNegative) || numNegative == 1 || numPositive == 1 {
			bestPositive, bestNegative, bestIndex = numPositive, numNegative, index
		}
	}
	if bestIndex != ^uint32(0) {
		return projectResolve, bestIndex, nil
	}
	return projectDone, 0, nil
}

func countRetained(deleted []bool) uint32 {
	var count uint32
	for _, remove := range deleted {
		if !remove {
			count++
		}
	}
	return count
}

// Project existentially eliminates deleted source bits into destination.
func (m *DocManager) Project(destination *DocManager, deleted []bool, source *Doc) (result *Doc, resultErr error) {
	if err := m.check(source); err != nil {
		return nil, err
	}
	if destination == nil || destination.closed || uint32(len(deleted)) != m.NumTBits() || destination.NumTBits() != countRetained(deleted) {
		return nil, arena.ErrBounds
	}
	positive, err := destination.vectors.Project(deleted, source.positive)
	if err != nil {
		return nil, err
	}
	result, err = destination.AdoptTBV(positive)
	if err != nil {
		return nil, err
	}
	keepResult := false
	defer func() {
		if !keepResult {
			resultErr = errors.Join(resultErr, destination.Deallocate(result))
			result = nil
		}
	}()
	if len(source.negative.values) == 0 {
		keepResult = true
		return result, nil
	}
	todo := make([]*TBV, 0, len(source.negative.values))
	owned := make(map[*TBV]bool, len(source.negative.values))
	for _, negative := range source.negative.values {
		copy, copyErr := m.vectors.AllocateCopy(negative)
		if copyErr != nil {
			for value := range owned {
				_ = m.vectors.Deallocate(value)
			}
			return nil, copyErr
		}
		todo = append(todo, copy)
		owned[copy] = true
	}
	defer func() {
		for value := range owned {
			resultErr = errors.Join(resultErr, m.vectors.Deallocate(value))
		}
	}()
	done := false
	for len(todo) != 0 && !done {
		action, index, actionErr := m.pickResolvent(source.positive, todo, deleted)
		if actionErr != nil {
			return nil, actionErr
		}
		switch action {
		case projectIsEmpty:
			copy, copyErr := destination.vectors.AllocateCopy(result.positive)
			if copyErr != nil {
				return nil, copyErr
			}
			result.negative.values = append(result.negative.values, copy)
			done = true
		case projectMonolithic:
			done = true
		case projectNeg, projectPos:
			kept := todo[:0]
			for _, value := range todo {
				bit, readErr := m.vectors.Get(value, index)
				if readErr != nil {
					return nil, readErr
				}
				if bit == TBitX {
					kept = append(kept, value)
				} else if readErr = m.vectors.Deallocate(value); readErr != nil {
					return nil, readErr
				} else {
					delete(owned, value)
				}
			}
			todo = kept
		case projectResolve:
			unconstrained := make([]*TBV, 0, len(todo))
			zeros := make([]*TBV, 0, len(todo))
			ones := make([]*TBV, 0, len(todo))
			for _, value := range todo {
				bit, readErr := m.vectors.Get(value, index)
				if readErr != nil {
					return nil, readErr
				}
				switch bit {
				case TBitX:
					unconstrained = append(unconstrained, value)
				case TBit0:
					zeros = append(zeros, value)
				case TBit1:
					ones = append(ones, value)
				default:
					return nil, ErrDOC
				}
			}
			newTodo := unconstrained
			for _, one := range ones {
				for _, zero := range zeros {
					resolvent, allocErr := m.vectors.AllocateCopy(one)
					if allocErr != nil {
						return nil, allocErr
					}
					if allocErr = m.vectors.Set(resolvent, index, TBitX); allocErr != nil {
						_ = m.vectors.Deallocate(resolvent)
						return nil, allocErr
					}
					valid, intersectErr := m.vectors.SetAnd(resolvent, zero)
					if intersectErr != nil {
						_ = m.vectors.Deallocate(resolvent)
						return nil, intersectErr
					}
					if valid {
						if intersectErr = m.vectors.Set(resolvent, index, TBitX); intersectErr != nil {
							_ = m.vectors.Deallocate(resolvent)
							return nil, intersectErr
						}
						newTodo = append(newTodo, resolvent)
						owned[resolvent] = true
					} else if intersectErr = m.vectors.Deallocate(resolvent); intersectErr != nil {
						return nil, intersectErr
					}
				}
			}
			for _, value := range ones {
				if err = m.vectors.Deallocate(value); err != nil {
					return nil, err
				}
				delete(owned, value)
			}
			for _, value := range zeros {
				if err = m.vectors.Deallocate(value); err != nil {
					return nil, err
				}
				delete(owned, value)
			}
			todo = newTodo
		case projectDone:
			for _, value := range todo {
				projected, projectErr := destination.vectors.Project(deleted, value)
				if projectErr != nil {
					return nil, projectErr
				}
				equalPositive, equalErr := destination.vectors.Equals(result.positive, projected)
				if equalErr != nil {
					_ = destination.vectors.Deallocate(projected)
					return nil, equalErr
				}
				if equalPositive {
					if equalErr = result.negative.reset(destination.vectors); equalErr != nil {
						_ = destination.vectors.Deallocate(projected)
						return nil, equalErr
					}
					result.negative.values = append(result.negative.values, projected)
					break
				}
				duplicate := false
				if len(result.negative.values) != 0 {
					duplicate, equalErr = destination.vectors.Equals(result.negative.values[0], projected)
					if equalErr != nil {
						_ = destination.vectors.Deallocate(projected)
						return nil, equalErr
					}
				}
				if duplicate {
					if equalErr = destination.vectors.Deallocate(projected); equalErr != nil {
						return nil, equalErr
					}
				} else {
					result.negative.values = append(result.negative.values, projected)
				}
			}
			done = true
		}
	}
	for _, value := range todo {
		if err = m.vectors.Deallocate(value); err != nil {
			return nil, err
		}
		delete(owned, value)
	}
	todo = nil
	keepResult = true
	return result, nil
}

// Join concatenates two DOC bit spaces and enforces equality at paired bits.
func (m *DocManager) Join(left *Doc, right *Doc, leftManager *DocManager, leftColumns, rightColumns []uint32) (result *Doc, resultErr error) {
	if leftManager == nil || leftManager.closed || right == nil || right.owner == nil || right.owner.closed || len(leftColumns) != len(rightColumns) {
		return nil, ErrDOC
	}
	if err := leftManager.check(left); err != nil {
		return nil, err
	}
	rightManager := right.owner
	if m.NumTBits() != leftManager.NumTBits()+rightManager.NumTBits() {
		return nil, arena.ErrBounds
	}
	result, err := m.AllocateX()
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep && result != nil {
			resultErr = errors.Join(resultErr, m.Deallocate(result))
			result = nil
		}
	}()
	middle, high := leftManager.NumTBits(), m.NumTBits()
	if middle != 0 {
		if err = m.vectors.SetSlice(result.positive, left.positive, middle-1, 0); err != nil {
			return nil, err
		}
	}
	if rightManager.NumTBits() != 0 {
		if err = m.vectors.SetSlice(result.positive, right.positive, high-1, middle); err != nil {
			return nil, err
		}
	}
	for i := range leftColumns {
		leftIndex, rightIndex := leftColumns[i], middle+rightColumns[i]
		if leftIndex >= middle || rightColumns[i] >= rightManager.NumTBits() {
			return nil, arena.ErrBounds
		}
		leftValue, readErr := m.vectors.Get(result.positive, leftIndex)
		if readErr != nil {
			return nil, readErr
		}
		rightValue, readErr := m.vectors.Get(result.positive, rightIndex)
		if readErr != nil {
			return nil, readErr
		}
		if leftValue == TBitX {
			if rightValue != TBitX {
				if err = m.vectors.Set(result.positive, leftIndex, rightValue); err != nil {
					return nil, err
				}
			}
		} else if rightValue == TBitX {
			if err = m.vectors.Set(result.positive, rightIndex, leftValue); err != nil {
				return nil, err
			}
		} else if leftValue != rightValue {
			return nil, nil
		}
	}
	for i := range leftColumns {
		leftIndex, rightIndex := leftColumns[i], middle+rightColumns[i]
		leftValue, _ := m.vectors.Get(result.positive, leftIndex)
		rightValue, _ := m.vectors.Get(result.positive, rightIndex)
		if leftValue == TBitX && rightValue == TBitX {
			first, allocErr := m.vectors.AllocateCopy(result.positive)
			if allocErr != nil {
				return nil, allocErr
			}
			_ = m.vectors.Set(first, leftIndex, TBit0)
			_ = m.vectors.Set(first, rightIndex, TBit1)
			result.negative.values = append(result.negative.values, first)
			second, allocErr := m.vectors.AllocateCopy(result.positive)
			if allocErr != nil {
				return nil, allocErr
			}
			_ = m.vectors.Set(second, leftIndex, TBit1)
			_ = m.vectors.Set(second, rightIndex, TBit0)
			result.negative.values = append(result.negative.values, second)
		}
	}
	for _, negative := range left.negative.values {
		copy, allocErr := m.vectors.AllocateX()
		if allocErr != nil {
			return nil, allocErr
		}
		if middle != 0 {
			if allocErr = m.vectors.SetSlice(copy, negative, middle-1, 0); allocErr != nil {
				_ = m.vectors.Deallocate(copy)
				return nil, allocErr
			}
		}
		valid, intersectErr := m.vectors.SetAnd(copy, result.positive)
		if intersectErr != nil {
			_ = m.vectors.Deallocate(copy)
			return nil, intersectErr
		}
		if valid {
			result.negative.values = append(result.negative.values, copy)
		} else {
			_ = m.vectors.Deallocate(copy)
		}
	}
	for _, negative := range right.negative.values {
		copy, allocErr := m.vectors.AllocateX()
		if allocErr != nil {
			return nil, allocErr
		}
		if rightManager.NumTBits() != 0 {
			if allocErr = m.vectors.SetSlice(copy, negative, high-1, middle); allocErr != nil {
				_ = m.vectors.Deallocate(copy)
				return nil, allocErr
			}
		}
		valid, intersectErr := m.vectors.SetAnd(copy, result.positive)
		if intersectErr != nil {
			_ = m.vectors.Deallocate(copy)
			return nil, intersectErr
		}
		if valid {
			result.negative.values = append(result.negative.values, copy)
		} else {
			_ = m.vectors.Deallocate(copy)
		}
	}
	valid, err := m.WellFormed(result)
	if err != nil || !valid {
		return nil, errors.Join(err, fmt.Errorf("%w: joined DOC is not well formed", ErrDOC))
	}
	keep = true
	return result, nil
}

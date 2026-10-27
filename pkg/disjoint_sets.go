package apalache

import "fmt"

// DisjointSets is an immutable union-find structure. Find and Merge return a
// new value because path compression changes the internal representation.
type DisjointSets[T comparable] struct {
	parents map[T]T
	ranks   map[T]int
}

func NewDisjointSets[T comparable](elements ...T) DisjointSets[T] {
	return DisjointSets[T]{}.Add(elements...)
}

func (sets DisjointSets[T]) Contains(element T) bool {
	_, parentOK := sets.parents[element]
	_, rankOK := sets.ranks[element]
	return parentOK && rankOK
}

func (sets DisjointSets[T]) Add(elements ...T) DisjointSets[T] {
	parents := cloneMap(sets.parents)
	ranks := cloneMap(sets.ranks)
	for _, element := range elements {
		if _, exists := parents[element]; exists {
			continue
		}
		parents[element] = element
		ranks[element] = 1
	}
	return DisjointSets[T]{parents: parents, ranks: ranks}
}

func (sets DisjointSets[T]) Find(element T) (T, DisjointSets[T], error) {
	var zero T
	if !sets.Contains(element) {
		return zero, sets, fmt.Errorf("%v is not tracked by this structure", element)
	}
	representative := element
	for sets.parents[representative] != representative {
		representative = sets.parents[representative]
	}
	parents := cloneMap(sets.parents)
	for element != representative {
		next := parents[element]
		parents[element] = representative
		element = next
	}
	return representative, DisjointSets[T]{parents: parents, ranks: cloneMap(sets.ranks)}, nil
}

func (sets DisjointSets[T]) Merge(left, right T) (T, DisjointSets[T], error) {
	var zero T
	leftRepresentative, afterLeft, err := sets.Find(left)
	if err != nil {
		return zero, sets, err
	}
	rightRepresentative, afterRight, err := afterLeft.Find(right)
	if err != nil {
		return zero, sets, err
	}
	if leftRepresentative == rightRepresentative {
		return rightRepresentative, afterRight, nil
	}

	leftRank := sets.ranks[leftRepresentative]
	rightRank := sets.ranks[rightRepresentative]
	parent, child := leftRepresentative, rightRepresentative
	if leftRank < rightRank {
		parent, child = rightRepresentative, leftRepresentative
	}
	parents := cloneMap(afterRight.parents)
	parents[child] = parent
	ranks := cloneMap(sets.ranks)
	ranks[parent] = leftRank + rightRank
	return parent, DisjointSets[T]{parents: parents, ranks: ranks}, nil
}

func (sets DisjointSets[T]) Len() int { return len(sets.parents) }

func (sets DisjointSets[T]) NumSets() int { return len(sets.Representatives()) }

func (sets DisjointSets[T]) Elements() map[T]struct{} {
	result := make(map[T]struct{}, len(sets.parents))
	for element := range sets.parents {
		result[element] = struct{}{}
	}
	return result
}

func (sets DisjointSets[T]) Representatives() map[T]struct{} {
	result := make(map[T]struct{})
	for element, parent := range sets.parents {
		if element == parent {
			result[element] = struct{}{}
		}
	}
	return result
}

func cloneMap[K comparable, V any](source map[K]V) map[K]V {
	result := make(map[K]V, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

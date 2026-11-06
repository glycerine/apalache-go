package rel

// Source: src/muz/rel/dl_base.cpp table_signature::from_join_project
// union-find equivalence accounting for retained non-functional keys.

type sparseColumnEquivalence struct {
	parent []int
	rank   []uint8
}

func newSparseColumnEquivalence(size int) *sparseColumnEquivalence {
	result := &sparseColumnEquivalence{parent: make([]int, size), rank: make([]uint8, size)}
	for i := range result.parent {
		result.parent[i] = i
	}
	return result
}

func (equivalence *sparseColumnEquivalence) find(column int) int {
	root := column
	for equivalence.parent[root] != root {
		root = equivalence.parent[root]
	}
	for equivalence.parent[column] != column {
		next := equivalence.parent[column]
		equivalence.parent[column] = root
		column = next
	}
	return root
}

func (equivalence *sparseColumnEquivalence) merge(left, right int) {
	left = equivalence.find(left)
	right = equivalence.find(right)
	if left == right {
		return
	}
	if equivalence.rank[left] < equivalence.rank[right] {
		left, right = right, left
	}
	equivalence.parent[right] = left
	if equivalence.rank[left] == equivalence.rank[right] {
		equivalence.rank[left]++
	}
}

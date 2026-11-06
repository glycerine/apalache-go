package hilbert

// orderedIndex is the concrete semantic portion of the source value_index2.
// Native slices replace heap_trie storage; the dominance relation is unchanged.
type orderedIndex struct {
	owner  *Basis
	values []*vector
}

func (i *orderedIndex) insert(value *vector) {
	i.owner.stats.indexInsert++
	i.values = append(i.values, value)
}

func (i *orderedIndex) subsumes(candidate *vector) bool {
	i.owner.stats.indexFind++
	for _, existing := range i.values {
		i.owner.stats.indexComparisons++
		if i.owner.subsumes(candidate, existing) {
			i.owner.stats.indexHit++
			i.owner.stats.numSubsumptions++
			return true
		}
	}
	i.owner.stats.indexMiss++
	return false
}

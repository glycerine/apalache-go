package sat

// Merge replaces c by the sorted union of a and b. The receiver must be empty,
// as it is in the source cut constructors used by the AIG cut pipeline.
func (c *satCut) Merge(a, b satCut) bool {
	i, j := uint32(0), uint32(0)
	x, y := a.Elem(i), b.Elem(j)
	for x != ^uint32(0) || y != ^uint32(0) {
		id := x
		if y < id {
			id = y
		}
		if !c.Add(id) {
			return false
		}
		switch {
		case x < y:
			i++
			x = a.Elem(i)
		case y < x:
			j++
			y = b.Elem(j)
		default:
			i++
			j++
			x, y = a.Elem(i), b.Elem(j)
		}
	}
	return true
}

// ShiftTable embeds this cut's truth table in the domain of a superset cut.
func (c satCut) ShiftTable(super satCut) uint64 {
	if !c.SubsetOf(super) {
		return 0
	}
	var positions [satCutMaxSize]uint32
	for i, j := uint32(0), uint32(0); i < c.size; i++ {
		for super.elems[j] != c.elems[i] {
			j++
		}
		positions[i] = j
		j++
	}
	var shifted uint64
	for assignment := uint32(0); assignment < uint32(1)<<super.size; assignment++ {
		var projected uint32
		for i := uint32(0); i < c.size; i++ {
			projected |= assignment >> positions[i] & 1 << i
		}
		shifted |= c.Table() >> projected & 1 << assignment
	}
	return shifted
}

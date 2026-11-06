package opt

// Source: src/opt/maxres.cpp next_index and weighted hill-climb boundaries.

func (e *maxResEngine) weightBoundaries() []int {
	if len(e.active) == 0 {
		return []int{0}
	}
	boundaries := make([]int, 0, len(e.active))
	for index := 0; index < len(e.active); {
		weight := e.weights[e.active[index]]
		index++
		for index < len(e.active) && e.weights[e.active[index]].Cmp(weight) == 0 {
			index++
		}
		boundaries = append(boundaries, index)
	}
	return boundaries
}

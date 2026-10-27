package apalache

import "fmt"

// StableTopologicalSort applies Kahn's algorithm one layer at a time. Nodes
// within a layer retain their order in the input sequence.
func StableTopologicalSort[T comparable](incoming map[T]map[T]struct{}, unsorted []T) (sorted []T, cycle map[T]struct{}, err error) {
	if len(incoming) != len(unsorted) {
		return nil, nil, fmt.Errorf("incoming edge keys and nodes differ")
	}
	originalOrder := make(map[T]int, len(unsorted))
	for index, node := range unsorted {
		if _, duplicate := originalOrder[node]; duplicate {
			return nil, nil, fmt.Errorf("duplicate node in topological sort")
		}
		originalOrder[node] = index
		if _, exists := incoming[node]; !exists {
			return nil, nil, fmt.Errorf("incoming edge keys and nodes differ")
		}
	}

	edges := make(map[T]map[T]struct{}, len(incoming))
	for node, dependencies := range incoming {
		if _, exists := originalOrder[node]; !exists {
			return nil, nil, fmt.Errorf("incoming edge keys and nodes differ")
		}
		edges[node] = cloneMap(dependencies)
	}

	sorted = make([]T, 0, len(unsorted))
	for len(edges) > 0 {
		sinks := make([]T, 0)
		// Iterating over the original input, rather than a Go map, both gives
		// stability and exactly matches Scala's explicit order sort.
		for _, node := range unsorted {
			if dependencies, exists := edges[node]; exists && len(dependencies) == 0 {
				sinks = append(sinks, node)
			}
		}
		if len(sinks) == 0 {
			cycle = make(map[T]struct{}, len(edges))
			for node := range edges {
				cycle[node] = struct{}{}
			}
			return nil, cycle, nil
		}
		removed := make(map[T]struct{}, len(sinks))
		for _, node := range sinks {
			sorted = append(sorted, node)
			removed[node] = struct{}{}
			delete(edges, node)
		}
		for _, dependencies := range edges {
			for node := range removed {
				delete(dependencies, node)
			}
		}
	}
	return sorted, nil, nil
}

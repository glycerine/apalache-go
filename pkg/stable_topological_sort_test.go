package apalache

import (
	"math/rand"
	"reflect"
	"testing"
)

func edgeSet[T comparable](nodes ...T) map[T]struct{} {
	result := make(map[T]struct{}, len(nodes))
	for _, node := range nodes {
		result[node] = struct{}{}
	}
	return result
}

func TestStableTopologicalSortLayers(t *testing.T) {
	edges := map[string]map[string]struct{}{
		"f": edgeSet("b", "c"),
		"a": edgeSet[string](),
		"c": edgeSet("a"),
		"b": edgeSet("a"),
		"d": edgeSet[string](),
	}
	got, cycle, err := StableTopologicalSort(edges, []string{"f", "a", "c", "b", "d"})
	if err != nil || cycle != nil {
		t.Fatalf("sort failed: cycle=%v err=%v", cycle, err)
	}
	want := []string{"a", "d", "c", "b", "f"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStableTopologicalSortCycleWitness(t *testing.T) {
	edges := map[int]map[int]struct{}{1: edgeSet(2), 2: edgeSet(1), 3: edgeSet[int]()}
	got, cycle, err := StableTopologicalSort(edges, []int{1, 2, 3})
	if err != nil || got != nil || !reflect.DeepEqual(cycle, edgeSet(1, 2)) {
		t.Fatalf("got sorted=%v cycle=%v err=%v", got, cycle, err)
	}
}

func TestStableTopologicalSortRandomDAGs(t *testing.T) {
	random := rand.New(rand.NewSource(0))
	for iteration := 0; iteration < 10_000; iteration++ {
		size := 1 + random.Intn(20)
		order := random.Perm(size)
		edges := make(map[int]map[int]struct{}, size)
		for position, node := range order {
			edges[node] = edgeSet[int]()
			for earlier := 0; earlier < position; earlier++ {
				if random.Intn(4) == 0 {
					edges[node][order[earlier]] = struct{}{}
				}
			}
		}
		unsorted := random.Perm(size)
		sorted, cycle, err := StableTopologicalSort(edges, unsorted)
		if err != nil || cycle != nil || len(sorted) != size {
			t.Fatalf("iteration %d failed: cycle=%v err=%v", iteration, cycle, err)
		}
		visited := make(map[int]struct{}, size)
		for _, node := range sorted {
			for dependency := range edges[node] {
				if _, found := visited[dependency]; !found {
					t.Fatalf("iteration %d placed %d before dependency %d", iteration, node, dependency)
				}
			}
			visited[node] = struct{}{}
		}
	}
}

func TestStableTopologicalSortMatchesScalaGeneratedGraphs(t *testing.T) {
	random := rand.New(rand.NewSource(0x70a010))
	for iteration := 0; iteration < 10_000; iteration++ {
		size := 1 + random.Intn(10)
		uses := make([]int, size)
		for index := range uses {
			uses[index] = random.Intn(size)
		}
		threshold := random.Intn(size)
		unsorted := append([]int(nil), uses...)
		edges := make(map[int]map[int]struct{})
		for _, node := range uses {
			if node <= threshold {
				edges[node] = edgeSet[int]()
			} else {
				edges[node] = edgeSet(uses[node])
			}
		}

		sorted, witnesses, err := StableTopologicalSort(edges, unsorted)
		if err != nil {
			t.Fatalf("iteration %d rejected Scala-generated graph %v: %v", iteration, unsorted, err)
		}
		if witnesses == nil {
			visited := make(map[int]struct{}, len(sorted))
			for _, node := range sorted {
				for dependency := range edges[node] {
					if _, found := visited[dependency]; !found {
						t.Fatalf("iteration %d placed %d before dependency %d", iteration, node, dependency)
					}
				}
				visited[node] = struct{}{}
			}
			if len(unsorted) == len(edges) {
				originalLayer := make([]int, 0)
				sortedLayer := make([]int, 0)
				for _, node := range unsorted {
					if len(edges[node]) == 0 {
						originalLayer = append(originalLayer, node)
					}
				}
				for _, node := range sorted {
					if len(edges[node]) == 0 {
						sortedLayer = append(sortedLayer, node)
					}
				}
				if !reflect.DeepEqual(originalLayer, sortedLayer) {
					t.Fatalf("iteration %d changed stable layer zero: %v to %v", iteration, originalLayer, sortedLayer)
				}
			}
		} else if !witnessesContainCycle(edges, witnesses) {
			t.Fatalf("iteration %d returned non-cycle witnesses %v for %v", iteration, witnesses, edges)
		}
	}
}

func witnessesContainCycle(edges map[int]map[int]struct{}, witnesses map[int]struct{}) bool {
	const (
		unseen = iota
		active
		done
	)
	state := make(map[int]int, len(witnesses))
	var visit func(int) bool
	visit = func(node int) bool {
		state[node] = active
		for dependency := range edges[node] {
			if _, retained := witnesses[dependency]; !retained {
				continue
			}
			if state[dependency] == active || state[dependency] == unseen && visit(dependency) {
				return true
			}
		}
		state[node] = done
		return false
	}
	for node := range witnesses {
		if state[node] == unseen && visit(node) {
			return true
		}
	}
	return false
}

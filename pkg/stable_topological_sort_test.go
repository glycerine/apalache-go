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

package apalache

import (
	"reflect"
	"testing"
)

func TestDisjointSetsBadCallsAndAdd(t *testing.T) {
	empty := NewDisjointSets[int]()
	sets := empty.Add(1, 2, 3, 1, 2, 3)
	if empty.NumSets() != 0 || sets.NumSets() != 3 || sets.Len() != 3 {
		t.Fatalf("unexpected set counts: empty=%d sets=%d elements=%d", empty.NumSets(), sets.NumSets(), sets.Len())
	}
	if _, _, err := sets.Find(0); err == nil {
		t.Fatal("Find accepted an untracked element")
	}
	if _, _, err := sets.Merge(0, 1); err == nil {
		t.Fatal("Merge accepted an untracked left element")
	}
	if _, _, err := sets.Merge(1, 0); err == nil {
		t.Fatal("Merge accepted an untracked right element")
	}
}

func TestDisjointSetsMergeIsImmutable(t *testing.T) {
	sets := NewDisjointSets(1, 2, 3, 4)
	representative, same, err := sets.Merge(1, 1)
	if err != nil || representative != 1 || same.NumSets() != sets.NumSets() {
		t.Fatalf("dummy merge: representative=%d sets=%d err=%v", representative, same.NumSets(), err)
	}
	_, merged, err := sets.Merge(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if sets.NumSets() != 4 || merged.NumSets() != 3 {
		t.Fatalf("merge mutated its input: before=%d after=%d", sets.NumSets(), merged.NumSets())
	}
	left, _, _ := merged.Find(1)
	right, _, _ := merged.Find(3)
	if left != right {
		t.Fatalf("merged representatives differ: %d != %d", left, right)
	}
}

func TestDisjointSetsMergeAll(t *testing.T) {
	sets := NewDisjointSets(1, 2, 3, 4)
	for element := range sets.Elements() {
		_, sets, _ = sets.Merge(1, element)
	}
	if sets.NumSets() != 1 {
		t.Fatalf("got %d sets, want 1", sets.NumSets())
	}
}

func TestDisjointSetsDummyMergeEqualsFindScalaOracleCase(t *testing.T) {
	sets := NewDisjointSets(1, 2, 3, 4)
	_, sets, _ = sets.Merge(2, 3)
	_, sets, _ = sets.Merge(1, 2)
	mergeRepresentative, afterMerge, mergeErr := sets.Merge(1, 1)
	findRepresentative, afterFind, findErr := sets.Find(1)
	if mergeErr != nil || findErr != nil || mergeRepresentative != findRepresentative || !reflect.DeepEqual(afterMerge, afterFind) {
		t.Fatalf("dummy merge = (%d, %#v, %v), find = (%d, %#v, %v)",
			mergeRepresentative, afterMerge, mergeErr, findRepresentative, afterFind, findErr)
	}
}

func TestDisjointSetsCountsRootsBeforePathCompression(t *testing.T) {
	sets := NewDisjointSets(1, 2, 3, 4)
	_, sets, _ = sets.Merge(1, 2)
	_, sets, _ = sets.Merge(3, 4)
	_, sets, _ = sets.Merge(2, 4)
	if sets.NumSets() != 1 {
		t.Fatalf("got %d sets after merging two nontrivial trees, want 1", sets.NumSets())
	}
}

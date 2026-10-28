package apalache

import (
	"reflect"
	"testing"
)

func TestIntTupleIteratorScalaOrderAndExhaustion(t *testing.T) {
	iterator := NewIntTupleIterator([]int{1, 2})
	want := [][]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {0, 2}, {1, 2}}
	got := make([][]int, 0, len(want))
	for iterator.HasNext() {
		tuple, err := iterator.Next()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, tuple)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tuple order = %v, want %v", got, want)
	}
	if iterator.Remaining().Sign() != 0 {
		t.Fatalf("remaining = %s", iterator.Remaining())
	}
	if _, err := iterator.Next(); err == nil {
		t.Fatal("exhausted iterator returned another tuple")
	}
	got[0][0] = 99
	if got[1][0] != 1 {
		t.Fatal("returned tuples alias one another")
	}
}

func TestIntTupleIteratorEmptyDomainsAndProductLimit(t *testing.T) {
	for _, limits := range [][]int{{-1, 2}, {2, -1}, {-2, 2}} {
		iterator := NewIntTupleIterator(limits)
		if iterator.HasNext() || iterator.Remaining().Sign() != 0 {
			t.Fatalf("empty domain %v has %s tuples", limits, iterator.Remaining())
		}
	}
	tooLarge := NewIntTupleIterator([]int{MaxProductSize, 0})
	if _, err := tooLarge.Next(); err == nil {
		t.Fatal("oversized Cartesian product was enumerated")
	}
}

func TestProduct2IteratorScalaOrderAndEmptyInput(t *testing.T) {
	iterator := NewProduct2Iterator([]string{"a", "b"}, []int{1, 2, 3})
	want := []Pair[string, int]{
		{First: "a", Second: 1}, {First: "b", Second: 1},
		{First: "a", Second: 2}, {First: "b", Second: 2},
		{First: "a", Second: 3}, {First: "b", Second: 3},
	}
	got := make([]Pair[string, int], 0, len(want))
	for iterator.HasNext() {
		pair, err := iterator.Next()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, pair)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("product order = %v, want %v", got, want)
	}
	if NewProduct2Iterator([]string{}, []int{1}).HasNext() {
		t.Fatal("product with an empty left input was nonempty")
	}
}

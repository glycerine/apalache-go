package cliques

// Source: src/util/max_cliques.h:26-137.

import (
	"errors"
	"math"
	"math/bits"
)

var ErrLiteralRange = errors.New("cliques: literal index exceeds host slice range")

type uintSet []uint32

func (s uintSet) contains(value uint32) bool {
	word := uint64(value) >> 5
	return word < uint64(len(s)) && s[word]&(uint32(1)<<(value&31)) != 0
}
func (s *uintSet) insert(value uint32) error {
	word := uint64(value) >> 5
	if word >= uint64(maxInt()) {
		return ErrLiteralRange
	}
	if word >= uint64(len(*s)) {
		*s = append(*s, make([]uint32, int(word)+1-len(*s))...)
	}
	(*s)[word] |= uint32(1) << (value & 31)
	return nil
}
func (s uintSet) remove(value uint32) {
	word := uint64(value) >> 5
	if word < uint64(len(s)) {
		s[word] &^= uint32(1) << (value & 31)
	}
}
func (s uintSet) empty() bool {
	for _, word := range s {
		if word != 0 {
			return false
		}
	}
	return true
}
func (s uintSet) first() uint32 {
	for i, word := range s {
		if word != 0 {
			return uint32(i*32 + bits.TrailingZeros32(word))
		}
	}
	return uint32(len(s) * 32)
}
func cloneSet(src uintSet) uintSet { return append(uintSet(nil), src...) }
func maxInt() int                  { return int(^uint(0) >> 1) }

// Finder greedily partitions complement-paired unsigned literals. Negation is
// index XOR 1. The transitive-closure cache deliberately persists across calls,
// matching max_cliques.h; construct a new Finder for an independent query.
type Finder struct {
	next, closure   [][]uint32
	reachable       [2]uintSet
	seen, completed uintSet
	todo            []uint32
}

func New() *Finder { return &Finder{} }

func (f *Finder) ensureRows(largest uint32) error {
	if largest == math.MaxUint32 {
		return ErrLiteralRange
	}
	need := uint64(largest) + 1
	if need > uint64(maxInt()) {
		return ErrLiteralRange
	}
	if int(need) > len(f.next) {
		f.next = append(f.next, make([][]uint32, int(need)-len(f.next))...)
	}
	if int(need) > len(f.closure) {
		f.closure = append(f.closure, make([][]uint32, int(need)-len(f.closure))...)
	}
	return nil
}

func (f *Finder) AddEdge(src, dst uint32) error {
	largest := src
	if dst > largest {
		largest = dst
	}
	if src^1 > largest {
		largest = src ^ 1
	}
	if dst^1 > largest {
		largest = dst ^ 1
	}
	if err := f.ensureRows(largest); err != nil {
		return err
	}
	f.next[src] = append(f.next[src], dst)
	f.next[dst] = append(f.next[dst], src)
	return nil
}

func (f *Finder) reachableFrom(p uint32, goal uintSet, out *uintSet) error {
	f.seen = f.seen[:0]
	f.todo = append(f.todo[:0], p)
	for i := 0; i < len(f.todo); i++ {
		p = f.todo[i]
		if f.seen.contains(p) {
			continue
		}
		if err := f.seen.insert(p); err != nil {
			return err
		}
		if f.completed.contains(p) {
			if int(p) < len(f.closure) {
				for _, next := range f.closure[p] {
					if goal.contains(next) {
						if err := out.insert(next); err != nil {
							return err
						}
					}
				}
			}
			continue
		}
		next := p ^ 1
		if goal.contains(next) {
			if err := out.insert(next); err != nil {
				return err
			}
		}
		if int(next) < len(f.next) {
			f.todo = append(f.todo, f.next[next]...)
		}
	}
	for i := len(f.todo); i > 0; i-- {
		p = f.todo[i-1]
		if f.completed.contains(p) {
			continue
		}
		if err := f.completed.insert(p); err != nil {
			return err
		}
		if err := f.ensureRows(p); err != nil {
			return err
		}
		next := p ^ 1
		if goal.contains(next) {
			f.closure[p] = append(f.closure[p], next)
		} else if int(next) < len(f.next) {
			for _, successor := range f.next[next] {
				if int(successor) < len(f.closure) {
					f.closure[p] = append(f.closure[p], f.closure[successor]...)
				}
			}
		}
	}
	return nil
}

func (f *Finder) Cliques(literals []uint32) ([][]uint32, error) {
	var remaining uintSet
	for _, literal := range literals {
		if err := remaining.insert(literal); err != nil {
			return nil, err
		}
	}
	var result [][]uint32
	for !remaining.empty() {
		var clique []uint32
		turn := 0
		f.reachable[turn] = cloneSet(remaining)
		for !f.reachable[turn].empty() {
			literal := f.reachable[turn].first()
			f.reachable[turn].remove(literal)
			remaining.remove(literal)
			clique = append(clique, literal)
			if f.reachable[turn].empty() {
				break
			}
			other := 1 - turn
			f.reachable[other] = f.reachable[other][:0]
			if err := f.reachableFrom(literal, f.reachable[turn], &f.reachable[other]); err != nil {
				return nil, err
			}
			turn = other
		}
		if len(clique) > 1 {
			result = append(result, clique)
		}
	}
	return result, nil
}

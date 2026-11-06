package arith

// Source: src/tactic/arith/fm_tactic.cpp:1148-1193.
import (
	"math"
	"sort"
)

func (t *FMVariableTable) Cost(x uint32) uint32 {
	product := uint64(len(t.Index.Lower[x])) * uint64(len(t.Index.Upper[x]))
	if product > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(product)
}

func (t *FMVariableTable) Candidates() []uint32 {
	xs := make([]uint32, 0, len(t.Integer))
	for x := range t.Integer {
		if !t.IsForbidden(uint32(x)) {
			xs = append(xs, uint32(x))
		}
	}
	sort.SliceStable(xs, func(i, j int) bool {
		x, y := xs[i], xs[j]
		cx, cy := t.Cost(x), t.Cost(y)
		if cx == 0 {
			if cy > 0 {
				return true
			}
			return x < y
		}
		if cy == 0 {
			return false
		}
		xi, yi := t.IsInteger(x), t.IsInteger(y)
		return (!xi && yi) || (xi == yi && cx < cy)
	})
	return xs
}

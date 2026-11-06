package opt

// Source: src/opt/maxres.cpp sort_assumptions/compare_asm.

import "sort"

func (e *maxResEngine) sortAssumptions() {
	sort.SliceStable(e.active, func(i, j int) bool {
		return e.weights[e.active[i]].Cmp(e.weights[e.active[j]]) > 0
	})
}

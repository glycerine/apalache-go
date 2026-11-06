package smtcore

// Source: src/smt/theory_arith_aux.h max_min(vars) and
// src/smt/theory_arith_core.h m_asserted_bounds propagation and scope heads.

import "github.com/glycerine/g3/pkg/math/simplex"

func (t *SMTLinearArithmeticTheory) queueOptimizationBound(solver *simplex.Solver, bound smtArithmeticReplayBound) bool {
	old, found := solver.Upper(bound.column)
	if bound.lower {
		old, found = solver.Lower(bound.column)
	}
	if found && (bound.lower && bound.value.Cmp(&old) <= 0 || !bound.lower && bound.value.Cmp(&old) >= 0) {
		return false
	}
	for _, pending := range t.pendingBounds[t.pendingBoundHead:] {
		if pending.column == bound.column && pending.lower == bound.lower && (bound.lower && pending.value.Cmp(&bound.value) >= 0 || !bound.lower && pending.value.Cmp(&bound.value) <= 0) {
			return false
		}
	}
	bound.value = bound.value.Clone()
	t.pendingBounds = append(t.pendingBounds, bound)
	if t.state != nil {
		t.state.theoryProgress++
	}
	return true
}

// Keep the consumed prefix: popping a scope may restore an earlier head,
// including parent-scope bounds consumed while a child scope was active.
func (t *SMTLinearArithmeticTheory) drainOptimizationBounds() []smtArithmeticReplayBound {
	bounds := t.pendingBounds[t.pendingBoundHead:]
	t.pendingBoundHead = len(t.pendingBounds)
	t.derivedBounds = append(t.derivedBounds, bounds...)
	return bounds
}

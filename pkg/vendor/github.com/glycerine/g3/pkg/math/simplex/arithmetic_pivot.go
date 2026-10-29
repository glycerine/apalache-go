package simplex

// Source: src/smt/theory_arith_core.h select_lg_error_var,
// select_var_to_fix and make_feasible. These policies are consumed by the
// concrete SMT arithmetic theory using this shared exact tableau.

import "sort"

// SetArithmeticPivotPolicy configures the source SMT arithmetic selection,
// including its repeated-leaving-variable threshold for Bland's rule.
func (s *Solver) SetArithmeticPivotPolicy(strategy, blandThreshold uint32) {
	s.arithmeticPolicy = true
	s.arithmeticPivotStrategy = strategy
	s.arithmeticBlandThreshold = blandThreshold
}

func (s *Solver) selectArithmeticPatch() uint32 {
	if s.arithmeticBland || s.arithmeticPivotStrategy != 1 && s.arithmeticPivotStrategy != 2 {
		return s.selectPatch()
	}
	keys := make([]uint32, 0, len(s.patch))
	for v := range s.patch {
		keys = append(keys, v)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	best := NullVariable
	var bestError Infinitesimal
	for _, v := range keys {
		var delta Infinitesimal
		if s.belowLower(v) {
			delta = s.vars[v].lower.Clone()
			delta.Sub(&s.vars[v].value)
		} else if s.aboveUpper(v) {
			delta = s.vars[v].value.Clone()
			delta.Sub(&s.vars[v].upper)
		} else {
			continue
		}
		cmp := delta.Cmp(&bestError)
		if best == NullVariable || s.arithmeticPivotStrategy == 1 && cmp > 0 || s.arithmeticPivotStrategy == 2 && cmp < 0 {
			best = v
			bestError = delta
		}
	}
	if best == NullVariable {
		clear(s.patch)
	} else {
		delete(s.patch, best)
	}
	return best
}

func (s *Solver) checkArithmeticBlands(v uint32, left map[uint32]bool, repeated *uint32) {
	if s.arithmeticBland {
		return
	}
	if left[v] {
		(*repeated)++
		if *repeated > s.arithmeticBlandThreshold {
			s.arithmeticBland = true
		}
	} else {
		left[v] = true
	}
}

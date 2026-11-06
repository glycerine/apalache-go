package simplex

// Source: src/smt/theory_arith_core.h sign_row_conflict and explain_bound.

import "math/big"

type arithmeticBoundKey struct {
	column uint32
	lower  bool
}
type arithmeticBoundAtom struct {
	value        Infinitesimal
	dependencies []BoundDependency
}

// AddArithmeticBoundAtom retains an active asserted endpoint, including weaker
// atoms superseded by a stronger bound. Its lifetime belongs to this replay.
func (s *Solver) AddArithmeticBoundAtom(column uint32, lower bool, value Infinitesimal, dependencies []BoundDependency) {
	if s.arithmeticBoundAtoms == nil {
		s.arithmeticBoundAtoms = make(map[arithmeticBoundKey][]arithmeticBoundAtom)
	}
	key := arithmeticBoundKey{column, lower}
	s.arithmeticBoundAtoms[key] = append(s.arithmeticBoundAtoms[key], arithmeticBoundAtom{value.Clone(), cloneBoundDependencies(dependencies)})
}

// ExplainArithmeticInfeasibleRow implements sign_row_conflict and its optional
// stronger-lemmas relaxation. It records an owning explanation;
// publishing a logical conflict remains the SMT owner's responsibility.
func (s *Solver) ExplainArithmeticInfeasibleRow(integers map[uint32]bool, relax bool) error {
	row, err := s.InfeasibleRow()
	if err != nil {
		return err
	}
	base := s.infeasible
	x := &s.vars[base]
	below := s.belowLower(base)
	var delta Infinitesimal
	if relax {
		delta = x.value.Clone()
		if below {
			delta = x.lower.Clone()
			delta.Sub(&x.value)
		} else {
			delta.Sub(&x.upper)
		}
		var epsilon Infinitesimal
		if integers[base] {
			epsilon.Main.SetInt64(1)
		} else {
			epsilon.Epsilon.SetInt64(1)
		}
		delta.Sub(&epsilon)
		if delta.Sign() < 0 {
			delta = Infinitesimal{}
		}
	}
	entries, err := s.RowEntries(row)
	if err != nil {
		return err
	}
	var result []BoundDependency
	for _, entry := range entries {
		column := entry.Variable
		if column == base {
			continue
		}
		coefficient := new(big.Rat).Quo(entry.Coefficient, &x.baseCoeff)
		lower := below == (coefficient.Sign() > 0)
		dependencies, known := s.BoundDependencies(column, lower)
		if !known {
			s.RecordBoundConflictDependencies(nil, false)
			return nil
		}
		// Source has_justification guard.
		if len(dependencies) == 0 {
			continue
		}
		if delta.Sign() > 0 {
			coefficient.Abs(coefficient)
			old, _ := s.Upper(column)
			if lower {
				old, _ = s.Lower(column)
			}
			limit := delta.Clone()
			limit.Quo(coefficient)
			if lower {
				limit.Neg()
			}
			limit.Add(&old)
			value := old.Clone()
			var chosen *arithmeticBoundAtom
			atoms := s.arithmeticBoundAtoms[arithmeticBoundKey{column, lower}]
			for i := range atoms {
				candidate := &atoms[i]
				if lower && candidate.value.Cmp(&limit) >= 0 && candidate.value.Cmp(&value) < 0 || !lower && candidate.value.Cmp(&limit) <= 0 && candidate.value.Cmp(&value) > 0 {
					value = candidate.value.Clone()
					chosen = candidate
				}
			}
			if chosen != nil {
				difference := value.Clone()
				difference.Sub(&old)
				difference.Abs()
				difference.Mul(coefficient)
				delta.Sub(&difference)
				dependencies = chosen.dependencies
			}
		}
		for _, dependency := range dependencies {
			result = append(result, BoundDependency{Literal: dependency.Literal, Equality: dependency.Equality, Coefficient: *new(big.Rat).Mul(coefficient, &dependency.Coefficient)})
		}
	}
	dependencies, known := s.BoundDependencies(base, below)
	if !known {
		s.RecordBoundConflictDependencies(nil, false)
		return nil
	}
	result = append(result, dependencies...)
	s.RecordBoundConflictDependencies(result, true)
	return nil
}

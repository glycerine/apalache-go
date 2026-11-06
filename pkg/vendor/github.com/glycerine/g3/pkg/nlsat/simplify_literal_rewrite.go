package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:86-127.

import "github.com/glycerine/g3/pkg/sat"

func (s *Solver) rewriteContentClauses(replacements map[sat.BoolVar]sat.Literal) (uint32, error) {
	if len(replacements) == 0 {
		return 0, nil
	}
	originals := append([]*Clause(nil), s.clauses.originals...)
	var count uint32
	for _, clause := range originals {
		changed, tautology := false, false
		literals := make([]sat.Literal, 0, len(clause.Literals()))
		for _, literal := range clause.Literals() {
			replacement, ok := replacements[literal.Variable()]
			if !ok {
				literals = append(literals, literal)
				continue
			}
			changed = true
			if literal.Sign() {
				replacement = replacement.Negated()
			}
			if replacement == TrueLiteral {
				tautology = true
				break
			}
			if replacement != FalseLiteral {
				literals = append(literals, replacement)
			}
		}
		if !changed {
			continue
		}
		if !tautology {
			if len(literals) == 0 {
				literals = append(literals, FalseLiteral)
			}
			if _, err := s.clauses.mkClause(literals, clause.IsLearned(), clause.Assumptions()); err != nil {
				return count, err
			}
		}
		if err := s.clauses.deleteClause(clause); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

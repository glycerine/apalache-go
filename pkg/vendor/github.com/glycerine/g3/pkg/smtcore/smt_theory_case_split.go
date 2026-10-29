package smtcore

// Source: src/smt/smt_context.cpp mk_th_case_split,
// undo_th_case_split and propagate_th_case_split.

import "github.com/glycerine/g3/pkg/ast"

// MakeTheoryCaseSplit enforces at most one of the supplied signed literals.
// It does not add an at-least-one clause; that comes from the theory's own
// arrangement axiom. The input span is borrowed only during this call.
func (s *SMTInternalizerState) MakeTheoryCaseSplit(literals []SMTLiteral) error {
	if err := s.open(); err != nil {
		return err
	}
	for _, literal := range literals {
		if literal < 0 || int(literal.Var()) >= len(s.assignment) {
			return ast.ErrASTDomain
		}
	}
	if !s.theoryCaseSplit {
		for i, first := range literals {
			for _, second := range literals[i+1:] {
				if err := s.addClause([]SMTLiteral{first.Negated(), second.Negated()}, false, ast.ASTHandle{}); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if smtcoreSourceDebug {
		seen := make(map[SMTLiteral]struct{}, len(literals))
		for _, literal := range literals {
			_, duplicate := seen[literal]
			_, registered := s.allTheoryCaseSplitLiterals[literal]
			if duplicate || registered {
				return ast.ErrASTDomain
			}
			seen[literal] = struct{}{}
		}
	}
	if s.allTheoryCaseSplitLiterals == nil {
		s.allTheoryCaseSplitLiterals = make(map[SMTLiteral]struct{})
		s.literalTheoryCaseSplitSets = make(map[SMTLiteral][][]SMTLiteral)
	}
	group := append([]SMTLiteral(nil), literals...)
	for _, literal := range group {
		s.allTheoryCaseSplitLiterals[literal] = struct{}{}
	}
	s.theoryCaseSplitSets = append(s.theoryCaseSplitSets, group)
	for _, literal := range group {
		s.literalTheoryCaseSplitSets[literal] = append(s.literalTheoryCaseSplitSets[literal], group)
	}
	return nil
}

func (s *SMTInternalizerState) undoTheoryCaseSplits(limit int) error {
	if limit < 0 || limit > len(s.theoryCaseSplitSets) {
		return ast.ErrASTDomain
	}
	for len(s.theoryCaseSplitSets) > limit {
		last := len(s.theoryCaseSplitSets) - 1
		group := s.theoryCaseSplitSets[last]
		s.theoryCaseSplitSets[last] = nil
		s.theoryCaseSplitSets = s.theoryCaseSplitSets[:last]
		for i := len(group) - 1; i >= 0; i-- {
			literal := group[i]
			delete(s.allTheoryCaseSplitLiterals, literal)
			sets := s.literalTheoryCaseSplitSets[literal]
			if len(sets) != 0 {
				sets[len(sets)-1] = nil
				s.literalTheoryCaseSplitSets[literal] = sets[:len(sets)-1]
			}
		}
	}
	return nil
}

func (s *SMTInternalizerState) propagateTheoryCaseSplits(head int) (bool, error) {
	if len(s.allTheoryCaseSplitLiterals) == 0 {
		return true, nil
	}
	end := len(s.assignedLiterals)
	if head < 0 || head > end {
		return false, ast.ErrASTDomain
	}
	// The bound is captured after BCP. Assignments made by this loop are
	// processed by BCP/theory case splits on the next outer iteration.
	for ; head < end; head++ {
		literal := s.assignedLiterals[head]
		if _, found := s.allTheoryCaseSplitLiterals[literal]; !found {
			continue
		}
		for _, group := range s.literalTheoryCaseSplitSets[literal] {
			for _, other := range group {
				if other == literal {
					continue
				}
				if err := s.AssignLiteral(other.Negated(), NewSMTBooleanBinaryClause(literal)); err != nil {
					return false, err
				}
				if s.inconsistent {
					return false, nil
				}
			}
		}
	}
	return true, nil
}

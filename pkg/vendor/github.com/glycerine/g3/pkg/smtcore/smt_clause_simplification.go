package smtcore

// Source: src/smt/smt_context.cpp simplify_clause() and simplify_clauses().

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *SMTInternalizerState) replaceSatisfiedClauseReasons(record *SMTInternalizedClause) error {
	clause := record.clause
	proofs, err := s.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	for index := 0; index < 2; index++ {
		literal, err := clause.Literal(index)
		if err != nil {
			return err
		}
		data := &s.booleanData[literal.Var()]
		reason, ok := data.Justification().Clause()
		if !ok || reason != clause {
			continue
		}
		if !proofs {
			data.SetJustification(NewSMTBooleanAxiom())
			continue
		}
		antecedent := clause.Justification()
		if antecedent == nil {
			return ast.ErrASTDomain
		}
		parents := make([]SMTLiteral, 0, len(clause.literals)-1)
		for position, other := range clause.literals {
			if position != index {
				parents = append(parents, other.Negated())
			}
		}
		updated, err := NewSMTUnitResolutionJustification(antecedent.InRegion(), antecedent, parents)
		if err != nil {
			return err
		}
		wrapped, err := NewSMTBooleanJustification(updated)
		if err != nil {
			return err
		}
		data.SetJustification(wrapped)
		if !antecedent.InRegion() {
			if err = clause.SetJustification(nil); err != nil {
				return err
			}
			s.ownedJustifications = append(s.ownedJustifications, updated)
		} else if record.owner != nil {
			// Auxiliary clauses normally own their region proof wrapper through
			// the record. Earlier simplification can wrap it in unit-resolution
			// reasons, so the original owner may be below antecedent in the
			// chain. The replacement reason still borrows that original owner.
			s.ownedJustifications = append(s.ownedJustifications, record.owner)
			record.owner = nil
		}
	}
	return nil
}

func (s *SMTInternalizerState) simplifyClause(record *SMTInternalizedClause) (bool, error) {
	if record.Binary {
		for _, literal := range record.Literals {
			value, err := s.Assignment(literal)
			if err != nil {
				return false, err
			}
			if value == lbool.True {
				return true, nil
			}
		}
		return false, nil
	}
	clause := record.clause
	if clause == nil || clause.InReinitStack() {
		return false, nil
	}
	for index := 0; index < 2; index++ {
		value, err := s.Assignment(clause.literals[index])
		if err != nil {
			return false, err
		}
		if value == lbool.True {
			return true, s.replaceSatisfiedClauseReasons(record)
		}
	}
	removed := make([]SMTLiteral, 0)
	kept := 2
	for index := 2; index < len(clause.literals); index++ {
		literal := clause.literals[index]
		value, err := s.Assignment(literal)
		if err != nil {
			return false, err
		}
		switch value {
		case lbool.False:
			removed = append(removed, literal.Negated())
		case lbool.Undef:
			clause.literals[kept] = literal
			kept++
		case lbool.True:
			return true, s.replaceSatisfiedClauseReasons(record)
		}
	}
	if kept != len(clause.literals) {
		if err := clause.SetNumLiterals(kept); err != nil {
			return false, err
		}
		record.Literals = append(record.Literals[:0], clause.literals...)
	}
	if len(removed) != 0 {
		proofs, err := s.manager.ProofsEnabled()
		if err != nil {
			return false, err
		}
		if proofs {
			antecedent := clause.Justification()
			if antecedent == nil {
				return false, ast.ErrASTDomain
			}
			updated, err := NewSMTUnitResolutionJustification(antecedent.InRegion(), antecedent, removed)
			if err != nil {
				return false, err
			}
			if err = clause.SetJustification(updated); err != nil {
				return false, err
			}
		}
	}
	return false, nil
}

func (s *SMTInternalizerState) simplifyClausesAtBase(baseLevel uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.scopeLevel != baseLevel || s.simplifyHead == len(s.assignedLiterals) || s.simplifyCounter > 0 {
		return nil
	}
	if len(s.clauses) == 0 {
		return nil
	}
	start := 0
	if baseLevel != 0 {
		if int(baseLevel) > len(s.scopes) {
			return ast.ErrASTDomain
		}
		start = s.scopes[baseLevel-1].clauseLimit
	}
	s.simplifyHead = len(s.assignedLiterals)
	s.simplifyCounter = 0
	remove := make(map[int]struct{})
	for index := start; index < len(s.clauses); index++ {
		deleted, err := s.simplifyClause(&s.clauses[index])
		if err != nil {
			return err
		}
		if deleted {
			remove[index] = struct{}{}
			continue
		}
		if clause := s.clauses[index].clause; clause != nil {
			s.simplifyCounter += len(clause.literals)
		} else {
			s.simplifyCounter += len(s.clauses[index].Literals)
		}
	}
	return s.deleteLearnedClauseIndexes(remove)
}

package smtcore

// Source: src/smt/smt_context.cpp assign_core/bcp and
// src/smt/smt_internalizer.cpp clause/watch construction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *SMTInternalizerState) AssignLiteral(literal SMTLiteral, justification SMTBooleanJustification) error {
	return s.assignLiteralCore(literal, justification, false)
}

func (s *SMTInternalizerState) assignDecisionLiteral(literal SMTLiteral) error {
	return s.assignLiteralCore(literal, NewSMTBooleanAxiom(), true)
}

func (s *SMTInternalizerState) assignLiteralCore(literal SMTLiteral, justification SMTBooleanJustification, decision bool) error {
	if err := s.open(); err != nil {
		return err
	}
	if literal == SMTTrueLiteral {
		return nil
	}
	if literal == SMTFalseLiteral || literal.Var() < SMTFirstBoolVar || int(literal.Var()) >= len(s.boolExpr) {
		if literal == SMTFalseLiteral {
			s.setBooleanConflict(justification, SMTNullLiteral)
			return nil
		}
		return ast.ErrASTDomain
	}
	value, err := s.Assignment(literal)
	if err != nil {
		return err
	}
	if value == lbool.True {
		return nil
	}
	if value == lbool.False {
		// Source not_l is the already true opposite of the failed consequent.
		s.setBooleanConflict(justification, literal.Negated())
		return nil
	}
	want := lbool.True
	if literal.Sign() {
		want = lbool.False
	}
	if err = s.SetAssignment(literal.Var(), want); err != nil {
		return err
	}
	data := &s.booleanData[literal.Var()]
	data.SetJustification(justification)
	data.ScopeLevel = s.scopeLevel
	if s.searching && s.searchRestartAdaptive && data.PhaseAvailable {
		s.searchAgility *= s.searchAgilityFactor
		if !decision && data.Phase == literal.Sign() {
			s.searchAgility += 1 - s.searchAgilityFactor
		}
	}
	data.PhaseAvailable = true
	data.Phase = !literal.Sign()
	s.assignedLiterals = append(s.assignedLiterals, literal)
	if err := s.relevanceGoalAssigned(literal); err != nil {
		return err
	}
	if s.relevancy != nil {
		return s.relevancy.AssignmentEvent(s.boolExpr[literal.Var()], want == lbool.True)
	}
	return nil
}

// Source context::set_conflict keeps the first conflict while the current
// theory callback continues to publish assignments and clauses.
func (s *SMTInternalizerState) setBooleanConflict(justification SMTBooleanJustification, literal SMTLiteral) {
	if s.inconsistent {
		return
	}
	s.inconsistent = true
	s.conflict = justification
	s.conflictLiteral = literal
	s.statistics.NumConflicts++
}

func (s *SMTInternalizerState) Conflict() (SMTBooleanJustification, bool) {
	if s == nil || !s.inconsistent {
		return NullSMTBooleanJustification(), false
	}
	return s.conflict, true
}

func (s *SMTInternalizerState) ConflictLiteral() SMTLiteral {
	if s == nil || !s.inconsistent {
		return SMTNullLiteral
	}
	return s.conflictLiteral
}

// SetTheoryConflict installs a conflict explained by currently asserted
// theory literals. The justification is owned by the logical context scope.
// Optional parameters retain source theory-lemma rule metadata.
func (s *SMTInternalizerState) SetTheoryConflict(theory SMTTheoryID, literals []SMTLiteral, parameters ...ast.ASTParameter) error {
	if err := s.open(); err != nil {
		return err
	}
	if theory == SMTNullTheoryID || s.theories[theory] == nil || len(literals) == 0 {
		return ast.ErrASTDomain
	}
	if s.inconsistent {
		return nil
	}
	justification, err := NewSMTTheoryConflictJustification(s.manager, theory, literals, parameters)
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(justification)
	if err != nil {
		_ = justification.Release(s.manager)
		return err
	}
	s.ownedJustifications = append(s.ownedJustifications, justification)
	s.inconsistent = true
	s.conflict = wrapped
	s.conflictLiteral = SMTNullLiteral
	s.statistics.NumConflicts++
	return nil
}

func (s *SMTInternalizerState) makeClause(literals []SMTLiteral, kind SMTClauseKind, proof ast.ASTHandle) (*SMTClause, SMTJustification, error) {
	return s.makeClauseWithDeleteHandler(literals, kind, proof, nil)
}

func (s *SMTInternalizerState) makeClauseWithDeleteHandler(literals []SMTLiteral, kind SMTClauseKind, proof ast.ASTHandle, deleteHandler SMTClauseDeleteHandler) (*SMTClause, SMTJustification, error) {
	return s.makeClauseWithJustificationAndDeleteHandler(literals, kind, proof, nil, deleteHandler)
}

func (s *SMTInternalizerState) makeClauseWithJustificationAndDeleteHandler(literals []SMTLiteral, kind SMTClauseKind, proof ast.ASTHandle, justification SMTJustification, deleteHandler SMTClauseDeleteHandler) (*SMTClause, SMTJustification, error) {
	proofs, err := s.manager.ProofsEnabled()
	if err != nil {
		return nil, nil, err
	}
	if !proof.IsNull() && justification != nil {
		return nil, nil, ast.ErrASTDomain
	}
	owner := justification
	if !proof.IsNull() {
		owner, err = NewSMTJustificationProofWrapper(s.manager, proof, kind == SMTClauseAux)
		if err != nil {
			return nil, nil, err
		}
	} else if proofs && owner == nil {
		owner = &SMTJustificationBase{heap: kind != SMTClauseAux}
	}
	clause, err := NewSMTClause(s.manager, literals, kind, owner, deleteHandler, false, nil)
	if err != nil {
		if owner != nil {
			err = errors.Join(err, owner.Release(s.manager))
		}
		return nil, nil, err
	}
	for index := 0; index < 2; index++ {
		literal, err := clause.Literal(index)
		if err != nil {
			return nil, nil, errors.Join(err, clause.Close())
		}
		if err = s.watches[literal.Negated().Index()].InsertClause(clause); err != nil {
			return nil, nil, errors.Join(err, clause.Close())
		}
	}
	return clause, owner, nil
}

func (s *SMTInternalizerState) closeClause(record *SMTInternalizedClause) error {
	if record == nil {
		return nil
	}
	if record.Binary {
		if len(record.Literals) != 2 {
			return ast.ErrASTDomain
		}
		first, second := record.Literals[0], record.Literals[1]
		if !s.watches[first.Negated().Index()].RemoveLiteral(second) || !s.watches[second.Negated().Index()].RemoveLiteral(first) {
			return ast.ErrASTDomain
		}
		record.Binary = false
		return nil
	}
	if record.clause == nil {
		return nil
	}
	for index := 0; index < 2; index++ {
		literal, err := record.clause.Literal(index)
		if err != nil {
			return err
		}
		if !s.watches[literal.Negated().Index()].RemoveClause(record.clause) {
			return ast.ErrASTDomain
		}
	}
	var result error
	// Auxiliary clauses do not own their optional proof justification. Lemma
	// clauses do, so their record only keeps a non-owning interface reference.
	if record.owner != nil && !record.clause.IsLemma() {
		result = errors.Join(result, record.owner.Release(s.manager))
	}
	record.owner = nil
	result = errors.Join(result, record.clause.Close())
	record.clause = nil
	return result
}

// Propagate runs source two-watched-literal Boolean constraint propagation.
func (s *SMTInternalizerState) Propagate() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if s.inconsistent {
		return false, nil
	}
	for s.propagationHead < len(s.assignedLiterals) {
		assigned := s.assignedLiterals[s.propagationHead]
		s.propagationHead++
		s.simplifyCounter--
		watch := &s.watches[assigned.Index()]
		for _, implied := range watch.Literals() {
			value, err := s.Assignment(implied)
			if err != nil {
				return false, err
			}
			justification := NewSMTBooleanBinaryClause(assigned)
			switch value {
			case lbool.False:
				s.inconsistent = true
				s.conflict = justification
				s.conflictLiteral = implied.Negated()
				s.statistics.NumConflicts++
				return false, nil
			case lbool.Undef:
				s.statistics.NumPropagations++
				if err = s.AssignLiteral(implied, justification); err != nil {
					return false, err
				}
			}
		}
		clauses := watch.Clauses()
		kept := 0
		for index := 0; index < len(clauses); index++ {
			clause := clauses[index]
			first, err := clause.Literal(0)
			if err != nil {
				return false, err
			}
			second, err := clause.Literal(1)
			if err != nil {
				return false, err
			}
			falseLiteral := assigned.Negated()
			if first == falseLiteral {
				if err = clause.SwapLiterals(0, 1); err != nil {
					return false, err
				}
				first, second = second, first
			}
			if second != falseLiteral {
				return false, ast.ErrASTDomain
			}
			firstValue, err := s.Assignment(first)
			if err != nil {
				return false, err
			}
			if firstValue == lbool.True {
				clauses[kept] = clause
				kept++
				continue
			}
			moved := false
			for candidateIndex := 2; candidateIndex < len(clause.Literals()); candidateIndex++ {
				candidate, err := clause.Literal(candidateIndex)
				if err != nil {
					return false, err
				}
				candidateValue, err := s.Assignment(candidate)
				if err != nil {
					return false, err
				}
				if candidateValue == lbool.False {
					continue
				}
				if err = s.watches[candidate.Negated().Index()].InsertClause(clause); err != nil {
					return false, err
				}
				if err = clause.SetLiteral(1, candidate); err != nil {
					return false, err
				}
				if err = clause.SetLiteral(candidateIndex, falseLiteral); err != nil {
					return false, err
				}
				moved = true
				break
			}
			if moved {
				continue
			}
			clauses[kept] = clause
			kept++
			if firstValue == lbool.False {
				for rest := index + 1; rest < len(clauses); rest++ {
					clauses[kept] = clauses[rest]
					kept++
				}
				if err = watch.SetClauseCount(kept); err != nil {
					return false, err
				}
				s.inconsistent = true
				s.conflict = NewSMTBooleanClause(clause)
				s.conflictLiteral = SMTNullLiteral
				s.statistics.NumConflicts++
				return false, nil
			}
			s.statistics.NumPropagations++
			if err = s.AssignLiteral(first, NewSMTBooleanClause(clause)); err != nil {
				return false, err
			}
		}
		if err := watch.SetClauseCount(kept); err != nil {
			return false, err
		}
	}
	return true, nil
}

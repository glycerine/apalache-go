package smtcore

// Source: src/smt/smt_context.cpp reinit_clauses. Clause objects and their
// justifications survive search backtracking. Only retired bindings and their
// two watches are repaired; relevance belongs to its original scope.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *SMTInternalizerState) keepClauseAcrossPop(record *SMTInternalizedClause) bool {
	return !s.closing && s.pendingPop != nil && s.pendingPop.preserve && record.clause != nil && record.clause.IsLemma()
}

func (s *SMTInternalizerState) reinitializeLearnedClause(saved *smtPreservedLearnedClause) error {
	if !saved.installed {
		if len(saved.sourceAtoms) != 0 {
			if len(saved.sourceAtoms) != len(saved.clause.literals) {
				return ast.ErrASTDomain
			}
			for saved.bindHead < len(saved.sourceAtoms) {
				atom := saved.sourceAtoms[saved.bindHead]
				not, _, err := s.manager.IsNot(atom.expression)
				if err != nil {
					return err
				}
				// Source uses a nongate context for a NOT that owns a Boolean variable.
				if err = s.Internalize(atom.expression, !not); err != nil {
					return err
				}
				variable, found := s.BoolVar(atom.expression)
				if !found {
					return ast.ErrASTDomain
				}
				saved.clause.literals[saved.bindHead] = NewSMTLiteral(variable, atom.sign)
				saved.bindHead++
			}
			literals := saved.clause.literals
			first, err := s.selectWatchLiteral(literals, 0)
			if err != nil {
				return err
			}
			literals[0], literals[first] = literals[first], literals[0]
			second, err := s.selectWatchLiteral(literals, 1)
			if err != nil {
				return err
			}
			literals[1], literals[second] = literals[second], literals[1]
			record := &s.clauses[saved.recordIndex]
			if record.clause != saved.clause || record.closeWatches != 2 {
				return ast.ErrASTDomain
			}
			record.Literals = saved.clause.literals
			for _, literal := range saved.clause.literals[:2] {
				if err = s.watches[literal.Negated().Index()].InsertClause(saved.clause); err != nil {
					return err
				}
			}
			record.closeWatches = 0
			saved.keepReinit = true
		}
		literals := saved.clause.literals
		saved.installed = true
		firstValue, err := s.Assignment(literals[0])
		if err != nil {
			return err
		}
		secondValue, err := s.Assignment(literals[1])
		if err != nil {
			return err
		}
		allOtherFalse := secondValue == lbool.False
		for _, literal := range literals[2:] {
			value, e := s.Assignment(literal)
			if e != nil {
				return e
			}
			allOtherFalse = allOtherFalse && value == lbool.False
		}
		justification := NewSMTBooleanClause(saved.clause)
		if firstValue == lbool.False && allOtherFalse {
			s.setBooleanConflict(justification, SMTNullLiteral)
			saved.keepReinit = true
		} else if allOtherFalse && firstValue == lbool.Undef {
			saved.keepReinit = true
			s.statistics.NumPropagations++
			assigned := len(s.assignedLiterals)
			if err = s.AssignLiteral(literals[0], justification); err != nil {
				saved.pending = s.learnedAssignmentRemainder(literals[0], firstValue, assigned, justification)
				return err
			}
		}
	}
	if err := s.finishLearnedAssignment(&saved.pending); err != nil {
		return err
	}
	if s.scopeLevel <= s.searchBaseLevel || !saved.keepReinit {
		if err := saved.clause.ReleaseAtoms(); err != nil {
			return err
		}
		saved.clause.atoms = nil
		saved.clause.reinit, saved.clause.reinternalizeAtoms = false, false
	}
	return nil
}

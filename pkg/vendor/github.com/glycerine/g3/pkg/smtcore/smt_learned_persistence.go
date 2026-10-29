package smtcore

// Source: src/smt/smt_context.cpp learned-clause reinitialization across
// search backtracking and base-scope lifetime.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtPersistentLearnedUnit struct {
	expression                 ast.ASTHandle
	proof                      ast.ASTHandle
	sign                       bool
	ownerLevel, installedLevel uint32
}

type smtPreservedLearnedClause struct {
	atoms    []smtRetainedLemmaAtom
	proof    ast.ASTHandle
	activity uint32
}

func (s *SMTInternalizerState) beginSearchPersistence(base uint32, params SMTParamsState) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.searching || base != s.scopeLevel {
		return ast.ErrASTDomain
	}
	s.relevanceFIFO.baseBooleanLimit = int32(len(s.boolExpr))
	if s.relevanceGoal.heap != nil {
		s.generation = 0
	}
	s.searching = true
	s.searchBaseLevel = base
	s.searchUserBaseLevel = base
	s.searchAgility = 0
	s.searchAgilityFactor = params.AgilityFactor
	s.searchRestartAdaptive = params.RestartAdaptive
	return nil
}

func (s *SMTInternalizerState) endSearchPersistence() (result error) {
	s.relevanceFIFO.baseBooleanLimit = -1
	s.searching = false
	s.searchBaseLevel = 0
	s.searchUserBaseLevel = 0
	s.searchAgility = 0
	s.searchAgilityFactor = 0
	s.searchRestartAdaptive = false
	kept := 0
	for index := range s.learnedUnits {
		if s.learnedUnits[index].installedLevel == s.learnedUnits[index].ownerLevel {
			result = errors.Join(result, s.releaseLearnedUnit(&s.learnedUnits[index]))
			continue
		}
		if kept != index {
			s.learnedUnits[kept] = s.learnedUnits[index]
		}
		kept++
	}
	s.learnedUnits = s.learnedUnits[:kept]
	return result
}

func (s *SMTInternalizerState) retainProof(proof ast.ASTHandle) (ast.ASTHandle, error) {
	if proof.IsNull() {
		return proof, nil
	}
	if err := s.manager.IncRef(proof); err != nil {
		return ast.ASTHandle{}, err
	}
	return proof, nil
}

func (s *SMTInternalizerState) releaseProof(proof ast.ASTHandle) error {
	if proof.IsNull() {
		return nil
	}
	return s.manager.DecRef(proof)
}

func (s *SMTInternalizerState) learnedClauseProof(clause *SMTClause) (ast.ASTHandle, error) {
	proofs, err := s.manager.ProofsEnabled()
	if err != nil || !proofs {
		return ast.ASTHandle{}, err
	}
	wrapper, ok := clause.Justification().(*SMTJustificationProofWrapper)
	if !ok {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return wrapper.Proof()
}

func (s *SMTInternalizerState) preserveLearnedClauses(limit int) ([]smtPreservedLearnedClause, error) {
	if limit < 0 || limit > len(s.clauses) {
		return nil, ast.ErrASTDomain
	}
	var result []smtPreservedLearnedClause
	for index := limit; index < len(s.clauses); index++ {
		clause := s.clauses[index].clause
		if clause == nil || !clause.IsLearned() {
			continue
		}
		atoms, err := s.retainLemmaAtoms(clause.literals)
		if err != nil {
			_ = s.releasePreservedLearnedClauses(result)
			return nil, err
		}
		proof, err := s.learnedClauseProof(clause)
		if err == nil {
			proof, err = s.retainProof(proof)
		}
		if err != nil {
			_ = s.releaseLemmaAtoms(atoms)
			_ = s.releasePreservedLearnedClauses(result)
			return nil, err
		}
		activity, err := clause.Activity()
		if err != nil {
			_ = s.releaseLemmaAtoms(atoms)
			_ = s.releaseProof(proof)
			_ = s.releasePreservedLearnedClauses(result)
			return nil, err
		}
		result = append(result, smtPreservedLearnedClause{atoms: atoms, proof: proof, activity: activity})
	}
	return result, nil
}

func (s *SMTInternalizerState) releasePreservedLearnedClauses(clauses []smtPreservedLearnedClause) (result error) {
	for index := len(clauses) - 1; index >= 0; index-- {
		result = errors.Join(result, s.releaseProof(clauses[index].proof))
		result = errors.Join(result, s.releaseLemmaAtoms(clauses[index].atoms))
	}
	return result
}

func (s *SMTInternalizerState) restoreLearnedClauses(clauses []smtPreservedLearnedClause) (result error) {
	defer func() { result = errors.Join(result, s.releasePreservedLearnedClauses(clauses)) }()
	for _, saved := range clauses {
		literals, err := s.reinternalizeLemmaAtoms(saved.atoms)
		if err != nil {
			return err
		}
		before := len(s.clauses)
		if err = s.addLearnedClauseWithProof(literals, saved.proof, false); err != nil {
			return err
		}
		if len(s.clauses) > before {
			clause := s.clauses[len(s.clauses)-1].clause
			if clause != nil && clause.IsLearned() {
				if err = clause.SetActivity(saved.activity); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *SMTInternalizerState) captureLearnedUnit(literal SMTLiteral, proof ast.ASTHandle) error {
	expression, err := s.BoolExpression(literal.Var())
	if err != nil {
		return err
	}
	if err = s.manager.IncRef(expression); err != nil {
		return err
	}
	proof, err = s.retainProof(proof)
	if err != nil {
		return errors.Join(err, s.manager.DecRef(expression))
	}
	s.learnedUnits = append(s.learnedUnits, smtPersistentLearnedUnit{
		expression: expression, proof: proof, sign: literal.Sign(), ownerLevel: s.searchBaseLevel, installedLevel: s.scopeLevel,
	})
	return nil
}

func (s *SMTInternalizerState) releaseLearnedUnit(unit *smtPersistentLearnedUnit) (result error) {
	result = errors.Join(result, s.releaseProof(unit.proof))
	if !unit.expression.IsNull() {
		result = errors.Join(result, s.manager.DecRef(unit.expression))
	}
	*unit = smtPersistentLearnedUnit{}
	return result
}

func (s *SMTInternalizerState) restoreLearnedUnits(level uint32) error {
	for index := range s.learnedUnits {
		unit := &s.learnedUnits[index]
		if unit.ownerLevel > level || unit.installedLevel <= level {
			continue
		}
		if err := s.Internalize(unit.expression, true); err != nil {
			return err
		}
		literal, err := s.Literal(unit.expression)
		if err != nil {
			return err
		}
		if unit.sign {
			literal = literal.Negated()
		}
		if err = s.addLearnedClauseWithProof([]SMTLiteral{literal}, unit.proof, false); err != nil {
			return err
		}
		unit.installedLevel = level
	}
	return nil
}

func (s *SMTInternalizerState) discardLearnedUnitsAbove(level uint32) (result error) {
	kept := 0
	for index := range s.learnedUnits {
		if s.learnedUnits[index].ownerLevel > level {
			result = errors.Join(result, s.releaseLearnedUnit(&s.learnedUnits[index]))
			continue
		}
		if kept != index {
			s.learnedUnits[kept] = s.learnedUnits[index]
		}
		kept++
	}
	s.learnedUnits = s.learnedUnits[:kept]
	return result
}

func (s *SMTInternalizerState) releaseLearnedUnits() (result error) {
	for index := len(s.learnedUnits) - 1; index >= 0; index-- {
		result = errors.Join(result, s.releaseLearnedUnit(&s.learnedUnits[index]))
	}
	s.learnedUnits = nil
	return result
}

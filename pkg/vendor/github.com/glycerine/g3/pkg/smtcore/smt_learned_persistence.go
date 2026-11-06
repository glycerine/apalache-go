package smtcore

// Source: src/smt/smt_context.cpp learned-clause reinitialization across
// search backtracking and base-scope lifetime.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtPersistentLearnedUnit struct {
	expression                 ast.ASTHandle
	proof                      ast.ASTHandle
	sign                       bool
	ownerLevel, installedLevel uint32
	discard                    bool
	pending                    smtLearnedAssignment
}

type smtPreservedLearnedClause struct {
	atoms       []smtRetainedLemmaAtom
	proof       ast.ASTHandle
	activity    uint32
	installed   bool
	clause      *SMTClause
	pending     smtLearnedAssignment
	inPlace     bool
	recordIndex int
	sourceAtoms []smtClauseAtom // borrowed from the surviving clause owner
	bindHead    int
	keepReinit  bool
}

// Assignment publication precedes context/relevance notifications. Preserve
// the exact remainder when clause installation fails after that publication.
type smtLearnedAssignment struct {
	literal       SMTLiteral
	justification SMTBooleanJustification
	stage         uint8
}

const (
	smtLearnedAssignmentDone uint8 = iota
	smtLearnedAssignmentContext
	smtLearnedAssignmentMetadata
	smtLearnedAssignmentGoal
	smtLearnedAssignmentEvent
)

func (s *SMTInternalizerState) finishLearnedAssignment(p *smtLearnedAssignment) error {
	if p.stage == smtLearnedAssignmentDone {
		return nil
	}
	literal := p.literal
	expression := s.boolExpr[literal.Var()]
	if p.stage == smtLearnedAssignmentContext {
		if err := s.context.SetAssignment(expression, lbool.FromBool(!literal.Sign())); err != nil {
			return err
		}
		p.stage = smtLearnedAssignmentMetadata
	}
	if p.stage == smtLearnedAssignmentMetadata {
		data := &s.booleanData[literal.Var()]
		data.SetJustification(p.justification)
		data.ScopeLevel = s.scopeLevel
		if s.searching && s.searchRestartAdaptive && data.PhaseAvailable {
			s.searchAgility *= s.searchAgilityFactor
			if data.Phase == literal.Sign() {
				s.searchAgility += 1 - s.searchAgilityFactor
			}
		}
		data.PhaseAvailable, data.Phase = true, !literal.Sign()
		s.assignedLiterals = append(s.assignedLiterals, literal)
		p.stage = smtLearnedAssignmentGoal
	}
	if p.stage == smtLearnedAssignmentGoal {
		if err := s.relevanceGoalAssigned(literal); err != nil {
			return err
		}
		p.stage = smtLearnedAssignmentEvent
	}
	if p.stage == smtLearnedAssignmentEvent {
		// The published literal is already in assignedLiterals. Its relevance
		// callback belongs to the later post-BCP batch, including on retry.
		p.stage = smtLearnedAssignmentDone
	}
	return nil
}

func (s *SMTInternalizerState) learnedAssignmentRemainder(literal SMTLiteral, before lbool.LBool, assigned int, justification SMTBooleanJustification) smtLearnedAssignment {
	if before != lbool.Undef || s.assignment[literal.Index()] != lbool.True {
		return smtLearnedAssignment{}
	}
	stage := smtLearnedAssignmentContext
	if len(s.assignedLiterals) > assigned {
		stage = smtLearnedAssignmentGoal
	}
	return smtLearnedAssignment{literal: literal, justification: justification, stage: stage}
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
	for index := len(s.learnedUnits) - 1; index >= 0; index-- {
		unit := &s.learnedUnits[index]
		if unit.discard || unit.installedLevel == unit.ownerLevel {
			if err := s.releaseLearnedUnit(unit); err != nil {
				return err
			}
			s.removeLearnedUnit(index)
		}
	}
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

func (s *SMTInternalizerState) releaseLearnedRoot(root *ast.ASTHandle) error {
	if root.IsNull() {
		return nil
	}
	consumed, err := s.manager.DecRefConsumed(*root)
	if consumed {
		*root = ast.ASTHandle{}
	}
	if err != nil {
		return err
	}
	if !consumed {
		return ast.ErrASTPending
	}
	return nil
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

// A failed acquisition returns its owned prefix to the pending Pop; it must
// release that prefix before attempting another capture.
func (s *SMTInternalizerState) preserveLearnedClauses(limit int) ([]smtPreservedLearnedClause, error) {
	if limit < 0 || limit > len(s.clauses) {
		return nil, ast.ErrASTDomain
	}
	var result []smtPreservedLearnedClause
	livePop := s.pendingPop != nil && s.pendingPop.preserve && s.pendingPop.stage == 0
	booleanLimit, kept := 0, 0
	if livePop {
		booleanLimit = s.scopes[s.pendingPop.level].booleanLimit + int(SMTFirstBoolVar)
	}
	for index := limit; index < len(s.clauses); index++ {
		clause := s.clauses[index].clause
		if clause == nil || !clause.IsLemma() {
			continue
		}
		recordIndex := limit + kept
		kept++
		if livePop {
			retired := false
			for _, literal := range clause.literals {
				retired = retired || int(literal.Var()) >= booleanLimit
			}
			if !retired && !(clause.InReinitStack() && !clause.ReinternalizeAtoms()) {
				// C++ keeps the object, proof owner, literal storage and watches.
				// There is no snapshot or reconstruction for surviving bindings.
				continue
			}
		}
		result = append(result, smtPreservedLearnedClause{})
		saved := &result[len(result)-1]
		if livePop {
			saved.inPlace, saved.clause, saved.recordIndex = true, clause, recordIndex
			saved.sourceAtoms = clause.atoms
			// Source clause objects already own their atoms and proof. Do not
			// acquire a second snapshot merely to keep the same object alive.
			continue
		}
		var err error
		saved.atoms, err = s.captureLemmaAtoms(clause.literals)
		if err != nil {
			return result, err
		}
		proof, err := s.learnedClauseProof(clause)
		if err == nil {
			saved.proof, err = s.retainProof(proof)
		}
		if err != nil {
			return result, err
		}
		saved.activity, err = clause.Activity()
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *SMTInternalizerState) releasePreservedLearnedClauses(clauses []smtPreservedLearnedClause) (result error) {
	for index := len(clauses) - 1; index >= 0; index-- {
		if err := s.releaseLearnedRoot(&clauses[index].proof); err != nil {
			return err
		}
		if err := s.releaseLemmaAtoms(clauses[index].atoms); err != nil {
			return err
		}
	}
	return result
}

func (s *SMTInternalizerState) restoreLearnedClauses(clauses []smtPreservedLearnedClause) error {
	for index := range clauses {
		saved := &clauses[index]
		if saved.inPlace {
			if err := s.reinitializeLearnedClause(saved); err != nil {
				return err
			}
			continue
		}
		if !saved.installed {
			literals, err := s.reinternalizeLemmaAtoms(saved.atoms)
			if err != nil {
				return err
			}
			before, assigned := len(s.clauses), len(s.assignedLiterals)
			first := literals[0]
			assignment := s.assignment[first.Index()]
			err = s.addLearnedClauseWithProof(literals, saved.proof, false)
			if len(s.clauses) > before {
				saved.installed = true
				saved.clause = s.clauses[len(s.clauses)-1].clause
				if err != nil {
					record := s.clauses[len(s.clauses)-1]
					justification := NewSMTBooleanClause(record.clause)
					if record.Binary {
						justification = NewSMTBooleanBinaryClause(record.Literals[1].Negated())
					}
					saved.pending = s.learnedAssignmentRemainder(first, assignment, assigned, justification)
				}
			}
			if err != nil {
				return err
			}
			saved.installed = true
		}
		if err := s.finishLearnedAssignment(&saved.pending); err != nil {
			return err
		}
		if saved.clause != nil && saved.clause.IsLearned() {
			if err := saved.clause.SetActivity(saved.activity); err != nil {
				return err
			}
		}
	}
	return s.releasePreservedLearnedClauses(clauses)
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
		unit := smtPersistentLearnedUnit{expression: expression, discard: true}
		cleanup := s.releaseLearnedUnit(&unit)
		if cleanup != nil {
			s.learnedUnits = append(s.learnedUnits, unit)
		}
		return errors.Join(err, cleanup)
	}
	s.learnedUnits = append(s.learnedUnits, smtPersistentLearnedUnit{
		expression: expression, proof: proof, sign: literal.Sign(), ownerLevel: s.searchBaseLevel, installedLevel: s.scopeLevel,
	})
	return nil
}

func (s *SMTInternalizerState) releaseLearnedUnit(unit *smtPersistentLearnedUnit) (result error) {
	if err := s.releaseLearnedRoot(&unit.proof); err != nil {
		return err
	}
	if err := s.releaseLearnedRoot(&unit.expression); err != nil {
		return err
	}
	*unit = smtPersistentLearnedUnit{}
	return result
}

func (s *SMTInternalizerState) restoreLearnedUnits(level uint32) error {
	for index := range s.learnedUnits {
		unit := &s.learnedUnits[index]
		if unit.discard {
			continue
		}
		if err := s.finishLearnedAssignment(&unit.pending); err != nil {
			return err
		}
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
		assignment, assigned, owners := s.assignment[literal.Index()], len(s.assignedLiterals), len(s.ownedJustifications)
		if err = s.addLearnedClauseWithProof([]SMTLiteral{literal}, unit.proof, false); err != nil {
			justification := NewSMTBooleanAxiom()
			if len(s.ownedJustifications) > owners {
				var cause error
				justification, cause = NewSMTBooleanJustification(s.ownedJustifications[len(s.ownedJustifications)-1])
				if cause != nil {
					return errors.Join(err, cause)
				}
			}
			unit.pending = s.learnedAssignmentRemainder(literal, assignment, assigned, justification)
			if unit.pending.stage != smtLearnedAssignmentDone {
				unit.installedLevel = level
			}
			return err
		}
		unit.installedLevel = level
	}
	return nil
}

func (s *SMTInternalizerState) discardLearnedUnitsAbove(level uint32) (result error) {
	for index := len(s.learnedUnits) - 1; index >= 0; index-- {
		if s.learnedUnits[index].discard || s.learnedUnits[index].ownerLevel > level {
			if err := s.releaseLearnedUnit(&s.learnedUnits[index]); err != nil {
				return err
			}
			s.removeLearnedUnit(index)
		}
	}
	return result
}

func (s *SMTInternalizerState) releaseLearnedUnits() (result error) {
	for index := len(s.learnedUnits) - 1; index >= 0; index-- {
		if err := s.releaseLearnedUnit(&s.learnedUnits[index]); err != nil {
			return err
		}
		s.learnedUnits = s.learnedUnits[:index]
	}
	s.learnedUnits = nil
	return result
}

func (s *SMTInternalizerState) removeLearnedUnit(index int) {
	copy(s.learnedUnits[index:], s.learnedUnits[index+1:])
	s.learnedUnits[len(s.learnedUnits)-1] = smtPersistentLearnedUnit{}
	s.learnedUnits = s.learnedUnits[:len(s.learnedUnits)-1]
}

func (s *SMTInternalizerState) discardLearnedUnitsFrom(limit int) error {
	if limit < 0 || limit > len(s.learnedUnits) {
		return ast.ErrASTDomain
	}
	for index := limit; index < len(s.learnedUnits); index++ {
		s.learnedUnits[index].discard = true
	}
	for index := len(s.learnedUnits) - 1; index >= limit; index-- {
		if err := s.releaseLearnedUnit(&s.learnedUnits[index]); err != nil {
			return err
		}
		s.learnedUnits = s.learnedUnits[:index]
	}
	return nil
}

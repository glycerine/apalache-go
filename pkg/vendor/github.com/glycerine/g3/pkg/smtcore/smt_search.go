package smtcore

// Source: src/smt/smt_context.cpp decide(), bounded_search(), resolve_conflict(),
// and final_check().

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

type smtSearchProgress struct {
	theory                                uint64
	variables, clauses, assigned          int
	equalities, disequalities             int
	queuedEqualities, processedEqualities int
}

type smtRetainedLemmaAtom struct {
	expression ast.ASTHandle
	sign       bool
	relevant   bool
}

func (s *SMTInternalizerState) retainLemmaAtoms(literals []SMTLiteral) ([]smtRetainedLemmaAtom, error) {
	atoms := make([]smtRetainedLemmaAtom, 0, len(literals))
	for _, literal := range literals {
		expression, err := s.BoolExpression(literal.Var())
		if err != nil {
			_ = s.releaseLemmaAtoms(atoms)
			return nil, err
		}
		if err = s.manager.IncRef(expression); err != nil {
			_ = s.releaseLemmaAtoms(atoms)
			return nil, err
		}
		relevant := false
		if s.relevancy != nil {
			relevant = s.relevancy.IsRelevant(expression)
		}
		atoms = append(atoms, smtRetainedLemmaAtom{expression: expression, sign: literal.Sign(), relevant: relevant})
	}
	return atoms, nil
}

func (s *SMTInternalizerState) releaseLemmaAtoms(atoms []smtRetainedLemmaAtom) (result error) {
	for index := len(atoms) - 1; index >= 0; index-- {
		result = errors.Join(result, s.manager.DecRef(atoms[index].expression))
	}
	return result
}

func (s *SMTInternalizerState) reinternalizeLemmaAtoms(atoms []smtRetainedLemmaAtom) ([]SMTLiteral, error) {
	literals := make([]SMTLiteral, len(atoms))
	for index, atom := range atoms {
		if err := s.Internalize(atom.expression, true); err != nil {
			return nil, err
		}
		literal, err := s.Literal(atom.expression)
		if err != nil {
			return nil, err
		}
		if atom.sign {
			literal = literal.Negated()
		}
		if atom.relevant && s.relevancy != nil && !s.relevancy.IsRelevant(atom.expression) {
			if err = s.relevancy.MarkRelevant(atom.expression); err != nil {
				return nil, err
			}
		}
		literals[index] = literal
	}
	return literals, nil
}

func (s *SMTInternalizerState) searchProgress() smtSearchProgress {
	return smtSearchProgress{
		theory:    s.theoryProgress,
		variables: len(s.boolExpr), clauses: len(s.clauses), assigned: len(s.assignedLiterals),
		equalities: len(s.theoryEqualities), disequalities: len(s.theoryDisequalities),
		queuedEqualities: len(s.pending), processedEqualities: s.pendingHead,
	}
}

func smtDecisionLiteral(variable int32, phase lbool.LBool) SMTLiteral {
	return NewSMTLiteral(variable, phase == lbool.False)
}

func (k *SMTKernel) searchResourceLimitExceeded(timer *stopwatch.Timer, nextProgressSample *uint32) (bool, error) {
	timedOut, err := timer.MSTimeout(k.smtState.Timeout)
	if err != nil {
		return false, err
	}
	if timedOut {
		k.lastFailure = SMTFailureTimeout
		return true, nil
	}
	if k.callback != nil {
		k.callback.FastProgressSample()
		if k.smtState.ProgressSamplingFreq > 0 {
			if nextProgressSample == nil {
				return false, ast.ErrASTHandle
			}
			due, sampleErr := timer.MSTimeout(*nextProgressSample + 1)
			if sampleErr != nil {
				return false, sampleErr
			}
			if due {
				k.callback.SlowProgressSample()
				seconds, sampleErr := timer.GetSeconds()
				if sampleErr != nil {
					return false, sampleErr
				}
				*nextProgressSample = uint32(seconds*1000) + k.smtState.ProgressSamplingFreq
			}
		}
	}
	exceeded, err := k.manager.Canceled()
	if err != nil {
		return false, err
	}
	if exceeded {
		if k.manager.Limit.CancelFlag() {
			k.lastFailure = SMTFailureCanceled
		} else {
			k.lastFailure = SMTFailureResourceLimit
		}
		return true, nil
	}
	above, err := k.aboveMemoryWatermark()
	if err != nil {
		return false, err
	}
	if above {
		k.lastFailure = SMTFailureMemory
		return true, nil
	}
	return false, nil
}

func (k *SMTKernel) learnedBackjumpLevel(resolution SMTConflictResolution, baseLevel uint32) (uint32, error) {
	level := resolution.NewScopeLevel
	if !k.smtState.DelayUnits || len(resolution.Lemma) != 1 || resolution.ConflictLevel <= baseLevel+1 ||
		k.internalizer.quantifiers == nil || k.internalizer.quantifiers.Empty() ||
		uint32(len(k.internalizer.learnedUnits)) >= k.smtState.DelayUnitsThreshold {
		return level, nil
	}
	proofs, err := k.manager.ProofsEnabled()
	if err != nil {
		return 0, err
	}
	if !proofs {
		level = resolution.ConflictLevel - 1
	}
	return level, nil
}

// search runs the retained CDCL control loop. Learned clauses are installed at
// the computed backjump scope and therefore participate in the next propagation
// pass instead of re-exploring the conflicting branch.
func (k *SMTKernel) search() (result lbool.LBool, resultErr error) {
	if err := k.internalizer.InitializeTheorySearch(); err != nil {
		return lbool.Undef, err
	}
	if err := k.manager.ResetRef(k.unsatProof); err != nil {
		return lbool.Undef, err
	}
	queue := k.caseSplitQueue
	if queue == nil {
		if err := k.configureQueueHeuristics(k.smtState, false); err != nil {
			return lbool.Undef, err
		}
		queue = k.caseSplitQueue
	}
	// Source init_search resets only the default phase, not cache counters.
	queue.phaseDefault = false
	baseLevel := k.internalizer.scopeLevel
	conflictBase := k.internalizer.statistics.NumConflicts
	restarts := newSMTRestartState(k.smtState)
	var nextProgressSample uint32
	timer, err := stopwatch.NewTimer(k.timerClock)
	if err != nil {
		return lbool.Undef, err
	}
	resolver, err := NewSMTConflictResolver(k.internalizer)
	if err != nil {
		return lbool.Undef, errors.Join(err, timer.Close())
	}
	if err = k.internalizer.resetBooleanActivityCache(); err != nil {
		return lbool.Undef, errors.Join(err, resolver.Close(), timer.Close())
	}
	if err = k.internalizer.beginSearchPersistence(baseLevel, k.smtState); err != nil {
		return lbool.Undef, errors.Join(err, resolver.Close(), timer.Close())
	}
	if k.assumptions != nil {
		k.internalizer.searchUserBaseLevel--
	}
	defer func() {
		if closeErr := resolver.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("smt search close resolver: %w", closeErr))
		}
		if closeErr := timer.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("smt search close timer: %w", closeErr))
		}
		if count := k.internalizer.scopeLevel - baseLevel; count != 0 {
			if popErr := k.internalizer.PopScopes(count); popErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("smt search restore base scope: %w", popErr))
			}
			if resultErr != nil {
				result = lbool.Undef
			}
		}
		if persistenceErr := k.internalizer.endSearchPersistence(); persistenceErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("smt search end persistence: %w", persistenceErr))
		}
	}()
	for {
		exceeded, err := k.searchResourceLimitExceeded(timer, &nextProgressSample)
		if err != nil {
			return lbool.Undef, fmt.Errorf("smt search resource check: %w", err)
		}
		if exceeded {
			return lbool.Undef, nil
		}
		consistent, err := k.internalizer.PropagateCore()
		if err != nil {
			return lbool.Undef, fmt.Errorf("smt search propagate: %w", err)
		}
		if !consistent {
			if err = queue.PrepareConflict(); err != nil {
				return lbool.Undef, fmt.Errorf("smt search prepare conflict: %w", err)
			}
			conflict, found := k.internalizer.Conflict()
			if !found {
				return lbool.Undef, fmt.Errorf("smt search missing conflict: %w", ast.ErrASTDomain)
			}
			conflictLiteral := k.internalizer.ConflictLiteral()
			userBaseLevel := k.internalizer.searchUserBaseLevel
			resolution, err := resolver.resolve(conflict, conflictLiteral, baseLevel, userBaseLevel)
			if err != nil {
				return lbool.Undef, fmt.Errorf("smt search resolve conflict: %w", err)
			}
			if !resolution.Resolved {
				proof, proofErr := resolver.LemmaProof()
				if proofErr != nil {
					return lbool.Undef, fmt.Errorf("smt search conflict proof: %w", proofErr)
				}
				if err = k.manager.SetRef(k.unsatProof, proof); err != nil {
					return lbool.Undef, fmt.Errorf("smt search retain unsat proof: %w", err)
				}
				if err = k.captureUnsatCore(resolver, conflict, conflictLiteral); err != nil {
					return lbool.Undef, fmt.Errorf("smt search capture unsat core: %w", err)
				}
				return lbool.False, nil
			}
			newScopeLevel, err := k.learnedBackjumpLevel(resolution, baseLevel)
			if err != nil {
				return lbool.Undef, fmt.Errorf("smt search backjump level: %w", err)
			}
			if newScopeLevel < baseLevel || newScopeLevel >= k.internalizer.scopeLevel {
				return lbool.Undef, fmt.Errorf("smt search invalid backjump level: %w", ast.ErrASTDomain)
			}
			proof, proofErr := resolver.LemmaProof()
			if proofErr != nil {
				return lbool.Undef, fmt.Errorf("smt search lemma proof: %w", proofErr)
			}
			retained, err := k.internalizer.retainLemmaAtoms(resolution.Lemma)
			if err != nil {
				return lbool.Undef, fmt.Errorf("smt search retain lemma atoms: %w", err)
			}
			if err = k.internalizer.PopScopes(k.internalizer.scopeLevel - newScopeLevel); err != nil {
				err = errors.Join(err, k.internalizer.releaseLemmaAtoms(retained))
				return lbool.Undef, fmt.Errorf("smt search backjump: %w", err)
			}
			resolution.Lemma, err = k.internalizer.reinternalizeLemmaAtoms(retained)
			err = errors.Join(err, k.internalizer.releaseLemmaAtoms(retained))
			if err != nil {
				return lbool.Undef, fmt.Errorf("smt search reinternalize learned clause: %w", err)
			}
			if err = k.internalizer.AddLearnedClauseWithProof(resolution.Lemma, proof); err != nil {
				return lbool.Undef, fmt.Errorf("smt search learn clause: %w", err)
			}
			if err = k.internalizer.recordConflictForClauseMaintenance(); err != nil {
				return lbool.Undef, fmt.Errorf("smt search collect learned clauses: %w", err)
			}
			k.internalizer.decayBooleanActivity()
			if err = queue.RecordConflict(); err != nil {
				return lbool.Undef, fmt.Errorf("smt phase cache counter: %w", err)
			}
			restarts.conflicts++
			if k.internalizer.statistics.NumConflicts-conflictBase > k.smtState.MaxConflicts {
				k.lastFailure = SMTFailureConflictLimit
				return lbool.Undef, nil
			}
			if restarts.due(k.internalizer.scopeLevel, baseLevel) {
				if err = k.restartSearch(baseLevel, &restarts); err != nil {
					return lbool.Undef, fmt.Errorf("smt search restart: %w", err)
				}
			}
			continue
		}
		exceeded, err = k.searchResourceLimitExceeded(timer, &nextProgressSample)
		if err != nil {
			return lbool.Undef, fmt.Errorf("smt search resource recheck: %w", err)
		}
		if exceeded {
			return lbool.Undef, nil
		}
		// Source simplification is restricted to the user base, below the
		// temporary assumption scope. Discarding reasons there loses cores.
		if k.smtState.SimplifyClauses && k.assumptions == nil {
			if err = k.internalizer.simplifyClausesAtBase(baseLevel); err != nil {
				return lbool.Undef, fmt.Errorf("smt search simplify clauses: %w", err)
			}
		}

		variable, phase, err := queue.NextCaseSplit()
		if err != nil {
			return lbool.Undef, fmt.Errorf("smt search case split: %w", err)
		}
		if variable != SMTNullBoolVar {
			if err = k.internalizer.PushScope(); err != nil {
				return lbool.Undef, fmt.Errorf("smt search push decision scope: %w", err)
			}
			k.internalizer.statistics.NumDecisions++
			if err = k.internalizer.assignDecisionLiteral(smtDecisionLiteral(variable, phase)); err != nil {
				return lbool.Undef, fmt.Errorf("smt search decision: %w", err)
			}
			continue
		}

		before := k.internalizer.searchProgress()
		status, err := k.internalizer.FinalCheckTheories()
		if err != nil {
			return lbool.Undef, fmt.Errorf("smt search final check: %w", err)
		}
		restartForModel := false
		if status == SMTFinalCheckDone && k.internalizer.quantifierManager != nil {
			modelStatus, err := k.internalizer.quantifierManager.CheckModel()
			if err != nil {
				return lbool.Undef, fmt.Errorf("smt search quantifier model check: %w", err)
			}
			switch modelStatus {
			case SMTQuantifierModelSAT:
			case SMTQuantifierModelUnknown:
				k.lastFailure = SMTFailureQuantifiers
				return lbool.Undef, nil
			case SMTQuantifierModelRestart:
				status = SMTFinalCheckContinue
				restartForModel = true
			default:
				return lbool.Undef, ast.ErrASTDomain
			}
		}
		switch status {
		case SMTFinalCheckDone:
			if err = k.captureRelevantLabels(); err != nil {
				return lbool.Undef, fmt.Errorf("smt search capture labels: %w", err)
			}
			if err = k.captureLiteralResults(baseLevel); err != nil {
				return lbool.Undef, fmt.Errorf("smt search capture literal results: %w", err)
			}
			if err = k.buildGeneratedModel(); err != nil {
				return lbool.Undef, fmt.Errorf("smt search build model: %w", err)
			}
			return lbool.True, nil
		case SMTFinalCheckGiveUp:
			k.lastFailure = SMTFailureTheory
			return lbool.Undef, nil
		case SMTFinalCheckContinue:
			if manager := k.internalizer.quantifierManager; restartForModel || manager != nil && len(manager.modelFinder.pendingConstraints) != 0 {
				if err = k.restartSearchAtBase(baseLevel, &restarts, true); err != nil {
					return lbool.Undef, err
				}
				continue
			}
			consistent, err = k.internalizer.PropagateCore()
			if err != nil {
				return lbool.Undef, fmt.Errorf("smt search final propagation: %w", err)
			}
			if !consistent {
				continue
			}
			if k.internalizer.searchProgress() == before {
				available, splitErr := queue.hasCaseSplit()
				if splitErr != nil {
					return lbool.Undef, fmt.Errorf("smt search final case split: %w", splitErr)
				}
				if available {
					continue
				}
				k.lastFailure = SMTFailureUnknown
				k.unknown = "theory made no progress"
				return lbool.Undef, nil
			}
		default:
			return lbool.Undef, ast.ErrASTDomain
		}
	}
}

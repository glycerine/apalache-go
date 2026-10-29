package smtcore

// Source: src/smt/theory_arith_core.h final_check_core and final_check_eh.
// The replay tableau tracks actual assignment changes at stage boundaries.

import (
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTLinearArithmeticTheory) finalCheckIntegerFeasibility() (SMTFinalCheckStatus, error) {
	if t.integer || t.mixed {
		if _, _, fractional := t.fractionalVariable(t.solver); fractional {
			if t.ignoreIntegers {
				return SMTFinalCheckGiveUp, nil
			}
			// Only a replay of logical assertions can justify a conflict;
			// temporary candidate-model bounds are excluded here.
			bounds, status, err := t.buildSimplex()
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if status == lbool.False {
				return SMTFinalCheckContinue, t.derivedBoundConflict(bounds)
			}
			if status == lbool.Undef {
				return SMTFinalCheckGiveUp, nil
			}
			if conflict, err := t.checkArithmeticGCD(bounds); err != nil {
				return SMTFinalCheckGiveUp, err
			} else if conflict {
				return SMTFinalCheckContinue, nil
			}
			if t.euclideanEnabled {
				progress, err := t.applyEuclideanSolver(bounds)
				if err != nil {
					return SMTFinalCheckGiveUp, err
				}
				if t.state.inconsistent {
					return SMTFinalCheckContinue, nil
				}
				if progress {
					status, err = bounds.MakeFeasible()
					if err != nil || status == lbool.Undef {
						return SMTFinalCheckGiveUp, err
					}
					if status == lbool.False {
						return SMTFinalCheckContinue, t.derivedBoundConflict(bounds)
					}
				}
			}
			if err = bounds.RemoveArithmeticFixedBases(); err != nil {
				return SMTFinalCheckGiveUp, err
			}
			integers := t.arithmeticIntegerMask()
			if _, err = bounds.PatchArithmeticIntegerValues(integers); err != nil {
				return SMTFinalCheckGiveUp, err
			}
			bounds.FixArithmeticIntegerNonBases(integers)
			status, err = bounds.MakeFeasible()
			if err != nil || status == lbool.Undef {
				return SMTFinalCheckGiveUp, err
			}
			if status == lbool.False {
				return SMTFinalCheckContinue, t.derivedBoundConflict(bounds)
			}
			t.solver = bounds
			if _, _, fractional := t.fractionalVariable(bounds); !fractional {
				return SMTFinalCheckDone, nil
			}
			column, found := bounds.ArithmeticIntegerBase(integers, &t.nonlinearRandom)
			if !found {
				return SMTFinalCheckGiveUp, nil
			}
			return t.branchOrCutIntegerColumn(column)
		}
	}
	return SMTFinalCheckDone, nil
}

func (t *SMTLinearArithmeticTheory) finalCheckNonlinearArithmetic() (SMTFinalCheckStatus, error) {
	satisfied, err := t.checkNonlinearMonomialAssignments()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	complete, conflict, err := t.checkNonlinearAssertions(satisfied)
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if conflict {
		return SMTFinalCheckContinue, nil
	}
	if (!satisfied || !complete) && t.nonlinearEnabled && len(t.nonlinearColumns) != 0 {
		if t.nonlinearRounds > t.nonlinearRoundLimit {
			return SMTFinalCheckGiveUp, nil
		}
		t.nonlinearRounds++
		if progress, err := t.maxMinNonlinearVariables(); err != nil {
			return SMTFinalCheckGiveUp, err
		} else if progress {
			if t.state.inconsistent {
				return SMTFinalCheckContinue, nil
			}
			if fixed, err := t.propagateFixedSharedValues(); err != nil {
				return SMTFinalCheckGiveUp, err
			} else if fixed {
				return SMTFinalCheckContinue, nil
			}
			satisfied, err = t.checkNonlinearMonomialAssignments()
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			complete, conflict, err = t.checkNonlinearAssertions(satisfied)
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if conflict {
				return SMTFinalCheckContinue, nil
			}
			if satisfied && complete {
				return SMTFinalCheckDone, nil
			}
		}
		// Complete a candidate in a separate replay. Its temporary point
		// bounds must not become logical assertions if completion fails.
		if !satisfied {
			satisfied, err = t.completeNonlinearCandidate()
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
		}
		if satisfied {
			complete, conflict, err = t.checkNonlinearAssertions(true)
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if conflict {
				return SMTFinalCheckContinue, nil
			}
		}
	}
	if !satisfied || !complete {
		if t.nonlinearEnabled && len(t.nonlinearColumns) != 0 {
			progress, err := t.runNonlinearStrategies()
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if progress {
				return SMTFinalCheckContinue, nil
			}
		}
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
}

func (t *SMTLinearArithmeticTheory) finalCheckArithmeticCore() (SMTFinalCheckStatus, error) {
	start := t.finalCheckIndex
	result := SMTFinalCheckDone
	for {
		if !t.state.manager.Limit.Inc() {
			return SMTFinalCheckGiveUp, nil
		}
		before := make([]simplex.Infinitesimal, len(t.simplexVariables))
		for i, column := range t.simplexVariables {
			before[i] = t.solver.Value(column)
		}
		stage := t.finalCheckIndex
		var status SMTFinalCheckStatus
		var err error
		switch stage {
		case 0:
			status, err = t.finalCheckIntegerFeasibility()
		case 1:
			var progress bool
			progress, err = t.assumeSharedEqualities()
			status = SMTFinalCheckDone
			if progress {
				status = SMTFinalCheckContinue
			}
		default:
			status, err = t.finalCheckNonlinearArithmetic()
		}
		t.finalCheckIndex = (t.finalCheckIndex + 1) % 3
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		changed := false
		for i, value := range before {
			current := t.solver.Value(t.simplexVariables[i])
			if value.Cmp(&current) != 0 {
				changed = true
				break
			}
		}
		if changed {
			t.changedAssignment = true
			t.state.theoryProgress++
		}
		if status == SMTFinalCheckDone && stage != 1 && !t.liberalFinalCheck && t.changedAssignment {
			status = SMTFinalCheckContinue
		}
		if status == SMTFinalCheckContinue {
			return status, nil
		}
		if status == SMTFinalCheckGiveUp {
			result = status
		}
		if t.finalCheckIndex == start {
			// Native reports unsupported operations only after the full round;
			// feasibility and shared equalities may still make progress first.
			if result == SMTFinalCheckDone && t.unsupported {
				return SMTFinalCheckGiveUp, nil
			}
			return result, nil
		}
	}
}

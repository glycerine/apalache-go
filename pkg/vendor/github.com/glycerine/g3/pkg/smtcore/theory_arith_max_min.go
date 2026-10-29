package smtcore

// Source: src/smt/theory_arith_nl.h max_min_nl_vars and
// src/smt/theory_arith_aux.h max_min(variable), max_min(vars).

import (
	"math/big"

	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTLinearArithmeticTheory) normalizeOptimizedBound(column uint32, lower bool, bound simplex.Infinitesimal) simplex.Infinitesimal {
	if !t.integer && !t.integerColumns[column] {
		return bound
	}
	integer := floorSMTRational(&bound.Main)
	if lower && (!bound.Main.IsInt() || bound.Epsilon.Sign() > 0) {
		integer.Add(integer, big.NewInt(1))
	} else if !lower && bound.Main.IsInt() && bound.Epsilon.Sign() < 0 {
		integer.Sub(integer, big.NewInt(1))
	}
	bound.Infinity.SetInt64(0)
	bound.Main.SetInt(integer)
	bound.Epsilon.SetInt64(0)
	return bound
}

func (t *SMTLinearArithmeticTheory) maxMinNonlinearVariables() (bool, error) {
	solver, status, err := t.buildSimplex()
	if err != nil || status == lbool.Undef {
		return false, err
	}
	if status == lbool.False {
		return true, t.derivedBoundConflict(solver)
	}
	integers := make(map[uint32]bool, len(t.integerColumns)+len(t.atoms)+1)
	shared := make(map[uint32]bool)
	integers[0] = true
	for variable, column := range t.simplexVariables {
		integers[column] = t.integer || t.integerColumns[column]
		node, err := t.Enode(SMTTheoryVar(variable))
		if err != nil {
			return false, err
		}
		shared[column], err = t.sharedArithmeticNode(node)
		if err != nil {
			return false, err
		}
	}
	for i := range t.atoms {
		if t.atoms[i].supported {
			integers[t.nextSimplex+uint32(i)] = t.integralAtom(&t.atoms[i])
		}
	}
	solver.MoveArithmeticNonBasesToBounds(integers)
	status, err = solver.MakeFeasible()
	if err != nil || status == lbool.Undef {
		return false, err
	}
	if status == lbool.False {
		return true, t.derivedBoundConflict(solver)
	}
	var columns []uint32
	found := make(map[uint32]bool)
	mark := func(column uint32) {
		if !found[column] {
			found[column] = true
			columns = append(columns, column)
		}
	}
	for _, column := range t.nonlinearColumns {
		mark(column)
		for _, factor := range t.nonlinearFactors[column] {
			mark(factor)
		}
	}
	propagateNow := false
	for _, column := range columns {
		for _, maximize := range []bool{true, false} {
			value := solver.Value(column)
			bound, hasBound := solver.Lower(column)
			if maximize {
				bound, hasBound = solver.Upper(column)
			}
			if hasBound && value.Cmp(&bound) == 0 {
				continue
			}
			t.statistics.maxMin++
			result, err := solver.ArithmeticMaxMin(column, maximize, simplex.ArithmeticOptConfig{IntegerColumns: integers, SharedColumns: shared, MaxEfforts: 10 + t.state.random.Bounded(20), Limit: &t.state.manager.Limit})
			if err != nil {
				return false, err
			}
			if result.Status == simplex.ArithmeticOptimized {
				value := t.normalizeOptimizedBound(column, !maximize, result.Value.Clone())
				explanation := smtArithmeticWeightedBoundExplanation(solver, result.Bounds)
				t.queueOptimizationBound(solver, smtArithmeticReplayBound{column: column, lower: !maximize, value: value, explanation: explanation})
				propagateNow = propagateNow || !result.HasShared
			}
		}
	}
	progress := false
	var pending []smtArithmeticReplayBound
	if propagateNow {
		pending = t.drainOptimizationBounds()
	}
	for _, bound := range pending {
		changed, err := smtTightenReplaySimplexBound(solver, bound.column, bound.lower, bound.value, bound.explanation)
		if err != nil {
			t.recordArithmeticBoundConflict(solver, bound.column, bound.lower, bound.explanation)
			return true, t.derivedBoundConflict(solver)
		}
		if changed {
			progress = true
		}
	}
	status, err = solver.MakeFeasible()
	if err != nil || status == lbool.Undef {
		return false, err
	}
	if status == lbool.False {
		return true, t.derivedBoundConflict(solver)
	}
	if progress {
		t.solver = solver
		t.state.theoryProgress++
		return true, nil
	}
	return t.publishOptimizedArithmeticAssignment(solver)
}

func (t *SMTLinearArithmeticTheory) publishOptimizedArithmeticAssignment(solver *simplex.Solver) (bool, error) {
	// Source final_check_eh rechecks interface equalities after assignment
	// movement. Shared columns may move; accepting a candidate still requires
	// integrality and exact product checks before that second final-check pass.
	changed := false
	for _, column := range t.simplexVariables {
		old, value := t.solver.Value(column), solver.Value(column)
		if old.Cmp(&value) != 0 {
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	if t.integer || t.mixed {
		if _, _, fractional := t.fractionalVariable(solver); fractional {
			return false, nil
		}
	}
	old := t.solver
	t.solver = solver
	valid, err := t.checkNonlinearMonomialAssignments()
	if err != nil || !valid {
		t.solver = old
		return false, err
	}
	complete, conflict, err := t.checkNonlinearAssertions(true)
	if err != nil || !complete {
		t.solver = old
		return false, err
	}
	if conflict {
		return true, nil
	}
	t.state.theoryProgress++
	return true, nil
}

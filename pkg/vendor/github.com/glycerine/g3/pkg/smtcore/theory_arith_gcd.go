package smtcore

// Source: src/smt/theory_arith_int.h gcd_test and ext_gcd_test. Eager and
// adaptive modes are excluded: their source flags have no retained setter.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/simplex"
	"sort"
	"strconv"
	"strings"
)

// Constants become bounds, so comparisons with the same linear
// coefficients share a source expression column even when RHSs differ.
func smtArithmeticCoefficientKey(form *smtLinearForm) string {
	columns := make([]uint32, 0, len(form.terms))
	for column := range form.terms {
		columns = append(columns, column)
	}
	sort.Slice(columns, func(i, j int) bool { return columns[i] < columns[j] })
	var key strings.Builder
	for _, column := range columns {
		key.WriteString(strconv.FormatUint(uint64(column), 10))
		key.WriteByte(':')
		key.WriteString(form.terms[column].RatString())
		key.WriteByte(';')
	}
	return key.String()
}

func (t *SMTLinearArithmeticTheory) arithmeticIntegerMask() map[uint32]bool {
	integers := make(map[uint32]bool, len(t.simplexVariables)+len(t.atoms)+1)
	integers[0] = true
	for _, column := range t.simplexVariables {
		integers[column] = t.integer || t.integerColumns[column]
	}
	for i := range t.atoms {
		if t.atoms[i].supported {
			integers[t.nextSimplex+uint32(i)] = t.integralAtom(&t.atoms[i])
		}
	}
	return integers
}

// A failed deferred check uses a replay of current logical assertions;
// temporary candidate model values never justify a conflict.
func (t *SMTLinearArithmeticTheory) checkArithmeticGCD(solver *simplex.Solver) (bool, error) {
	if !t.gcdEnabled {
		return false, nil
	}
	result, checks, bounds, err := solver.ArithmeticGCDWithExplanation(t.arithmeticIntegerMask())
	t.statistics.gcdTests += checks
	if err != nil || result == simplex.ArithmeticGCDPass {
		return false, err
	}
	t.statistics.gcdConflicts++
	return true, t.arithmeticGCDConflict(solver, bounds)
}

func (t *SMTLinearArithmeticTheory) arithmeticGCDConflict(solver *simplex.Solver, bounds []simplex.ArithmeticBoundUse) (result error) {
	m := t.state.manager
	proofs, err := m.ProofsEnabled()
	if err != nil {
		return err
	}
	antecedents, explained, err := smtArithmeticExplainBounds(solver, bounds, proofs, t.equalityBounds)
	if err != nil {
		return err
	}
	// Derived bounds without dependency chains still require the conservative
	// assertion set. Do not attach invented numerical weights to that fallback.
	if !explained {
		antecedents, err = t.allArithmeticAntecedents()
		if err != nil {
			return err
		}
	}
	parameters, err := antecedents.parameters(m, "gcd-test")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(m, parameters)) }()
	t.recordArithmeticConflict()
	return t.setArithmeticConflict(&antecedents, parameters...)
}

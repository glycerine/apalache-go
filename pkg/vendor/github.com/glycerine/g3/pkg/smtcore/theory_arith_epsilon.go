package smtcore

// Source: src/smt/theory_arith_core.h refine_epsilon. Preserve distinctions
// between shared infinitesimal assignments when constructing rational models.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/math/simplex"
)

func (t *SMTLinearArithmeticTheory) refineModelDelta() error {
	var shared []uint32
	for variable, column := range t.simplexVariables {
		if t.integer || t.integerColumns[column] {
			continue
		}
		node, err := t.Enode(SMTTheoryVar(variable))
		if err != nil {
			return err
		}
		isShared, err := t.sharedArithmeticNode(node)
		if err != nil {
			return err
		}
		if isShared {
			shared = append(shared, column)
		}
	}
	for {
		values := make(map[string]simplex.Infinitesimal, len(shared))
		refine := false
		for _, column := range shared {
			inf := t.solver.Value(column)
			if inf.Infinity.Sign() != 0 {
				continue
			}
			value := new(big.Rat).Mul(&inf.Epsilon, &t.delta)
			value.Add(value, &inf.Main)
			key := value.RatString()
			if previous, found := values[key]; found {
				if previous.Cmp(&inf) != 0 {
					refine = true
					break
				}
			} else {
				values[key] = inf
			}
		}
		if !refine {
			return nil
		}
		t.delta.Quo(&t.delta, big.NewRat(2, 1))
	}
}

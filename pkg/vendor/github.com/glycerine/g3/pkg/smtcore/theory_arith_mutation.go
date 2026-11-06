package smtcore

// Source: src/smt/theory_arith_aux.h mutate_assignment. Freedom intervals
// preserve the current tableau constraints; changed models are revalidated
// by the integer, shared-equality and nonlinear final-check stages.

import "github.com/glycerine/g3/pkg/math/simplex"

func smtArithmeticColumnFixed(solver *simplex.Solver, column uint32) bool {
	lower, hasLower := solver.Lower(column)
	upper, hasUpper := solver.Upper(column)
	return hasLower && hasUpper && lower.Cmp(&upper) == 0
}

func (t *SMTLinearArithmeticTheory) mutateArithmeticAssignment() error {
	if err := t.solver.RemoveArithmeticFixedBases(); err != nil {
		return err
	}
	values := make(map[smtArithmeticSharedValueKey]SMTTheoryVar)
	marked := make(map[SMTTheoryVar]bool)
	var candidates []SMTTheoryVar
	for variable, column := range t.simplexVariables {
		node, err := t.Enode(SMTTheoryVar(variable))
		if err != nil {
			return err
		}
		if t.state.relevancy != nil && !t.state.relevancy.IsRelevant(node.owner) {
			continue
		}
		shared, err := t.sharedArithmeticNode(node)
		if err != nil {
			return err
		}
		if !shared {
			continue
		}
		sort, err := t.state.manager.GetSort(node.owner)
		if err != nil {
			return err
		}
		value := t.solver.Value(column)
		key := smtArithmeticSharedValueKey{sort, value.Infinity.RatString(), value.Main.RatString(), value.Epsilon.RatString()}
		other, found := values[key]
		if !found {
			values[key] = SMTTheoryVar(variable)
			continue
		}
		otherNode, err := t.Enode(other)
		if err != nil {
			return err
		}
		if node.root == otherNode.root {
			continue
		}
		if !smtArithmeticColumnFixed(t.solver, column) {
			candidates = append(candidates, SMTTheoryVar(variable))
		} else if !smtArithmeticColumnFixed(t.solver, t.simplexVariables[other]) && !marked[other] {
			marked[other] = true
			candidates = append(candidates, other)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
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
	for _, variable := range candidates {
		column := t.simplexVariables[variable]
		if row, basic := t.solver.BasicRow(column); basic {
			entries, err := t.solver.RowEntries(row)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.Variable == column || smtArithmeticColumnFixed(t.solver, entry.Variable) {
					continue
				}
				updated, err := t.solver.ArithmeticRandomUpdate(entry.Variable, integers, &t.nonlinearRandom)
				if err != nil {
					return err
				}
				if updated {
					break
				}
			}
		} else if _, err := t.solver.ArithmeticRandomUpdate(column, integers, &t.nonlinearRandom); err != nil {
			return err
		}
	}
	return nil
}

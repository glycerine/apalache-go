package smtcore

// Source: src/smt/theory_arith_aux.h assume_eqs_core and delayed_assume_eqs.

import "github.com/glycerine/g3/pkg/ast"

type smtArithmeticEqualityCandidate struct {
	left, right SMTTheoryVar
}

type smtArithmeticSharedValueKey struct {
	sort                    ast.ASTHandle
	infinity, main, epsilon string
}

func (t *SMTLinearArithmeticTheory) delayedAssumeEqualities() (bool, error) {
	for t.assumeEqualityHead < len(t.assumeEqualityCandidates) {
		pair := t.assumeEqualityCandidates[t.assumeEqualityHead]
		t.assumeEqualityHead++
		leftColumn, rightColumn := t.simplexVariables[pair.left], t.simplexVariables[pair.right]
		leftValue, rightValue := t.solver.Value(leftColumn), t.solver.Value(rightColumn)
		if leftValue.Cmp(&rightValue) != 0 {
			continue
		}
		left, err := t.Enode(pair.left)
		if err != nil {
			return false, err
		}
		right, err := t.Enode(pair.right)
		if err != nil {
			return false, err
		}
		if progress, err := t.state.assumeEquality(left, right); err != nil || progress {
			return progress, err
		}
	}
	return false, nil
}

func (t *SMTLinearArithmeticTheory) assumeSharedEqualities() (bool, error) {
	if progress, err := t.delayedAssumeEqualities(); err != nil || progress {
		return progress, err
	}
	if t.liberalFinalCheck {
		if err := t.mutateArithmeticAssignment(); err != nil {
			return false, err
		}
	}
	values := make(map[smtArithmeticSharedValueKey]SMTTheoryVar)
	for variable, column := range t.simplexVariables {
		node, err := t.Enode(SMTTheoryVar(variable))
		if err != nil {
			return false, err
		}
		if t.state.relevancy != nil && !t.state.relevancy.IsRelevant(node.owner) {
			continue
		}
		shared, err := t.sharedArithmeticNode(node)
		if err != nil {
			return false, err
		}
		if !shared {
			continue
		}
		sort, err := t.state.manager.GetSort(node.owner)
		if err != nil {
			return false, err
		}
		value := t.solver.Value(column)
		k := smtArithmeticSharedValueKey{sort, value.Infinity.RatString(), value.Main.RatString(), value.Epsilon.RatString()}
		other, found := values[k]
		if !found {
			values[k] = SMTTheoryVar(variable)
			continue
		}
		otherNode, err := t.Enode(other)
		if err != nil {
			return false, err
		}
		if otherNode.root != node.root {
			t.assumeEqualityCandidates = append(t.assumeEqualityCandidates, smtArithmeticEqualityCandidate{other, SMTTheoryVar(variable)})
		}
	}
	return t.delayedAssumeEqualities()
}

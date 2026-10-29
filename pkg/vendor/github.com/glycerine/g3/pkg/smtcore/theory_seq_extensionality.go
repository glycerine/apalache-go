package smtcore

// Source: src/smt/theory_seq.cpp check_extensionality.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (t *SMTSequenceTheory) checkSequenceExtensionality() (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	var earlier []*SMTEnode
	for _, node := range t.variables {
		if node == nil || node.Root() != node {
			continue
		}
		sort, err := state.manager.GetSort(node.owner)
		if err != nil {
			return false, err
		}
		_, sequence, err := state.manager.SequenceSortElement(sort)
		if err != nil {
			return false, err
		}
		if !sequence {
			continue
		}
		if len(earlier) != 0 && state.isRelevant(node.owner) {
			shared, _, err := state.sharedTheoryNode(node)
			if err != nil {
				return false, err
			}
			// With one sequence-theory variable the remaining owner-specific
			// is_shared hook is the source base hook, which returns false.
			if shared {
				progress, err := t.trySequenceInterfaceRoot(node, sort, earlier)
				if err != nil || progress {
					return progress, err
				}
			}
		}
		// Source earlier roots remain candidates even when they are unmarked
		// or unshared. Relevance/sharing filter only the current root.
		earlier = append(earlier, node)
	}
	return false, nil
}

func (t *SMTSequenceTheory) trySequenceInterfaceRoot(node *SMTEnode, sort ast.ASTHandle, earlier []*SMTEnode) (progress bool, result error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	parts, _, err := t.expandWordExpression(node.owner, nil, make(map[ast.ASTHandle]struct{}))
	if err != nil {
		return false, err
	}
	canonical, err := t.makeWord(parts, sort)
	if err != nil {
		return false, err
	}
	ref, err := state.manager.NewRef(canonical)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, state.manager.DelRef(&ref)) }()
	for _, previous := range earlier {
		previousSort, err := state.manager.GetSort(previous.owner)
		if err != nil {
			return false, err
		}
		if previousSort != sort || state.context.isDisequal(node, previous) || t.exclusions.Contains(node.owner, previous.owner) {
			continue
		}
		candidate, impossible, err := t.sequenceInterfaceCandidate(canonical, previous.owner, sort)
		if err != nil {
			return false, err
		}
		if impossible {
			if err = t.exclusions.Update(node.owner, previous.owner); err != nil {
				return false, err
			}
			continue
		}
		if !candidate {
			continue
		}
		// Source sequence theory inherits the base mk_eq_atom order by AST ID.
		left, right := node, previous
		leftID, err := left.OwnerID()
		if err != nil {
			return false, err
		}
		rightID, err := right.OwnerID()
		if err != nil {
			return false, err
		}
		if leftID > rightID {
			left, right = right, left
		}
		return state.assumeEquality(left, right)
	}
	return false, nil
}

func (t *SMTSequenceTheory) sequenceInterfaceCandidate(canonical, previous, sort ast.ASTHandle) (candidate, impossible bool, result error) {
	state, err := t.State()
	if err != nil {
		return false, false, err
	}
	parts, _, err := t.expandWordExpression(previous, nil, make(map[ast.ASTHandle]struct{}))
	if err != nil {
		return false, false, err
	}
	word, err := t.makeWord(parts, sort)
	if err != nil {
		return false, false, err
	}
	rw, err := rewriter.NewSequenceRewriter(state.manager)
	if err != nil {
		return false, false, err
	}
	reduction, err := rw.ReduceEquality(canonical, word)
	if err != nil {
		return false, false, err
	}
	defer func() { result = errors.Join(result, reduction.Close()) }()
	if reduction.Inconsistent {
		return false, true, nil
	}
	for i, left := range reduction.EqualitiesLeft {
		if t.exclusions.Contains(left, reduction.EqualitiesRight[i]) {
			return false, false, nil
		}
	}
	if len(reduction.Left) != 0 || len(reduction.Right) != 0 {
		left, err := t.makeWord(reduction.Left, sort)
		if err != nil {
			return false, false, err
		}
		leftRef, err := state.manager.NewRef(left)
		if err != nil {
			return false, false, err
		}
		defer func() { result = errors.Join(result, state.manager.DelRef(&leftRef)) }()
		right, err := t.makeWord(reduction.Right, sort)
		if err != nil {
			return false, false, err
		}
		rightRef, err := state.manager.NewRef(right)
		if err != nil {
			return false, false, err
		}
		defer func() { result = errors.Join(result, state.manager.DelRef(&rightRef)) }()
		if t.exclusions.Contains(left, right) {
			return false, false, nil
		}
	}
	return true, false, nil
}

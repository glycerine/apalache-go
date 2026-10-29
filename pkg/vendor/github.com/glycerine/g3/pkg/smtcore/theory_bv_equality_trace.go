package smtcore

// Source: src/smt/theory_bv.cpp:227,1106-1109,1164,1181,1384-1397.

import (
	"fmt"
	"io"
)

func (t *SMTBitVectorTheory) traceBVStaticDisequality(left, right SMTTheoryVar) error {
	return t.emitBVTrace("bv_diseq_axiom", "mk_new_diseq_axiom", 227, func(out io.Writer) error {
		if _, err := io.WriteString(out, "found new diseq axiom\n"); err != nil {
			return err
		}
		if err := t.DisplayVariable(out, left); err != nil {
			return err
		}
		return t.DisplayVariable(out, right)
	})
}

func (t *SMTBitVectorTheory) traceBVNewEquality(left, right SMTTheoryVar) error {
	if err := t.emitBVTrace("bv_eq", "new_eq_eh", 1106, func(out io.Writer) error {
		leftNode, err := t.Enode(left)
		if err != nil {
			return err
		}
		rightNode, err := t.Enode(right)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(out, "new_eq: "); err != nil {
			return err
		}
		if err = t.displayBVTraceExpression(out, leftNode.owner); err != nil {
			return err
		}
		if _, err = io.WriteString(out, " = "); err != nil {
			return err
		}
		if err = t.displayBVTraceExpression(out, rightNode.owner); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	return t.emitBVTrace("bv", "new_eq_eh", 1107, func(out io.Writer) error {
		state, err := t.State()
		if err != nil {
			return err
		}
		leftNode, err := t.Enode(left)
		if err != nil {
			return err
		}
		rightNode, err := t.Enode(right)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "new_eq_eh v%d = v%d relevant1: %t relevant2: %t\n", left, right, state.context.isRelevant(leftNode.owner), state.context.isRelevant(rightNode.owner))
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVAssignment(variable int32, isTrue bool) error {
	return t.emitBVTrace("bv", "assign_eh", 1164, func(out io.Writer) error {
		state, err := t.State()
		if err != nil {
			return err
		}
		expression, err := state.BoolExpression(variable)
		if err != nil {
			return err
		}
		id, err := state.manager.GetASTID(expression)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "assert: v%d #%d is_true: %t\n", variable, id, isTrue)
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVPropagationQueueSize(size int) error {
	return t.emitBVTrace("bv", "assign_eh", 1181, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "%d\n", size)
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVMerge(first, second SMTTheoryVar, tag string) error {
	line := 1384
	if tag == "bv_bit_prop" {
		line = 1385
	}
	return t.emitBVTrace(tag, "merge_eh", line, func(out io.Writer) error {
		firstNode, err := t.Enode(first)
		if err != nil {
			return err
		}
		secondNode, err := t.Enode(second)
		if err != nil {
			return err
		}
		firstID, err := firstNode.OwnerID()
		if err != nil {
			return err
		}
		secondID, err := secondNode.OwnerID()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "merging: #%d #%d\n", firstID, secondID)
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVMergeConflict() error {
	return t.emitBVTrace("bv", "merge_eh", 1387, func(out io.Writer) error {
		_, err := io.WriteString(out, "conflict detected\n")
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVMergeSize(size int) error {
	return t.emitBVTrace("bv", "merge_eh", 1397, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "bits size: %d\n", size)
		return err
	})
}

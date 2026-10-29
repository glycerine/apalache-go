package smtcore

// Source: src/smt/theory_bv.cpp:1203-1245,1269-1272,1424-1427.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTBitVectorTheory) traceBVPropagating(variable SMTTheoryVar, index uint32, value lbool.LBool, line int) error {
	return t.emitBVTrace("bv_bit_prop", "propagate_bits", line, func(out io.Writer) error {
		node, err := t.Enode(variable)
		if err != nil {
			return err
		}
		id, err := node.OwnerID()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "propagating #%d[%d] = %s\n", id, index, value.String())
		return err
	})
}

func writeBVSourceLiteral(out io.Writer, literal SMTLiteral) error {
	if literal.Sign() {
		if _, err := io.WriteString(out, "-"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(out, "%d", literal.Var())
	return err
}

func (t *SMTBitVectorTheory) traceBVPropagationConflict(first, second SMTLiteral) error {
	return t.emitBVTrace("bv_bit_prop", "propagate_bits", 1223, func(out io.Writer) error {
		if _, err := io.WriteString(out, "inconsistent "); err != nil {
			return err
		}
		if err := writeBVSourceLiteral(out, first); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
		if err := writeBVSourceLiteral(out, second); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVPropagationDone() error {
	return t.emitBVTrace("bv_bit_prop", "propagate_bits", 1232, func(out io.Writer) error {
		_, err := io.WriteString(out, "done propagating\n")
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVAssignBit(consequent, antecedent SMTLiteral, from, to SMTTheoryVar, index uint32, propagateClass bool) error {
	return t.emitBVTrace("bv_bit_prop", "assign_bit", 1242, func(out io.Writer) error {
		state, err := t.State()
		if err != nil {
			return err
		}
		first, err := t.Enode(from)
		if err != nil {
			return err
		}
		second, err := t.Enode(to)
		if err != nil {
			return err
		}
		firstID, err := first.OwnerID()
		if err != nil {
			return err
		}
		secondID, err := second.OwnerID()
		if err != nil {
			return err
		}
		if _, err = io.WriteString(out, "assigning: "); err != nil {
			return err
		}
		if err = consequent.DisplayCompact(out, state.manager, state.boolExpr); err != nil {
			return err
		}
		if _, err = io.WriteString(out, " using "); err != nil {
			return err
		}
		if err = antecedent.DisplayCompact(out, state.manager, state.boolExpr); err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, " #%d #%d idx: %d\npropagate_eqc: %t\n", firstID, secondID, index, propagateClass); err != nil {
			return err
		}
		return nil
	})
}

func (t *SMTBitVectorTheory) traceBVAssignBitOccurrence(entry smtBVBitOccurrence, to SMTTheoryVar, index uint32) error {
	return t.emitBVTrace("assign_bit_bug", "assign_bit", 1269, func(out io.Writer) error {
		entryRoot, err := t.privateRoot(entry.variable)
		if err != nil {
			return err
		}
		targetRoot, err := t.privateRoot(to)
		if err != nil {
			return err
		}
		node, err := t.Enode(entry.variable)
		if err != nil {
			return err
		}
		id, err := node.OwnerID()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "curr->m_var: v%d, curr->m_idx: %d, v2: v%d, idx: %d\nfind(curr->m_var): v%d, find(v2): v%d\nis bit of #%d\n", entry.variable, entry.index, to, index, entryRoot, targetRoot, id)
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVMergeInconsistent(first, second SMTTheoryVar, index int) error {
	return t.emitBVTrace("bv", "merge_eh", 1424, func(out io.Writer) error {
		if _, err := io.WriteString(out, "inconsistent "); err != nil {
			return err
		}
		if err := t.DisplayVariable(out, first); err != nil {
			return err
		}
		if err := t.DisplayVariable(out, second); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "idx: %d\n", index)
		return err
	})
}

func (t *SMTBitVectorTheory) traceBVMergeComplement(first, second SMTTheoryVar, index int, bit1, bit2 SMTLiteral) error {
	if !diagnostic.DebugBuild || bit1 != bit2.Negated() {
		return nil
	}
	return t.emitBVTrace("bv_bug", "merge_eh", 1416, func(out io.Writer) error {
		if err := t.DisplayVariable(out, first); err != nil {
			return err
		}
		if err := t.DisplayVariable(out, second); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "idx: %d\n", index)
		return err
	})
}

package smtcore

// Source: src/smt/theory_bv.cpp:1334-1339,1670-1764;
// src/smt/theory_bv.h:256 after_merge_eh.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// These are the retained BV SASSERT predicates. Keep their operand evaluation
// behind the actual source assertion switch, including find(owner) and indexing.
func bvAssert(line int32, condition string, evaluate func() (bool, error)) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	holds, err := evaluate()
	if err != nil || holds {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/smt/theory_bv.cpp", line, condition)
}

func (t *SMTBitVectorTheory) checkBVInvariant() error {
	if !diagnostic.DebugBuild {
		return nil
	}
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	for v := SMTTheoryVar(0); int(v) < len(t.variables); v++ {
		if err = t.checkBVAssignment(v); err != nil {
			return err
		}
		if err = t.checkBVZeroOneBits(v); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTBitVectorTheory) checkBVAfterMerge(root SMTTheoryVar) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	return t.checkBVZeroOneBits(root)
}

func (t *SMTBitVectorTheory) checkBVAssignment(v SMTTheoryVar) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	s, err := t.State()
	if err != nil {
		return err
	}
	if v < 0 || int(v) >= len(t.privateFind) {
		return ast.ErrASTDomain
	}
	if t.privateFind[v] != v {
		return nil
	}
	node, err := t.Enode(v)
	if err != nil {
		return err
	}
	if !s.context.isRelevant(node.owner) {
		return nil
	}
	bits2 := t.bitLiterals[v]
	for v1 := v; ; v1 = t.privateNext[v1] {
		if v1 < 0 || int(v1) >= len(t.bitLiterals) {
			return ast.ErrASTDomain
		}
		bits1 := t.bitLiterals[v1]
		if err = bvAssert(1684, "bits1.size() == bits2.size()", func() (bool, error) { return len(bits1) == len(bits2), nil }); err != nil {
			return err
		}
		for i, bit1 := range bits1 {
			if i >= len(bits2) {
				return ast.ErrASTDomain
			}
			bit2 := bits2[i]
			val1, err := s.Assignment(bit1)
			if err != nil {
				return err
			}
			val2, err := s.Assignment(bit2)
			if err != nil {
				return err
			}
			if err = t.traceBVAssignmentMismatch(v1, v, uint32(i), bit1, bit2, val1, val2); err != nil {
				return err
			}
			if err = bvAssert(1697, "val1 == val2", func() (bool, error) { return val1 == val2, nil }); err != nil {
				return err
			}
		}
		if err = bvAssert(1699, "ctx.is_relevant(get_enode(v1))", func() (bool, error) {
			n, e := t.Enode(v1)
			if e != nil {
				return false, e
			}
			return s.context.isRelevant(n.owner), nil
		}); err != nil {
			return err
		}
		if t.privateNext[v1] == v {
			break
		}
	}
	return nil
}

func (t *SMTBitVectorTheory) checkBVZeroOneBits(v SMTTheoryVar) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	s, err := t.State()
	if err != nil {
		return err
	}
	if s.inconsistent {
		return nil
	}
	if v < 0 || int(v) >= len(t.privateFind) {
		return ast.ErrASTDomain
	}
	if t.privateFind[v] != v {
		return nil
	}
	node, err := t.Enode(v)
	if err != nil {
		return err
	}
	isBV, err := s.manager.IsBitVector(node.owner)
	if err != nil || !isBV {
		return err
	}
	width, err := s.manager.BitVectorSize(node.owner)
	if err != nil {
		return err
	}
	bits := [2][]bool{make([]bool, width), make([]bool, width)}
	count := 0
	for current := v; ; current = t.privateNext[current] {
		if current < 0 || int(current) >= len(t.bitLiterals) {
			return ast.ErrASTDomain
		}
		for i, literal := range t.bitLiterals[current] {
			if literal.Var() != SMTTrueBoolVar {
				continue
			}
			if uint64(i) >= uint64(width) {
				return ast.ErrASTDomain
			}
			sign := 0
			if literal == SMTTrueLiteral {
				sign = 1
			}
			if err = bvAssert(1728, "!bits[!is_true][i]", func() (bool, error) { return !bits[1-sign][i], nil }); err != nil {
				return err
			}
			if !bits[sign][i] {
				bits[sign][i] = true
				count++
			}
		}
		if t.privateNext[current] == v {
			break
		}
	}
	summary := t.zeroOneBits[v]
	if err = bvAssert(1740, "_bits.size() == num_bits", func() (bool, error) { return len(summary) == count, nil }); err != nil {
		return err
	}
	found := make([]bool, width)
	for _, bit := range summary {
		if err = bvAssert(1746, "find(it->m_owner) == v", func() (bool, error) { root, e := t.privateRoot(bit.variable); return root == v, e }); err != nil {
			return err
		}
		sign := 0
		if bit.isTrue {
			sign = 1
		}
		if err = bvAssert(1747, "bits[it->m_is_true][it->m_idx]", func() (bool, error) {
			if uint64(bit.index) >= uint64(width) {
				return false, ast.ErrASTDomain
			}
			return bits[sign][bit.index], nil
		}); err != nil {
			return err
		}
		if err = bvAssert(1748, "!already_found[it->m_idx]", func() (bool, error) {
			if uint64(bit.index) >= uint64(width) {
				return false, ast.ErrASTDomain
			}
			return !found[bit.index], nil
		}); err != nil {
			return err
		}
		if uint64(bit.index) >= uint64(width) {
			return ast.ErrASTDomain
		}
		found[bit.index] = true
	}
	return nil
}

func (t *SMTBitVectorTheory) traceBVAssignmentMismatch(v1, v2 SMTTheoryVar, index uint32, bit1, bit2 SMTLiteral, val1, val2 lbool.LBool) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	var operandErr error
	err := tags.EmitConditional("bv_bug", "check_assignment", "src/smt/theory_bv.cpp", 1691, func() bool { return val1 != val2 }, func(out io.Writer) error {
		out = assertedTraceWriter{out}
		_, _ = fmt.Fprintf(out, "equivalence class is inconsistent, i: %d\n", index)
		if operandErr = t.DisplayVariable(out, v1); operandErr != nil {
			return operandErr
		}
		if operandErr = t.DisplayVariable(out, v2); operandErr != nil {
			return operandErr
		}
		for i, bit := range []SMTLiteral{bit1, bit2} {
			val := val1
			if i == 1 {
				val = val2
			}
			if bit.Var() < 0 || int(bit.Var()) >= len(t.state.booleanData) {
				operandErr = ast.ErrASTDomain
				return operandErr
			}
			level := t.state.booleanData[bit.Var()].ScopeLevel
			_, _ = fmt.Fprintf(out, "val%d: %s lvl: %d bit ", i+1, val.String(), level)
			operandErr = smtPBWriteLiteral(out, bit)
			if operandErr != nil {
				return operandErr
			}
			_, _ = io.WriteString(out, "\n")
		}
		return nil
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

package smtcore

// Source: src/util/union_find.h:48-152, specialized for theory_bv;
// src/smt/theory_bv.cpp:33-39,1319-1332,1383-1504,1554-1581.

import "github.com/glycerine/g3/pkg/ast"

type smtBVPrivateTrailKind uint8

const (
	smtBVNewVariable smtBVPrivateTrailKind = iota
	smtBVPrivateMerge
	smtBVNewAtom
	smtBVNewOccurrence
	smtBVNewComparison
)

type smtBVPrivateTrail struct {
	kind    smtBVPrivateTrailKind
	child   SMTTheoryVar
	boolean int32
}

type smtBVZeroOneBit struct {
	variable SMTTheoryVar
	index    uint32
	isTrue   bool
}

func (t *SMTBitVectorTheory) privateRoot(variable SMTTheoryVar) (SMTTheoryVar, error) {
	if variable < 0 || int(variable) >= len(t.privateFind) {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	for t.privateFind[variable] != variable {
		variable = t.privateFind[variable]
	}
	return variable, nil
}

// The callback precedes the forest link. An error interrupts that source
// prefix; a logical conflict is a normal return and still commits the link.
func (t *SMTBitVectorTheory) mergePrivateClasses(left, right SMTTheoryVar) error {
	r1, err := t.privateRoot(left)
	if err != nil {
		return err
	}
	r2, err := t.privateRoot(right)
	if err != nil {
		return err
	}
	if r1 == r2 {
		return nil
	}
	if t.privateSize[r1] > t.privateSize[r2] {
		r1, r2 = r2, r1
	}
	// Source swaps only roots, retaining the original argument order.
	if err = t.mergeBitClasses(r2, r1, right, left); err != nil {
		return err
	}
	t.privateFind[r1] = r2
	t.privateSize[r2] += t.privateSize[r1]
	t.privateNext[r1], t.privateNext[r2] = t.privateNext[r2], t.privateNext[r1]
	t.privateTrail = append(t.privateTrail, smtBVPrivateTrail{kind: smtBVPrivateMerge, child: r1})
	return t.checkBVAfterMerge(r2)
}

// This undo path deliberately reads only native IDs and summaries: core
// enodes can already have been deleted when theory pop callbacks execute.
func (t *SMTBitVectorTheory) restorePrivateTrail(scope smtBVScope) error {
	if scope.private < 0 || scope.private > len(t.privateTrail) {
		return ast.ErrASTDomain
	}
	for len(t.privateTrail) > scope.private {
		event := t.privateTrail[len(t.privateTrail)-1]
		switch event.kind {
		case smtBVPrivateMerge:
			child := event.child
			root := t.privateFind[child]
			t.privateSize[root] -= t.privateSize[child]
			t.privateFind[child] = child
			t.privateNext[child], t.privateNext[root] = t.privateNext[root], t.privateNext[child]
			bits := t.zeroOneBits[root]
			keep := 0
			for j := len(bits) - 1; j >= 0; j-- {
				ownerRoot, err := t.privateRoot(bits[j].variable)
				if err != nil {
					return err
				}
				if ownerRoot == root {
					keep = j + 1
					break
				}
			}
			t.zeroOneBits[root] = bits[:keep]
		case smtBVNewVariable:
			n := len(t.privateFind) - 1
			if n < 0 {
				return ast.ErrASTDomain
			}
			t.privateFind, t.privateNext, t.privateSize = t.privateFind[:n], t.privateNext[:n], t.privateSize[:n]
		case smtBVNewOccurrence:
			entries := t.bitAtoms[event.boolean]
			if err := bvAssert(217, "m_atom->m_occs", func() (bool, error) { return len(entries) != 0, nil }); err != nil {
				return err
			}
			if len(entries) == 0 || len(t.bitOccurrenceTrail) == 0 {
				return ast.ErrASTDomain
			}
			t.bitAtoms[event.boolean] = entries[:len(entries)-1]
			t.bitOccurrenceTrail = t.bitOccurrenceTrail[:len(t.bitOccurrenceTrail)-1]
		case smtBVNewComparison:
			delete(t.comparisonAtoms, event.boolean)
		case smtBVNewAtom:
			delete(t.bitAtoms, event.boolean)
			t.bitAtomTrail = t.bitAtomTrail[:len(t.bitAtomTrail)-1]
		default:
			return ast.ErrASTDomain
		}
		t.privateTrail = t.privateTrail[:len(t.privateTrail)-1]
	}
	if len(t.bitAtomTrail) != scope.atoms || len(t.bitOccurrenceTrail) != scope.occurrences {
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SMTBitVectorTheory) mergeZeroOneBits(root, child SMTTheoryVar) (bool, error) {
	if len(t.zeroOneBits[child]) == 0 {
		return true, nil
	}
	node, err := t.Enode(root)
	if err != nil {
		return false, err
	}
	width, err := t.state.manager.BitVectorSize(node.owner)
	if err != nil {
		return false, err
	}
	if err = bvAssert(1463, "bv_size == get_bv_size(r2)", func() (bool, error) {
		childNode, e := t.Enode(child)
		if e != nil {
			return false, e
		}
		childWidth, e := t.state.manager.BitVectorSize(childNode.owner)
		return width == childWidth, e
	}); err != nil {
		return false, err
	}
	for sign := range t.mergeAux {
		for uint64(len(t.mergeAux[sign])) < uint64(width)+1 {
			t.mergeAux[sign] = append(t.mergeAux[sign], SMTNullTheoryVar)
		}
	}
	if err = t.assertBVMergeAux(1472, width); err != nil {
		return false, err
	}
	for _, bit := range t.zeroOneBits[root] {
		sign := 0
		if bit.isTrue {
			sign = 1
		}
		t.mergeAux[sign][bit.index] = bit.variable
	}
	reset := func() {
		for _, bit := range t.zeroOneBits[root] {
			sign := 0
			if bit.isTrue {
				sign = 1
			}
			t.mergeAux[sign][bit.index] = SMTNullTheoryVar
		}
	}
	for _, bit := range t.zeroOneBits[child] {
		sign := 0
		if bit.isTrue {
			sign = 1
		}
		other := t.mergeAux[1-sign][bit.index]
		if other != SMTNullTheoryVar {
			if err = bvAssert(1486, "m_bits[v1][it->m_idx] == ~(m_bits[v2][it->m_idx])", func() (bool, error) {
				if other < 0 || bit.variable < 0 || int(other) >= len(t.bitLiterals) || int(bit.variable) >= len(t.bitLiterals) || uint64(bit.index) >= uint64(len(t.bitLiterals[other])) || uint64(bit.index) >= uint64(len(t.bitLiterals[bit.variable])) {
					return false, ast.ErrASTDomain
				}
				return t.bitLiterals[other][bit.index] == t.bitLiterals[bit.variable][bit.index].Negated(), nil
			}); err != nil {
				return false, err
			}
			if err = bvAssert(1487, "m_bits[v1].size() == m_bits[v2].size()", func() (bool, error) {
				return len(t.bitLiterals[other]) == len(t.bitLiterals[bit.variable]), nil
			}); err != nil {
				return false, err
			}
			if err = t.newStaticBitDisequality(other, bit.variable, bit.index); err != nil {
				return false, err
			}
			reset()
			return false, nil
		}
		if t.mergeAux[sign][bit.index] == SMTNullTheoryVar {
			// Source does not update merge_aux here: its snapshot represents
			// only the first class, even as missing entries are appended.
			t.zeroOneBits[root] = append(t.zeroOneBits[root], bit)
		}
	}
	reset()
	if err = t.assertBVMergeAux(1499, width); err != nil {
		return false, err
	}
	return true, nil
}

func (t *SMTBitVectorTheory) assertBVMergeAux(line int32, width uint32) error {
	return bvAssert(line, "m_merge_aux[0][i] == null_theory_var || m_merge_aux[1][i] == null_theory_var", func() (bool, error) {
		if uint64(width) > uint64(len(t.mergeAux[0])) || uint64(width) > uint64(len(t.mergeAux[1])) {
			return false, ast.ErrASTDomain
		}
		for i := uint32(0); i < width; i++ {
			if t.mergeAux[0][i] != SMTNullTheoryVar && t.mergeAux[1][i] != SMTNullTheoryVar {
				return false, nil
			}
		}
		return true, nil
	})
}

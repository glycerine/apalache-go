package smtcore

// Source: src/smt/theory_bv.cpp:330-351,1163-1281,1383-1457.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source expand_diseq's e1/e2/diff expr_refs survive recursive
// internalization even when that visit drains the shared circuit temporaries.
func (t *SMTBitVectorTheory) internalizeBitDifference(left, right SMTLiteral) (result SMTLiteral, resultErr error) {
	s, err := t.State()
	if err != nil {
		return SMTNullLiteral, err
	}
	first, err := s.literalExpression(left)
	if err != nil {
		return SMTNullLiteral, err
	}
	firstRoot, err := s.manager.NewRef(first)
	if err != nil {
		return SMTNullLiteral, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&firstRoot)) }()
	second, err := s.literalExpression(right)
	if err != nil {
		return SMTNullLiteral, err
	}
	secondRoot, err := s.manager.NewRef(second)
	if err != nil {
		return SMTNullLiteral, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&secondRoot)) }()
	difference, err := t.boolXor(first, second)
	if err != nil {
		return SMTNullLiteral, err
	}
	differenceRoot, err := s.manager.NewRef(difference)
	if err != nil {
		return SMTNullLiteral, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&differenceRoot)) }()
	if err = s.Internalize(difference, true); err != nil {
		return SMTNullLiteral, err
	}
	return s.Literal(difference)
}

func (t *SMTBitVectorTheory) findWatchedPosition(variable SMTTheoryVar) error {
	if variable < 0 || int(variable) >= len(t.bitLiterals) {
		return ast.ErrASTDomain
	}
	bits := t.bitLiterals[variable]
	initial := t.watchedPosition[variable]
	for ; uint64(t.watchedPosition[variable]) < uint64(len(bits)); t.watchedPosition[variable]++ {
		if err := t.traceBVWatchedLiteral(bits[t.watchedPosition[variable]]); err != nil {
			return err
		}
		value, err := t.state.Assignment(bits[t.watchedPosition[variable]])
		if err != nil {
			return err
		}
		if value == lbool.Undef {
			if err = t.traceBVMovedWatch(variable, t.watchedPosition[variable], 339); err != nil {
				return err
			}
			return nil
		}
	}
	t.watchedPosition[variable] = 0
	for ; t.watchedPosition[variable] < initial; t.watchedPosition[variable]++ {
		if uint64(t.watchedPosition[variable]) >= uint64(len(bits)) {
			return ast.ErrASTDomain
		}
		value, err := t.state.Assignment(bits[t.watchedPosition[variable]])
		if err != nil {
			return err
		}
		if value == lbool.Undef {
			if err = t.traceBVMovedWatch(variable, t.watchedPosition[variable], 346); err != nil {
				return err
			}
			return nil
		}
	}
	if err := t.traceBVFixedWatch(variable); err != nil {
		return err
	}
	return t.fixedVariable(variable)
}

func (t *SMTBitVectorTheory) propagateBits() error {
	s, err := t.State()
	if err != nil {
		return err
	}
	for i := 0; i < len(t.propagationQueue); i++ {
		entry := t.propagationQueue[i]
		v, index := entry.variable, entry.index
		if v < 0 || int(v) >= len(t.bitLiterals) || uint64(index) >= uint64(len(t.bitLiterals[v])) {
			return ast.ErrASTDomain
		}
		if t.watchedPosition[v] == index {
			if err = t.findWatchedPosition(v); err != nil {
				return err
			}
		}
		bit := t.bitLiterals[v][index]
		value, err := s.Assignment(bit)
		if err != nil {
			return err
		}
		if value == lbool.Undef {
			continue
		}
		if err = t.traceBVPropagating(v, index, value, 1203); err != nil {
			return err
		}
		antecedent := bit
		if value == lbool.False {
			antecedent = antecedent.Negated()
		}
		for other := t.privateNext[v]; other != v; other = t.privateNext[other] {
			if uint64(index) >= uint64(len(t.bitLiterals[other])) {
				return ast.ErrASTDomain
			}
			consequent := t.bitLiterals[other][index]
			if err = bvAssert(1212, "bit != ~bit2", func() (bool, error) { return bit != consequent.Negated(), nil }); err != nil {
				return err
			}
			otherValue, err := s.Assignment(consequent)
			if err != nil {
				return err
			}
			if err = t.traceBVPropagating(other, index, otherValue, 1214); err != nil {
				return err
			}
			if value != otherValue {
				if value == lbool.False {
					consequent = consequent.Negated()
				}
				if err = t.assignBit(consequent, v, other, index, antecedent, false); err != nil {
					return err
				}
				if s.inconsistent {
					if err = t.traceBVPropagationConflict(bit, t.bitLiterals[other][index]); err != nil {
						return err
					}
					t.propagationQueue = t.propagationQueue[:0]
					return nil
				}
			}
		}
	}
	t.propagationQueue = t.propagationQueue[:0]
	return t.traceBVPropagationDone()
}

func (t *SMTBitVectorTheory) assignBit(consequent SMTLiteral, from, to SMTTheoryVar, index uint32, antecedent SMTLiteral, propagateClass bool) error {
	t.statistics.bit2core++
	s, err := t.State()
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
	if err = bvAssert(1239, "ctx.get_assignment(antecedent) == l_true", func() (bool, error) {
		value, e := s.Assignment(antecedent)
		return value == lbool.True, e
	}); err != nil {
		return err
	}
	if err = bvAssert(1240, "m_bits[v2][idx].var() == consequent.var()", func() (bool, error) {
		if to < 0 || int(to) >= len(t.bitLiterals) || uint64(index) >= uint64(len(t.bitLiterals[to])) {
			return false, ast.ErrASTDomain
		}
		return t.bitLiterals[to][index].Var() == consequent.Var(), nil
	}); err != nil {
		return err
	}
	if err = bvAssert(1241, "consequent.var() != antecedent.var()", func() (bool, error) { return consequent.Var() != antecedent.Var(), nil }); err != nil {
		return err
	}
	if err = t.traceBVAssignBit(consequent, antecedent, from, to, index, propagateClass); err != nil {
		return err
	}
	owner := &smtBVBitEqualityJustification{SMTJustificationBase: NewSMTJustificationBase(true), first: first, second: second, theory: t.FamilyID(), consequent: consequent, antecedent: antecedent}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return err
	}
	s.ownedJustifications = append(s.ownedJustifications, owner)
	if consequent == SMTFalseLiteral {
		t.statistics.conflicts++
		s.setBooleanConflict(wrapped, SMTNullLiteral)
		return nil
	}
	if err = s.AssignLiteral(consequent, wrapped); err != nil {
		return err
	}
	equality, err := t.MkEqualityLiteral(first.owner, second.owner, false)
	if err != nil {
		return err
	}
	if err = s.addTheoryAxiom(t.FamilyID(), []SMTLiteral{consequent.Negated(), antecedent, equality.Negated()}); err != nil {
		return err
	}
	if t.watchedPosition[to] == index {
		if err = t.findWatchedPosition(to); err != nil {
			return err
		}
	}
	entries, exists := t.bitAtoms[consequent.Var()]
	if err = bvAssert(1265, "a->is_bit()", func() (bool, error) { return exists, nil }); err != nil {
		return err
	}
	if !exists {
		return ast.ErrASTDomain
	}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if err = t.traceBVAssignBitOccurrence(entry, to, index); err != nil {
			return err
		}
		entryRoot, err := t.privateRoot(entry.variable)
		if err != nil {
			return err
		}
		targetRoot, err := t.privateRoot(to)
		if err != nil {
			return err
		}
		if propagateClass || entryRoot != targetRoot || entry.index != index {
			t.propagationQueue = append(t.propagationQueue, entry)
		}
	}
	return nil
}

func (t *SMTBitVectorTheory) mergeBitClasses(root, child, first, second SMTTheoryVar) error {
	if err := t.traceBVMerge(first, second, "bv"); err != nil {
		return err
	}
	if err := t.traceBVMerge(first, second, "bv_bit_prop"); err != nil {
		return err
	}
	consistent, err := t.mergeZeroOneBits(root, child)
	if err != nil {
		return err
	}
	if !consistent {
		return t.traceBVMergeConflict()
	}
	t.propagationQueue = t.propagationQueue[:0]
	if err = bvAssert(1394, "bits1.size() == bits2.size()", func() (bool, error) {
		return len(t.bitLiterals[first]) == len(t.bitLiterals[second]), nil
	}); err != nil {
		return err
	}
	if len(t.bitLiterals[first]) != len(t.bitLiterals[second]) {
		return ast.ErrASTDomain
	}
	if err = t.traceBVMergeSize(len(t.bitLiterals[first])); err != nil {
		return err
	}
	s := t.state
	for {
		changed := false
		for index := 0; index < len(t.bitLiterals[first]); index++ {
			bit1, bit2 := t.bitLiterals[first][index], t.bitLiterals[second][index]
			if err = t.traceBVMergeComplement(first, second, index, bit1, bit2); err != nil {
				return err
			}
			if err = bvAssert(1417, "bit1 != ~bit2", func() (bool, error) { return bit1 != bit2.Negated(), nil }); err != nil {
				return err
			}
			value1, err := s.Assignment(bit1)
			if err != nil {
				return err
			}
			value2, err := s.Assignment(bit2)
			if err != nil {
				return err
			}
			if value1 == value2 {
				continue
			}
			changed = true
			if value1 != lbool.Undef && value2 != lbool.Undef {
				if err = t.traceBVMergeInconsistent(first, second, index); err != nil {
					return err
				}
			}
			if value1 != lbool.Undef {
				if value1 == lbool.False {
					bit1, bit2 = bit1.Negated(), bit2.Negated()
				}
				err = t.assignBit(bit2, first, second, uint32(index), bit1, true)
			} else if value2 != lbool.Undef {
				if value2 == lbool.False {
					bit1, bit2 = bit1.Negated(), bit2.Negated()
				}
				err = t.assignBit(bit1, second, first, uint32(index), bit2, true)
			}
			if err != nil {
				return err
			}
			if s.inconsistent {
				return nil
			}
			if value1 != lbool.Undef && value2 != lbool.Undef && value1 != value2 {
				return diagnostic.BorrowProcessDebugContext().UnreachableSource("src/smt/theory_bv.cpp", 1447)
			}
		}
		if !changed {
			break
		}
	}
	return t.propagateBits()
}

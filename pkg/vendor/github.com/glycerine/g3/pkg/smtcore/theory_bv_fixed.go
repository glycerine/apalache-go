package smtcore

// Source: src/smt/theory_bv.cpp:426-490; theory_bv.h value_sort_pair.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtBVFixedKey struct {
	width uint32
	value string
}

// The source cache is not backtracked. Each hit validates the old variable
// against its current slot, sort, width, assignment and numeric value.
func (t *SMTBitVectorTheory) fixedVariable(variable SMTTheoryVar) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	node, err := t.Enode(variable)
	if err != nil {
		return err
	}
	value, fixed, err := t.fixedLiteralValue(variable)
	if err != nil {
		return err
	}
	if !fixed {
		return ast.ErrASTDomain
	}
	width, err := s.manager.BitVectorSize(node.owner)
	if err != nil {
		return err
	}
	key := smtBVFixedKey{width: width, value: value.String()}
	old, found := t.fixedVariables[key]
	if !found {
		t.fixedVariables[key] = variable
		return nil
	}
	valid := old >= 0 && int(old) < len(t.variables)
	if valid {
		isBV, err := s.manager.IsBitVector(t.variables[old].owner)
		if err != nil {
			return err
		}
		valid = isBV
	}
	if valid {
		oldWidth, err := s.manager.BitVectorSize(t.variables[old].owner)
		if err != nil {
			return err
		}
		valid = oldWidth == width
		if valid {
			if err = bvAssert(469, "get_bv_size(v) == get_bv_size(v2)", func() (bool, error) { return width == oldWidth, nil }); err != nil {
				return err
			}
		}
	}
	if valid {
		oldValue, oldFixed, err := t.fixedLiteralValue(old)
		if err != nil {
			return err
		}
		valid = oldFixed && value.Cmp(oldValue) == 0
	}
	if !valid {
		delete(t.fixedVariables, key)
		t.fixedVariables[key] = variable
		return nil
	}
	other := t.variables[old]
	if node.root == other.root {
		return nil
	}
	if err = t.traceBVFixedEquality(variable, old); err != nil {
		return err
	}
	t.statistics.theoryToCoreEq++
	if err = t.addFixedEquality(variable, old); err != nil {
		return err
	}
	owner := &smtBVFixedEqualityJustification{SMTJustificationBase: NewSMTJustificationBase(true), theory: t, first: variable, second: old}
	s.ownedJustifications = append(s.ownedJustifications, owner)
	if err = s.assignEquality(node, other, NewSMTEqualityJustification(owner)); err != nil {
		return err
	}
	// Source retains its original representative, not the new equal word.
	t.fixedVariables[key] = old
	return nil
}

func (t *SMTBitVectorTheory) addFixedEquality(first, second SMTTheoryVar) error {
	t.statistics.eqDynamic++
	if first > second {
		first, second = second, first
	}
	left, err := t.Enode(first)
	if err != nil {
		return err
	}
	right, err := t.Enode(second)
	if err != nil {
		return err
	}
	s := t.state
	width, err := s.manager.BitVectorSize(left.owner)
	if err != nil {
		return err
	}
	wordEquality, err := t.MkEqualityLiteral(left.owner, right.owner, true)
	if err != nil {
		return err
	}
	if err = t.traceBVFixedAxiom(left.owner, right.owner); err != nil {
		return err
	}
	equalities := make([]SMTLiteral, 0, int(width)+1)
	for index := uint32(0); index < width; index++ {
		equality, err := t.addFixedBitEquality(left.owner, right.owner, wordEquality, t.bitLiterals[first][index], t.bitLiterals[second][index], index)
		if err != nil {
			return err
		}
		equalities = append(equalities, equality.Negated())
	}
	equalities = append(equalities, wordEquality)
	return s.addTheoryAxiom(t.FamilyID(), equalities)
}

func (t *SMTBitVectorTheory) addFixedBitEquality(left, right ast.ASTHandle, wordEquality, first, second SMTLiteral, index uint32) (result SMTLiteral, resultErr error) {
	s := t.state
	e1, err := s.manager.MkBitVectorBit2Bool(index, left)
	if err != nil {
		return SMTNullLiteral, err
	}
	root1, err := s.manager.NewRef(e1)
	if err != nil {
		return SMTNullLiteral, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&root1)) }()
	e2, err := s.manager.MkBitVectorBit2Bool(index, right)
	if err != nil {
		return SMTNullLiteral, err
	}
	root2, err := s.manager.NewRef(e2)
	if err != nil {
		return SMTNullLiteral, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&root2)) }()
	equality, err := t.MkEqualityLiteral(e1, e2, true)
	if err != nil {
		return SMTNullLiteral, err
	}
	for _, clause := range [][]SMTLiteral{
		{first, second.Negated(), equality.Negated()},
		{first.Negated(), second, equality.Negated()},
		{first, second, equality},
		{first.Negated(), second.Negated(), equality},
		{equality, wordEquality.Negated()},
	} {
		if err = s.addTheoryAxiom(t.FamilyID(), clause); err != nil {
			return SMTNullLiteral, err
		}
	}
	return equality, nil
}

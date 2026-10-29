package smtcore

// Source: src/smt/theory_lra.cpp:1994-2142, retained fixed-bound equalities.

import (
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lp"
)

type smtLRAConstraintBound struct {
	constraint uint32
	value      big.Rat
}
type smtLRATermBoundChange struct {
	index    uint32
	lower    bool
	previous smtLRAConstraintBound
}
type smtLRAFixedKey struct {
	value   string
	integer bool
}

func (t *SMTLRATheory) fixedKey(v SMTTheoryVar, value *big.Rat) (smtLRAFixedKey, error) {
	n, err := t.Enode(v)
	if err != nil {
		return smtLRAFixedKey{}, err
	}
	sort, err := t.state.manager.GetSort(n.owner)
	if err != nil {
		return smtLRAFixedKey{}, err
	}
	kind, err := t.state.manager.GetDeclKind(sort)
	return smtLRAFixedKey{value.RatString(), kind == 1}, err
}

func (t *SMTLRATheory) setTermBound(index, witness uint32, value *big.Rat, lower bool) {
	if !t.numeric.solver.IsTerm(index) {
		return
	}
	index -= lp.LARTermStartIndex
	vec := &t.upperTerms
	if lower {
		vec = &t.lowerTerms
	}
	for len(*vec) <= int(index) {
		*vec = append(*vec, smtLRAConstraintBound{constraint: smtLRANoIndex})
	}
	previous := &(*vec)[index]
	cmp := previous.value.Cmp(value)
	if previous.constraint != smtLRANoIndex && (lower && cmp >= 0 || !lower && cmp <= 0) {
		return
	}
	change := smtLRATermBoundChange{index: index, lower: lower}
	change.previous.constraint = previous.constraint
	change.previous.value.Set(&previous.value)
	t.termBoundTrail = append(t.termBoundTrail, change)
	next := smtLRAConstraintBound{constraint: witness}
	next.value.Set(value)
	(*vec)[index] = next
}

func (t *SMTLRATheory) hasFixedBound(index uint32, value *big.Rat, lower bool) (uint32, bool, error) {
	if !t.numeric.solver.IsTerm(index) {
		return t.numeric.hasRationalBound(index, lower, value)
	}
	v := t.variableForLPIndex(index)
	if err := t.emitLRA("has_bound", 2057, func(out io.Writer) error { return smtWriteTheoryText(out, fmt.Sprintf("%d %d\n", index, v)) }); err != nil {
		return smtLRANoIndex, false, err
	}
	if v != SMTNullTheoryVar {
		n, err := t.Enode(v)
		if err != nil {
			return smtLRANoIndex, false, err
		}
		actual, number, err := t.numeral(n.owner)
		if err != nil {
			return smtLRANoIndex, false, err
		}
		if number && actual.Cmp(value) == 0 {
			return smtLRANoIndex, true, nil
		}
	}
	index -= lp.LARTermStartIndex
	vec := t.upperTerms
	if lower {
		vec = t.lowerTerms
	}
	if uint64(index) >= uint64(len(vec)) {
		return smtLRANoIndex, false, nil
	}
	b := &vec[index]
	return b.constraint, b.constraint != smtLRANoIndex && b.value.Cmp(value) == 0, nil
}

func (t *SMTLRATheory) propagateFixedBounds(b *smtLRABound, witness uint32, truth bool) error {
	if !t.config.PropagateEqs || t.conflictCount >= t.config.PropagationThreshold || !truth || witness == smtLRANoIndex {
		return nil
	}
	index, ok := t.numeric.index(b.variable)
	if !ok {
		return ast.ErrASTDomain
	}
	lower := !b.upper
	t.setTermBound(index, witness, &b.value, lower)
	_, fixed, err := t.hasFixedBound(index, &b.value, !lower)
	if err != nil || !fixed {
		return err
	}
	return t.fixedVariable(b.variable, &b.value)
}

func (t *SMTLRATheory) fixedVariable(v SMTTheoryVar, value *big.Rat) error {
	key, err := t.fixedKey(v, value)
	if err != nil {
		return err
	}
	other, found := t.fixedVariables[key]
	if !found || other < 0 || int(other) >= t.NumVariables() {
		t.fixedVariables[key] = v
		return nil
	}
	left, err := t.Enode(v)
	if err != nil {
		return err
	}
	right, err := t.Enode(other)
	if err != nil {
		return err
	}
	if left.Root() == right.Root() {
		t.fixedVariables[key] = v
		return nil
	}
	vi, err := t.numeric.getVarIndex(v)
	if err != nil {
		return err
	}
	wi, err := t.numeric.getVarIndex(other)
	if err != nil {
		return err
	}
	if err = t.emitLRA("fixed_var_eh", 2098, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "fixed: "); err != nil {
			return err
		}
		if err := t.displayLRAExpression(out, left.owner); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " "); err != nil {
			return err
		}
		if err := t.displayLRAExpression(out, right.owner); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " "+value.RatString()+" "); err != nil {
			return err
		}
		_, found, err := t.hasFixedBound(wi, value, true)
		if err != nil {
			return err
		}
		flag := 0
		if found {
			flag = 1
		}
		return smtWriteTheoryText(out, fmt.Sprintf("%d\n", flag))
	}); err != nil {
		return err
	}
	lo2, fixed, err := t.hasFixedBound(wi, value, true)
	if err != nil || !fixed {
		return err
	}
	hi2, fixed, err := t.hasFixedBound(wi, value, false)
	if err != nil || !fixed {
		return err
	}
	lo1, fixed, err := t.hasFixedBound(vi, value, true)
	if err != nil {
		return err
	}
	if !fixed {
		return ast.ErrASTDomain
	}
	hi1, fixed, err := t.hasFixedBound(vi, value, false)
	if err != nil {
		return err
	}
	if !fixed {
		return ast.ErrASTDomain
	}
	hook := smtLRABoundPropagation{owner: t}
	for _, witness := range []uint32{lo1, hi1, lo2, hi2} {
		if err = hook.Consume(nil, witness); err != nil {
			return err
		}
	}
	t.fixedEqualities++
	owner, err := NewSMTExtTheoryEqualityPropagationJustification(t.state.manager, t.FamilyID(), hook.core, nil, left, right, nil)
	if err != nil {
		return err
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	if err = t.emitLRA("fixed_var_eh", 2116, func(out io.Writer) error {
		for _, literal := range hook.core {
			if err := t.state.DisplayDetailedLiteral(out, literal); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, "\n"); err != nil {
				return err
			}
		}
		if err := smtWriteTheoryText(out, " ==> "); err != nil {
			return err
		}
		if err := t.displayLRAExpression(out, left.owner); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " = "); err != nil {
			return err
		}
		if err := t.displayLRAExpression(out, right.owner); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	if err := t.checkValidation(smtLRAValidateEquality, hook.core, SMTFalseLiteral, left, right); err != nil {
		return err
	}
	return t.state.assignEquality(left, right, NewSMTEqualityJustification(owner))
}

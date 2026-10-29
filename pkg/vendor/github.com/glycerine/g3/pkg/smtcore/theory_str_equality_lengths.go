package smtcore

// Source: src/smt/theory_str.cpp get_arith_value, get_len_value and the
// check_length_consistency family. Only actual core numeral roots are read.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) arithmeticValue(expression ast.ASTHandle) (*big.Int, bool, error) {
	node := t.state.context.enodes[expression]
	if node == nil {
		return nil, false, nil
	}
	text, _, numeral, err := t.state.manager.ArithmeticNumeralText(node.Root().Owner())
	if err != nil || !numeral {
		return nil, false, err
	}
	value, parsed := new(big.Rat).SetString(text)
	if !parsed {
		return nil, false, ast.ErrASTDomain
	}
	if !value.IsInt() {
		return nil, false, nil
	}
	return new(big.Int).Set(value.Num()), true, nil
}

func (t *SMTStringTheory) lengthValue(expression ast.ASTHandle) (value *big.Int, known bool, result error) {
	m := t.state.manager
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, false, err
	}
	arithmetic, mixed := t.state.Theory(family).(*SMTLinearArithmeticTheory)
	if !mixed || !arithmetic.mixed {
		return nil, false, nil
	}
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	value = new(big.Int)
	work := []ast.ASTHandle{expression}
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		left, right, concat, err := m.SequenceConcatParts(current)
		if err != nil {
			return value, false, err
		}
		if concat {
			// Source get_len_value visits the right child first.
			work = append(work, left, right)
			continue
		}
		text, literal, err := m.SequenceStringValue(current)
		if err != nil {
			return value, false, err
		}
		if literal {
			value.Add(value, new(big.Int).SetUint64(uint64(len(text))))
			continue
		}
		length, err := m.MkSequenceLength(current)
		if err != nil {
			return value, false, err
		}
		if err = m.SetRef(ref, length); err != nil {
			return value, false, err
		}
		part, exists, err := t.arithmeticValue(length)
		if err != nil || !exists {
			return value, false, err
		}
		value.Add(value, part)
	}
	return value, true, nil
}

type smtStringLengthPremises struct {
	theory *SMTStringTheory
	values []ast.ASTHandle
	refs   []ast.ASTRef
}

func (p *smtStringLengthPremises) close() (result error) {
	// Source premise collections are expr_ref_vectors (forward release).
	for i := range p.refs {
		result = errors.Join(result, p.theory.state.manager.DelRef(&p.refs[i]))
	}
	return result
}

func (p *smtStringLengthPremises) add(expression ast.ASTHandle) error {
	ref, err := p.theory.state.manager.NewRef(expression)
	if err != nil {
		return err
	}
	p.refs = append(p.refs, ref)
	p.values = append(p.values, expression)
	return nil
}

func (p *smtStringLengthPremises) length(expression ast.ASTHandle, value *big.Int) error {
	m := p.theory.state.manager
	length, err := m.MkSequenceLength(expression)
	if err != nil {
		return err
	}
	numeral, err := m.ArithmeticIntegerNumeralText(value.String())
	if err != nil {
		return err
	}
	equality, err := m.MkEq(length, numeral)
	if err != nil {
		return err
	}
	return p.add(equality)
}

func (p *smtStringLengthPremises) refuse(left, right ast.ASTHandle) error {
	m := p.theory.state.manager
	equality, err := m.MkEq(left, right)
	if err != nil {
		return err
	}
	if err = p.add(equality); err != nil {
		return err
	}
	conjunction, err := m.MkAndArray(p.values)
	if err != nil {
		return err
	}
	negative, err := m.MkNot(conjunction)
	if err != nil {
		return err
	}
	return p.theory.assertAxiom(negative)
}

func (t *SMTStringTheory) checkLengthConstant(expression, constant ast.ASTHandle) (consistent bool, result error) {
	m := t.state.manager
	text, literal, err := m.SequenceStringValue(constant)
	if err != nil || !literal {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return false, err
	}
	target := new(big.Int).SetUint64(uint64(len(text)))
	concat, err := m.IsSequenceConcat(expression)
	if err != nil {
		return false, err
	}
	if !concat {
		value, known, err := t.lengthValue(expression)
		if err != nil {
			return false, err
		}
		if !known || value.Cmp(target) != 0 {
			err = t.assertStringLengthEquality(expression, constant)
		}
		return !known || value.Cmp(target) == 0, err
	}
	leaves, err := t.concatLeaves(expression)
	if err != nil {
		return false, err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	sum := new(big.Int)
	for _, leaf := range leaves {
		value, known, err := t.lengthValue(leaf)
		if err != nil {
			return false, err
		}
		if !known {
			continue
		}
		if _, literal, err := m.SequenceStringValue(leaf); err != nil {
			return false, err
		} else if !literal {
			if err = premises.length(leaf, value); err != nil {
				return false, err
			}
		}
		sum.Add(sum, value)
		if sum.Cmp(target) > 0 {
			return false, premises.refuse(expression, constant)
		}
	}
	return true, nil
}

func (t *SMTStringTheory) checkLengthConcats(left, right ast.ASTHandle) (consistent bool, result error) {
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	sums := [2]*big.Int{new(big.Int), new(big.Int)}
	fixed := [2]bool{true, true}
	for side, expression := range []ast.ASTHandle{left, right} {
		leaves, err := t.concatLeaves(expression)
		if err != nil {
			return false, err
		}
		for _, leaf := range leaves {
			value, known, err := t.lengthValue(leaf)
			if err != nil {
				return false, err
			}
			if !known {
				fixed[side] = false
				continue
			}
			sums[side].Add(sums[side], value)
			if _, literal, err := t.state.manager.SequenceStringValue(leaf); err != nil {
				return false, err
			} else if !literal {
				if err = premises.length(leaf, value); err != nil {
					return false, err
				}
			}
		}
	}
	comparison := sums[0].Cmp(sums[1])
	conflict := fixed[0] && fixed[1] && comparison != 0 || !fixed[0] && fixed[1] && comparison > 0 || fixed[0] && !fixed[1] && comparison < 0
	if conflict {
		return false, premises.refuse(left, right)
	}
	return true, nil
}

func (t *SMTStringTheory) checkLengthConcatVariable(concat, variable ast.ASTHandle) (consistent bool, result error) {
	target, known, err := t.lengthValue(variable)
	if err != nil || !known {
		return true, err
	}
	leaves, err := t.concatLeaves(concat)
	if err != nil {
		return false, err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	sum := new(big.Int)
	for _, leaf := range leaves {
		value, known, err := t.lengthValue(leaf)
		if err != nil {
			return false, err
		}
		if !known {
			continue
		}
		if _, literal, err := t.state.manager.SequenceStringValue(leaf); err != nil {
			return false, err
		} else if !literal && value.Sign() != 0 {
			if err = premises.length(leaf, value); err != nil {
				return false, err
			}
		}
		sum.Add(sum, value)
		if sum.Cmp(target) > 0 {
			if err = premises.length(variable, target); err != nil {
				return false, err
			}
			return false, premises.refuse(concat, variable)
		}
	}
	return true, nil
}

func (t *SMTStringTheory) checkLengthVariables(left, right ast.ASTHandle) (consistent bool, result error) {
	l, lk, err := t.lengthValue(left)
	if err != nil {
		return false, err
	}
	r, rk, err := t.lengthValue(right)
	if err != nil {
		return false, err
	}
	if !lk || !rk || l.Cmp(r) == 0 {
		return true, nil
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err = premises.length(left, l); err != nil {
		return false, err
	}
	if err = premises.length(right, r); err != nil {
		return false, err
	}
	return false, premises.refuse(left, right)
}

func (t *SMTStringTheory) checkLengthConsistency(left, right ast.ASTHandle) (bool, error) {
	m := t.state.manager
	_, ll, err := m.SequenceStringValue(left)
	if err != nil {
		return false, err
	}
	_, rl, err := m.SequenceStringValue(right)
	if err != nil {
		return false, err
	}
	if ll && rl {
		return true, nil
	}
	if ll {
		return t.checkLengthConstant(right, left)
	}
	if rl {
		return t.checkLengthConstant(left, right)
	}
	lc, err := m.IsSequenceConcat(left)
	if err != nil {
		return false, err
	}
	rc, err := m.IsSequenceConcat(right)
	if err != nil {
		return false, err
	}
	if lc && rc {
		return t.checkLengthConcats(left, right)
	}
	if lc {
		return t.checkLengthConcatVariable(left, right)
	}
	if rc {
		return t.checkLengthConcatVariable(right, left)
	}
	return t.checkLengthVariables(left, right)
}

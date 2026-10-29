package smtcore

// Source: src/smt/theory_str.cpp eval_concat, simplify_parent and
// the initial concat shortcuts/parent dispatch of handle_equality.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) evalStringConcat(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	m := t.state.manager
	l, lk, err := t.eqcStringValue(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, rk, err := t.eqcStringValue(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if lk && rk {
		first, _, err := m.SequenceStringValue(l)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		second, _, err := m.SequenceStringValue(r)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return t.stringLiteral(first + second)
	}
	if lk {
		text, _, err := m.SequenceStringValue(l)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if text == "" {
			return right, nil
		}
	} else if rk {
		text, _, err := m.SequenceStringValue(r)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if text == "" {
			return left, nil
		}
	}
	return ast.ASTHandle{}, nil
}

func (t *SMTStringTheory) simplifyEqualityParents(left, right ast.ASTHandle) error {
	l, lk, err := t.eqcStringValue(left)
	if err != nil {
		return err
	}
	r, rk, err := t.eqcStringValue(right)
	if err != nil {
		return err
	}
	if lk && !rk {
		return t.simplifyStringParents(right, l)
	}
	if !lk && rk {
		return t.simplifyStringParents(left, r)
	}
	return nil
}

func (t *SMTStringTheory) stringConcatEqualityShortcut(left, right ast.ASTHandle) (bool, error) {
	m := t.state.manager
	l0, l1, lc, err := m.SequenceConcatParts(left)
	if err != nil {
		return false, err
	}
	r0, r1, rc, err := m.SequenceConcatParts(right)
	if err != nil || !lc || !rc {
		return false, err
	}
	if err := t.simplifyEqualityParents(left, right); err != nil {
		return false, err
	}
	if l0 == r0 {
		same, err := t.sameStringClass(l1, r1)
		if err != nil || same {
			return same, err
		}
	}
	if l1 == r1 {
		return t.sameStringClass(l0, r0)
	}
	return false, nil
}

func (t *SMTStringTheory) simplifyStringParents(expression, constant ast.ASTHandle) error {
	if err := t.state.Internalize(expression, false); err != nil {
		return err
	}
	for _, member := range t.stringClass(expression) {
		node := t.state.context.enodes[member]
		if node == nil {
			return ast.ErrASTDomain
		}
		// Source snapshots the current member's parent list before adding
		// axioms. Boolean parents introduced by publication are not revisited.
		for _, parent := range node.Parents() {
			if err := t.simplifyStringParent(member, constant, parent); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTStringTheory) simplifyStringParent(member, constant ast.ASTHandle, parent *SMTEnode) (result error) {
	m := t.state.manager
	first, second, concat, err := m.SequenceConcatParts(parent.Owner())
	if err != nil || !concat {
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(expression ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil || expression.IsNull() {
			return expression, err
		}
		return expression, locals.add(expression)
	}
	// Both direct argument positions are independent, including x ++ x.
	for i, argument := range []ast.ASTHandle{first, second} {
		if argument != member {
			continue
		}
		other := second
		left, right := constant, second
		if i == 1 {
			other, left, right = first, first, constant
		}
		evaluated, err := pin(t.evalStringConcat(left, right))
		if err != nil {
			return err
		}
		if !evaluated.IsNull() {
			otherValue, _, err := t.eqcStringValue(other)
			if err != nil {
				return err
			}
			if err := t.publishParentSimplification(member, constant, other, otherValue, parent.Owner(), evaluated); err != nil {
				return err
			}
		} else {
			memberConcat, err := m.IsSequenceConcat(member)
			if err != nil {
				return err
			}
			if memberConcat {
				simplified, err := pin(t.concatTerm(left, right))
				if err != nil {
					return err
				}
				if err := t.publishParentSimplification(member, constant, member, member, parent.Owner(), simplified); err != nil {
					return err
				}
			}
		}
	}
	// Cases 3/4: combine a new class constant with an adjacent literal in
	// the other operand's concat, preserving its remaining operand.
	for i, argument := range []ast.ASTHandle{first, second} {
		if argument != member {
			continue
		}
		other := second
		if i == 1 {
			other = first
		}
		left, right, nested, err := m.SequenceConcatParts(other)
		if err != nil {
			return err
		}
		if !nested {
			continue
		}
		literal, rest := left, right
		if i == 1 {
			literal, rest = right, left
		}
		if _, isLiteral, err := m.SequenceStringValue(literal); err != nil {
			return err
		} else if !isLiteral {
			continue
		}
		if err := t.foldParentBoundary(member, constant, literal, rest, parent.Owner(), i == 0, i == 0); err != nil {
			return err
		}
	}
	// Cases 5/6 look one parent level further. Newly introduced equality
	// parents are not concats, so they add no new folding candidates.
	for _, trailingLiteral := range []bool{true, false} {
		for _, grandparent := range parent.Parents() {
			left, right, outerConcat, err := m.SequenceConcatParts(grandparent.Owner())
			if err != nil {
				return err
			}
			if !outerConcat {
				continue
			}
			if trailingLiteral && second == member && left == parent.Owner() {
				if _, literal, err := m.SequenceStringValue(right); err != nil {
					return err
				} else if literal {
					if err := t.foldParentBoundary(member, constant, right, first, grandparent.Owner(), true, false); err != nil {
						return err
					}
				}
			}
			if !trailingLiteral && first == member && right == parent.Owner() {
				if _, literal, err := m.SequenceStringValue(left); err != nil {
					return err
				} else if literal {
					if err := t.foldParentBoundary(member, constant, left, second, grandparent.Owner(), false, true); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// Constant byte order and rebuilt operand order are separate in the
// nested-operand and grandparent cases.
func (t *SMTStringTheory) foldParentBoundary(member, constant, literal, rest, parent ast.ASTHandle, constantFirst, combinedFirst bool) (result error) {
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	left, right := constant, literal
	if !constantFirst {
		left, right = literal, constant
	}
	combined, err := t.evalStringConcat(left, right)
	if err != nil {
		return err
	}
	if combined.IsNull() {
		return ast.ErrASTDomain
	}
	if err := locals.add(combined); err != nil {
		return err
	}
	left, right = combined, rest
	if !combinedFirst {
		left, right = rest, combined
	}
	simplified, err := t.concatTerm(left, right)
	if err != nil {
		return err
	}
	if err := locals.add(simplified); err != nil {
		return err
	}
	return t.publishParentSimplification(member, constant, member, member, parent, simplified)
}

func (t *SMTStringTheory) publishParentSimplification(member, constant, other, otherValue, parent, simplified ast.ASTHandle) (result error) {
	same, err := t.sameStringClass(parent, simplified)
	if err != nil || same {
		return err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	// Native guards the pending member equality even if it is reflexive.
	equality, err := t.state.manager.MkEq(member, constant)
	if err != nil {
		return err
	}
	if err := premises.add(equality); err != nil {
		return err
	}
	if err := premises.equate(other, otherValue); err != nil {
		return err
	}
	conclusion, err := t.state.manager.MkEq(parent, simplified)
	if err != nil {
		return err
	}
	return premises.imply(conclusion)
}

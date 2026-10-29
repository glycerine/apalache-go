package smtcore

// Source: src/smt/smt_conflict_resolution.cpp:58-248 and the corresponding
// public marking operations in smt_conflict_resolution.h.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

type SMTEnodeEquality struct{ First, Second *SMTEnode }
type SMTCongruenceUse struct{ First, Second *SMTEnode }
type SMTEqualityUse struct{ First, Second, Common *SMTEnode }
type smtEnodePairKey struct{ first, second *SMTEnode }

type SMTAntecedentExpander struct {
	self            *SMTAntecedentExpander
	enodeLiterals   map[*SMTEnode]SMTLiteral
	antecedents     []SMTLiteral
	justifications  []SMTJustification
	justificationAt int
	equalities      []SMTEnodeEquality
	processed       map[smtEnodePairKey]struct{}
	congruenceUses  []SMTCongruenceUse
	equalityUses    []SMTEqualityUse
	active          bool
}

func NewSMTAntecedentExpander() *SMTAntecedentExpander {
	e := &SMTAntecedentExpander{enodeLiterals: make(map[*SMTEnode]SMTLiteral), processed: make(map[smtEnodePairKey]struct{})}
	e.self = e
	return e
}

func (e *SMTAntecedentExpander) valid() error {
	if e == nil || e.self != e || e.enodeLiterals == nil || e.processed == nil {
		return ast.ErrASTHandle
	}
	return nil
}
func (e *SMTAntecedentExpander) SetENodeLiteral(node *SMTEnode, literal SMTLiteral) error {
	if err := e.valid(); err != nil {
		return err
	}
	if node == nil || literal == SMTNullLiteral {
		return ast.ErrASTDomain
	}
	e.enodeLiterals[node] = literal
	return nil
}
func (e *SMTAntecedentExpander) LiteralForENode(node *SMTEnode) (SMTLiteral, error) {
	if err := e.valid(); err != nil {
		return SMTNullLiteral, err
	}
	literal, found := e.enodeLiterals[node]
	if !found {
		if node != nil {
			return SMTNullLiteral, fmt.Errorf("missing Boolean enode literal for %v: %w", node.owner, ast.ErrASTDomain)
		}
		return SMTNullLiteral, ast.ErrASTDomain
	}
	return literal, nil
}
func (e *SMTAntecedentExpander) MarkLiteral(literal SMTLiteral) error {
	if !e.active || literal == SMTNullLiteral {
		return ast.ErrASTDomain
	}
	e.antecedents = append(e.antecedents, literal)
	return nil
}
func (e *SMTAntecedentExpander) MarkJustification(justification SMTJustification) error {
	if !e.active || justification == nil {
		return ast.ErrASTDomain
	}
	if justification.IsMarked() {
		return nil
	}
	if err := justification.SetMarked(true); err != nil {
		return err
	}
	e.justifications = append(e.justifications, justification)
	return nil
}
func (e *SMTAntecedentExpander) canonicalPair(first, second *SMTEnode) (smtEnodePairKey, error) {
	if first == nil || second == nil || first.manager == nil || first.manager != second.manager {
		return smtEnodePairKey{}, ast.ErrASTDomain
	}
	firstID, err := first.OwnerID()
	if err != nil {
		return smtEnodePairKey{}, err
	}
	secondID, err := second.OwnerID()
	if err != nil {
		return smtEnodePairKey{}, err
	}
	if firstID == secondID {
		return smtEnodePairKey{}, ast.ErrASTDomain
	}
	if firstID > secondID {
		first, second = second, first
	}
	return smtEnodePairKey{first, second}, nil
}
func (e *SMTAntecedentExpander) MarkEquality(first, second *SMTEnode) error {
	if !e.active {
		return ast.ErrASTDomain
	}
	if first == second {
		return nil
	}
	key, err := e.canonicalPair(first, second)
	if err != nil {
		return err
	}
	if _, found := e.processed[key]; found {
		return nil
	}
	e.processed[key] = struct{}{}
	e.equalities = append(e.equalities, SMTEnodeEquality{key.first, key.second})
	return nil
}
func (e *SMTAntecedentExpander) MarkJustifiedEquality(first, second *SMTEnode, justification SMTEqualityJustification) error {
	if !e.active {
		return ast.ErrASTDomain
	}
	return e.expandEqualityJustification(first, second, justification)
}

func (e *SMTAntecedentExpander) expandEqualityJustification(first, second *SMTEnode, justification SMTEqualityJustification) error {
	switch justification.Kind() {
	case SMTEqualityJustificationAxiom:
		return nil
	case SMTEqualityJustificationEquation:
		literal, ok := justification.Literal()
		if !ok {
			return ast.ErrASTDomain
		}
		return e.MarkLiteral(literal)
	case SMTEqualityJustificationObject:
		object, ok := justification.Justification()
		if !ok || object == nil {
			return ast.ErrASTDomain
		}
		return e.MarkJustification(object)
	case SMTEqualityJustificationCongruence:
		if first == nil || second == nil || len(first.arguments) != len(second.arguments) {
			return ast.ErrASTDomain
		}
		e.congruenceUses = append(e.congruenceUses, SMTCongruenceUse{first, second})
		commuted, ok := justification.UsedCommutativity()
		if !ok {
			return ast.ErrASTDomain
		}
		if commuted {
			if len(first.arguments) != 2 {
				return ast.ErrASTDomain
			}
			if err := e.MarkEquality(first.arguments[0], second.arguments[1]); err != nil {
				return err
			}
			return e.MarkEquality(first.arguments[1], second.arguments[0])
		}
		for i := range first.arguments {
			if err := e.MarkEquality(first.arguments[i], second.arguments[i]); err != nil {
				return err
			}
		}
		return nil
	default:
		return ast.ErrASTDomain
	}
}

func smtCommonTransAncestor(first, second *SMTEnode) (*SMTEnode, error) {
	if first == nil || second == nil || first.root == nil {
		return nil, ast.ErrASTDomain
	}
	if first.root != second.root {
		return nil, fmt.Errorf("distinct equality roots: %w", ast.ErrASTDomain)
	}
	if !first.TransReaches(first.root) || !second.TransReaches(second.root) {
		return nil, fmt.Errorf("broken equality proof path: %w", ast.ErrASTDomain)
	}
	seen := make(map[*SMTEnode]struct{})
	for current := first; current != nil; current = current.trans.Target {
		seen[current] = struct{}{}
	}
	for current := second; current != nil; current = current.trans.Target {
		if _, found := seen[current]; found {
			return current, nil
		}
	}
	return nil, ast.ErrASTDomain
}
func (e *SMTAntecedentExpander) expandBranch(node, ancestor *SMTEnode) error {
	if !node.TransReaches(ancestor) {
		return ast.ErrASTDomain
	}
	for node != ancestor {
		next := node.trans.Target
		if next == nil {
			return ast.ErrASTDomain
		}
		if err := e.expandEqualityJustification(node, next, node.trans.Justification); err != nil {
			return err
		}
		node = next
	}
	return nil
}
func (e *SMTAntecedentExpander) expandEquality(first, second *SMTEnode) error {
	common, err := smtCommonTransAncestor(first, second)
	if err != nil {
		return fmt.Errorf("find equality proof ancestor: %w", err)
	}
	if err = e.expandBranch(first, common); err != nil {
		return fmt.Errorf("expand first equality proof branch: %w", err)
	}
	if err = e.expandBranch(second, common); err != nil {
		return fmt.Errorf("expand second equality proof branch: %w", err)
	}
	e.equalityUses = append(e.equalityUses, SMTEqualityUse{first, second, common})
	return nil
}
func (e *SMTAntecedentExpander) process() error {
	for {
		for e.justificationAt < len(e.justifications) {
			justification := e.justifications[e.justificationAt]
			e.justificationAt++
			if err := justification.MarkAntecedents(e); err != nil {
				return err
			}
		}
		for len(e.equalities) != 0 {
			i := len(e.equalities) - 1
			equality := e.equalities[i]
			e.equalities = e.equalities[:i]
			if err := e.expandEquality(equality.First, equality.Second); err != nil {
				return err
			}
		}
		if e.justificationAt == len(e.justifications) {
			return nil
		}
	}
}
func (e *SMTAntecedentExpander) cleanup() error {
	var result error
	for _, justification := range e.justifications {
		if justification.IsMarked() {
			result = errors.Join(result, justification.SetMarked(false))
		}
	}
	e.antecedents, e.justifications, e.equalities = nil, nil, nil
	e.justificationAt = 0
	e.processed = make(map[smtEnodePairKey]struct{})
	e.active = false
	return result
}
func (e *SMTAntecedentExpander) ExpandJustification(justification SMTJustification) (result []SMTLiteral, err error) {
	if err = e.valid(); err != nil {
		return nil, err
	}
	if e.active || justification == nil {
		return nil, ast.ErrASTDomain
	}
	e.active = true
	defer func() { err = errors.Join(err, e.cleanup()) }()
	if err = e.MarkJustification(justification); err != nil {
		return nil, err
	}
	if err = e.process(); err != nil {
		return nil, err
	}
	return append([]SMTLiteral(nil), e.antecedents...), nil
}
func (e *SMTAntecedentExpander) ExpandEquality(first, second *SMTEnode) (result []SMTLiteral, err error) {
	if err = e.valid(); err != nil {
		return nil, err
	}
	if e.active {
		return nil, ast.ErrASTDomain
	}
	e.active = true
	defer func() { err = errors.Join(err, e.cleanup()) }()
	if err = e.MarkEquality(first, second); err != nil {
		return nil, err
	}
	if err = e.process(); err != nil {
		return nil, err
	}
	return append([]SMTLiteral(nil), e.antecedents...), nil
}
func (e *SMTAntecedentExpander) CongruenceUses() []SMTCongruenceUse {
	if e == nil {
		return nil
	}
	return append([]SMTCongruenceUse(nil), e.congruenceUses...)
}
func (e *SMTAntecedentExpander) EqualityUses() []SMTEqualityUse {
	if e == nil {
		return nil
	}
	return append([]SMTEqualityUse(nil), e.equalityUses...)
}
func (e *SMTAntecedentExpander) ResetUses() {
	if e != nil {
		e.congruenceUses = nil
		e.equalityUses = nil
	}
}

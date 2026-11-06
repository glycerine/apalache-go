package duality

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *Engine) counterexampleHasQuantifiedTheory() bool {
	tree := e.counterexample.Tree()
	if tree == nil {
		return false
	}
	return tree.hasMappedQuantifiedTheory()
}

func (g *RPFP) hasMappedQuantifiedTheory() bool {
	for _, edge := range g.edges {
		if edge != nil && edge.Map != nil && edge.Map.quantifiedTheory {
			return true
		}
	}
	return false
}

// validateQuantifiedCounterexample prevents the theory overapproximation used
// for positive quantified relation premises from publishing an unsupported SAT
// result. Every logical proof obligation is checked in a fresh, context-free
// solver and every asserted leaf must be an actual transformed source clause
// or configured background axiom.
func (e *Engine) validateQuantifiedCounterexample() (result error) {
	m := e.source.context.manager
	proof, err := e.counterexample.Proof(e.config.Factory, e.theory...)
	if err != nil {
		return fmt.Errorf("quantified counterexample proof construction: %w", err)
	}
	defer func() { result = errors.Join(result, m.DelRef(&proof)) }()
	h, err := m.RefGet(proof)
	if err != nil {
		return err
	}
	checker, err := proofs.NewChecker(m)
	if err != nil {
		return err
	}
	valid, side, err := checker.Check(h)
	defer func() { result = errors.Join(result, checker.ReleaseSideConditions(side)) }()
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("quantified counterexample structural proof: %w", ErrProofWitness)
	}
	closed, err := proofs.IsClosed(m, h)
	if err != nil {
		return err
	}
	if !closed {
		return fmt.Errorf("quantified counterexample open proof: %w", ErrProofWitness)
	}
	native, err := e.config.Factory.New(m, nil, false, false, false, "")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, native.Close()) }()
	for _, condition := range side {
		if err := native.Push(); err != nil {
			return err
		}
		negated, err := m.MkNot(condition)
		if err != nil {
			return err
		}
		if err := native.Assert(negated); err != nil {
			return err
		}
		status, err := native.CheckSat(nil)
		if err != nil {
			return err
		}
		if err := native.Pop(1); err != nil {
			return err
		}
		if status != lbool.False {
			return fmt.Errorf("quantified counterexample side condition: %w", ErrProofWitness)
		}
	}
	allowed := make(map[ast.ASTHandle]bool, len(e.source.sourceClauses)+len(e.theory))
	for _, ref := range e.source.sourceClauses {
		fact, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		allowed[fact] = true
	}
	for _, axiom := range e.theory {
		fact, err := axiom.Raw()
		if err != nil {
			return err
		}
		allowed[fact] = true
	}
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{h}
	for len(stack) > 0 {
		i := len(stack) - 1
		current := stack[i]
		stack = stack[:i]
		if seen[current] {
			continue
		}
		seen[current] = true
		asserted, err := m.IsProofKind(current, ast.ASTPrAsserted)
		if err != nil {
			return err
		}
		if asserted {
			fact, err := m.GetProofFact(current)
			if err != nil {
				return err
			}
			if !allowed[fact] {
				return fmt.Errorf("quantified counterexample asserted leaf provenance: %w", ErrProofWitness)
			}
		}
		parents, err := m.GetProofNumParents(current)
		if err != nil {
			return err
		}
		for j := uint32(0); j < parents; j++ {
			parent, err := m.GetProofParent(current, j)
			if err != nil {
				return err
			}
			stack = append(stack, parent)
		}
	}
	return nil
}

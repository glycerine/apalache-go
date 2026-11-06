package rewriter

// Source: src/ast/rewriter/elim_bounds.cpp;
// src/ast/rewriter/elim_bounds.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type elimBoundsPair struct {
	lower ast.ASTHandle
	upper ast.ASTHandle
}

// ElimBoundsConfiguration removes one-sided difference-logic bounds from
// universal quantifiers when the bounded variable occurs nowhere else.
type ElimBoundsConfiguration struct {
	DefaultConfiguration
	self    *ElimBoundsConfiguration
	manager *ast.ASTManager
	arith   ast.FamilyID
	closed  bool
}

func NewElimBoundsConfiguration(m *ast.ASTManager) (*ElimBoundsConfiguration, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	arith, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	c := &ElimBoundsConfiguration{manager: m, arith: arith}
	c.self = c
	c.defaults.manager = m
	return c, nil
}

func (c *ElimBoundsConfiguration) open() error {
	if c == nil || c.self != c || c.closed || c.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (c *ElimBoundsConfiguration) appKind(h ast.ASTHandle, family ast.FamilyID, kind int32) (bool, error) {
	k, err := c.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := c.manager.GetFamilyID(h)
	if err != nil || f != family {
		return false, err
	}
	op, err := c.manager.GetDeclKind(h)
	return op == kind, err
}

func (c *ElimBoundsConfiguration) arithmeticNumeral(h ast.ASTHandle) (bool, error) {
	_, _, matched, err := c.manager.ArithmeticNumeralText(h)
	return matched, err
}

func (c *ElimBoundsConfiguration) minusOne(h ast.ASTHandle) (bool, error) {
	text, integer, matched, err := c.manager.ArithmeticNumeralText(h)
	return matched && integer && text == "-1", err
}

func (c *ElimBoundsConfiguration) variable(h ast.ASTHandle) (bool, error) {
	kind, err := c.manager.GetKind(h)
	return kind == ast.ASTVar, err
}

func (c *ElimBoundsConfiguration) isBound(h ast.ASTHandle) (elimBoundsPair, bool, error) {
	if err := c.open(); err != nil {
		return elimBoundsPair{}, false, err
	}
	negated := false
	isNot, err := c.appKind(h, c.manager.BasicFamilyID(), ast.ASTOpNot)
	if err != nil {
		return elimBoundsPair{}, false, err
	}
	if isNot {
		if n, err := c.manager.GetNumArgs(h); err != nil {
			return elimBoundsPair{}, false, err
		} else if n != 1 {
			return elimBoundsPair{}, false, nil
		}
		h, err = c.manager.GetArg(h, 0)
		if err != nil {
			return elimBoundsPair{}, false, err
		}
		negated = true
	}

	le, err := c.appKind(h, c.arith, 2)
	if err != nil {
		return elimBoundsPair{}, false, err
	}
	ge := false
	if !le {
		ge, err = c.appKind(h, c.arith, 3)
		if err != nil {
			return elimBoundsPair{}, false, err
		}
	}
	if !le && !ge {
		return elimBoundsPair{}, false, nil
	}
	if n, err := c.manager.GetNumArgs(h); err != nil {
		return elimBoundsPair{}, false, err
	} else if n != 2 {
		return elimBoundsPair{}, false, nil
	}
	lhs, err := c.manager.GetArg(h, 0)
	if err != nil {
		return elimBoundsPair{}, false, err
	}
	rhs, err := c.manager.GetArg(h, 1)
	if err != nil {
		return elimBoundsPair{}, false, err
	}
	numeral, err := c.arithmeticNumeral(rhs)
	if err != nil || !numeral {
		return elimBoundsPair{}, false, err
	}

	var pair elimBoundsPair
	isVar, err := c.variable(lhs)
	if err != nil {
		return pair, false, err
	}
	if isVar {
		pair.upper = lhs
	} else {
		add, err := c.appKind(lhs, c.arith, 6)
		if err != nil || !add {
			return pair, false, err
		}
		if n, err := c.manager.GetNumArgs(lhs); err != nil {
			return pair, false, err
		} else if n != 2 {
			return pair, false, nil
		}
		first, err := c.manager.GetArg(lhs, 0)
		if err != nil {
			return pair, false, err
		}
		second, err := c.manager.GetArg(lhs, 1)
		if err != nil {
			return pair, false, err
		}
		if firstVar, err := c.variable(first); err != nil {
			return pair, false, err
		} else if firstVar {
			pair.upper = first
		} else if ground, err := c.manager.IsGround(first); err != nil {
			return pair, false, err
		} else if !ground {
			return pair, false, nil
		}
		mul, err := c.appKind(second, c.arith, 9)
		if err != nil || !mul {
			return pair, false, err
		}
		if n, err := c.manager.GetNumArgs(second); err != nil {
			return pair, false, err
		} else if n != 2 {
			return pair, false, nil
		}
		coefficient, err := c.manager.GetArg(second, 0)
		if err != nil {
			return pair, false, err
		}
		minusOne, err := c.minusOne(coefficient)
		if err != nil || !minusOne {
			return pair, false, err
		}
		term, err := c.manager.GetArg(second, 1)
		if err != nil {
			return pair, false, err
		}
		if termVar, err := c.variable(term); err != nil {
			return pair, false, err
		} else if termVar {
			pair.lower = term
		} else if ground, err := c.manager.IsGround(term); err != nil {
			return pair, false, err
		} else if !ground {
			return pair, false, nil
		}
	}

	lessEqual := le
	if negated {
		lessEqual = !lessEqual
	}
	if !lessEqual {
		pair.lower, pair.upper = pair.upper, pair.lower
	}
	return pair, true, nil
}

func (c *ElimBoundsConfiguration) atoms(body ast.ASTHandle) ([]ast.ASTHandle, error) {
	isOr, err := c.appKind(body, c.manager.BasicFamilyID(), ast.ASTOpOr)
	if err != nil {
		return nil, err
	}
	if !isOr {
		return []ast.ASTHandle{body}, nil
	}
	count, err := c.manager.GetNumArgs(body)
	if err != nil {
		return nil, err
	}
	atoms := make([]ast.ASTHandle, count)
	for i := range atoms {
		atoms[i], err = c.manager.GetArg(body, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return atoms, nil
}

func (c *ElimBoundsConfiguration) publishRewrite(q, value ast.ASTHandle, out, proof ast.ASTRef) error {
	if err := c.manager.SetRef(out, value); err != nil {
		return err
	}
	p, err := c.manager.MkProofRewrite(q, value)
	if err != nil {
		return err
	}
	return c.manager.SetRef(proof, p)
}

func (c *ElimBoundsConfiguration) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (result bool, err error) {
	if err = c.open(); err != nil {
		return false, err
	}
	forall, err := c.manager.IsForall(q)
	if err != nil || !forall {
		return false, err
	}
	declarations, err := c.manager.GetNumDeclarations(q)
	if err != nil {
		return false, err
	}
	atoms, err := c.atoms(body)
	if err != nil {
		return false, err
	}
	used, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return false, err
	}
	for _, atom := range atoms {
		if _, bound, x := c.isBound(atom); x != nil {
			return false, x
		} else if !bound {
			if x = used.Process(atom); x != nil {
				return false, x
			}
		}
	}
	if used.UsesAllVars(declarations) {
		return false, nil
	}

	lowers := make(map[ast.ASTHandle]struct{})
	uppers := make(map[ast.ASTHandle]struct{})
	candidateSet := make(map[ast.ASTHandle]struct{})
	candidates := make([]ast.ASTHandle, 0)
	addCandidate := func(v ast.ASTHandle) {
		if _, low := lowers[v]; low {
			return
		}
		if _, up := uppers[v]; up {
			return
		}
		candidateSet[v] = struct{}{}
		candidates = append(candidates, v)
	}
	for _, atom := range atoms {
		pair, bound, x := c.isBound(atom)
		if x != nil {
			return false, x
		}
		if !bound {
			continue
		}
		if pair.lower != (ast.ASTHandle{}) {
			index, x := c.manager.GetVarIndex(pair.lower)
			if x != nil {
				return false, x
			}
			if used.Contains(index) == (ast.ASTHandle{}) && index < declarations {
				addCandidate(pair.lower)
				lowers[pair.lower] = struct{}{}
			}
		}
		if pair.upper != (ast.ASTHandle{}) {
			index, x := c.manager.GetVarIndex(pair.upper)
			if x != nil {
				return false, x
			}
			if used.Contains(index) == (ast.ASTHandle{}) && index < declarations {
				addCandidate(pair.upper)
				uppers[pair.upper] = struct{}{}
			}
		}
	}
	for _, v := range candidates {
		_, low := lowers[v]
		_, up := uppers[v]
		if low && up {
			delete(candidateSet, v)
		}
	}
	if len(candidateSet) == 0 {
		return false, nil
	}

	kept := make([]ast.ASTHandle, 0, len(atoms))
	for _, atom := range atoms {
		pair, bound, x := c.isBound(atom)
		if x != nil {
			return false, x
		}
		if bound {
			_, removeLower := candidateSet[pair.lower]
			_, removeUpper := candidateSet[pair.upper]
			if removeLower || removeUpper {
				continue
			}
		}
		kept = append(kept, atom)
	}
	if len(kept) == len(atoms) {
		return false, nil
	}
	if len(kept) == 0 {
		value, x := c.manager.MkFalse()
		if x != nil {
			return false, x
		}
		return true, c.publishRewrite(q, value, out, proof)
	}
	newBody := kept[0]
	if len(kept) > 1 {
		newBody, err = c.manager.MkOrArray(kept)
		if err != nil {
			return false, err
		}
	}
	updated, err := c.manager.UpdateQuantifierBody(q, newBody)
	if err != nil {
		return false, err
	}
	updatedRef, err := c.manager.NewRef(updated)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, c.manager.DelRef(&updatedRef)) }()
	eliminated, err := ElimUnusedVarsUFBV(c.manager, updated)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, c.manager.DelRef(&eliminated)) }()
	value, err := c.manager.RefGet(eliminated)
	if err != nil {
		return false, err
	}
	return true, c.publishRewrite(q, value, out, proof)
}

func (c *ElimBoundsConfiguration) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

// ElimBounds is the complete source elim_bounds_rw traversal owner.
type ElimBounds struct {
	self      *ElimBounds
	config    *ElimBoundsConfiguration
	traversal *Traversal
	closed    bool
}

func NewElimBounds(m *ast.ASTManager) (*ElimBounds, error) {
	c, err := NewElimBoundsConfiguration(m)
	if err != nil {
		return nil, err
	}
	r := &ElimBounds{config: c}
	r.self = r
	r.traversal, err = NewTraversal(m, c)
	return r, err
}

func (r *ElimBounds) Rewrite(root ast.ASTHandle, out ast.ASTRef) error {
	if r == nil || r.self != r || r.closed || r.traversal == nil {
		return ast.ErrASTHandle
	}
	return r.traversal.Rewrite(root, out)
}

func (r *ElimBounds) RewriteWithProof(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if r == nil || r.self != r || r.closed || r.traversal == nil || r.config == nil {
		return ast.ErrASTHandle
	}
	if err := r.traversal.Rewrite(root, out); err != nil {
		return err
	}
	proofs, err := r.config.manager.ProofsEnabled()
	if err != nil || !proofs {
		return err
	}
	value, err := r.config.manager.RefGet(out)
	if err != nil {
		return err
	}
	var p ast.ASTHandle
	if value == root {
		p, err = r.config.manager.MkProofReflexivity(root)
	} else {
		p, err = r.config.manager.MkProofRewrite(root, value)
	}
	if err != nil {
		return err
	}
	return r.config.manager.SetRef(proof, p)
}

func (r *ElimBounds) Close() (result error) {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		result = errors.Join(result, r.traversal.Close())
	}
	if r.config != nil {
		result = errors.Join(result, r.config.Close())
	}
	r.closed = true
	return result
}

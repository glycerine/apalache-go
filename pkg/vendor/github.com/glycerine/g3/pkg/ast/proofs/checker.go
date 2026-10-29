package proofs

// Source: src/ast/proofs/proof_checker.cpp/.h.
// Package proofs contains the retained proof validation algorithms layered on
// top of ast_manager. It deliberately uses ASTManager's public surface so the
// package remains a leaf of ast.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// ReleaseSideConditions releases handles returned by Check.
func (c *Checker) ReleaseSideConditions(side []ast.ASTHandle) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	var result error
	for _, h := range side {
		result = errors.Join(result, c.manager.DecRef(h))
	}
	return result
}

// Checker validates proof DAGs produced by ASTManager.
type Checker struct {
	manager *ast.ASTManager
}

func NewChecker(m *ast.ASTManager) (*Checker, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofMode(); err != nil {
		return nil, err
	}
	return &Checker{manager: m}, nil
}

// Check validates every reachable proof once. A structurally invalid proof is
// reported as (false, sideConditions, nil); manager/lifetime failures are
// returned as errors.
// Check returns owning side-condition handles. The caller must call DecRef on
// each returned handle, including when the proof is invalid.
func (c *Checker) Check(proof ast.ASTHandle) (valid bool, side []ast.ASTHandle, result error) {
	if c == nil || c.manager == nil {
		return false, nil, ast.ErrASTHandle
	}
	defer func() {
		owned := 0
		for i := range side {
			if err := c.manager.IncRef(side[i]); err != nil {
				for j := 0; j < owned; j++ {
					_ = c.manager.DecRef(side[j])
				}
				side = nil
				valid = false
				result = err
				return
			}
			owned++
		}
	}()
	todo := []ast.ASTHandle{proof}
	marked := make(map[ast.ASTHandle]struct{})
	for len(todo) != 0 {
		last := len(todo) - 1
		p := todo[last]
		todo = todo[:last]
		if _, found := marked[p]; found {
			continue
		}
		marked[p] = struct{}{}
		parents, err := c.parents(p)
		if err != nil {
			return false, side, err
		}
		todo = append(todo, parents...)
		ok, added, err := c.checkBasic(p, parents)
		if err != nil || !ok {
			return ok, append(side, added...), err
		}
		side = append(side, added...)
	}
	return true, side, nil
}

func (c *Checker) parents(p ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := c.manager.GetProofNumParents(p)
	if err != nil {
		return nil, err
	}
	r := make([]ast.ASTHandle, 0, n)
	for i := uint32(0); i < n; i++ {
		q, e := c.manager.GetProofParent(p, i)
		if e != nil {
			return nil, e
		}
		r = append(r, q)
	}
	return r, nil
}

func (c *Checker) fact(p ast.ASTHandle) (ast.ASTHandle, bool, error) {
	yes, err := c.manager.ProofHasFact(p)
	if err != nil || !yes {
		return ast.ASTHandle{}, false, err
	}
	f, err := c.manager.GetProofFact(p)
	return f, err == nil, err
}

func (c *Checker) app(e ast.ASTHandle) (ast.ASTHandle, []ast.ASTHandle, bool, error) {
	kind, err := c.manager.GetKind(e)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, nil, false, err
	}
	d, err := c.manager.GetDecl(e)
	if err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	n, err := c.manager.GetNumArgs(e)
	if err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	args := make([]ast.ASTHandle, 0, n)
	for i := uint32(0); i < n; i++ {
		a, e := c.manager.GetArg(e, i)
		if e != nil {
			return ast.ASTHandle{}, nil, false, e
		}
		args = append(args, a)
	}
	return d, args, true, nil
}

func (c *Checker) op(e ast.ASTHandle, op int32, arity int) ([]ast.ASTHandle, bool, error) {
	fid, err := c.manager.GetFamilyID(e)
	if err != nil || fid != c.manager.BasicFamilyID() {
		return nil, false, err
	}
	k, err := c.manager.GetDeclKind(e)
	if err != nil || k != op {
		return nil, false, err
	}
	_, args, yes, err := c.app(e)
	if err != nil || !yes || arity >= 0 && len(args) != arity {
		return nil, false, err
	}
	return args, true, nil
}

func (c *Checker) binary(e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error) {
	d, args, yes, err := c.app(e)
	if err != nil || !yes || len(args) != 2 {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return d, args[0], args[1], true, nil
}

func (c *Checker) equiv(e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	for _, k := range [...]int32{ast.ASTOpOEq, ast.ASTOpEq, ast.ASTOpIff} {
		args, yes, err := c.op(e, k, 2)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if yes {
			return args[0], args[1], true, nil
		}
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}

func (c *Checker) negated(a, b ast.ASTHandle) (bool, error) {
	for _, pair := range [][2]ast.ASTHandle{{a, b}, {b, a}} {
		args, yes, err := c.op(pair[0], ast.ASTOpNot, 1)
		if err != nil {
			return false, err
		}
		if yes && args[0] == pair[1] {
			return true, nil
		}
	}
	return false, nil
}

func sameEquivalence(c *Checker, a, b ast.ASTHandle) (bool, error) {
	if a == b {
		return true, nil
	}
	ka, err := c.manager.GetDeclKind(a)
	if err != nil {
		return false, err
	}
	kb, err := c.manager.GetDeclKind(b)
	if err != nil {
		return false, err
	}
	is := func(k int32) bool { return k == ast.ASTOpEq || k == ast.ASTOpIff }
	return is(ka) && is(kb), nil
}

func (c *Checker) sideOr(fact ast.ASTHandle, parents []ast.ASTHandle) (ast.ASTHandle, error) {
	terms := []ast.ASTHandle{fact}
	for _, p := range parents {
		f, yes, err := c.fact(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !yes {
			continue
		}
		n, err := c.manager.MkNot(f)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		terms = append(terms, n)
	}
	return c.manager.MkOrArray(terms)
}

func invalid(err error) (bool, []ast.ASTHandle, error) {
	if errors.Is(err, ast.ErrASTDomain) {
		return false, nil, nil
	}
	return false, nil, err
}

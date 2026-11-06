package qe

// Source: src/qe/qe_arith_plugin.cpp arith_plugin.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

type arithmeticPluginKey struct{ variable, formula ast.ASTHandle }

// ArithmeticPlugin is the retained linear-arithmetic QE plugin.  It owns all
// cached roots, bounds processors, branch results and emitted constraints.
type ArithmeticPlugin struct {
	self            *ArithmeticPlugin
	context         *SolverContext
	manager         *ast.ASTManager
	util            *ArithmeticQETermUtil
	replacer        *astrewriter.ExprSafeReplace
	family          ast.FamilyID
	bounds          map[arithmeticPluginKey]*arithmeticPluginBounds
	branches        map[ArithmeticBranchKey]*ArithmeticPluginBranch
	constraints     []ArithmeticConstraint
	pendingFresh    []ast.ASTRef
	pendingEquation *ArithmeticEquationElimination
	normalizedAtoms []ast.ASTRef
	closed          bool
}

func NewArithmeticPlugin(context *SolverContext) (*ArithmeticPlugin, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	m, err := context.Manager()
	if err != nil {
		return nil, err
	}
	u, err := NewArithmeticQETermUtil(m)
	if err != nil {
		return nil, err
	}
	r, err := astrewriter.NewExprSafeReplace(m)
	if err != nil {
		_ = u.Close()
		return nil, err
	}
	p := &ArithmeticPlugin{context: context, manager: m, util: u, replacer: r,
		family: u.family, bounds: make(map[arithmeticPluginKey]*arithmeticPluginBounds),
		branches: make(map[ArithmeticBranchKey]*ArithmeticPluginBranch)}
	p.self = p
	return p, nil
}

func (p *ArithmeticPlugin) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	if p.context == nil || p.manager == nil || p.util == nil || p.replacer == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (p *ArithmeticPlugin) FamilyID() ast.FamilyID {
	if p == nil {
		return ast.NullFamilyID
	}
	return p.family
}

func (p *ArithmeticPlugin) Close() (result error) {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	for key, entry := range p.bounds {
		result = errors.Join(result, entry.close(p.manager))
		delete(p.bounds, key)
	}
	for key, branch := range p.branches {
		result = errors.Join(result, branch.close(p.manager))
		delete(p.branches, key)
	}
	for i := len(p.normalizedAtoms) - 1; i >= 0; i-- {
		result = errors.Join(result, p.manager.DelRef(&p.normalizedAtoms[i]))
	}
	p.normalizedAtoms = nil
	result = errors.Join(result, p.clearConstraints(), p.clearPendingFresh(), p.discardPendingEquation(), p.replacer.Close(), p.util.Close())
	if result == nil {
		p.closed = true
		p.context = nil
		p.manager = nil
		p.replacer = nil
		p.util = nil
	}
	return result
}

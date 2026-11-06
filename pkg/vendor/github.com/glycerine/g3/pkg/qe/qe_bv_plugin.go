package qe

// Source: src/qe/qe_bv_plugin.cpp constructor and factory.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// BitVectorPlugin owns its safe replacer and borrows its solver context.
type BitVectorPlugin struct {
	self     *BitVectorPlugin
	context  *SolverContext
	manager  *ast.ASTManager
	replacer *astrewriter.ExprSafeReplace
	family   ast.FamilyID
	closed   bool
}

func NewBitVectorPlugin(context *SolverContext) (*BitVectorPlugin, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	manager, err := context.Manager()
	if err != nil {
		return nil, err
	}
	// Source constructs bv_util over an already registered declaration plugin;
	// it only obtains the existing family id here.
	family, err := manager.BitVectorUtilityFamilyID()
	if err != nil {
		return nil, err
	}
	replacer, err := astrewriter.NewExprSafeReplace(manager)
	if err != nil {
		return nil, err
	}
	p := &BitVectorPlugin{context: context, manager: manager, replacer: replacer, family: family}
	p.self = p
	return p, nil
}

func (p *BitVectorPlugin) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	if p.context == nil || p.manager == nil || p.replacer == nil || p.family == ast.NullFamilyID {
		return ast.ErrASTHandle
	}
	return nil
}

func (p *BitVectorPlugin) FamilyID() ast.FamilyID {
	if p == nil {
		return ast.NullFamilyID
	}
	return p.family
}

func (p *BitVectorPlugin) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.replacer != nil {
		if err := p.replacer.Close(); err != nil {
			return err
		}
		p.replacer = nil
	}
	p.closed = true
	return nil
}

var _ SolverPlugin = (*BitVectorPlugin)(nil)

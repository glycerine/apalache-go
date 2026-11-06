package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

type nlPluginKey struct{ variable, formula ast.ASTHandle }
type nlPluginEntry struct {
	variable, formula ast.ASTRef
	branches          *NLBranchConditions
}

func (e *nlPluginEntry) close(m *ast.ASTManager) error {
	return errors.Join(e.branches.Close(), m.DelRef(&e.formula), m.DelRef(&e.variable))
}

type NonlinearArithmeticPlugin struct {
	self          *NonlinearArithmeticPlugin
	context       *SolverContext
	manager       *ast.ASTManager
	util          *NonlinearArithmeticUtil
	replacer      *astrewriter.ExprSafeReplace
	factor        *astrewriter.FactorRewriter
	weights       *NLWeightCache
	family        ast.FamilyID
	produceModels bool
	cache         map[nlPluginKey]*nlPluginEntry
	constraints   []ArithmeticConstraint
	closed        bool
}

func NewNonlinearArithmeticPlugin(context *SolverContext, produceModels bool) (*NonlinearArithmeticPlugin, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	m, err := context.Manager()
	if err != nil {
		return nil, err
	}
	u, err := NewNonlinearArithmeticUtil(m)
	if err != nil {
		return nil, err
	}
	if err = u.SetEnableLinear(true); err != nil {
		_ = u.Close()
		return nil, err
	}
	r, err := astrewriter.NewExprSafeReplace(m)
	if err != nil {
		_ = u.Close()
		return nil, err
	}
	f, err := astrewriter.NewFactorRewriter(m)
	if err != nil {
		_ = r.Close()
		_ = u.Close()
		return nil, err
	}
	w, err := NewNLWeightCache(m, true)
	if err != nil {
		_ = f.Close()
		_ = r.Close()
		_ = u.Close()
		return nil, err
	}
	p := &NonlinearArithmeticPlugin{context: context, manager: m, util: u, replacer: r, factor: f, weights: w, family: u.family, produceModels: produceModels, cache: make(map[nlPluginKey]*nlPluginEntry)}
	p.self = p
	return p, nil
}
func (p *NonlinearArithmeticPlugin) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	return nil
}
func (p *NonlinearArithmeticPlugin) FamilyID() ast.FamilyID {
	if p == nil {
		return ast.NullFamilyID
	}
	return p.family
}
func (p *NonlinearArithmeticPlugin) MkAtom(ast.ASTHandle, bool) (ast.ASTHandle, bool, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	return ast.ASTHandle{}, false, nil
}
func (p *NonlinearArithmeticPlugin) Weight(variable *ast.ContainsApp, formula ast.ASTHandle) (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	if variable == nil {
		return 0, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return 0, err
	}
	return p.weights.Weight(target, formula)
}
func (p *NonlinearArithmeticPlugin) Solve(*ConjunctionEnum, ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return false, nil
}
func (p *NonlinearArithmeticPlugin) Close() (result error) {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	for key, e := range p.cache {
		result = errors.Join(result, e.close(p.manager))
		delete(p.cache, key)
	}
	for i := len(p.constraints) - 1; i >= 0; i-- {
		result = errors.Join(result, p.manager.DelRef(&p.constraints[i].Formula))
	}
	result = errors.Join(result, p.weights.Close(), p.factor.Close(), p.replacer.Close(), p.util.Close())
	if result == nil {
		p.closed = true
	}
	return result
}

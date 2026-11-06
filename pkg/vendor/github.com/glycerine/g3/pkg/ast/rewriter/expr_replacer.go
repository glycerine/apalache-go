package rewriter

// Source: src/ast/rewriter/expr_replacer.cpp:24-112;
// src/ast/rewriter/expr_replacer.h:27-56.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type defaultExprReplacerConfig struct {
	defaultRewriterConfig
	substitution ast.ASTExprSubstitution
	hasSubst     bool
	used         ast.ASTDependency
}

// SimplifyingExprReplacer is the source th_rewriter2expr_replacer adapter. It
// owns the theory rewriter and transfers each accumulated dependency result to
// its caller.
type SimplifyingExprReplacer struct {
	self     *SimplifyingExprReplacer
	rewriter *TheoryRewriter
	closed   bool
}

func NewSimplifyingExprReplacer(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) (*SimplifyingExprReplacer, error) {
	r := &SimplifyingExprReplacer{}
	r.self = r
	var err error
	r.rewriter, err = NewTheoryRewriter(m, context, globals, params)
	return r, err
}

func (r *SimplifyingExprReplacer) open() error {
	if r == nil || r.self != r || r.closed || r.rewriter == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (r *SimplifyingExprReplacer) Manager() *ast.ASTManager {
	if r.open() != nil {
		return nil
	}
	return r.rewriter.Manager()
}

func (r *SimplifyingExprReplacer) SetSubstitution(substitution *ast.ASTExprSubstitution) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.rewriter.SetSubstitution(substitution)
}

func (r *SimplifyingExprReplacer) Rewrite(term ast.ASTHandle, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.rewriter.Rewrite(term, out)
}

func (r *SimplifyingExprReplacer) RewriteWithProof(term ast.ASTHandle, out, proof ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.rewriter.RewriteWithProof(term, out, proof)
}

func (r *SimplifyingExprReplacer) RewriteWithProofAndDependency(term ast.ASTHandle, out, proof ast.ASTRef) (ast.ASTDependency, error) {
	if err := r.RewriteWithProof(term, out, proof); err != nil {
		return ast.ASTDependency{}, err
	}
	return r.transferDependency()
}

// RewriteWithDependency returns an owning dependency reference. The caller
// releases a nonnull result with ASTManager.DependencyDec.
func (r *SimplifyingExprReplacer) RewriteWithDependency(term ast.ASTHandle, out ast.ASTRef) (ast.ASTDependency, error) {
	if err := r.Rewrite(term, out); err != nil {
		return ast.ASTDependency{}, err
	}
	return r.transferDependency()
}

func (r *SimplifyingExprReplacer) transferDependency() (ast.ASTDependency, error) {
	dependency, err := r.rewriter.UsedDependencies()
	if err != nil || dependency.IsNull() {
		return dependency, err
	}
	manager := r.rewriter.Manager()
	if err = manager.DependencyInc(dependency); err != nil {
		return ast.ASTDependency{}, err
	}
	if err = r.rewriter.ResetUsedDependencies(); err != nil {
		_ = manager.DependencyDec(dependency)
		return ast.ASTDependency{}, err
	}
	return dependency, nil
}

func (r *SimplifyingExprReplacer) ApplySubstitution(source, definition, proof ast.ASTHandle, target ast.ASTRef) (result error) {
	if err := r.open(); err != nil {
		return err
	}
	m := r.rewriter.Manager()
	substitution, err := m.NewExprSubstitution()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelExprSubstitution(&substitution)) }()
	if err = m.ExprSubstitutionInsert(substitution, source, definition, proof, ast.ASTDependency{}); err != nil {
		return err
	}
	if err = r.SetSubstitution(&substitution); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.SetSubstitution(nil)) }()
	term, err := m.RefGet(target)
	if err != nil {
		return err
	}
	return r.Rewrite(term, target)
}

func (r *SimplifyingExprReplacer) NumSteps() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.rewriter.NumSteps()
}

func (r *SimplifyingExprReplacer) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	return r.rewriter.Reset()
}

func (r *SimplifyingExprReplacer) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.rewriter != nil {
		if err := r.rewriter.Close(); err != nil {
			return err
		}
		r.rewriter = nil
	}
	r.closed = true
	return nil
}

func (c *defaultExprReplacerConfig) maxStepsExceeded(uint32) (bool, error) {
	// The source calls cooperate("simplifier") and never imposes a step limit.
	return false, nil
}

func (c *defaultExprReplacerConfig) getSubst(t ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	if !c.hasSubst {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	var value, proof ast.ASTHandle
	var dependency ast.ASTDependency
	found, err := c.manager.ExprSubstitutionFindDependency(c.substitution, t, &value, &proof, &dependency)
	if err != nil || !found || dependency.IsNull() {
		return value, proof, found, err
	}
	joined, err := c.manager.DependencyJoin(c.used, dependency)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if err = c.manager.DependencyInc(joined); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if !c.used.IsNull() {
		if err = c.manager.DependencyDec(c.used); err != nil {
			_ = c.manager.DependencyDec(joined)
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
	}
	c.used = joined
	return value, proof, true, nil
}

// DefaultExprReplacer is the source default_expr_replacer specialization. It
// borrows an optional expression substitution and owns its traversal cache and
// accumulated dependency reference.
type DefaultExprReplacer struct {
	engine rewriterEngine
	cfg    defaultExprReplacerConfig
}

func NewDefaultExprReplacer(m *ast.ASTManager) (*DefaultExprReplacer, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(DefaultExprReplacer{}).Size())
	if err := m.BudgetField.Reserve(n); err != nil {
		return nil, err
	}
	r := &DefaultExprReplacer{}
	r.cfg.manager = m
	r.engine.header = n
	proofs, err := m.ProofsEnabled()
	if err != nil {
		m.BudgetField.Release(n)
		return nil, err
	}
	if err := r.engine.initMode(m, &r.cfg, proofs); err != nil {
		// initMode may already own caches, shifters and result references.
		// Its Close releases both those partial children and the header.
		return nil, errors.Join(err, r.Close())
	}
	return r, nil
}

func (r *DefaultExprReplacer) Manager() *ast.ASTManager {
	if r == nil {
		return nil
	}
	return r.cfg.manager
}

func (r *DefaultExprReplacer) releaseUsedDependency() error {
	if r.cfg.used.IsNull() {
		return nil
	}
	d := r.cfg.used
	r.cfg.used = ast.ASTDependency{}
	return r.cfg.manager.DependencyDec(d)
}

// SetSubstitution performs the source cleanup before changing the borrowed
// substitution. A nil argument clears it.
func (r *DefaultExprReplacer) SetSubstitution(s *ast.ASTExprSubstitution) error {
	if r == nil || r.cfg.manager == nil {
		return ast.ErrASTHandle
	}
	if s != nil {
		m, err := r.cfg.manager.ExprSubstitutionManager(*s)
		if err != nil {
			return err
		}
		if m != r.cfg.manager {
			return ast.ErrASTHandle
		}
	}
	if err := r.releaseUsedDependency(); err != nil {
		return err
	}
	if err := r.engine.Cleanup(); err != nil {
		return err
	}
	r.cfg.hasSubst = s != nil
	if s == nil {
		r.cfg.substitution = ast.ASTExprSubstitution{}
		return nil
	}
	r.cfg.substitution = *s
	return nil
}

// Rewrite applies the current substitution and leaves dependencies owned by
// the replacer until Reset, SetSubstitution, Close, or RewriteWithDependency.
func (r *DefaultExprReplacer) Rewrite(t ast.ASTHandle, out ast.ASTRef) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Rewrite(t, out)
}

func (r *DefaultExprReplacer) RewriteWithProof(t ast.ASTHandle, out, proof ast.ASTRef) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.RewriteWithProof(t, out, proof)
}

func (r *DefaultExprReplacer) RewriteWithProofAndDependency(t ast.ASTHandle, out, proof ast.ASTRef) (ast.ASTDependency, error) {
	if err := r.RewriteWithProof(t, out, proof); err != nil {
		return ast.ASTDependency{}, err
	}
	return r.transferDependency()
}

// RewriteWithDependency transfers the accumulated dependency reference to the
// caller. A nonnull result is owning and must be released with DependencyDec.
func (r *DefaultExprReplacer) RewriteWithDependency(t ast.ASTHandle, out ast.ASTRef) (ast.ASTDependency, error) {
	if err := r.Rewrite(t, out); err != nil {
		return ast.ASTDependency{}, err
	}
	return r.transferDependency()
}

func (r *DefaultExprReplacer) transferDependency() (ast.ASTDependency, error) {
	d := r.cfg.used
	r.cfg.used = ast.ASTDependency{}
	if !d.IsNull() {
		if err := r.engine.Reset(); err != nil {
			r.cfg.used = d
			return ast.ASTDependency{}, err
		}
	}
	return d, nil
}

func (r *DefaultExprReplacer) NumSteps() (uint32, error) {
	if r == nil || r.cfg.manager == nil {
		return 0, ast.ErrASTHandle
	}
	if err := r.engine.open(); err != nil {
		return 0, err
	}
	return r.engine.numSteps, nil
}

func (r *DefaultExprReplacer) Reset() error {
	if r == nil {
		return ast.ErrASTHandle
	}
	if err := r.releaseUsedDependency(); err != nil {
		return err
	}
	return r.engine.Reset()
}

// ApplySubstitution installs a one-entry substitution for one rewrite and then
// clears it, matching expr_replacer::scoped_set_subst.
func (r *DefaultExprReplacer) ApplySubstitution(source, definition, proof ast.ASTHandle, target ast.ASTRef) (result error) {
	if r == nil || r.cfg.manager == nil {
		return ast.ErrASTHandle
	}
	m := r.cfg.manager
	s, err := m.NewExprSubstitution()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelExprSubstitution(&s)) }()
	if err = m.ExprSubstitutionInsert(s, source, definition, proof, ast.ASTDependency{}); err != nil {
		return err
	}
	if err = r.SetSubstitution(&s); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.SetSubstitution(nil)) }()
	t, err := m.RefGet(target)
	if err != nil {
		return err
	}
	return r.Rewrite(t, target)
}

func (r *DefaultExprReplacer) Close() (result error) {
	if r == nil {
		return ast.ErrASTHandle
	}
	result = r.releaseUsedDependency()
	return errors.Join(result, r.engine.Close())
}

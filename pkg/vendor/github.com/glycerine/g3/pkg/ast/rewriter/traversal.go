package rewriter

// Source: src/ast/rewriter/rewriter.h:194-408;
// src/ast/rewriter/rewriter_def.h:23-750; src/model/model_evaluator.cpp:282-289.

import (
	"github.com/glycerine/g3/pkg/ast"
	"reflect"
)

// RewriteStatus preserves the source bounded revisit depths and final statuses.
type RewriteStatus = brStatus

const (
	Rewrite1      RewriteStatus = brRewrite1
	Rewrite2      RewriteStatus = brRewrite2
	Rewrite3      RewriteStatus = brRewrite3
	RewriteFull   RewriteStatus = brRewriteFull
	RewriteDone   RewriteStatus = brDone
	RewriteFailed RewriteStatus = brFailed
)

// Configuration is borrowed by the original traversal. Callback result
// and proof references belong to the traversal. Embed DefaultConfiguration for
// the source defaults. Resource callbacks may return the source exception as a
// Go error; a true step-limit result produces the original max-steps exception.
type Configuration interface {
	CacheAllResults() bool
	CacheResults() bool
	FlatAssoc(d ast.ASTHandle) (bool, error)
	RewritePatterns() bool
	MaxScopesExceeded(n uint32) bool
	MaxFramesExceeded(n uint32) bool
	MaxStepsExceeded(n uint32) (bool, error)
	PreVisit(t ast.ASTHandle) (bool, error)
	ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (RewriteStatus, error)
	ReduceVar(v ast.ASTHandle, out, proof ast.ASTRef) (bool, error)
	ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (bool, error)
	GetMacro(d ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error)
	GetSubst(t ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error)
	Reset()
	Cleanup()
}

// DefaultConfiguration supplies the original default_rewriter_cfg behavior.
type DefaultConfiguration struct{ defaults defaultRewriterConfig }

func (c *DefaultConfiguration) CacheAllResults() bool { return c.defaults.cacheAllResults() }
func (c *DefaultConfiguration) CacheResults() bool    { return c.defaults.cacheResults() }
func (c *DefaultConfiguration) FlatAssoc(d ast.ASTHandle) (bool, error) {
	return c.defaults.flatAssoc(d)
}
func (c *DefaultConfiguration) RewritePatterns() bool { return c.defaults.rewritePatterns() }
func (c *DefaultConfiguration) MaxScopesExceeded(n uint32) bool {
	return c.defaults.maxScopesExceeded(n)
}
func (c *DefaultConfiguration) MaxFramesExceeded(n uint32) bool {
	return c.defaults.maxFramesExceeded(n)
}
func (c *DefaultConfiguration) MaxStepsExceeded(n uint32) (bool, error) {
	return c.defaults.maxStepsExceeded(n)
}
func (c *DefaultConfiguration) PreVisit(t ast.ASTHandle) (bool, error) { return c.defaults.preVisit(t) }
func (c *DefaultConfiguration) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (RewriteStatus, error) {
	return c.defaults.reduceApp(d, args, out, proof)
}
func (c *DefaultConfiguration) ReduceVar(v ast.ASTHandle, out, proof ast.ASTRef) (bool, error) {
	return c.defaults.reduceVar(v, out, proof)
}
func (c *DefaultConfiguration) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (bool, error) {
	return c.defaults.reduceQuantifier(q, body, patterns, noPatterns, out, proof)
}
func (c *DefaultConfiguration) GetMacro(d ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error) {
	return c.defaults.getMacro(d)
}
func (c *DefaultConfiguration) GetSubst(t ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	return c.defaults.getSubst(t)
}
func (c *DefaultConfiguration) Reset()   { c.defaults.reset() }
func (c *DefaultConfiguration) Cleanup() { c.defaults.cleanup() }

// The adapter is inline cold configuration storage; all traversal frames and
// argument views keep the original concrete handle representation.
type traversalConfiguration struct{ client Configuration }

func (c *traversalConfiguration) cacheAllResults() bool { return c.client.CacheAllResults() }
func (c *traversalConfiguration) cacheResults() bool    { return c.client.CacheResults() }
func (c *traversalConfiguration) flatAssoc(d ast.ASTHandle) (bool, error) {
	return c.client.FlatAssoc(d)
}
func (c *traversalConfiguration) rewritePatterns() bool { return c.client.RewritePatterns() }
func (c *traversalConfiguration) maxScopesExceeded(n uint32) bool {
	return c.client.MaxScopesExceeded(n)
}
func (c *traversalConfiguration) maxFramesExceeded(n uint32) bool {
	return c.client.MaxFramesExceeded(n)
}
func (c *traversalConfiguration) maxStepsExceeded(n uint32) (bool, error) {
	return c.client.MaxStepsExceeded(n)
}
func (c *traversalConfiguration) preVisit(t ast.ASTHandle) (bool, error) { return c.client.PreVisit(t) }
func (c *traversalConfiguration) reduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (RewriteStatus, error) {
	return c.client.ReduceApp(d, args, out, proof)
}
func (c *traversalConfiguration) reduceVar(v ast.ASTHandle, out, proof ast.ASTRef) (bool, error) {
	return c.client.ReduceVar(v, out, proof)
}
func (c *traversalConfiguration) reduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (bool, error) {
	return c.client.ReduceQuantifier(q, body, patterns, noPatterns, out, proof)
}
func (c *traversalConfiguration) getMacro(d ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error) {
	return c.client.GetMacro(d)
}
func (c *traversalConfiguration) getSubst(t ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	return c.client.GetSubst(t)
}
func (c *traversalConfiguration) reset()   { c.client.Reset() }
func (c *traversalConfiguration) cleanup() { c.client.Cleanup() }

// Traversal exposes the original rewriter_tpl specialization across package
// boundaries. The configuration must outlive this owner.
type Traversal struct {
	engine rewriterEngine
	cfg    traversalConfiguration
}

// NewTraversal charges the enclosing owner once. A nonnil partial owner returned
// with an error must be closed, as with the other explicit AST owners.
func NewTraversal(m *ast.ASTManager, cfg Configuration) (*Traversal, error) {
	return newTraversalMode(m, cfg, false)
}

// NewProofTraversal constructs the proof-generating source specialization.
// The manager must have proofs enabled before construction.
func NewProofTraversal(m *ast.ASTManager, cfg Configuration) (*Traversal, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	proofs, e := m.ProofsEnabled()
	if e != nil {
		return nil, e
	}
	if !proofs {
		return nil, ast.ErrASTDomain
	}
	return newTraversalMode(m, cfg, true)
}

func newTraversalMode(m *ast.ASTManager, cfg Configuration, proofs bool) (*Traversal, error) {
	if m == nil || cfg == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(Traversal{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	r := &Traversal{}
	r.cfg.client = cfg
	r.engine.header = bytes
	return r, r.engine.initMode(m, &r.cfg, proofs)
}
func (r *Traversal) Rewrite(t ast.ASTHandle, out ast.ASTRef) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Rewrite(t, out)
}

// EnableModelBooleanShortCircuit avoids evaluating dead recursive model
// branches after AND/OR already has a determining operand. Ordinary rewriting
// retains source traversal order; this correction is confined to model use.
func (r *Traversal) EnableModelBooleanShortCircuit() error {
	if err := r.engine.open(); err != nil {
		return err
	}
	if r.engine.core.proofGen {
		return ast.ErrASTDomain
	}
	r.engine.modelBooleanShortCircuit = true
	return nil
}
func (r *Traversal) RewriteWithProof(t ast.ASTHandle, out, proof ast.ASTRef) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.RewriteWithProof(t, out, proof)
}
func (r *Traversal) Resume(out ast.ASTRef) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Resume(out)
}
func (r *Traversal) ResumeWithProof(out, proof ast.ASTRef) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.ResumeWithProof(out, proof)
}
func (r *Traversal) Reset() error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Reset()
}
func (r *Traversal) Cleanup() error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Cleanup()
}
func (r *Traversal) Close() error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Close()
}
func (r *Traversal) SetBindings(args []ast.ASTHandle) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.SetBindings(args)
}
func (r *Traversal) SetInvBindings(args []ast.ASTHandle) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.SetInvBindings(args)
}
func (r *Traversal) SetCancelCheck(enabled bool) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	if e := r.engine.open(); e != nil {
		return e
	}
	return r.engine.core.SetCancelCheck(enabled)
}
func (r *Traversal) NumSteps() (uint32, error) {
	if r == nil {
		return 0, ast.ErrASTHandle
	}
	if e := r.engine.open(); e != nil {
		return 0, e
	}
	return r.engine.numSteps, nil
}
func (r *Traversal) CacheSize() (uint32, error) {
	if r == nil {
		return 0, ast.ErrASTHandle
	}
	if e := r.engine.open(); e != nil {
		return 0, e
	}
	return r.engine.core.getCacheSize()
}

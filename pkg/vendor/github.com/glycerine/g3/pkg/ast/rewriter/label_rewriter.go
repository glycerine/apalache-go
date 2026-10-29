package rewriter

// Source: src/ast/rewriter/label_rewriter.cpp;
// src/ast/rewriter/label_rewriter.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type labelRewriterConfiguration struct {
	DefaultConfiguration
	manager *ast.ASTManager
}

func (c *labelRewriterConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, _ ast.ASTRef) (RewriteStatus, error) {
	family, err := c.manager.GetFamilyID(decl)
	if err != nil || family != 1 {
		return RewriteFailed, err
	}
	kind, err := c.manager.GetDeclKind(decl)
	if err != nil || kind != 0 {
		return RewriteFailed, err
	}
	if ast.ASTSourceDebug && args.Count() != 1 {
		return RewriteFailed, ast.ErrASTDomain
	}
	arg, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	if err = c.manager.SetRef(out, arg); err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, nil
}

// LabelRewriter removes OP_LABEL applications throughout an expression and
// optionally advances an existing proof by a rewrite and modus ponens.
type LabelRewriter struct {
	self      *LabelRewriter
	manager   *ast.ASTManager
	config    labelRewriterConfiguration
	traversal *Traversal
	closed    bool
}

func NewLabelRewriter(m *ast.ASTManager) (*LabelRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	r := &LabelRewriter{manager: m}
	r.self = r
	r.config.manager = m
	traversal, err := NewTraversal(m, &r.config)
	r.traversal = traversal
	if err != nil {
		if traversal != nil {
			return r, errors.Join(err, traversal.Close())
		}
		return r, err
	}
	return r, nil
}

func (r *LabelRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	return nil
}

// RemoveLabels updates formula in place. If proof contains a proof and the
// formula changes, proof is replaced by modus_ponens(proof,rewrite(old,new)).
func (r *LabelRewriter) RemoveLabels(formula, proof ast.ASTRef) (result error) {
	if err := r.open(); err != nil {
		return err
	}
	old, err := r.manager.RefGet(formula)
	if err != nil {
		return err
	}
	tmp, err := r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.manager.DelRef(&tmp)) }()
	if err = r.traversal.Rewrite(old, tmp); err != nil {
		return err
	}
	rewritten, err := r.manager.RefGet(tmp)
	if err != nil {
		return err
	}
	priorProof, err := r.manager.RefGet(proof)
	if err != nil {
		return err
	}
	if !priorProof.IsNull() && old != rewritten {
		rewrite, err := r.manager.MkProofRewrite(old, rewritten)
		if err != nil {
			return err
		}
		rewriteRef, err := r.manager.NewRef(rewrite)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, r.manager.DelRef(&rewriteRef)) }()
		nextProof, err := r.manager.MkProofModusPonens(priorProof, rewrite)
		if err != nil {
			return err
		}
		if err = r.manager.SetRef(proof, nextProof); err != nil {
			return err
		}
	}
	return r.manager.SetRef(formula, rewritten)
}

func (r *LabelRewriter) Close() error {
	if r == nil || r.self != r || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
	}
	r.closed = true
	return nil
}

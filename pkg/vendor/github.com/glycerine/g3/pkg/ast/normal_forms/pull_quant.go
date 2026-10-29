package normal_forms

// Source: src/ast/normal_forms/pull_quant.cpp reduce_app,
// reduce_quantifier and pull_nested_quant::imp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type pullQuantConfiguration struct {
	rewriter.DefaultConfiguration
	core *pullQuantCore
}

func (c *pullQuantConfiguration) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	m := c.core.manager
	family, err := m.GetFamilyID(d)
	if err != nil || family != m.BasicFamilyID() {
		return rewriter.RewriteFailed, err
	}
	op, err := m.GetDeclKind(d)
	if err != nil || (op != ast.ASTOpNot && op != ast.ASTOpAnd && op != ast.ASTOpOr) {
		return rewriter.RewriteFailed, err
	}
	children := make([]ast.ASTHandle, args.Count())
	for i := range children {
		children[i], err = args.At(uint32(i))
		if err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	value, changed, err := c.core.pullBooleanApp(d, op, children)
	if err != nil || !changed {
		return rewriter.RewriteFailed, err
	}
	if err = m.SetRef(out, value); err != nil {
		return rewriter.RewriteFailed, err
	}
	before, err := m.MkApp(d, children)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	p, err := m.MkProofPullQuant(before, value)
	if err == nil {
		err = m.SetRef(proof, p)
	}
	return rewriter.RewriteDone, err
}

func (c *pullQuantConfiguration) ReduceQuantifier(q, body ast.ASTHandle, _, _ ast.AppArguments, out, proof ast.ASTRef) (bool, error) {
	m := c.core.manager
	forall, err := m.IsForall(q)
	if err != nil {
		return false, err
	}
	if !forall {
		return false, ast.ErrASTPending // source requires Skolem normal form
	}
	kind, err := m.GetKind(body)
	if err != nil || kind != ast.ASTQuantifier {
		return false, err
	}
	nested, err := m.IsForall(body)
	if err != nil || !nested {
		return false, err
	}
	value, err := c.core.pullDirectForall(q, body)
	if err != nil {
		return false, err
	}
	if err = m.SetRef(out, value); err != nil {
		return false, err
	}
	p, err := m.MkProofPullQuant(q, value)
	if err == nil {
		err = m.SetRef(proof, p)
	}
	return err == nil, err
}

// PullQuant pulls quantifiers through Boolean connectives and merges nested
// universals. Input must be in Skolem normal form. The traversal owns its
// cached formulas and proofs until reset or close.
type PullQuant struct {
	core      pullQuantCore
	config    pullQuantConfiguration
	traversal *rewriter.Traversal
	proofs    bool
	closed    bool
}

func NewPullQuant(m *ast.ASTManager) (*PullQuant, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	proofs, err := m.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	p := &PullQuant{core: pullQuantCore{manager: m}, proofs: proofs}
	p.config.core = &p.core
	p.core.shift, err = rewriter.NewVarShifter(m)
	if err != nil {
		return p, err
	}
	if proofs {
		p.traversal, err = rewriter.NewProofTraversal(m, &p.config)
	} else {
		p.traversal, err = rewriter.NewTraversal(m, &p.config)
	}
	return p, err
}

func (p *PullQuant) RewriteWithProof(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if p == nil || p.closed || p.traversal == nil {
		return ast.ErrASTHandle
	}
	if !p.proofs {
		if err := p.core.manager.ResetRef(proof); err != nil {
			return err
		}
		return p.traversal.Rewrite(root, out)
	}
	return p.traversal.RewriteWithProof(root, out, proof)
}

func (p *PullQuant) Reset() error {
	if p == nil || p.closed || p.traversal == nil {
		return ast.ErrASTHandle
	}
	return p.traversal.Reset()
}

func (p *PullQuant) Close() error {
	if p == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.traversal != nil {
		if err := p.traversal.Close(); err != nil {
			return err
		}
		p.traversal = nil
	}
	if p.core.shift != nil {
		if err := p.core.shift.Close(); err != nil {
			return err
		}
		p.core.shift = nil
	}
	p.closed = true
	return nil
}

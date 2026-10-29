package normal_forms

// Source: src/ast/normal_forms/pull_quant.cpp pull_nested_quant::imp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type pullNestedConfiguration struct {
	rewriter.DefaultConfiguration
	pull       *PullQuant
	out, proof ast.ASTRef
}

func (c *pullNestedConfiguration) GetSubst(root ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	m := c.pull.core.manager
	kind, err := m.GetKind(root)
	if err != nil || kind != ast.ASTQuantifier {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if err = c.pull.RewriteWithProof(root, c.out, c.proof); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	value, err := m.RefGet(c.out)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	p, err := m.RefGet(c.proof)
	return value, p, err == nil, err
}

// PullNestedQuantifiers applies PullQuant to quantified subformulas, preserving
// surrounding applications and composing congruence proofs for them.
type PullNestedQuantifiers struct {
	config    pullNestedConfiguration
	traversal *rewriter.Traversal
	closed    bool
}

func NewPullNestedQuantifiers(m *ast.ASTManager) (*PullNestedQuantifiers, error) {
	p := &PullNestedQuantifiers{}
	var err error
	p.config.pull, err = NewPullQuant(m)
	if err != nil {
		return p, err
	}
	p.config.out, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return p, err
	}
	p.config.proof, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return p, err
	}
	if p.config.pull.proofs {
		p.traversal, err = rewriter.NewProofTraversal(m, &p.config)
	} else {
		p.traversal, err = rewriter.NewTraversal(m, &p.config)
	}
	return p, err
}

func (p *PullNestedQuantifiers) RewriteWithProof(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if p == nil || p.closed || p.traversal == nil {
		return ast.ErrASTHandle
	}
	if !p.config.pull.proofs {
		if err := p.config.pull.core.manager.ResetRef(proof); err != nil {
			return err
		}
		return p.traversal.Rewrite(root, out)
	}
	return p.traversal.RewriteWithProof(root, out, proof)
}

func (p *PullNestedQuantifiers) Reset() error {
	if p == nil || p.closed || p.traversal == nil {
		return ast.ErrASTHandle
	}
	return p.traversal.Reset()
}

func (p *PullNestedQuantifiers) Close() error {
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
	if p.config.pull != nil {
		m := p.config.pull.core.manager
		var err error
		if p.config.proof != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&p.config.proof))
		}
		if p.config.out != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&p.config.out))
		}
		if err != nil {
			return err
		}
		if err = p.config.pull.Close(); err != nil {
			return err
		}
		p.config.pull = nil
	}
	p.closed = true
	return nil
}

package smtcore

// Source: src/smt/elim_term_ite.h and src/smt/elim_term_ite.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/normal_forms"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type smtElimTermITEConfiguration struct {
	rewriter.DefaultConfiguration
	manager     *ast.ASTManager
	names       *normal_forms.DefinedNames
	definitions []*ast.JustifiedExpr
}

func (c *smtElimTermITEConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (status rewriter.RewriteStatus, result error) {
	family, err := c.manager.GetFamilyID(decl)
	if err != nil || family != c.manager.BasicFamilyID() {
		return rewriter.RewriteFailed, err
	}
	kind, err := c.manager.GetDeclKind(decl)
	if err != nil || kind != ast.ASTOpIte {
		return rewriter.RewriteFailed, err
	}
	rangeSort, err := c.manager.GetRange(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if rangeSort == boolean {
		return rewriter.RewriteFailed, nil
	}
	application, err := c.manager.RewriterMakeApp(decl, args)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	refs := make([]ast.ASTRef, 4)
	for i := range refs {
		refs[i], err = c.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			for j := i - 1; j >= 0; j-- {
				err = errors.Join(err, c.manager.DelRef(&refs[j]))
			}
			return rewriter.RewriteFailed, err
		}
	}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, c.manager.DelRef(&refs[i]))
		}
	}()
	created, err := c.names.MkName(application, refs[0], refs[1], refs[2], refs[3])
	if err != nil || !created {
		return rewriter.RewriteFailed, err
	}
	name, err := c.manager.RefGet(refs[2])
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	nameProof, err := c.manager.RefGet(refs[3])
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	definition, err := c.manager.RefGet(refs[0])
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	definitionProof, err := c.manager.RefGet(refs[1])
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	justified, err := ast.NewJustifiedExpr(c.manager, definition, definitionProof)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if err = c.manager.SetRef(out, name); err != nil {
		return rewriter.RewriteFailed, errors.Join(err, justified.Close())
	}
	if err = c.manager.SetRef(proof, nameProof); err != nil {
		return rewriter.RewriteFailed, errors.Join(err, justified.Close())
	}
	c.definitions = append(c.definitions, justified)
	return rewriter.RewriteDone, nil
}

type SMTTermITEEliminator struct {
	self      *SMTTermITEEliminator
	manager   *ast.ASTManager
	config    smtElimTermITEConfiguration
	traversal *rewriter.Traversal
	proofs    bool
	closed    bool
}

func NewSMTTermITEEliminator(m *ast.ASTManager, names *normal_forms.DefinedNames) (*SMTTermITEEliminator, error) {
	if m == nil || names == nil {
		return nil, ast.ErrASTDomain
	}
	proofs, err := m.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	e := &SMTTermITEEliminator{manager: m, proofs: proofs}
	e.self = e
	e.config.manager, e.config.names = m, names
	if proofs {
		e.traversal, err = rewriter.NewProofTraversal(m, &e.config)
	} else {
		e.traversal, err = rewriter.NewTraversal(m, &e.config)
	}
	if err != nil {
		if e.traversal != nil {
			err = errors.Join(err, e.traversal.Close())
		}
		return e, err
	}
	return e, nil
}

func (e *SMTTermITEEliminator) open() error {
	if e == nil || e.self != e || e.closed || e.manager == nil || e.traversal == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (e *SMTTermITEEliminator) Rewrite(expression ast.ASTHandle, out, proof ast.ASTRef) error {
	if err := e.open(); err != nil {
		return err
	}
	if e.proofs {
		return e.traversal.RewriteWithProof(expression, out, proof)
	}
	return e.traversal.Rewrite(expression, out)
}

// NewDefinitions returns a detached slice of borrowed definition owners.
func (e *SMTTermITEEliminator) NewDefinitions() ([]*ast.JustifiedExpr, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	return append([]*ast.JustifiedExpr(nil), e.config.definitions...), nil
}

func (e *SMTTermITEEliminator) Reset() (result error) {
	if err := e.open(); err != nil {
		return err
	}
	for len(e.config.definitions) != 0 {
		last := len(e.config.definitions) - 1
		if err := e.config.definitions[last].Close(); err != nil {
			return err
		}
		e.config.definitions = e.config.definitions[:last]
	}
	return e.traversal.Reset()
}

func (e *SMTTermITEEliminator) Close() error {
	if e == nil || e.self != e {
		return ast.ErrASTHandle
	}
	if e.closed {
		return nil
	}
	if err := e.Reset(); err != nil {
		return err
	}
	if err := e.traversal.Close(); err != nil {
		return err
	}
	e.closed = true
	return nil
}

package tactic

// Source: src/tactic/equiv_proof_converter.cpp; src/tactic/equiv_proof_converter.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type equivProofConverter struct {
	proofConverterBase
	manager     *ast.ASTManager
	replacement *replaceProofConverter
}

func NewEquivProofConverter(m *ast.ASTManager) (*equivProofConverter, error) {
	replacement, err := NewReplaceProofConverter(m)
	if err != nil {
		return nil, err
	}
	c := &equivProofConverter{manager: m, replacement: replacement}
	if err = c.init(c, m.Budget(), uint64(reflect.TypeOf(*c).Size())); err != nil {
		return nil, errors.Join(err, replacement.Close())
	}
	return c, nil
}

func (c *equivProofConverter) Insert(first, second ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if first == second {
		return nil
	}
	asserted, err := c.manager.MkProofAsserted(first)
	if err != nil {
		return err
	}
	rewrite, err := c.manager.MkProofRewrite(first, second)
	if err != nil {
		return err
	}
	proof, err := c.manager.MkProofModusPonens(asserted, rewrite)
	if err != nil {
		return err
	}
	return c.replacement.Insert(proof)
}

func (c *equivProofConverter) Convert(m *ast.ASTManager, sources []ast.ASTHandle, out ast.ASTRef) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.replacement.Convert(m, sources, out)
}

func (c *equivProofConverter) Translate(tr *ast.ASTTranslation) (ProofConverter, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.replacement.Translate(tr)
}

func (c *equivProofConverter) Cancel() error {
	if err := c.open(); err != nil {
		return err
	}
	return c.replacement.Cancel()
}

func (c *equivProofConverter) Display(w io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.replacement.Display(w)
}

func (c *equivProofConverter) Manager() *ast.ASTManager {
	if c.open() != nil {
		return nil
	}
	return c.manager
}

func (c *equivProofConverter) Close() error {
	if c == nil || c.self != &c.proofConverterBase {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if err := c.replacement.Close(); err != nil {
		return err
	}
	c.replacement = nil
	c.manager = nil
	c.release()
	return nil
}

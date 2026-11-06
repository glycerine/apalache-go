package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

// ProofCoreAccumulator owns the union of asserted leaves across native proofs.
// It survives solver pops/checks until installed as the RPFP's proof core.
// Source: duality_rpfp.cpp:1151-1156,2978-2980,3017-3024,3125-3128.
type ProofCoreAccumulator struct {
	context *Context
	terms   map[ast.ASTHandle]ast.ASTRef
	closed  bool
}

func NewProofCoreAccumulator(context *Context) (*ProofCoreAccumulator, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	return &ProofCoreAccumulator{context: context, terms: make(map[ast.ASTHandle]ast.ASTRef)}, nil
}
func (c *ProofCoreAccumulator) open() error {
	if c == nil || c.closed {
		return ErrWrapperOwner
	}
	return c.context.open()
}
func (c *ProofCoreAccumulator) AddCurrentProof(g *RPFP) (result error) {
	if err := c.open(); err != nil {
		return err
	}
	if err := g.solverReady(); err != nil {
		return err
	}
	if g.context != c.context {
		return ErrContextMismatch
	}
	proof, err := g.solver.Proof()
	if err != nil {
		return err
	}
	if proof.IsNull() {
		return ast.ErrASTHandle
	}
	manager, err := interp.NewManager(c.context.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, manager.Close()) }()
	assumptions, err := interp.ProofAssumptions(manager, proof)
	if err != nil {
		return err
	}
	for _, term := range assumptions {
		if err := c.includeHandle(term); err != nil {
			return err
		}
	}
	return nil
}
func (c *ProofCoreAccumulator) Contains(term *Expr) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if err := term.open(); err != nil {
		return false, err
	}
	if term.value.object.context != c.context {
		return false, ErrContextMismatch
	}
	h, err := term.Raw()
	if err != nil {
		return false, err
	}
	_, found := c.terms[h]
	return found, nil
}

// Install transfers the owning set, replacing the graph's previous proof core.
// The accumulator is consumed only after that previous core has been released.
func (c *ProofCoreAccumulator) Install(g *RPFP) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := g.open(); err != nil {
		return err
	}
	if g.context != c.context {
		return ErrContextMismatch
	}
	if err := g.ClearProofCore(); err != nil {
		return err
	}
	g.proofCore = c.terms
	c.terms = nil
	c.closed = true
	return nil
}
func (c *ProofCoreAccumulator) Close() error {
	if c == nil || c.closed {
		return nil
	}
	for key, ref := range c.terms {
		err := c.context.manager.DelRef(&ref)
		if ref == (ast.ASTRef{}) {
			delete(c.terms, key)
		} else {
			c.terms[key] = ref
		}
		if err != nil {
			return err
		}
	}
	c.terms = nil
	c.closed = true
	return nil
}

func (c *ProofCoreAccumulator) includeHandle(h ast.ASTHandle) error {
	if _, found := c.terms[h]; found {
		return nil
	}
	ref, err := c.context.manager.NewRef(h)
	if err != nil {
		return err
	}
	c.terms[h] = ref
	return nil
}

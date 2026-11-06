package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// ComputeProofCore lazily owns asserted leaves of the current native proof.
// Source: duality_rpfp.cpp:1151-1163.
func (g *RPFP) ComputeProofCore() (result error) {
	if err := g.solverReady(); err != nil {
		return err
	}
	if g.proofCore != nil {
		return nil
	}
	core, err := NewProofCoreAccumulator(g.context)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, core.Close()) }()
	if err = core.AddCurrentProof(g); err != nil {
		return err
	}
	return core.Install(g)
}

// EdgeUsedInProof tests the edge formula and its localized constraints against
// asserted proof leaves. Cached assumption-literal translation is separate.
// Source: duality_rpfp.cpp:4227-4236.
func (g *RPFP) EdgeUsedInProof(edge *Edge) (bool, error) {
	if err := g.open(); err != nil {
		return false, err
	}
	if edge == nil || edge.Owner != g || edge.closed {
		return false, ErrWrapperOwner
	}
	if err := g.ComputeProofCore(); err != nil {
		return false, err
	}
	refs := append([]ast.ASTRef{edge.dual}, edge.constraints...)
	for _, ref := range refs {
		if ref == (ast.ASTRef{}) {
			continue
		}
		term, err := g.context.manager.RefGet(ref)
		if err != nil {
			return false, err
		}
		if _, found := g.proofCore[term]; found {
			return true, nil
		}
	}
	return false, nil
}

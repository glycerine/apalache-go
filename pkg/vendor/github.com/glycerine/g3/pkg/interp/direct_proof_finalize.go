package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp final simplification and
// universal closure for proofs without pending localization variables.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Interpolate simplifies a proof of false and universally closes any free
// de-Bruijn variables. The returned reference is owned by the caller.
func (p *DirectProofInterpolator) Interpolate(proof ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return ast.ASTRef{}, err
	}
	simplified, err := p.base.manager.ReallySimplify(proof)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.base.manager.ast.DelRef(&simplified)) }()
	handle, err := p.base.manager.ast.RefGet(simplified)
	if err != nil {
		return ast.ASTRef{}, err
	}
	handle, err = p.addLocalizationQuantifiers(handle)
	if err != nil {
		return ast.ASTRef{}, err
	}
	closed, err := p.base.manager.CloseUniversally(handle)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.base.manager.ast.NewRef(closed)
}

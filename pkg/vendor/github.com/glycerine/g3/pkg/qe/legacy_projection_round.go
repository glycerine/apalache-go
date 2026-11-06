package qe

// Source: src/muz/spacer/spacer_util.cpp:507-581, qe_project's
// repeated qe_lite and theory-rewriter stages for array auxiliaries.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *LegacyProjection) projectArithmeticRound(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle) (result *legacyFormulaProjection, resultErr error) {
	scope, err := ast.NewScopedNoProof(p.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		if resultErr != nil && result != nil {
			resultErr = closeTransferredLegacyOuterArithmetic(result, resultErr)
			result = nil
		}
	}()
	return p.arithmetic.projectFormula(md, variables, formula)
}

// qe_project constructs its array projection and following theory rewrite
// inside scoped_no_proof, then restores the caller's proof-generation mode.
func (p *LegacyProjection) projectArrayRound(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, reduceAllSelects bool) (result *legacyArrayProjectionResult, resultErr error) {
	scope, err := ast.NewScopedNoProof(p.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		if resultErr != nil && result != nil {
			resultErr = closeTransferredLegacyOuterArray(result, resultErr)
			result = nil
		}
	}()
	result, resultErr = p.array.projectFormula(md, variables, formula, reduceAllSelects)
	if resultErr == nil {
		resultErr = p.rewriteProjectionCurrent(result.formula)
	}
	return result, resultErr
}

func (p *LegacyProjection) rewriteProjectionCurrent(current ast.ASTRef) (resultErr error) {
	root, err := p.manager.RefGet(current)
	if err != nil {
		return err
	}
	rewritten, err := p.arithmetic.rewriteLegacy(root)
	if err != nil {
		return err
	}
	root, err = p.manager.RefGet(rewritten)
	if err != nil {
		return closeLegacyProjectionRewriteResult(p.manager, rewritten, err)
	}
	err = p.manager.SetRef(current, root)
	return closeLegacyProjectionRewriteResult(p.manager, rewritten, err)
}

func (p *LegacyProjection) prepareProjectionRound(variables []ast.ASTHandle, current ast.ASTRef) (refs []ast.ASTRef, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, truncateLiteVector(p.manager, &refs, 0))
		}
	}()
	for _, variable := range variables {
		ref, err := p.manager.NewRef(variable)
		if err != nil {
			return refs, err
		}
		refs = append(refs, ref)
	}
	lite, err := NewLite(p.manager, nil, false, p.config)
	if err != nil {
		return refs, err
	}
	err = lite.EliminateConstants(&refs, current)
	err = errors.Join(err, lite.Close())
	if err != nil {
		return refs, err
	}
	if err = p.rewriteProjectionCurrent(current); err != nil {
		return refs, err
	}
	return refs, nil
}

package pattern

import "github.com/glycerine/g3/pkg/ast"

// PullQuantifierNoProof exposes the retained pull_quant algorithm to MBQI.
// The returned reference is owned by the caller. Input formulas must be in
// skolem normal form, as required by the C++ pull_quant consumer.
func PullQuantifierNoProof(manager *ast.ASTManager, root ast.ASTHandle) (ast.ASTRef, error) {
	if manager == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	return (&patternCandidates{manager: manager}).pullNoProof(root)
}

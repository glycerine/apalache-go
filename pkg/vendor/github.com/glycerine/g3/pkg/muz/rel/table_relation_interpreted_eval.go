package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

func (r *TableRelation) acceptsInterpretedFact(rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle, fact Fact) (accepted bool, resultErr error) {
	bindings := make([]ast.ASTHandle, tableRelationBindingSize(sorts))
	references := make([]ast.ASTRef, 0, len(sorts)+len(conditions))
	defer func() {
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, r.manager.manager.DelRef(&references[index]))
		}
	}()
	for variable, sort := range sorts {
		reference, err := r.manager.DecodeValue(sort, fact[columns[variable]])
		if err != nil {
			return false, err
		}
		references = append(references, reference)
		handle, err := r.manager.manager.RefGet(reference)
		if err != nil {
			return false, err
		}
		bindings[tableRelationBindingIndex(len(bindings), variable)] = handle
	}
	for _, condition := range conditions {
		out, err := r.manager.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return false, err
		}
		references = append(references, out)
		if err = rewriter.RewriteWithBindings(condition, bindings, out); err != nil {
			return false, err
		}
		rewritten, err := r.manager.manager.RefGet(out)
		if err != nil {
			return false, err
		}
		if r.manager.manager.IsFalse(rewritten) {
			return false, nil
		}
		if !r.manager.manager.IsTrue(rewritten) {
			return false, ErrTableRelation
		}
	}
	return true, nil
}

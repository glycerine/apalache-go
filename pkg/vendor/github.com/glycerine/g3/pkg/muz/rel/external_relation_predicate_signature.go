package rel

import (
	"github.com/glycerine/g3/pkg/ast"
)

// externalPredicateSorts validates only the predicate shape required by the
// source external relation plugin. Unlike finite-table signatures, external
// callbacks admit infinite domain sorts such as Int and Real.
func externalPredicateSorts(manager *ast.ASTManager, predicate ast.ASTHandle) ([]ast.ASTHandle, error) {
	if manager == nil || predicate.IsNull() {
		return nil, ErrTableRelation
	}
	kind, err := manager.GetKind(predicate)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTFuncDecl {
		return nil, ErrTableRelation
	}
	rangeSort, err := manager.GetRange(predicate)
	if err != nil {
		return nil, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	if rangeSort != boolean {
		return nil, ErrTableRelation
	}
	arity, err := manager.GetArity(predicate)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		sorts[i], err = manager.GetDomain(predicate, i)
		if err != nil {
			return nil, err
		}
	}
	return sorts, nil
}

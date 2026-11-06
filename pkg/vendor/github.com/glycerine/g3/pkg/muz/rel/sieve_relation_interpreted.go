package rel

// Source: src/muz/rel/dl_sieve_relation.cpp
// sieve_relation_plugin::mk_filter_interpreted_fn.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpreted applies condition to the selected inner columns. A
// condition mentioning an ignored column is deliberately an identity: that
// column denotes its full domain, so filtering it would be unsound for the
// source sieve over-approximation.
func (r *SieveRelation) FilterInterpreted(relations *RelationManager, sorts []ast.ASTHandle, theory *rewriter.TheoryRewriter, condition ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	if relations == nil || relations.open() != nil || len(sorts) != r.signature.Len() {
		return ErrSieve
	}
	signature, err := tableRelationSignature(relations, sorts)
	if err != nil {
		return err
	}
	if !signature.equal(r.signature) {
		return ErrSieve
	}
	manager := relations.manager
	kind, err := manager.GetKind(condition)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	conditionSort, err := manager.GetSort(condition)
	if err != nil {
		return err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return err
	}
	if conditionSort != boolean {
		return ast.ErrASTDomain
	}
	used, err := ast.NewASTUsedVars(manager)
	if err != nil {
		return err
	}
	if err = used.Collect(condition); err != nil {
		return err
	}
	if used.MaxFoundVarIndexPlusOne() > uint32(len(sorts)) {
		return ast.ErrASTDomain
	}
	columns := make(map[uint32]int, used.NumVars())
	variableSorts := make(map[uint32]ast.ASTHandle, used.NumVars())
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		declaration := used.Contains(variable)
		if declaration == (ast.ASTHandle{}) {
			continue
		}
		if !r.IsInnerColumn(int(variable)) {
			return nil
		}
		// ASTUsedVars stores the variable sort at each used de-Bruijn index.
		actualSort := declaration
		if actualSort != sorts[variable] {
			return fmt.Errorf("%w: interpreted variable sort mismatch", ErrSieve)
		}
		columns[variable] = r.sigToInner[variable]
		variableSorts[variable] = actualSort
	}
	if theory == nil {
		return ErrSieve
	}
	innerSorts := make([]ast.ASTHandle, len(r.innerToSig))
	for innerColumn, outerColumn := range r.innerToSig {
		innerSorts[innerColumn] = sorts[outerColumn]
	}
	inner := &TableRelation{manager: relations, sorts: innerSorts, table: r.inner}
	if err = inner.FilterInterpreted(theory, []ast.ASTHandle{condition}, columns, variableSorts); err != nil {
		return fmt.Errorf("%w: %v", ErrSieve, err)
	}
	return nil
}

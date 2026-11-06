package rel

// Source: dl_table_relation.cpp table_relation_plugin::universal_target_union_fn.

import "github.com/glycerine/g3/pkg/ast"

// UnionUDocFromTable adds source's finite rows to the actual DOC target. All
// arguments are borrowed. Delta accumulates only facts absent from target;
// existing delta facts are preserved, matching the source union operation.
// The caller establishes equal original source/target sorts; FiniteTable
// carries only their encoded finite cardinalities, which are checked here.
func UnionUDocFromTable(target *TypedUDocRelation, source FiniteTable, manager *RelationManager, delta *TypedUDocRelation) error {
	if target == nil || target.open() != nil || source == nil || manager == nil || manager.open() != nil || manager.manager != target.factory.manager {
		return ErrDOC
	}
	// Source check_relation::mk_union_fn casts both operands before the
	// table-plugin fallback. A checked DOC/table pair cannot reach this route.
	if target.FormulaChecked() {
		return ErrDOC
	}
	// This concrete specialization keeps target and delta in one DOC factory.
	if delta != nil && (delta.FormulaChecked() || !target.sameShape(delta)) {
		return ErrDOC
	}
	expected, err := tableRelationSignature(manager, target.sorts)
	if err != nil {
		return err
	}
	actual, err := source.Signature()
	if err != nil {
		return err
	}
	if !expected.equal(actual) {
		return ErrTableRelation
	}
	facts, err := source.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		if err := unionUDocTableFact(target, fact, manager, delta); err != nil {
			return err
		}
	}
	return nil
}

func unionUDocTableFact(target *TypedUDocRelation, fact Fact, manager *RelationManager, delta *TypedUDocRelation) (resultErr error) {
	owner := &typedUDocFormulaOwner{manager: manager.manager}
	defer func() { resultErr = finishTypedUDocFormula(owner, resultErr) }()
	values, err := manager.DecodeFact(target.sorts, fact)
	owner.roots = values
	if err != nil {
		return err
	}
	handles := make([]ast.ASTHandle, len(values))
	for i, value := range values {
		handles[i], err = manager.manager.RefGet(value)
		if err != nil {
			return err
		}
	}
	if delta == nil {
		_, err = target.AddFact(handles)
		return err
	}
	found, err := target.ContainsFact(handles)
	if err != nil || found {
		return err
	}
	if err = target.AddNewFact(handles); err != nil {
		return err
	}
	_, err = delta.AddFact(handles)
	return err
}

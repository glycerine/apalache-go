package rel

// Source: src/muz/rel/dl_external_relation.cpp, project and rename functions.

import "github.com/glycerine/g3/pkg/ast"

// Sorts returns a caller-owned copy of the relation signature.
func (r *ExternalRelation) Sorts() ([]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), r.sorts...), nil
}

func (r *ExternalRelation) reduceRelation(app ast.ASTHandle, args, sorts []ast.ASTHandle) (out *ExternalRelation, resultErr error) {
	appRef, err := r.ast.NewRef(app)
	if err != nil {
		return nil, err
	}
	owner := &externalRelationConstructionOwner{manager: r.ast, temps: []ast.ASTRef{appRef}}
	defer func() {
		resultErr = owner.dispose(resultErr)
		if resultErr != nil {
			out = nil
		}
	}()
	decl, err := r.ast.GetDecl(app)
	if err != nil {
		return nil, err
	}
	value, err := r.reduce(decl, args)
	if err != nil {
		return nil, err
	}
	ref, err := r.ast.NewRef(value)
	if err != nil {
		return nil, err
	}
	r.plugin.users++
	out = &ExternalRelation{plugin: r.plugin, manager: r.manager, ast: r.ast, sorts: append([]ast.ASTHandle(nil), sorts...), relation: ref}
	owner.result = out
	return out, nil
}

// Project removes sorted, distinct columns and invokes the project reduction.
func (r *ExternalRelation) Project(removed []uint32) (*ExternalRelation, error) {
	expression, err := r.Relation()
	if err != nil {
		return nil, err
	}
	if len(removed) == 0 {
		return nil, ast.ErrASTDomain
	}
	sorts := make([]ast.ASTHandle, 0, len(r.sorts))
	next := 0
	for i, sort := range r.sorts {
		if next < len(removed) && removed[next] == uint32(i) {
			next++
			continue
		}
		sorts = append(sorts, sort)
	}
	if next != len(removed) {
		return nil, ast.ErrASTDomain
	}
	app, err := r.ast.MkDatalogProject(expression, removed)
	if err != nil {
		return nil, err
	}
	return r.reduceRelation(app, []ast.ASTHandle{expression}, sorts)
}

// Rename rotates the source signature and invokes the rename reduction.
func (r *ExternalRelation) Rename(cycle []uint32) (*ExternalRelation, error) {
	expression, err := r.Relation()
	if err != nil {
		return nil, err
	}
	if len(cycle) < 2 {
		return nil, ast.ErrASTDomain
	}
	seen := make([]bool, len(r.sorts))
	for _, column := range cycle {
		if uint64(column) >= uint64(len(seen)) || seen[column] {
			return nil, ast.ErrASTDomain
		}
		seen[column] = true
	}
	sorts := append([]ast.ASTHandle(nil), r.sorts...)
	for i, column := range cycle {
		sorts[column] = r.sorts[cycle[(i+1)%len(cycle)]]
	}
	app, err := r.ast.MkDatalogRename(expression, cycle)
	if err != nil {
		return nil, err
	}
	return r.reduceRelation(app, []ast.ASTHandle{expression}, sorts)
}

// JoinExternalRelations concatenates signatures and invokes the source join
// reduction for relations created by the same external plugin.
func JoinExternalRelations(left, right *ExternalRelation, leftColumns, rightColumns []uint32) (*ExternalRelation, error) {
	if left.open() != nil || right.open() != nil || left.plugin != right.plugin || len(leftColumns) != len(rightColumns) {
		return nil, ErrTableRelation
	}
	indices := make([]uint32, 0, 2*len(leftColumns))
	for i, l := range leftColumns {
		r := rightColumns[i]
		if uint64(l) >= uint64(len(left.sorts)) || uint64(r) >= uint64(len(right.sorts)) || left.sorts[l] != right.sorts[r] {
			return nil, ast.ErrASTDomain
		}
		indices = append(indices, l, r)
	}
	l, err := left.Relation()
	if err != nil {
		return nil, err
	}
	r, err := right.Relation()
	if err != nil {
		return nil, err
	}
	app, err := left.ast.MkDatalogJoin(l, r, indices)
	if err != nil {
		return nil, err
	}
	sorts := append(append([]ast.ASTHandle(nil), left.sorts...), right.sorts...)
	return left.reduceRelation(app, []ast.ASTHandle{l, r}, sorts)
}

// Union invokes the source union or widening assignment callback. Delta, when
// provided, is the second output expression supplied to that callback.
func (r *ExternalRelation) Union(source, delta *ExternalRelation, widen bool) (resultErr error) {
	if r.open() != nil || source.open() != nil || r.plugin != source.plugin || len(r.sorts) != len(source.sorts) {
		return ErrTableRelation
	}
	for i := range r.sorts {
		if r.sorts[i] != source.sorts[i] {
			return ErrTableRelation
		}
	}
	if delta != nil {
		if delta.open() != nil || delta.plugin != r.plugin || len(delta.sorts) != len(r.sorts) {
			return ErrTableRelation
		}
		for i := range r.sorts {
			if delta.sorts[i] != r.sorts[i] {
				return ErrTableRelation
			}
		}
	}
	target, err := r.Relation()
	if err != nil {
		return err
	}
	input, err := source.Relation()
	if err != nil {
		return err
	}
	var app ast.ASTHandle
	if widen {
		app, err = r.ast.MkDatalogWiden(target, input)
	} else {
		app, err = r.ast.MkDatalogUnion(target, input)
	}
	if err != nil {
		return err
	}
	appRef, err := r.ast.NewRef(app)
	if err != nil {
		return err
	}
	attempt := &externalAssignmentAttempt{manager: r.ast, roots: []ast.ASTRef{appRef}}
	defer func() { resultErr = finishExternalAssignment(attempt, resultErr) }()
	decl, err := r.ast.GetDecl(app)
	if err != nil {
		return err
	}
	outputs := []ast.ASTHandle{target}
	if delta != nil {
		value, err := delta.Relation()
		if err != nil {
			return err
		}
		outputs = append(outputs, value)
	}
	return r.plugin.reduceAssign(decl, []ast.ASTHandle{target, input}, outputs)
}

package rel

// Source: src/muz/rel/dl_external_relation.cpp, filter operations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *ExternalRelation) applyAssignment(app ast.ASTHandle, args, outputs []ast.ASTHandle) (resultErr error) {
	ref, err := r.ast.NewRef(app)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&ref)) }()
	decl, err := r.ast.GetDecl(app)
	if err != nil {
		return err
	}
	return r.plugin.reduceAssign(decl, args, outputs)
}

// FilterInterpreted invokes the source filter assignment with a Boolean
// application over the relation columns.
func (r *ExternalRelation) FilterInterpreted(condition ast.ASTHandle) error {
	expression, err := r.Relation()
	if err != nil {
		return err
	}
	kind, err := r.ast.GetKind(condition)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	app, err := r.ast.MkDatalogFilter(expression, condition)
	if err != nil {
		return err
	}
	args := []ast.ASTHandle{expression}
	return r.applyAssignment(app, args, args)
}

func (r *ExternalRelation) filterEquality(column uint32, value ast.ASTHandle, valueFirst bool) (resultErr error) {
	if r.open() != nil || uint64(column) >= uint64(len(r.sorts)) {
		return ast.ErrASTDomain
	}
	variable, err := r.ast.MkVar(column, r.sorts[column])
	if err != nil {
		return err
	}
	variableRef, err := r.ast.NewRef(variable)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&variableRef)) }()
	left, right := variable, value
	if valueFirst {
		left, right = value, variable
	}
	equality, err := r.ast.MkEq(left, right)
	if err != nil {
		return err
	}
	ref, err := r.ast.NewRef(equality)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&ref)) }()
	return r.FilterInterpreted(equality)
}

func (r *ExternalRelation) FilterEqual(column uint32, value ast.ASTHandle) error {
	return r.filterEquality(column, value, false)
}

// FilterIdentical sends one source equality filter for every column after the
// first. The order of callback invocations matches the input column list.
func (r *ExternalRelation) FilterIdentical(columns []uint32) (resultErr error) {
	if err := r.open(); err != nil {
		return err
	}
	for i, column := range columns {
		if uint64(column) >= uint64(len(r.sorts)) || i > 0 && r.sorts[column] != r.sorts[columns[0]] {
			return ast.ErrASTDomain
		}
	}
	if len(columns) < 2 {
		return nil
	}
	first, err := r.ast.MkVar(columns[0], r.sorts[columns[0]])
	if err != nil {
		return err
	}
	ref, err := r.ast.NewRef(first)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&ref)) }()
	for _, column := range columns[1:] {
		if err = r.filterEquality(column, first, true); err != nil {
			return err
		}
	}
	return nil
}

// FilterNegation invokes the source negative relation filter assignment.
func (r *ExternalRelation) FilterNegation(negative *ExternalRelation, columns, negativeColumns []uint32) error {
	if r.open() != nil || negative.open() != nil || r.plugin != negative.plugin || len(columns) != len(negativeColumns) {
		return ErrTableRelation
	}
	indices := make([]uint32, 0, 2*len(columns))
	for i, column := range columns {
		n := negativeColumns[i]
		if uint64(column) >= uint64(len(r.sorts)) || uint64(n) >= uint64(len(negative.sorts)) || r.sorts[column] != negative.sorts[n] {
			return ast.ErrASTDomain
		}
		indices = append(indices, column, n)
	}
	target, err := r.Relation()
	if err != nil {
		return err
	}
	other, err := negative.Relation()
	if err != nil {
		return err
	}
	app, err := r.ast.MkDatalogNegationFilter(target, other, indices)
	if err != nil {
		return err
	}
	args := []ast.ASTHandle{target, other}
	return r.applyAssignment(app, args, args[:1])
}

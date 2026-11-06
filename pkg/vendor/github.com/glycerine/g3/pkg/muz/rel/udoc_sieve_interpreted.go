package rel

// Source: dl_sieve_relation.cpp mk_filter_interpreted_fn. A condition using
// any column outside the inner sieve is an identity; otherwise variables are
// remapped to the inner relation's dense column numbering.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (r *UDocSieve) FilterInterpreted(condition ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) (resultErr error) {
	if err := r.open(); err != nil {
		return err
	}
	used, err := ast.NewASTUsedVars(r.manager)
	if err != nil {
		return err
	}
	if err = used.Collect(condition); err != nil {
		return err
	}
	if used.NumVars() == 0 {
		return r.inner.FilterInterpreted(condition)
	}
	var maximum uint32
	for variable := range sorts {
		if used.Contains(variable).IsNull() {
			continue
		}
		if variable > maximum {
			maximum = variable
		}
		column, found := columns[variable]
		if !found || column < 0 || column >= len(r.sigToInner) {
			return ErrSieve
		}
		if r.sigToInner[column] < 0 {
			// The source sieve cannot refine an ignored original column.
			return nil
		}
	}
	if uint32(len(sorts)) < used.NumVars() {
		return ErrSieve
	}
	bindings := make([]ast.ASTHandle, uint64(maximum)+1)
	var refs []ast.ASTRef
	defer func() {
		for index := len(refs) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[index]))
		}
	}()
	for variable := range sorts {
		if used.Contains(variable).IsNull() {
			continue
		}
		column := columns[variable]
		inner := r.sigToInner[column]
		value, cause := r.manager.MkVar(uint32(inner), r.sorts[column])
		if cause != nil {
			return cause
		}
		ref, cause := r.manager.NewRef(value)
		if cause != nil {
			return cause
		}
		refs = append(refs, ref)
		bindings[variable] = value
	}
	substitution, err := rewriter.NewVarSubstitution(r.manager, false)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	out, err := r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	refs = append(refs, out)
	if err = substitution.Substitute(condition, bindings, out); err != nil {
		return err
	}
	mapped, err := r.manager.RefGet(out)
	if err != nil {
		return err
	}
	return r.inner.FilterInterpreted(mapped)
}

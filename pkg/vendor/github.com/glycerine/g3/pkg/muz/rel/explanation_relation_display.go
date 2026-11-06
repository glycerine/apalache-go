package rel

// Source: src/muz/rel/dl_mk_explanations.cpp, explanation_relation formula
// and display methods.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// ToFormula returns an owning equality between variable zero and the first
// explanation, as in the source relation's to_formula implementation.
func (r *ExplanationRelation) ToFormula() (ast.ASTRef, error) {
	if err := r.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if len(r.data) == 0 || r.data[0] == (ast.ASTRef{}) {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	value, err := r.manager.RefGet(r.data[0])
	if err != nil {
		return ast.ASTRef{}, err
	}
	sort, err := r.manager.GetSort(value)
	if err != nil {
		return ast.ASTRef{}, err
	}
	variable, err := r.manager.MkVar(0, sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	equality, err := r.manager.MkEq(variable, value)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return r.manager.NewRef(equality)
}

// Display follows the source display, including its repeated first-column
// explanation when a relation has multiple columns.
func (r *ExplanationRelation) Display(out io.Writer) error {
	if err := r.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if r.empty {
		_, err := io.WriteString(out, "<empty explanation relation>\n")
		return err
	}
	printer, err := ast.NewLegacySMTPrinter(r.manager)
	if err != nil {
		return err
	}
	for i := range r.sorts {
		if i != 0 {
			if _, err = io.WriteString(out, ", "); err != nil {
				return err
			}
		}
		if r.data[0] == (ast.ASTRef{}) {
			if _, err = io.WriteString(out, "<undefined>"); err != nil {
				return err
			}
		} else {
			value, err := r.manager.RefGet(r.data[0])
			if err != nil {
				return err
			}
			if err = printer.DisplayExpr(out, value, nil); err != nil {
				return err
			}
		}
	}
	_, err = fmt.Fprintln(out)
	return err
}

// UnionForeign applies the source foreign-relation union. The foreign
// relation contributes only its empty/nonempty state to this operation.
func (r *ExplanationRelation) UnionForeign(sourceEmpty bool, delta *ExplanationRelation) error {
	if err := r.open(); err != nil {
		return err
	}
	if delta != nil {
		if err := delta.open(); err != nil {
			return err
		}
		if delta.manager != r.manager {
			return ast.ErrASTHandle
		}
	}
	if sourceEmpty {
		return nil
	}
	if err := r.assignData(make([]ast.ASTHandle, len(r.sorts))); err != nil {
		return err
	}
	if delta != nil {
		return delta.assignData(make([]ast.ASTHandle, len(delta.sorts)))
	}
	return nil
}

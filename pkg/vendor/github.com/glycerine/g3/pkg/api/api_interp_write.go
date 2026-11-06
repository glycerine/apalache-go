package api

// Source: src/api/api_interp.cpp:447-467, tree-pattern construction for
// Z3_write_interpolation_problem.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

// WriteInterpolationProblem writes the source interpolation file format for
// sequence constraints (parents == nil) or a tree whose root is the last
// constraint. Theory formulas follow the constraints, matching the source
// vector append order.
func (c *InterpolationContext) WriteInterpolationProblem(out io.Writer, constraints []ast.ASTHandle, parents []int, theory []ast.ASTHandle) (resultErr error) {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if len(constraints) == 0 {
		if len(parents) != 0 {
			return interp.NewError("interpolation parents without constraints")
		}
		return nil
	}
	if parents != nil {
		if len(parents) != len(constraints) || parents[len(parents)-1] != interp.ScopeTop {
			return interp.NewError("invalid interpolation parent vector")
		}
		for child := 0; child < len(parents)-1; child++ {
			if parents[child] <= child || parents[child] >= len(parents) {
				return interp.NewError("interpolation parents must follow children")
			}
		}
	}

	for _, formula := range append(append([]ast.ASTHandle(nil), constraints...), theory...) {
		formulaSort, err := c.manager.GetSort(formula)
		if err != nil {
			return err
		}
		boolean, err := c.terms.IsBoolType(formulaSort)
		if err != nil {
			return err
		}
		if !boolean {
			return ast.ErrASTDomain
		}
	}

	refs := make([]ast.ASTRef, 0, len(constraints)*2)
	defer func() {
		for index := len(refs) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&refs[index]))
		}
	}()
	hold := func(expression ast.ASTHandle) error {
		ref, err := c.manager.NewRef(expression)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		return nil
	}
	and := func(parts []ast.ASTHandle) (ast.ASTHandle, error) {
		if len(parts) == 1 {
			return parts[0], nil
		}
		return c.manager.MkAndArray(parts)
	}

	children := make([][]ast.ASTHandle, len(constraints))
	for index := 0; index < len(constraints)-1; index++ {
		parts := append(children[index], constraints[index])
		node, err := and(parts)
		if err != nil {
			return err
		}
		marked, err := c.manager.MkInterp(node)
		if err != nil {
			return err
		}
		if err = hold(marked); err != nil {
			return err
		}
		parent := index + 1
		if parents != nil {
			parent = parents[index]
		}
		children[parent] = append(children[parent], marked)
	}
	rootParts := append(children[len(constraints)-1], constraints[len(constraints)-1])
	pattern, err := and(rootParts)
	if err != nil {
		return err
	}
	allAssertions := make([]ast.ASTHandle, 0, len(theory)+len(constraints))
	allAssertions = append(allAssertions, constraints...)
	allAssertions = append(allAssertions, theory...)
	return interp.PrettyPrintProblem(c.manager, allAssertions, pattern, out)
}

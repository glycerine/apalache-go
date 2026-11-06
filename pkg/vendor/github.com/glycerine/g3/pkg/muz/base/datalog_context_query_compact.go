package base

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func queryUsedSorts(manager *ast.ASTManager, body ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	used, err := ast.NewASTUsedVars(manager)
	if err != nil {
		return nil, false, err
	}
	if err = used.Collect(body); err != nil {
		return nil, false, err
	}
	sorts := make([]ast.ASTHandle, used.MaxFoundVarIndexPlusOne())
	dense := true
	for index := range sorts {
		sorts[index] = used.Contains(uint32(index))
		if sorts[index].IsNull() {
			dense = false
		}
	}
	return sorts, dense, nil
}

func (c *DatalogContext) compactQueryVariables(body ast.ASTHandle) (_ ast.ASTRef, _ []ast.ASTHandle, resultErr error) {
	sorts, dense, err := queryUsedSorts(c.manager, body)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	if dense {
		result, err := c.manager.NewRef(body)
		return result, sorts, err
	}
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	arguments := make([]ast.ASTHandle, len(sorts))
	compact := make([]ast.ASTHandle, 0, len(sorts))
	for index, sort := range sorts {
		if sort.IsNull() {
			arguments[index], err = c.manager.MkVar(0, boolean)
		} else {
			arguments[index], err = c.manager.MkVar(uint32(len(compact)), sort)
			compact = append(compact, sort)
		}
		if err != nil {
			return ast.ASTRef{}, nil, err
		}
	}
	substitution, err := rewriter.NewVarSubstitution(c.manager, false)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	result, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	if err = substitution.Substitute(body, arguments, result); err != nil {
		return ast.ASTRef{}, nil, errors.Join(err, c.manager.DelRef(&result))
	}
	return result, compact, nil
}

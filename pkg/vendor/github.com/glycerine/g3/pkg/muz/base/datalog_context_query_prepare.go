package base

// Source: src/muz/base/dl_rule.cpp rule_manager::mk_query implicit binding
// and existential-prefix removal.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *DatalogContext) prepareQuery(query ast.ASTHandle) (_ ast.ASTRef, names []usymbol.Symbol, resultErr error) {
	bound, err := c.BindVariables(query, false)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&bound)) }()
	boundHandle, err := c.manager.RefGet(bound)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	hoister, err := rewriter.NewQuantifierHoisterWithoutRewrite(c.manager)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, hoister.Close()) }()
	hoisted, err := hoister.PullQuantifierIndexed(false, boundHandle, true, false)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, hoisted.Close()) }()
	body, err := c.manager.RefGet(hoisted.Formula)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	result, err := c.manager.NewRef(body)
	if err != nil {
		return ast.ASTRef{}, nil, err
	}
	return result, append([]usymbol.Symbol(nil), hoisted.Names...), nil
}

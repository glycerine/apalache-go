package base

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// AddQuery ports rule_manager::mk_query into the owning DatalogContext. It
// returns the new borrowed output-predicate declaration.
func (c *DatalogContext) AddQuery(query ast.ASTHandle) (_ ast.ASTHandle, resultErr error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	prepared, names, err := c.prepareQuery(query)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, closeQueryRefs(c.manager, prepared)) }()
	body, err := c.manager.RefGet(prepared)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	compacted, sorts, err := c.compactQueryVariables(body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, closeQueryRefs(c.manager, compacted)) }()
	body, err = c.manager.RefGet(compacted)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sorts, names, err = c.querySignature(sorts, names)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	prefix, err := c.manager.Symbols.Intern("query")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	predicate, err := c.FreshHeadPredicate(prefix, usymbol.Symbol{}, sorts)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.rules.SetOutputPredicate(predicate); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.addQueryRule(body, predicate, sorts, names); err != nil {
		return ast.ASTHandle{}, err
	}
	return predicate, nil
}

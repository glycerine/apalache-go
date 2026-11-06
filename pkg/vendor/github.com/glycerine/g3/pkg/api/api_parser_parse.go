package api

// Source: src/api/api_parsers.cpp:39-108.

import (
	"errors"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

// ParseSMT2 parses assertions without mutating the owning solver session. The
// returned reference owns their conjunction in the owning context's manager.
func (c *SMTContext) ParseSMT2(input io.Reader, bindings SMT2Bindings) (result ast.ASTRef, resultErr error) {
	if input == nil {
		return result, ErrSMTContextOwner
	}
	manager, err := c.Manager()
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	parser, err := c.newSMT2ParserContext(bindings)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, parser.Close()) }()
	if err = smt2.ParseAssertions(parser, input); err != nil {
		return result, err
	}
	assertions, err := parser.Assertions()
	if err != nil {
		return result, err
	}
	conjunction, err := manager.MkAndArray(assertions)
	if err != nil {
		return result, err
	}
	return manager.NewRef(conjunction)
}

func (c *SMTContext) ParseSMT2String(input string, bindings SMT2Bindings) (ast.ASTRef, error) {
	return c.ParseSMT2(strings.NewReader(input), bindings)
}

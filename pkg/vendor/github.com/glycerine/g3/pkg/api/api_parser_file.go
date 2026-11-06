package api

// Source: src/api/api_parsers.cpp:88-108.

import (
	"errors"
	"os"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) ParseSMT2File(path string, bindings SMT2Bindings) (result ast.ASTRef, resultErr error) {
	input, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, input.Close()) }()
	return c.ParseSMT2(input, bindings)
}

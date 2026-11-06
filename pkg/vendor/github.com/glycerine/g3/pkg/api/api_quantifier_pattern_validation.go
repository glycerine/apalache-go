package api

// Source: src/api/api_quant.cpp, mk_quantifier_ex_core pattern validation.

import (
	"github.com/glycerine/g3/pkg/ast"
	parserutil "github.com/glycerine/g3/pkg/parsers/util"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func validateQuantifierPatterns(m *ast.ASTManager, qid usymbol.Symbol, count uint32, patterns []ast.ASTHandle) error {
	if len(patterns) == 0 {
		return nil
	}
	recursive, err := m.RecursiveFunctionQID()
	if err != nil {
		return err
	}
	if qid == recursive {
		return nil
	}
	validator, err := parserutil.NewPatternValidator(m, nil)
	if err != nil {
		return err
	}
	for _, pattern := range patterns {
		valid, e := validator.ValidateNew(count, pattern, 0, 0)
		if e != nil {
			return e
		}
		if !valid {
			return ast.ErrASTDomain
		}
	}
	return nil
}

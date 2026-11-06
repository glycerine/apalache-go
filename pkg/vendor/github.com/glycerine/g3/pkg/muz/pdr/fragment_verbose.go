package pdr

// Source: pdr_util.cpp test_diff_logic::operator() rejection diagnostics.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c *arithmeticFragmentClassifier) verboseRejection(expression ast.ASTHandle) (result error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	prefix := "non-diff: "
	if c.utvpi {
		prefix = "non-utvpi: "
	}
	if err = out.AppendCString(prefix); err != nil {
		return err
	}
	context, globals, err := c.manager.RewriterParameterServices()
	if err != nil {
		return err
	}
	var format ast.ASTHandle
	if context != nil && globals != nil {
		format, err = c.manager.SMT2PPExprTreeWithParams(expression, nil, context, globals, nil)
	} else {
		format, err = c.manager.SMT2PPExprTree(expression, nil)
	}
	if err != nil {
		return err
	}
	formats, err := c.manager.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formats.DelRef(&ref)) }()
	if context != nil && globals != nil {
		empty := context.EmptyValue()
		err = c.manager.PrintFormat(out, format, context, &empty, globals)
	} else {
		err = c.manager.PrintFormat(out, format, nil, nil, nil)
	}
	if err != nil {
		return err
	}
	return out.AppendCString("\n")
}

package clp

// Source: src/muz/clp/clp_context.cpp search diagnostics.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (e *Engine) verboseSearch(depth uint32, index int) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return out.AppendCString(fmt.Sprintf("search %d %d\n", depth, index))
}

func (e *Engine) verboseRule(index int, head ast.ASTHandle) (result error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(2)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(fmt.Sprintf("%d ", index)); err != nil {
		return err
	}
	context, globals, err := e.manager.RewriterParameterServices()
	if err != nil {
		return err
	}
	var format ast.ASTHandle
	if context != nil && globals != nil {
		format, err = e.manager.SMT2PPExprTreeWithParams(head, nil, context, globals, nil)
	} else {
		format, err = e.manager.SMT2PPExprTree(head, nil)
	}
	if err != nil {
		return err
	}
	formats, err := e.manager.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return err
	}
	owner := &clpRefOwner{manager: formats}
	owner.take(ref)
	defer func() { result = errors.Join(result, owner.dispose()) }()
	if context != nil && globals != nil {
		empty := context.EmptyValue()
		err = e.manager.PrintFormat(out, format, context, &empty, globals)
	} else {
		err = e.manager.PrintFormat(out, format, nil, nil, nil)
	}
	if err != nil {
		return err
	}
	return out.AppendCString("\n")
}

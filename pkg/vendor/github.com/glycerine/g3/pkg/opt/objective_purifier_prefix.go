package opt

// Source: src/opt/opt_context.cpp:1100-1103. mk_pp uses the SMT2 printer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (p *ObjectivePurifier) definitionPrefix(term ast.ASTHandle) (_ string, resultErr error) {
	document, err := p.manager.SMT2PPExprTree(term, nil)
	if err != nil {
		return "", err
	}
	formats, err := p.manager.GetFormatManager()
	if err != nil {
		return "", err
	}
	ref, err := formats.NewRef(document)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: p.manager.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, stream.Close()) }()
	context, globals, err := p.manager.RewriterParameterServices()
	if err != nil {
		return "", err
	}
	if context != nil && globals != nil {
		empty := context.EmptyValue()
		err = p.manager.PrintFormat(stream, document, context, &empty, globals)
	} else {
		err = p.manager.PrintFormat(stream, document, nil, nil, nil)
	}
	if err != nil {
		return "", err
	}
	return stream.String()
}

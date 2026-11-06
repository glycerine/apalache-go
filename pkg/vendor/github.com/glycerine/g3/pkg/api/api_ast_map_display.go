package api

// Source: src/api/api_ast_map.cpp:139-157.

import (
	"bytes"

	"github.com/glycerine/g3/pkg/ast"
)

func writePublicASTMapNode(printer *ast.SimpleASTPrinterContext, out *bytes.Buffer, h ast.ASTHandle) error {
	kind, err := printer.Manager().GetKind(h)
	if err != nil {
		return err
	}
	switch kind {
	case ast.ASTSort:
		return printer.DisplaySort(out, h, 0)
	case ast.ASTFuncDecl:
		return printer.DisplayFuncDecl(out, h, 0)
	default:
		return printer.DisplayExpr(out, h, 0)
	}
}

func (m *ASTMap) String() (string, error) {
	if err := m.check(); err != nil {
		return "", err
	}
	keys, err := m.object.keys()
	if err != nil {
		return "", err
	}
	printer, err := ast.NewSimpleASTPrinterContext(m.owner.manager, nil, nil)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	out.WriteString("(ast-map")
	for _, key := range keys {
		value, err := m.object.find(key)
		if err != nil {
			return "", err
		}
		out.WriteString("\n  (")
		if err = writePublicASTMapNode(printer, &out, key); err != nil {
			return "", err
		}
		out.WriteString("\n   ")
		if err = writePublicASTMapNode(printer, &out, value); err != nil {
			return "", err
		}
		out.WriteByte(')')
	}
	out.WriteByte(')')
	return out.String(), nil
}

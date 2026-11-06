package base

// Source: src/muz/base/hnf.cpp:168-172,337,524-531. mk_pp derives from
// mk_ismt2_pp, so all three sites use the same SMT2 expression printer.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (n *HornNormalizer) writeHornTraceExpr(w io.Writer, h ast.ASTHandle) (result error) {
	m := n.manager
	context, globals, err := m.RewriterParameterServices()
	if err != nil {
		return err
	}
	var format ast.ASTHandle
	if context != nil && globals != nil {
		format, err = m.SMT2PPExprTreeWithParams(h, nil, context, globals, nil)
	} else {
		format, err = m.SMT2PPExprTree(h, nil)
	}
	if err != nil {
		return err
	}
	// mk_ismt2_pp's format_ref releases the printer tree after insertion.
	formats, err := m.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if context != nil && globals != nil {
		empty := context.EmptyValue()
		err = m.PrintFormat(stream, format, context, &empty, globals)
	} else {
		err = m.PrintFormat(stream, format, nil, nil, nil)
	}
	if err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, body+"\n")
	return err
}

func (n *HornNormalizer) traceHornBody(h ast.ASTHandle) error {
	tags := n.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("hnf", "eliminate_disjunctions", "src/muz/base/hnf.cpp", 337,
		func() bool { return true }, func(w io.Writer) error { return n.writeHornTraceExpr(w, h) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (n *HornNormalizer) traceHornResult(formula ast.ASTHandle, result *HornNormalization, alreadyHorn bool) error {
	tags := n.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace("hnf") {
		return nil
	}
	for _, site := range []struct {
		line  int
		label string
	}{{168, "==>\n"}, {524, "HNF result:\n"}} {
		if alreadyHorn && site.line == 168 {
			continue
		}
		err := tags.EmitConditional("hnf", "operator()", "src/muz/base/hnf.cpp", site.line,
			func() bool { return true }, func(w io.Writer) error {
				if err := n.writeHornTraceExpr(w, formula); err != nil {
					return err
				}
				if _, err := io.WriteString(w, site.label); err != nil {
					return err
				}
				for _, ref := range result.Formulas {
					h, err := n.manager.RefGet(ref)
					if err != nil {
						return err
					}
					if err = n.writeHornTraceExpr(w, h); err != nil {
						return err
					}
				}
				return nil
			})
		if err != nil && !tags.IsStreamFailure(err) {
			return err
		}
	}
	return nil
}

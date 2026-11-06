package model

// Source: src/model/model_v2_pp.cpp:22-83. The retained extension-model trace
// uses this concrete ordered constant/function display.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (md *ModelCore) writeV2Expr(out io.Writer, h ast.ASTHandle, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (result error) {
	format, err := md.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: md.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := context.EmptyValue()
	if err = md.manager.PrintFormat(stream, format, context, &empty, globals); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	return err
}

func (md *ModelCore) v2Name(d ast.ASTHandle) (string, error) {
	name, err := md.manager.GetASTName(d)
	if err != nil {
		return "", err
	}
	return md.manager.Symbols.String(name)
}

// WriteV2 displays the retained model_v2_pp constant and function graph in
// source registration order. The parameters and globals are borrowed.
func (md *ModelCore) WriteV2(out io.Writer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, partial bool) error {
	if err := md.open(); err != nil {
		return err
	}
	if out == nil || context == nil || globals == nil || globals.Context != context {
		return ast.ErrASTHandle
	}
	write := func(s string) error { _, err := io.WriteString(out, s); return err }
	for _, d := range md.ConstantDeclarations {
		name, err := md.v2Name(d)
		if err != nil {
			return err
		}
		if err = write(name + " -> "); err != nil {
			return err
		}
		value, err := md.ConstantInterpretation(d)
		if err != nil {
			return err
		}
		if err = md.writeV2Expr(out, value, context, globals); err != nil {
			return err
		}
		if err = write("\n"); err != nil {
			return err
		}
	}
	for _, d := range md.functionDeclarations {
		name, err := md.v2Name(d)
		if err != nil {
			return err
		}
		if err = write(name + " -> {\n"); err != nil {
			return err
		}
		g, err := md.FunctionInterpretation(d)
		if err != nil {
			return err
		}
		if g == nil {
			return ast.ErrASTPending
		}
		if err = g.open(); err != nil {
			return err
		}
		for _, entry := range g.entries {
			if err = write("  "); err != nil {
				return err
			}
			for _, arg := range entry.args {
				if err = md.writeV2Expr(out, arg, context, globals); err != nil {
					return err
				}
				if err = write(" "); err != nil {
					return err
				}
			}
			if err = write("-> "); err != nil {
				return err
			}
			if err = md.writeV2Expr(out, entry.result, context, globals); err != nil {
				return err
			}
			if err = write("\n"); err != nil {
				return err
			}
		}
		elsePrefix := "  "
		if len(g.entries) != 0 {
			elsePrefix = "  else -> "
		}
		if err = write(elsePrefix); err != nil {
			return err
		}
		if partial || g.fallback.IsNull() {
			if err = write("#unspecified"); err != nil {
				return err
			}
		} else if err = md.writeV2Expr(out, g.fallback, context, globals); err != nil {
			return err
		}
		if err = write("\n}\n"); err != nil {
			return err
		}
	}
	return nil
}

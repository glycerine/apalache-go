package arith

// Source: src/tactic/arith/fm_tactic.cpp:184,197,204,207,213,240,243.

import (
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (c *FMModelConverter) traceModel(md *model.ModelCore, line int, withConverter bool) error {
	return emitFMTrace(c.manager, c.services, "fm_mc", "operator()", line, func(out io.Writer) error {
		if err := md.WriteV2(out, c.services.Context, c.services.Globals, false); err != nil {
			return err
		}
		if withConverter {
			return c.Display(out)
		}
		return nil
	})
}
func (c *FMModelConverter) traceVariable(variable ast.ASTHandle, value ast.ASTHandle) error {
	line := 197
	if !value.IsNull() {
		line = 240
	}
	return emitFMTrace(c.manager, c.services, "fm_mc", "operator()", line, func(out io.Writer) error {
		symbol, err := c.manager.GetASTName(variable)
		if err != nil {
			return err
		}
		name, err := c.manager.Symbols.Display(symbol)
		if err != nil {
			return err
		}
		if value.IsNull() {
			return fmDisplayWrite(out, "processing "+name+"\n")
		}
		if err = fmDisplayWrite(out, name+" --> "); err != nil {
			return err
		}
		if err = writeFMExpression(c.manager, c.services, out, value); err != nil {
			return err
		}
		return fmDisplayWrite(out, "\n")
	})
}
func (c *FMModelConverter) traceBound(kind fmBoundKind, value *big.Rat, clause ast.ASTHandle) error {
	line := 204
	if kind == fmLowerBound {
		line = 207
	} else if kind == fmUpperBound {
		line = 213
	}
	return emitFMTrace(c.manager, c.services, "fm_mc", "operator()", line, func(out io.Writer) error {
		if kind == fmNoBound {
			if err := fmDisplayWrite(out, "no bound for:\n"); err != nil {
				return err
			}
		} else {
			direction := "lower"
			if kind == fmUpperBound {
				direction = "upper"
			}
			if _, err := fmt.Fprintf(out, "%s bound: %s for:\n", direction, value.RatString()); err != nil {
				return err
			}
		}
		if err := writeFMExpression(c.manager, c.services, out, clause); err != nil {
			return err
		}
		return fmDisplayWrite(out, "\n")
	})
}

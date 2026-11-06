package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp:166,198.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *factorCall) traceAddVisit(index int, h ast.ASTHandle) error {
	return c.owner.emitFactorTrace("mk_adds", 166, func(w io.Writer, g *uparams.GlobalParamsService) error {
		if _, err := io.WriteString(w, strconv.Itoa(index)+" "); err != nil {
			return err
		}
		if err := c.owner.writeFactorAST(w, h, g); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}

func (c *factorCall) traceAdds() error {
	return c.owner.emitFactorTrace("mk_adds", 198, func(w io.Writer, g *uparams.GlobalParamsService) error {
		for _, term := range c.adds {
			sign := "+"
			if !term.positive {
				sign = "-"
			}
			if _, err := io.WriteString(w, sign); err != nil {
				return err
			}
			if err := c.owner.writeFactorAST(w, term.expr, g); err != nil {
				return err
			}
			if _, err := io.WriteString(w, " "); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}

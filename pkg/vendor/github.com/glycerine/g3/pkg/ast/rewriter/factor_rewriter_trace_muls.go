package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp:219-227.

import (
	"io"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *factorCall) traceMuls() error {
	return c.owner.emitFactorTrace("mk_muls", 219, func(w io.Writer, g *uparams.GlobalParamsService) error {
		for _, row := range c.muls {
			for _, h := range row {
				if err := c.owner.writeFactorAST(w, h, g); err != nil {
					return err
				}
				if _, err := io.WriteString(w, " "); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}

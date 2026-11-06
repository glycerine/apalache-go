package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp:332-338.

import (
	"io"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *factorCall) traceFactors() error {
	return c.owner.emitFactorTrace("extract_factors", 332, func(w io.Writer, g *uparams.GlobalParamsService) error {
		for _, h := range c.factors {
			if err := c.owner.writeFactorAST(w, h, g); err != nil {
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

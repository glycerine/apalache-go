package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp:56,77,90,104,119.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *factorCall) traceRelation(w io.Writer, g *uparams.GlobalParamsService, lhs ast.ASTHandle, op string, rhs ast.ASTHandle) error {
	if err := c.owner.writeFactorAST(w, lhs, g); err != nil {
		return err
	}
	if _, err := io.WriteString(w, " "+op+" "); err != nil {
		return err
	}
	if err := c.owner.writeFactorAST(w, rhs, g); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func (c *factorCall) traceRelationRefusal(op int32, lhs, rhs ast.ASTHandle) error {
	line, spelling, function := 56, "=", "mk_eq"
	if op == factorOpLE {
		line, spelling, function = 77, "<=", "mk_le"
	}
	if op == factorOpLT {
		line, spelling, function = 104, "<", "mk_lt"
	}
	return c.owner.emitFactorTrace(function, line, func(w io.Writer, g *uparams.GlobalParamsService) error {
		return c.traceRelation(w, g, lhs, spelling, rhs)
	})
}

func (c *factorCall) traceLessEqualResult(lhs, rhs, result ast.ASTHandle) error {
	return c.owner.emitFactorTrace("mk_le", 90, func(w io.Writer, g *uparams.GlobalParamsService) error {
		if err := c.traceRelation(w, g, lhs, "<=", rhs); err != nil {
			return err
		}
		if err := c.owner.writeFactorAST(w, result, g); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}

func (c *factorCall) traceStrictResult(result ast.ASTHandle) error {
	return c.owner.emitFactorTrace("mk_lt", 119, func(w io.Writer, g *uparams.GlobalParamsService) error {
		if err := c.owner.writeFactorAST(w, result, g); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}

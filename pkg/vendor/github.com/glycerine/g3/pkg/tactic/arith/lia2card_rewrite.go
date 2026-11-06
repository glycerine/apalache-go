package arith

// Source: src/tactic/arith/lia2card_tactic.cpp lia_rewriter_cfg.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type lia2cardRewriteConfig struct {
	rewriter.DefaultConfiguration
	manager         *ast.ASTManager
	arithmetic      ast.FamilyID
	zeroOne         map[ast.ASTHandle]bool
	compileEquality bool
	trace           *diagnostic.TraceTags
	owner           *LIA2CardTactic
}

func (*lia2cardRewriteConfig) RewritePatterns() bool                 { return false }
func (*lia2cardRewriteConfig) FlatAssoc(ast.ASTHandle) (bool, error) { return false, nil }

func (c *lia2cardRewriteConfig) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	m := c.manager
	if err := m.SetRef(proof, ast.ASTHandle{}); err != nil {
		return rewriter.RewriteFailed, err
	}
	if args.Count() != 2 {
		return rewriter.RewriteFailed, nil
	}
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	op, err := m.GetDeclKind(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	equality := family == m.BasicFamilyID() && op == ast.ASTOpEq
	if !equality && (family != c.arithmetic || op < 2 || op > 5) {
		return rewriter.RewriteFailed, nil
	}
	left, err := args.At(0)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	right, err := args.At(1)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if equality {
		sort, e := m.GetSort(left)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		sortFamily, e := m.GetFamilyID(sort)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if sortFamily != c.arithmetic {
			return rewriter.RewriteFailed, nil
		}
	}
	negate := false
	if !equality {
		if op == 3 || op == 4 {
			left, right = right, left
		}
		negate = op == 4 || op == 5
	}
	converted, matched, err := lia2cardComparison(m, left, right, c.zeroOne, equality, c.compileEquality, c.trace)
	if err != nil || !matched {
		return rewriter.RewriteFailed, err
	}
	value, err := m.RefGet(converted)
	if err == nil && negate {
		value, err = m.MkNot(value)
	}
	if err == nil && c.owner != nil {
		err = c.owner.traceComparison(decl, args, value)
	}
	if err == nil {
		err = m.SetRef(out, value)
	}
	err = errors.Join(err, m.DelRef(&converted))
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	return rewriter.RewriteDone, nil
}

package arith

// Source: src/tactic/arith/eq2bv_tactic.cpp, eq_rewriter_cfg.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type eq2bvRewriteConfig struct {
	rewriter.DefaultConfiguration
	plan         *eq2bvReplacements
	replacements map[ast.ASTHandle]int
}

func (*eq2bvRewriteConfig) RewritePatterns() bool                 { return false }
func (*eq2bvRewriteConfig) FlatAssoc(ast.ASTHandle) (bool, error) { return false, nil }
func (c *eq2bvRewriteConfig) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	m := c.plan.manager
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
	kind, err := m.GetDeclKind(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if family != m.BasicFamilyID() || kind != ast.ASTOpEq {
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
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		index, found := c.replacements[pair[0]]
		if !found {
			continue
		}
		text, _, found, err := m.ArithmeticNumeralText(pair[1])
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		if !found {
			continue
		}
		value, ok := new(big.Rat).SetString(text)
		if !ok || !value.IsInt() || value.Sign() < 0 || value.Num().BitLen() > 32 {
			return rewriter.RewriteFailed, ast.ErrASTDomain
		}
		replacement := c.plan.entries[index]
		fresh, err := m.RefGet(replacement.fresh)
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		numeral, err := m.BitVectorNumeralUnsigned(uint32(value.Num().Uint64()), replacement.width)
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		result, err := m.MkEq(fresh, numeral)
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		return rewriter.RewriteDone, m.SetRef(out, result)
	}
	return rewriter.RewriteFailed, nil
}

// eq2bvRewrite returns an owning result; the replacement plan must remain open
// while the traversal runs and until its results are installed in a goal.
func eq2bvRewrite(plan *eq2bvReplacements, root ast.ASTHandle) (result ast.ASTRef, err error) {
	if plan == nil || plan.manager == nil {
		return result, ast.ErrASTHandle
	}
	cfg := &eq2bvRewriteConfig{plan: plan, replacements: make(map[ast.ASTHandle]int, len(plan.entries))}
	for i, entry := range plan.entries {
		cfg.replacements[entry.original] = i
	}
	walk, err := rewriter.NewTraversal(plan.manager, cfg)
	if err != nil {
		return result, err
	}
	owned := false
	defer func() {
		err = errors.Join(err, walk.Close())
		if err != nil && owned {
			err = errors.Join(err, plan.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	result, err = plan.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	owned = true
	if err = walk.Rewrite(root, result); err != nil {
		return result, err
	}
	return result, nil
}

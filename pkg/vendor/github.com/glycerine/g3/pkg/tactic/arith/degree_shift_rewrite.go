package arith

// Source: src/tactic/arith/degree_shift_tactic.cpp, rw_cfg.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type degreeShiftRewriteConfig struct {
	rewriter.DefaultConfiguration
	plan         *degreeShiftPlan
	replacements map[ast.ASTHandle]int
	proofs       bool
}

func (c *degreeShiftRewriteConfig) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (status rewriter.RewriteStatus, err error) {
	m := c.plan.manager
	if args.Count() != 2 {
		return rewriter.RewriteFailed, nil
	}
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	kind, err := m.GetDeclKind(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if family != arithmetic || kind != 18 {
		return rewriter.RewriteFailed, nil
	}
	base, err := args.At(0)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	index, found := c.replacements[base]
	if !found {
		return rewriter.RewriteFailed, nil
	}
	exponent, err := args.At(1)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	text, _, found, err := m.ArithmeticNumeralText(exponent)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !found || !ok || !value.IsInt() || value.Sign() <= 0 {
		return rewriter.RewriteFailed, ast.ErrASTDomain
	}
	entry := c.plan.entries[index]
	quotient, remainder := new(big.Int).QuoRem(value.Num(), entry.degree, new(big.Int))
	if remainder.Sign() != 0 {
		return rewriter.RewriteFailed, ast.ErrASTDomain
	}
	fresh, err := m.RefGet(entry.fresh)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	result := fresh
	if quotient.Cmp(big.NewInt(1)) != 0 {
		numeral, e := m.ArithmeticRealNumeralText(quotient.String())
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		ref, e := m.NewRef(numeral)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		result, err = m.MkFamilyApp(arithmetic, 18, nil, []ast.ASTHandle{fresh, numeral})
		err = errors.Join(err, m.DelRef(&ref))
		if err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	if err = m.SetRef(out, result); err != nil {
		return rewriter.RewriteFailed, err
	}
	if c.proofs {
		original, e := m.RewriterMakeApp(decl, args)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		originalRef, e := m.NewRef(original)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&originalRef)) }()
		fact, e := m.MkEq(original, result)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		factRef, e := m.NewRef(fact)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&factRef)) }()
		premise, e := m.RefGet(entry.proof)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		lemma, e := m.MkProofTheoryLemma(arithmetic, fact, []ast.ASTHandle{premise}, nil)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.SetRef(proof, lemma); e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	return rewriter.RewriteDone, nil
}

func newDegreeShiftTraversal(plan *degreeShiftPlan, proofs bool) (*rewriter.Traversal, error) {
	if plan == nil || plan.manager == nil {
		return nil, ast.ErrASTHandle
	}
	cfg := &degreeShiftRewriteConfig{plan: plan, proofs: proofs, replacements: make(map[ast.ASTHandle]int, len(plan.entries))}
	for i, entry := range plan.entries {
		h, err := plan.manager.RefGet(entry.original)
		if err != nil {
			return nil, err
		}
		cfg.replacements[h] = i
	}
	if proofs {
		return rewriter.NewProofTraversal(plan.manager, cfg)
	}
	return rewriter.NewTraversal(plan.manager, cfg)
}

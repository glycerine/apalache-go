package spacer

// Source: src/muz/spacer/spacer_util.cpp adhoc_rewriter_rpp and mk_epp.

import (
	"errors"
	"io"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type extendedPrettyConfig struct {
	rewriter.DefaultConfiguration
	m          *ast.ASTManager
	arithmetic ast.FamilyID
}

func (c *extendedPrettyConfig) binary(h ast.ASTHandle, op int32) (a, b ast.ASTHandle, matched bool, err error) {
	kind, err := c.m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return a, b, false, err
	}
	family, err := c.m.GetFamilyID(h)
	if err != nil || family != c.arithmetic {
		return a, b, false, err
	}
	k, err := c.m.GetDeclKind(h)
	if err != nil || k != op {
		return a, b, false, err
	}
	n, err := c.m.GetNumArgs(h)
	if err != nil || n != 2 {
		return a, b, false, err
	}
	a, err = c.m.GetArg(h, 0)
	if err == nil {
		b, err = c.m.GetArg(h, 1)
	}
	return a, b, err == nil, err
}

func (c *extendedPrettyConfig) numeral(h ast.ASTHandle, want string) (bool, error) {
	text, _, matched, err := c.m.ArithmeticNumeralText(h)
	return matched && text == want, err
}

func (c *extendedPrettyConfig) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (_ rewriter.RewriteStatus, resultErr error) {
	family, err := c.m.GetFamilyID(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	op, err := c.m.GetDeclKind(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if family == c.m.BasicFamilyID() && op == ast.ASTOpNot && args.Count() == 1 {
		child, err := args.At(0)
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		for _, pair := range [][2]int32{{4, 3}, {2, 5}, {5, 2}, {3, 4}} {
			a, b, match, err := c.binary(child, pair[0])
			if err != nil {
				return rewriter.RewriteFailed, err
			}
			if match {
				h, err := c.m.MkFamilyApp(c.arithmetic, pair[1], nil, []ast.ASTHandle{a, b})
				if err == nil {
					err = c.m.SetRef(out, h)
				}
				return rewriter.RewriteDone, err
			}
		}
	}
	order := family == c.arithmetic && op >= 2 && op <= 5
	equality := family == c.m.BasicFamilyID() && op == ast.ASTOpEq
	if args.Count() != 2 || (!order && !equality) {
		return rewriter.RewriteFailed, nil
	}
	lhs, err := args.At(0)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	rhs, err := args.At(1)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	zero, err := c.numeral(rhs, "0")
	if err != nil || equality && !zero {
		return rewriter.RewriteFailed, err
	}
	a, negative, match, err := c.binary(lhs, 6)
	if err != nil || !match {
		return rewriter.RewriteFailed, err
	}
	coefficient, b, match, err := c.binary(negative, 9)
	if err != nil || !match {
		return rewriter.RewriteFailed, err
	}
	minusOne, err := c.numeral(coefficient, "-1")
	if err != nil || !minusOne {
		return rewriter.RewriteFailed, err
	}
	var h ast.ASTHandle
	if equality {
		h, err = c.m.MkEq(a, b)
	} else {
		if !zero {
			b, err = c.m.MkArithmeticAdd(b, rhs)
			if err != nil {
				return rewriter.RewriteFailed, err
			}
			// Source adhoc_rewriter_rpp keeps B+C in an expr_ref until
			// the comparison owns it, including failed construction.
			if err = c.m.IncRef(b); err != nil {
				return rewriter.RewriteFailed, err
			}
			right := b
			defer func() { resultErr = errors.Join(resultErr, c.m.DecRef(right)) }()
		}
		if err == nil {
			h, err = c.m.MkFamilyApp(c.arithmetic, op, nil, []ast.ASTHandle{a, b})
		}
	}
	if err == nil {
		err = c.m.SetRef(out, h)
	}
	return rewriter.RewriteDone, err
}

func rewriteExtendedPretty(m *ast.ASTManager, expression ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return result, err
	}
	traversal, err := rewriter.NewTraversal(m, &extendedPrettyConfig{m: m, arithmetic: family})
	if err != nil {
		if traversal != nil {
			err = errors.Join(err, traversal.Close())
		}
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, traversal.Close()) }()
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = traversal.Rewrite(expression, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}

func (p *PredicateTransformer) traceExtendedLemma(body ast.ASTHandle, level uint32) error {
	tags := p.manager.BorrowSourceTraceTags()
	err := tags.EmitShort("spacer.expand-add", func(out io.Writer) (resultErr error) {
		levelText := strconv.FormatUint(uint64(level), 10)
		if level == InfinityLevel {
			levelText = "oo"
		}
		_, _ = io.WriteString(out, "add-lemma: "+levelText+" "+p.name+" ")
		if err := p.writeExtendedPretty(out, body); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n\n")
		return err
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (p *PredicateTransformer) writeExtendedPretty(out io.Writer, body ast.ASTHandle) (resultErr error) {
	ref, err := rewriteExtendedPretty(p.manager, body)
	owner := &extendedPrettyResultOwner{manager: p.manager, value: ref}
	defer func() { resultErr = finishExtendedPrettyResult(owner, resultErr) }()
	if err != nil {
		return err
	}
	h, err := p.manager.RefGet(ref)
	if err != nil {
		return err
	}
	context, globals, err := p.manager.RewriterParameterServices()
	if err != nil {
		return err
	}
	if context == nil && p.owner != nil && p.owner.contexts[0] != nil {
		context = p.owner.contexts[0].config.Params
		globals = p.owner.contexts[0].config.Globals
	}
	profile := ast.SMT2PPExprProfile{DecimalPrecision: 10, BVLiterals: true, FlatAssoc: true}
	if context != nil && globals != nil {
		profile, err = ast.SMT2PPExprProfileFromParams(context, globals, nil)
		if err != nil {
			return err
		}
	}
	profile.MaxDepth, profile.MinAliasSize = math.MaxUint32, math.MaxUint32
	format, err := p.manager.SMT2PPExprTreeWithConfig(h, nil, ast.SMT2PPExprConfig{Profile: &profile, Globals: globals})
	if err != nil {
		return err
	}
	fm, err := p.manager.GetFormatManager()
	if err != nil {
		return err
	}
	document, err := fm.NewRef(format)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, fm.DelRef(&document)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: p.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, stream.Close()) }()
	if context != nil {
		empty := context.EmptyValue()
		err = p.manager.PrintFormat(stream, format, context, &empty, globals)
	} else {
		err = p.manager.PrintFormat(stream, format, nil, nil, nil)
	}
	if err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}

func (p *PredicateTransformer) traceExtendedNode(node *ProofObligation, minimumDepth uint32) error {
	tags := p.manager.BorrowSourceTraceTags()
	err := tags.EmitShort("spacer.expand-add", func(out io.Writer) error {
		body, err := p.manager.RefGet(node.post)
		if err != nil {
			return err
		}
		// The source streams n.level() as an unsigned value, unlike pp_level
		// in add_lemma_core, and subtracts the queue's minimum depth unsigned.
		_, _ = io.WriteString(out, "expand-node: "+p.name+" level: "+strconv.FormatUint(uint64(node.level), 10)+" depth: "+strconv.FormatUint(uint64(node.depth-minimumDepth), 10)+"\n")
		if err := p.writeExtendedPretty(out, body); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\n\n")
		return err
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

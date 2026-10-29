package ast

// Source: src/ast/ast_smt2_pp.cpp:270-291.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
	"strconv"
)

// Exact borrowed child frame tuples; completed returns never consult a reused
// primary owner. These fields occupy the actual enclosing lexical cold header.

type astFloatBVChild struct {
	numeral     astBVNormalizationReceipt
	literal     *astBVLiteralFrame
	invocation  uint64
	retired     bool
	sourceCause error
}

func (c *astFloatBVChild) sourceReady(p *astBitVectorPluginOwner) error {
	if e := c.numeral.sourceReady(p); e != nil {
		return e
	}
	if c.literal == nil || c.retired {
		return c.sourceCause
	}
	f := c.literal
	if f.self != f || f.owner != p || f.invocation != c.invocation || f.retired {
		return ErrASTHandle
	}
	return c.sourceCause
}
func (c *astFloatBVChild) cleanup(p *astBitVectorPluginOwner, abort bool) error {
	if e := c.numeral.cleanup(p, abort); e != nil {
		return e
	}
	if c.literal == nil || c.retired {
		return nil
	}
	f := c.literal
	if f.self != f || f.owner != p || f.invocation != c.invocation || f.retired {
		return ErrASTHandle
	}
	return f.destroyPhase(true, abort)
}
func (f *astFloatLiteralFrame) destroyBVParts(own bool) error {
	m := f.owner.owner
	p := m.bitVectorPlugin
	if p == nil {
		return nil
	}
	for i := len(f.bvChildren) - 1; i >= 0; i-- {
		var e error
		if own {
			e = f.bvChildren[i].cleanup(p, false)
		} else {
			e = f.bvChildren[i].sourceReady(p)
		}
		if e != nil {
			return e
		}
	}
	for i := len(f.bvInvocations) - 1; i >= 0; i-- {
		var e error
		if own {
			e = f.bvInvocations[i].Cleanup(false)
		} else {
			e = f.bvInvocations[i].SourceReady()
		}
		if e != nil {
			return e
		}
	}
	if own {
		if e := f.owner.numeric.CleanupValueCopy(&f.sigCopy, false); e != nil {
			return e
		}
	}
	// Full-expression rational temporaries precede their enclosing app_ref DT.
	for i := 1; i >= 0; i-- {
		if f.bvPlaced[i] && !f.bvConsumed[i] && (own || f.bvComplete[i]) {
			var e error
			if own {
				e = m.numerals.CleanupValue(&f.bvValues[i])
			} else {
				e = m.numerals.Del(&f.bvValues[i])
			}
			if e != nil {
				return e
			}
			f.bvConsumed[i] = true
		}
		if f.bvRefAcquired[i] && !f.bvRefConsumed[i] {
			consumed, e := m.decNode(f.bvRefs[i])
			if consumed {
				f.bvRefConsumed[i] = true
			}
			if e != nil {
				return e
			}
		}
		// e_sig is younger than sig, which is younger than e_biased_exp.
		if i == 1 && f.sigConstructed && !f.sigConsumed {
			var e error
			if own {
				e = f.owner.numeric.CleanupScopedInteger(&f.sig)
			} else {
				e = f.owner.numeric.DeleteScopedInteger(&f.sig)
			}
			if e != nil {
				return e
			}
			f.sigConsumed = true
		}
	}
	// Unjoined children remain named until every older completed lexical value
	// has been consumed; the final abort phase runs after buffer/v in destroy.
	return nil
}
func (f *astFloatLiteralFrame) abortBVParts() error {
	m := f.owner.owner
	for i := len(f.bvChildren) - 1; i >= 0; i-- {
		if e := f.bvChildren[i].cleanup(m.bitVectorPlugin, true); e != nil {
			return e
		}
	}
	for i := len(f.bvInvocations) - 1; i >= 0; i-- {
		if e := f.bvInvocations[i].Cleanup(true); e != nil {
			return e
		}
		if e := f.bvInvocations[i].EndContext(); e != nil {
			return e
		}
	}
	return f.owner.numeric.CleanupValueCopy(&f.sigCopy, true)
}
func (m *ASTManager) floatBitVectorSuffix(f *astFloatLiteralFrame, radix bool, g *uparams.GlobalParamsService) (ASTHandle, error) {
	fm := m.Format
	if fm == nil {
		return ASTHandle{}, ErrASTDomain
	}
	b := &f.buffer
	prefix := "(fp (_ bv"
	if radix {
		prefix = "(fp #b"
	}
	if _, e := b.WriteString(prefix); e != nil {
		return ASTHandle{}, e
	}
	sign := uint32(0)
	if f.value.Sign() {
		sign = 1
	}
	if _, e := b.WriteString(strconv.FormatInt(int64(int32(sign)), 10)); e != nil {
		return ASTHandle{}, e
	}
	if !radix {
		if _, e := b.WriteString(" 1)"); e != nil {
			return ASTHandle{}, e
		}
	}
	body, e := fm.FormatBuffer(b)
	if e != nil {
		return ASTHandle{}, e
	}
	appendPart := func(part ASTHandle) error {
		var err error
		body, err = fm.FormatCompose([]ASTHandle{body, part})
		return err
	}
	appendText := func(s string) error {
		part, err := fm.FormatString(s)
		if err != nil {
			return err
		}
		return appendPart(part)
	}
	if e = appendText(" "); e != nil {
		return ASTHandle{}, e
	}
	biasedExponent, e := f.owner.numeric.BiasExp(f.value.EBits(), f.value.Exponent())
	if e != nil {
		return ASTHandle{}, e
	}
	f.bvValues[0], e = m.numerals.NewValue()
	if e != nil {
		return ASTHandle{}, e
	}
	f.bvPlaced[0] = true
	if e = m.numerals.SetInt64WithInvocation(&f.bvValues[0], biasedExponent, &f.bvInvocations[0]); e != nil {
		return ASTHandle{}, e
	}
	f.bvComplete[0] = true
	node, e := m.mkBitVectorNumeralCaptured(&f.bvValues[0], f.value.EBits(), &f.bvChildren[0].numeral)
	if e != nil {
		return ASTHandle{}, e
	}
	if e = m.incNode(node); e != nil {
		return ASTHandle{}, e
	}
	f.bvRefs[0] = node
	f.bvRefAcquired[0] = true
	if e = m.numerals.Del(&f.bvValues[0]); e != nil {
		return ASTHandle{}, e
	}
	f.bvConsumed[0] = true
	printBV := func(i int, node astNodeRef) error {
		c := &f.bvChildren[i]
		part, err := m.ppBitVectorLiteralCore(m.external(node), false, radix, g, &c.literal)
		if c.literal != nil {
			c.invocation = c.literal.invocation
			c.retired = c.literal.retired
			c.sourceCause = c.literal.sourceCause
		}
		if err != nil {
			return err
		}
		return appendPart(part)
	}
	if e = printBV(1, node); e != nil {
		return ASTHandle{}, e
	}
	if e = appendText(" "); e != nil {
		return ASTHandle{}, e
	}
	f.sigConstructed = true
	if e = f.owner.numeric.CopySignificand(&f.sig, &f.value, &f.sigCopy); e != nil {
		return ASTHandle{}, e
	}
	f.bvValues[1], e = m.numerals.NewValue()
	if e != nil {
		return ASTHandle{}, e
	}
	f.bvPlaced[1] = true
	owner, e := f.owner.numeric.BorrowScopedInteger(&f.sig)
	if e != nil {
		return ASTHandle{}, e
	}
	if e = m.numerals.SetIntegerFromOwnerWithInvocation(&f.bvValues[1], owner, &f.sig, &f.bvInvocations[1]); e != nil {
		return ASTHandle{}, e
	}
	f.bvComplete[1] = true
	node, e = m.mkBitVectorNumeralCaptured(&f.bvValues[1], f.value.SBits()-1, &f.bvChildren[2].numeral)
	if e != nil {
		return ASTHandle{}, e
	}
	if e = m.incNode(node); e != nil {
		return ASTHandle{}, e
	}
	f.bvRefs[1] = node
	f.bvRefAcquired[1] = true
	if e = m.numerals.Del(&f.bvValues[1]); e != nil {
		return ASTHandle{}, e
	}
	f.bvConsumed[1] = true
	if e = printBV(3, node); e != nil {
		return ASTHandle{}, e
	}
	if e = appendText(")"); e != nil {
		return ASTHandle{}, e
	}
	return body, nil
}

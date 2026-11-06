package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:49-71,279-295.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type bv2RealTerm struct {
	first, second ast.ASTHandle
	divisor, root *unumeral.OwnedRational
}

// The arith_util numeral copy crosses an AST-manager call. Its concrete
// invocation owns any failed lower-operation roots until the local k closes.
type bv2RealArithmeticRead struct {
	manager       *ast.ASTManager
	value         unumeral.OwnedRational
	invocation    unumeral.RationalServiceInvocation
	ready, closed bool
}

func (r *bv2RealArithmeticRead) retire() error {
	if err := r.invocation.SourceReady(); err != nil {
		return err
	}
	if err := r.invocation.Cleanup(false); err != nil {
		return err
	}
	if err := r.invocation.Cleanup(true); err != nil {
		return err
	}
	if err := r.invocation.EndContext(); err != nil {
		return err
	}
	r.invocation = unumeral.RationalServiceInvocation{}
	return nil
}
func (r *bv2RealArithmeticRead) close() error {
	if r.closed {
		return nil
	}
	if err := r.invocation.Cleanup(false); err != nil {
		return err
	}
	if r.ready {
		if err := r.manager.NumeralService().CleanupValue(&r.value); err != nil {
			return err
		}
		r.ready = false
	}
	if err := r.invocation.Cleanup(true); err != nil {
		return err
	}
	if err := r.invocation.EndContext(); err != nil {
		return err
	}
	r.closed = true
	return nil
}

func (c *bv2RealCall) matchTerm(h ast.ASTHandle) (term bv2RealTerm, found bool) {
	if c.err != nil {
		return
	}
	term.divisor, term.root = c.number(), c.number()
	if c.err != nil {
		return
	}
	local := len(c.numbers)
	var read *bv2RealArithmeticRead
	defer func() {
		c.err = errors.Join(c.err, c.releaseNumbersTo(local))
		if read != nil {
			if err := read.close(); err != nil {
				c.readPending = append(c.readPending, read)
				c.err = errors.Join(c.err, err)
			}
		}
	}()
	m := c.u.manager
	s := m.NumeralService()
	kind, err := m.GetKind(h)
	if err != nil {
		c.err = err
		return
	}
	if kind == ast.ASTApp {
		decl, err := m.GetDecl(h)
		if err != nil {
			c.err = err
			return term, false
		}
		if sig, ok := c.u.reverse[decl]; ok {
			// obj_map::find copies a local bvr_sig before arity/output assignment.
			d, r := c.number(), c.number()
			if c.err != nil {
				return term, false
			}
			if c.err = s.Copy(d, sig.divisor); c.err != nil {
				return term, false
			}
			if c.err = s.Copy(r, sig.root); c.err != nil {
				return term, false
			}
			count, err := m.GetNumArgs(h)
			if err != nil {
				c.err = err
				return term, false
			}
			if c.err = bv2RealSourceAssert(55, "num_args == 2", count == 2); c.err != nil {
				return term, false
			}
			if count != 2 {
				c.err = ast.ErrASTDomain
				return term, false
			}
			a, err := m.GetArg(h, 0)
			if err != nil {
				c.err = err
				return term, false
			}
			b, err := m.GetArg(h, 1)
			if err != nil {
				c.err = err
				return term, false
			}
			if c.err = s.Copy(term.divisor, d); c.err != nil {
				return term, false
			}
			if c.err = s.Copy(term.root, r); c.err != nil {
				return term, false
			}
			if c.err = c.checkOwnedSignature(a, b, &sig, d, r); c.err != nil {
				return term, false
			}
			term.first = c.hold(a, nil)
			term.second = c.hold(b, nil)
			return term, c.err == nil
		}
	}
	read = &bv2RealArithmeticRead{manager: m}
	read.value, c.err = s.NewValue()
	read.ready = c.err == nil
	if c.err != nil {
		return term, false
	}
	integer := false
	yes, err := m.ArithmeticNumeralValueWithInvocation(c.u.arith, h, &read.value, &integer, &read.invocation)
	if err == nil {
		err = read.retire()
	}
	if err != nil {
		c.err = err
		return term, false
	}
	if !yes || integer {
		return term, false
	}
	denominator := c.takeRational(s.Denominator(&read.value))
	if c.err != nil {
		return term, false
	}
	if c.err = s.Copy(term.divisor, denominator); c.err != nil {
		return term, false
	}
	if c.err = s.Copy(term.root, &c.u.defaults[0]); c.err != nil {
		return term, false
	}
	numerator := c.takeRational(s.Numerator(&read.value))
	if c.err != nil {
		return term, false
	}
	term.first = c.signedNumeralOwned(numerator)
	if c.err != nil {
		return term, false
	}
	zero := c.number()
	if c.err == nil {
		c.err = s.SetInt64(zero, 0)
	}
	if c.err != nil {
		return term, false
	}
	term.second = c.signedNumeralOwned(zero)
	return term, c.err == nil
}

func (c *bv2RealCall) checkOwnedSignature(a, b ast.ASTHandle, sig *bv2RealSignature, d, r *unumeral.OwnedRational) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	s := c.u.manager.NumeralService()
	for i, v := range []*unumeral.OwnedRational{d, r} {
		integer, err := s.IsInt(v)
		if err != nil {
			return err
		}
		positive := false
		if integer {
			positive, err = s.IsPos(v)
			if err != nil {
				return err
			}
		}
		text := "sig.m_d.is_int() && sig.m_d.is_pos()"
		if i == 1 {
			text = "sig.m_r.is_int() && sig.m_r.is_pos()"
		}
		if err = bv2RealSourceAssert(int32(60+i), text, integer && positive); err != nil {
			return err
		}
	}
	return c.checkSignatureWidths(a, b, sig)
}

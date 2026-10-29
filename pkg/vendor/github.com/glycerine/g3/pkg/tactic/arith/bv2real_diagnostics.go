package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp assertions and TRACE sites.

import (
	"errors"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func bv2RealSourceAssert(line int32, condition string, ok bool) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled || ok {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/tactic/arith/bv2real_rewriter.cpp", line, condition)
}

func (c *bv2RealCall) assertBV(h ast.ASTHandle, line int32, condition string) {
	if c.err != nil {
		return
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		c.err = err
		return
	}
	sort, err := c.u.manager.GetSort(h)
	if err != nil {
		c.err = err
		return
	}
	ok, err := c.u.manager.IsBitVectorSort(sort)
	if err != nil {
		c.err = err
		return
	}
	if !ok {
		c.err = diagnostic.DebugAssertionFailure(d, "src/tactic/arith/bv2real_rewriter.cpp", line, condition)
	}
}

func (c *bv2RealCall) checkSignature(a, b ast.ASTHandle, sig *bv2RealSignature) error {
	if err := bv2RealSourceAssert(60, "sig.m_d.is_int() && sig.m_d.is_pos()", sig.divisor.IsInt() && sig.divisor.Sign() > 0); err != nil {
		return err
	}
	if err := bv2RealSourceAssert(61, "sig.m_r.is_int() && sig.m_r.is_pos()", sig.root.IsInt() && sig.root.Sign() > 0); err != nil {
		return err
	}
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	width, err := c.u.manager.BitVectorSize(a)
	if err != nil {
		return err
	}
	if err = bv2RealSourceAssert(62, "m_bv.get_bv_size(m) == sig.m_msz", width == sig.first); err != nil {
		return err
	}
	width, err = c.u.manager.BitVectorSize(b)
	if err != nil {
		return err
	}
	return bv2RealSourceAssert(63, "m_bv.get_bv_size(n) == sig.m_nsz", width == sig.second)
}

func (c *bv2RealCall) assertArity(family ast.FamilyID, kind int32, n int) error {
	line, arity, text := int32(0), 2, "num_args == 2"
	if family == c.u.arith {
		switch kind {
		case 2:
			line = 368
		case 3:
			line = 369
		case 4:
			line = 370
		case 5:
			line = 371
		case 10:
			line = 375
		case 8:
			line, arity, text = 379, 1, "num_args == 1"
		case 6:
			return bv2RealSourceAssert(603, "num_args > 0", n > 0)
		case 9:
			return bv2RealSourceAssert(625, "num_args > 0", n > 0)
		case 7:
			return bv2RealSourceAssert(657, "num_args > 0", n > 0)
		}
	} else if family == c.u.manager.BasicFamilyID() {
		switch kind {
		case ast.ASTOpEq:
			line = 388
		case ast.ASTOpIte:
			line, arity, text = 389, 3, "num_args == 3"
		}
	}
	if line == 0 {
		return nil
	}
	return bv2RealSourceAssert(line, text, n == arity)
}

func (u *BV2RealUtil) BindTrace(tags *diagnostic.TraceTags) error {
	if err := u.open(); err != nil {
		return err
	}
	u.trace = tags
	return nil
}

func (u *BV2RealUtil) traceTags() *diagnostic.TraceTags {
	if u.trace != nil {
		return u.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// A stopped formatter remains attached to its utility until Close can retire
// its actual owners. A failed ostream is sticky but does not stop evaluation
// of the later source printing operands.
type bv2RealTrace struct {
	u             *BV2RealUtil
	out           io.Writer
	inputs        []ast.ASTRef
	formatManager *ast.ASTManager
	format        ast.ASTRef
	stream        *utext.NumericTextStream
	parameters    *uparams.ModuleParamsOwner
}

func (w *bv2RealTrace) text(s string) { _, _ = io.WriteString(w.out, s) }

func (w *bv2RealTrace) writeAST(h ast.ASTHandle) error {
	m := w.u.manager
	input, err := m.NewRef(h)
	if err != nil {
		return err
	}
	w.inputs = append(w.inputs, input)
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil || g.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	if err = g.CheckOwner(g.Context, m.Budget()); err != nil {
		return err
	}
	if err = g.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return err
	}
	empty := g.Context.EmptyValue()
	p := &uparams.ModuleParamsOwner{Context: g.Context, Globals: g, Budget: m.Budget()}
	p.Self = p
	w.parameters = p
	p.Running = true
	frame, err := p.BeginHelper(&empty, "pp")
	if err == nil {
		_, err = p.FrameBool(frame, "decimal", false)
	}
	if err == nil {
		_, err = p.FrameUInt(frame, "decimal_precision", 10)
	}
	if err == nil {
		_, err = p.FrameBool(frame, "bv_literals", true)
	}
	if err == nil {
		_, err = p.FrameBool(frame, "fp_real_literals", false)
	}
	if err == nil {
		_, err = p.FrameBool(frame, "bv_neg", false)
	}
	if err == nil {
		_, err = p.FrameUInt(frame, "max_depth", 5)
	}
	if err == nil {
		_, err = p.FrameUInt(frame, "min_alias_size", 10)
	}
	if err == nil {
		_, err = p.FrameBool(frame, "flat_assoc", true)
	}
	if frame != nil {
		err = p.FinishHelper(frame, err)
	}
	p.Running = false
	if err != nil {
		return err
	}
	if err = p.CloseHelpers(); err != nil {
		return err
	}
	w.parameters = nil
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	var document ast.ASTHandle
	if kind == ast.ASTFuncDecl {
		document, err = m.SMT2PPDeclareFun(h)
	} else {
		document, err = m.SMT2PPExprTree(h, nil)
	}
	if err != nil {
		return err
	}
	w.formatManager, err = m.GetFormatManager()
	if err != nil {
		return err
	}
	w.format, err = w.formatManager.NewRef(document)
	if err != nil {
		return err
	}
	w.stream, err = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	if err = m.PrintFormat(w.stream, document, g.Context, &empty, g); err != nil {
		return err
	}
	text, err := w.stream.String()
	if err != nil {
		return err
	}
	w.text(text)
	return w.closeDocument()
}

func (w *bv2RealTrace) closeDocument() error {
	if w.parameters != nil {
		if err := w.parameters.CloseHelpers(); err != nil {
			return err
		}
		w.parameters = nil
	}
	if w.stream != nil {
		if err := w.stream.Close(); err != nil {
			return err
		}
		w.stream = nil
	}
	if w.format != (ast.ASTRef{}) {
		if err := w.formatManager.DelRef(&w.format); err != nil {
			return err
		}
	}
	return nil
}
func (w *bv2RealTrace) close() error {
	if err := w.closeDocument(); err != nil {
		return err
	}
	for i := len(w.inputs) - 1; i >= 0; i-- {
		if w.inputs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := w.u.manager.DelRef(&w.inputs[i]); err != nil {
			return err
		}
	}
	w.inputs = nil
	return nil
}

func (u *BV2RealUtil) traceApp(decl ast.ASTHandle, args []ast.ASTHandle) error {
	tags := u.traceTags()
	if !tags.ShouldTrace("bv2real_rewriter") {
		return nil
	}
	w := &bv2RealTrace{u: u}
	err := tags.EmitConditional("bv2real_rewriter", "mk_app_core", "src/tactic/arith/bv2real_rewriter.cpp", 359, func() bool { return true }, func(out io.Writer) error {
		w.out = out
		if err := w.writeAST(decl); err != nil {
			return err
		}
		w.text(" ")
		for _, arg := range args {
			if err := w.writeAST(arg); err != nil {
				return err
			}
			w.text(" ")
		}
		w.text("\n")
		return nil
	})
	if tags.IsStreamFailure(err) {
		err = nil
	}
	cleanup := w.close()
	if cleanup != nil {
		u.tracePending = append(u.tracePending, w)
	}
	return errors.Join(err, cleanup)
}

func (u *BV2RealUtil) traceDivisible(h ast.ASTHandle, overflow *big.Int, line int) error {
	tags := u.traceTags()
	if !tags.ShouldTrace("bv2real_rewriter") {
		return nil
	}
	w := &bv2RealTrace{u: u}
	err := tags.EmitConditional("bv2real_rewriter", "mk_is_divisible_by", "src/tactic/arith/bv2real_rewriter.cpp", line, func() bool { return true }, func(out io.Writer) error {
		w.out = out
		if err := w.writeAST(h); err != nil {
			return err
		}
		w.text(" " + overflow.String() + "\n")
		return nil
	})
	if tags.IsStreamFailure(err) {
		err = nil
	}
	cleanup := w.close()
	if cleanup != nil {
		u.tracePending = append(u.tracePending, w)
	}
	return errors.Join(err, cleanup)
}

func (u *BV2RealUtil) traceLE(line int, text string) error {
	tags := u.traceTags()
	err := tags.EmitConditional("bv2real_rewriter", "mk_le", "src/tactic/arith/bv2real_rewriter.cpp", line, func() bool { return true }, func(out io.Writer) error {
		_, err := io.WriteString(out, text)
		return err
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

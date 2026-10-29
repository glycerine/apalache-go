package protomodel

// Source: datatype_factory.cpp TRACE/CTRACE and ast_smt2_pp.cpp mk_ismt2_pp.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
	"io"
)

type datatypeFactoryTraceWriter struct {
	manager       *ast.ASTManager
	out           io.Writer
	formatManager *ast.ASTManager
	format        ast.ASTRef
	stream        *utext.NumericTextStream
	parameters    *uparams.ModuleParamsOwner
	profile       ast.SMT2PPExprProfile
}

func (f *DatatypeFactory) trace(function string, line int, condition func() bool, body func(*datatypeFactoryTraceWriter) error) error {
	tags := f.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace("datatype") {
		return nil
	}
	if condition == nil {
		condition = func() bool { return true }
	}
	w := &datatypeFactoryTraceWriter{manager: f.manager}
	err := tags.EmitConditional("datatype", function, "src/smt/proto_model/datatype_factory.cpp", line, condition, func(out io.Writer) error { w.out = out; return body(w) })
	if tags.IsStreamFailure(err) {
		err = nil
	}
	closeErr := w.close()
	if closeErr != nil {
		f.pending = append(f.pending, datatypeFactoryPending{trace: w})
	}
	return errors.Join(err, closeErr)
}
func (w *datatypeFactoryTraceWriter) text(s string) { _, _ = io.WriteString(w.out, s) }
func (w *datatypeFactoryTraceWriter) bit(b bool) {
	if b {
		w.text("1")
	} else {
		w.text("0")
	}
}
func (w *datatypeFactoryTraceWriter) name(h ast.ASTHandle) error {
	symbol, err := w.manager.GetASTName(h)
	if err != nil {
		return err
	}
	text, err := w.manager.Symbols.Display(symbol)
	if err != nil {
		return err
	}
	w.text(text)
	return nil
}

// mk_pp borrows its input. Only the constructed format_ref owns a document.
func (w *datatypeFactoryTraceWriter) ast(h ast.ASTHandle) error {
	if h.IsNull() {
		w.text("null")
		return nil
	}
	m := w.manager
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil || g.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	if err := g.CheckOwner(g.Context, m.Budget()); err != nil {
		return err
	}
	if err := g.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return err
	}
	empty := g.Context.EmptyValue()
	if err := w.constructPrinterParameters(g, &empty); err != nil {
		return err
	}
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	var document ast.ASTHandle
	switch kind {
	case ast.ASTSort:
		document, err = m.SMT2PPSort(h)
	case ast.ASTFuncDecl:
		document, err = m.SMT2PPDeclareFun(h)
	default:
		document, err = m.SMT2PPExprTreeWithConfig(h, nil, ast.SMT2PPExprConfig{Profile: &w.profile, Globals: g})
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
	return w.close()
}
func (w *datatypeFactoryTraceWriter) close() error {
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
func (w *datatypeFactoryTraceWriter) constructPrinterParameters(g *uparams.GlobalParamsService, empty *uparams.ParamsRef) error {
	p := &uparams.ModuleParamsOwner{Context: g.Context, Globals: g, Budget: w.manager.Budget()}
	p.Self = p
	w.parameters = p
	p.Running = true
	h, err := p.BeginHelper(empty, "pp")
	if err == nil {
		w.profile.Decimal, err = p.FrameBool(h, "decimal", false)
	}
	if err == nil {
		w.profile.DecimalPrecision, err = p.FrameUInt(h, "decimal_precision", 10)
	}
	if err == nil {
		w.profile.BVLiterals, err = p.FrameBool(h, "bv_literals", true)
	}
	if err == nil {
		w.profile.FPRealLiterals, err = p.FrameBool(h, "fp_real_literals", false)
	}
	if err == nil {
		w.profile.BVNegative, err = p.FrameBool(h, "bv_neg", false)
	}
	if err == nil {
		w.profile.MaxDepth, err = p.FrameUInt(h, "max_depth", 5)
	}
	if err == nil {
		w.profile.MinAliasSize, err = p.FrameUInt(h, "min_alias_size", 10)
	}
	if err == nil {
		w.profile.FlatAssoc, err = p.FrameBool(h, "flat_assoc", true)
	}
	if h != nil {
		err = p.FinishHelper(h, err)
	}
	p.Running = false
	if err != nil {
		return err
	}
	if err = p.CloseHelpers(); err != nil {
		return err
	}
	w.parameters = nil
	return nil
}

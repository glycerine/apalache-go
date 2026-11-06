package ast

// Source: src/ast/datatype_decl_plugin.cpp:914,937-980;
// src/ast/ast_smt2_pp.cpp:1211-1261; src/util/trace.h:40-46.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Concrete sort_ref/func_decl_ref full-expression owners. A document belongs
// to the format manager; the input temporaries belong to the AST manager.
type datatypeTraceWriter struct {
	manager       *ASTManager
	out           io.Writer
	inputs        []ASTRef
	formatManager *ASTManager
	format        ASTRef
	stream        *utext.NumericTextStream
	parameters    *uparams.ModuleParamsOwner
}

// Callers hold the AST lock. The existing public SMT2 printer acquires that
// lock for its queries, so release it for the trace and restore it on return.
// Disabled/erased traces never reach formatting, operands or temporary owners.
func (u *DatatypeUtil) traceDatatype(line int, body func(*datatypeTraceWriter) error) error {
	m := u.manager
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("util_bug") {
		return nil
	}
	m.mu.Unlock()
	defer m.mu.Lock()
	w := &datatypeTraceWriter{manager: m}
	function := "get_non_rec_constructor_core"
	if line == 914 {
		function = "get_non_rec_constructor"
	}
	err := tags.EmitConditional("util_bug", function, "src/ast/datatype_decl_plugin.cpp", line, func() bool { return true }, func(out io.Writer) error { w.out = out; return body(w) })
	if tags.IsStreamFailure(err) {
		err = nil
	}
	closeErr := w.close(false)
	if closeErr != nil {
		u.tracePending = append(u.tracePending, w)
	}
	return errors.Join(err, closeErr)
}

// An ostream refusal is sticky. Later reached operands must still evaluate.
func (w *datatypeTraceWriter) text(s string) { _, _ = io.WriteString(w.out, s) }

func (w *datatypeTraceWriter) ast(r astNodeRef) error {
	m := w.manager
	h := m.external(r)
	ref, err := m.NewRef(h)
	if err != nil {
		return err
	}
	w.inputs = append(w.inputs, ref)
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil || g.Context == nil {
		return uparams.ErrGlobalParamsOwner
	}
	if err = g.CheckOwner(g.Context, m.BudgetField); err != nil {
		return err
	}
	if err = g.Context.CheckServices(m.Symbols, m.numerals); err != nil {
		return err
	}
	empty := g.Context.EmptyValue()
	if err = w.constructPrinterParameters(g, &empty); err != nil {
		return err
	}
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	var document ASTHandle
	switch kind {
	case ASTSort:
		document, err = m.SMT2PPSort(h)
	case ASTFuncDecl:
		document, err = m.SMT2PPDeclareFun(h)
	default:
		return ErrASTDomain
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
	w.stream, err = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
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
	return w.closeDocument(false)
}

func (w *datatypeTraceWriter) end(s string) error {
	w.text(s)
	return w.close(false)
}

func (w *datatypeTraceWriter) closeDocument(parentLocked bool) error {
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
	if w.format != (ASTRef{}) {
		var err error
		if parentLocked && w.formatManager == w.manager {
			err = w.manager.delRef(&w.format)
		} else {
			err = w.formatManager.DelRef(&w.format)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// smt2_printer's constructor-local pp_params (1063-1082) is evaluated even
// for sort/declaration documents and even after the ostream has failed.
// Its eight scalar fields do not alter these two document kinds, but their
// lookups and module-reference destruction retain source ordering/lifetime.
func (w *datatypeTraceWriter) constructPrinterParameters(g *uparams.GlobalParamsService, empty *uparams.ParamsRef) error {
	p := &uparams.ModuleParamsOwner{Context: g.Context, Globals: g, Budget: w.manager.BudgetField}
	p.Self = p
	w.parameters = p
	p.Running = true
	h, err := p.BeginHelper(empty, "pp")
	if err == nil {
		_, err = p.FrameBool(h, "decimal", false)
	}
	if err == nil {
		_, err = p.FrameUInt(h, "decimal_precision", 10)
	}
	if err == nil {
		_, err = p.FrameBool(h, "bv_literals", true)
	}
	if err == nil {
		_, err = p.FrameBool(h, "fp_real_literals", false)
	}
	if err == nil {
		_, err = p.FrameBool(h, "bv_neg", false)
	}
	if err == nil {
		_, err = p.FrameUInt(h, "max_depth", 5)
	}
	if err == nil {
		_, err = p.FrameUInt(h, "min_alias_size", 10)
	}
	if err == nil {
		_, err = p.FrameBool(h, "flat_assoc", true)
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

func (w *datatypeTraceWriter) close(parentLocked bool) error {
	if err := w.closeDocument(parentLocked); err != nil {
		return err
	}
	for i := len(w.inputs) - 1; i >= 0; i-- {
		if w.inputs[i] == (ASTRef{}) {
			continue
		}
		var err error
		if parentLocked {
			err = w.manager.delRef(&w.inputs[i])
		} else {
			err = w.manager.DelRef(&w.inputs[i])
		}
		if err != nil {
			return err
		}
	}
	w.inputs = nil
	return nil
}

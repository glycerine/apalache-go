package rewriter

// Source: enum2bv_rewriter.cpp:84-88 throw_non_fd; ast_smt2_pp.cpp mk_pp.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
	"strings"
)

// This concrete mk_pp consumer borrows the input expression and owns only
// its parameter helpers, format_ref document and Go rendering stream.
// Native Go strings replace the source stringstream/string storage.
type enum2BVErrorPrinter struct {
	manager       *ast.ASTManager
	parameters    *uparams.ModuleParamsOwner
	profile       ast.SMT2PPExprProfile
	formatManager *ast.ASTManager
	format        ast.ASTRef
	stream        *utext.NumericTextStream
}

func (w *enum2BVErrorPrinter) render(expression ast.ASTHandle) (string, error) {
	m := w.manager
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil || g.Context == nil {
		return "", uparams.ErrGlobalParamsOwner
	}
	if err := g.CheckOwner(g.Context, m.Budget()); err != nil {
		return "", err
	}
	if err := g.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return "", err
	}
	empty := g.Context.EmptyValue()
	if err := w.constructParameters(g, &empty); err != nil {
		return "", err
	}
	document, err := m.SMT2PPExprTreeWithConfig(expression, nil, ast.SMT2PPExprConfig{Profile: &w.profile, Globals: g})
	if err != nil {
		return "", err
	}
	w.formatManager, err = m.GetFormatManager()
	if err != nil {
		return "", err
	}
	w.format, err = w.formatManager.NewRef(document)
	if err != nil {
		return "", err
	}
	w.stream, err = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return "", err
	}
	if err = m.PrintFormat(w.stream, document, g.Context, &empty, g); err != nil {
		return "", err
	}
	return w.stream.String()
}
func (w *enum2BVErrorPrinter) close() error {
	if w.parameters != nil {
		if err := w.parameters.CloseHelpers(); err != nil {
			return err
		}
		w.parameters = nil
	}
	if w.format != (ast.ASTRef{}) {
		if err := w.formatManager.DelRef(&w.format); err != nil {
			return err
		}
	}
	if w.stream != nil {
		if err := w.stream.Close(); err != nil {
			return err
		}
		w.stream = nil
	}
	return nil
}
func (r *Enum2BVRewriter) finishErrorPrinter(w *enum2BVErrorPrinter, result *error) {
	err := w.close()
	if err != nil {
		r.pending = append(r.pending, enum2BVPending{printer: w})
		r.closing = true
	}
	*result = errors.Join(*result, err)
}
func (r *Enum2BVRewriter) throwNonFD(expression ast.ASTHandle) (result error) {
	var message strings.Builder
	message.WriteString("unable to handle nested data-type expression ")
	printer := &enum2BVErrorPrinter{manager: r.manager}
	handled := false
	defer func() {
		if !handled {
			r.finishErrorPrinter(printer, &result)
		}
	}()
	text, err := printer.render(expression)
	if err != nil {
		return err
	}
	message.WriteString(text)
	// Source mk_pp's format_ref retires before strm.str()/exception creation.
	var closeErr error
	r.finishErrorPrinter(printer, &closeErr)
	handled = true
	if closeErr != nil {
		return closeErr
	}
	return newRewriterException(message.String())
}

func (w *enum2BVErrorPrinter) constructParameters(g *uparams.GlobalParamsService, empty *uparams.ParamsRef) error {
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

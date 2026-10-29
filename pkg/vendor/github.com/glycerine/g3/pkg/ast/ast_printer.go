package ast

// Source: src/ast/ast_printer.cpp:20-52; src/ast/ast_printer.h:24-58;
// src/ast/ast.cpp:115-137 parameter/declaration head display.

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"

	usmt2 "github.com/glycerine/g3/pkg/util/smt2"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// DefaultASTPrinter retains ast_printer's default #id displays. The source
// pretty-print methods are unreachable in this base type and return a domain
// error in the Go interface.
type DefaultASTPrinter struct{ manager *ASTManager }

// declarationHeadWriter is the concrete borrowed ostream used by parameter
// numerals in display_parameters. The rational service owns conversion scratch.
type declarationHeadWriter struct{ out io.Writer }

func (w declarationHeadWriter) AppendCString(s string) error {
	n, err := io.WriteString(w.out, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}
func (w declarationHeadWriter) AppendInt32(n int32) error {
	return w.AppendCString(strconv.FormatInt(int64(n), 10))
}
func (w declarationHeadWriter) AppendNumeralText(s string) error { return w.AppendCString(s) }

// DisplayDeclarationHead writes a declaration's name and its parameters, as
// consumed by smt::theory::display_app/display_flat_app (ast.cpp:115-137).
// Private parameters are included here; this is not the low-level AST printer.
func (m *ASTManager) DisplayDeclarationHead(out io.Writer, decl ASTHandle) error {
	if err := m.lock(); err != nil {
		return err
	}
	defer m.mu.Unlock()
	if out == nil {
		return ErrASTHandle
	}
	r, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return err
	}
	d := m.declAt(r.slot)
	if err = m.Symbols.WriteDisplay(out, d.name); err != nil {
		return err
	}
	if d.info.slot == 0 {
		return nil
	}
	info, err := m.info(d.info.slot, d.info.generation)
	if err != nil || info.count == 0 {
		return err
	}
	w := declarationHeadWriter{out}
	if err = w.AppendCString("["); err != nil {
		return err
	}
	for i := uint32(0); i < info.count; i++ {
		p, err := m.infoParameter(info, i)
		if err != nil {
			return err
		}
		value, err := m.parameter(p)
		if err != nil {
			return err
		}
		switch value.Kind {
		case ASTParameterInt:
			err = w.AppendInt32(value.integer)
		case ASTParameterSymbol:
			err = m.Symbols.WriteDisplay(out, value.symbol)
		case ASTParameterRational:
			err = m.numerals.InsertInto(w, &value.Rational)
		case ASTParameterAST:
			if err = m.validNode(value.ast); err == nil {
				err = w.AppendCString("#" + strconv.FormatUint(uint64(m.nodeAt(value.ast.slot).id), 10))
			}
		case ASTParameterDouble:
			err = w.AppendCString(strconv.FormatFloat(value.double, 'g', 6, 64))
		case ASTParameterExternal:
			err = w.AppendCString("@" + strconv.FormatUint(uint64(value.external), 10))
		default:
			err = ErrASTDomain
		}
		if err != nil {
			return err
		}
		if i+1 < info.count {
			if err = w.AppendCString(":"); err != nil {
				return err
			}
		}
	}
	return w.AppendCString("]")
}

func NewDefaultASTPrinter(m *ASTManager) (*DefaultASTPrinter, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	return &DefaultASTPrinter{manager: m}, nil
}

func (p *DefaultASTPrinter) displayID(out io.Writer, h ASTHandle) error {
	if p == nil || p.manager == nil || out == nil {
		return ErrASTHandle
	}
	id, err := p.manager.GetASTID(h)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, "#"+strconv.FormatUint(uint64(id), 10)+"\n")
	return err
}

func (p *DefaultASTPrinter) DisplaySort(out io.Writer, sort ASTHandle, _ uint32) error {
	return p.displayID(out, sort)
}
func (p *DefaultASTPrinter) DisplayExpr(out io.Writer, expr ASTHandle, _ uint32) error {
	return p.displayID(out, expr)
}
func (p *DefaultASTPrinter) DisplayFuncDecl(out io.Writer, decl ASTHandle, _ uint32) error {
	return p.displayID(out, decl)
}
func (p *DefaultASTPrinter) DisplayExprVariables(out io.Writer, expr ASTHandle, _ uint32, _ uint32, _ string, _ *[]usymbol.Symbol) error {
	return p.displayID(out, expr)
}
func (*DefaultASTPrinter) PrettySort(ASTHandle) (ASTHandle, error) {
	return ASTHandle{}, ErrASTDomain
}
func (*DefaultASTPrinter) PrettyFuncDecl(ASTHandle) (ASTHandle, error) {
	return ASTHandle{}, ErrASTDomain
}
func (*DefaultASTPrinter) PrettyExpr(ASTHandle) (ASTHandle, error) {
	return ASTHandle{}, ErrASTDomain
}

// SimpleASTPrinterContext is the concrete source context backed by the SMT2
// printer. Writers are borrowed; nil writers select stdout and stderr.
type SimpleASTPrinterContext struct {
	manager             *ASTManager
	regular, diagnostic io.Writer
}

func NewSimpleASTPrinterContext(m *ASTManager, regular, diagnostic io.Writer) (*SimpleASTPrinterContext, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	if regular == nil {
		regular = os.Stdout
	}
	if diagnostic == nil {
		diagnostic = os.Stderr
	}
	return &SimpleASTPrinterContext{manager: m, regular: regular, diagnostic: diagnostic}, nil
}

func (p *SimpleASTPrinterContext) Manager() *ASTManager {
	if p == nil {
		return nil
	}
	return p.manager
}
func (p *SimpleASTPrinterContext) RegularStream() io.Writer {
	if p == nil {
		return nil
	}
	return p.regular
}
func (p *SimpleASTPrinterContext) DiagnosticStream() io.Writer {
	if p == nil {
		return nil
	}
	return p.diagnostic
}
func (p *SimpleASTPrinterContext) PrettySort(sort ASTHandle) (ASTHandle, error) {
	if p == nil || p.manager == nil {
		return ASTHandle{}, ErrASTHandle
	}
	return p.manager.SMT2PPSort(sort)
}
func (p *SimpleASTPrinterContext) PrettyFuncDecl(decl ASTHandle) (ASTHandle, error) {
	if p == nil || p.manager == nil {
		return ASTHandle{}, ErrASTHandle
	}
	return p.manager.SMT2PPDeclareFun(decl)
}
func (p *SimpleASTPrinterContext) PrettyExpr(expr ASTHandle) (ASTHandle, error) {
	if p == nil || p.manager == nil {
		return ASTHandle{}, ErrASTHandle
	}
	return p.manager.SMT2PPExprTree(expr, nil)
}

func (p *SimpleASTPrinterContext) PrettyExprVariables(expr ASTHandle, numVars uint32, prefix string, names *[]usymbol.Symbol) (ASTHandle, error) {
	if p == nil || p.manager == nil || names == nil {
		return ASTHandle{}, ErrASTHandle
	}
	if prefix == "" {
		prefix = "x"
	}
	if prefix == "a" {
		prefix = "_a"
	}
	generated := make([]usymbol.Symbol, 0, numVars)
	for i := uint32(0); i < numVars; i++ {
		text := prefix + "!" + strconv.FormatUint(uint64(i), 10)
		s, err := p.manager.Symbols.Intern(text)
		if err != nil {
			return ASTHandle{}, err
		}
		quoted, err := usmt2.IsSMT2QuotedSymbol(p.manager.Symbols, s)
		if err != nil {
			return ASTHandle{}, err
		}
		if quoted {
			text, err = usmt2.MkSMT2QuotedSymbol(p.manager.Symbols, s, nil)
			if err != nil {
				return ASTHandle{}, err
			}
			s, err = p.manager.Symbols.Intern(text)
			if err != nil {
				return ASTHandle{}, err
			}
		}
		generated = append(generated, s)
	}
	*names = append(*names, generated...)
	// The source reverses its private lookup stack while leaving var_names in
	// creation order. SMT2PPExprTree expects innermost-to-outermost lookup order.
	lookup := append([]usymbol.Symbol(nil), generated...)
	for i, j := 0, len(lookup)-1; i < j; i, j = i+1, j-1 {
		lookup[i], lookup[j] = lookup[j], lookup[i]
	}
	return p.manager.SMT2PPExprTree(expr, lookup)
}

func (p *SimpleASTPrinterContext) displayFormat(out io.Writer, format ASTHandle, indent uint32) (result error) {
	if p == nil || p.manager == nil || out == nil {
		return ErrASTHandle
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: p.manager.BudgetField})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if err = p.manager.PrintFormat(stream, format, nil, nil, nil); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	if indent != 0 {
		text = strings.Repeat(" ", int(indent)) + strings.ReplaceAll(text, "\n", "\n"+strings.Repeat(" ", int(indent)))
	}
	_, err = io.WriteString(out, text)
	return err
}

func (p *SimpleASTPrinterContext) DisplaySort(out io.Writer, sort ASTHandle, indent uint32) error {
	format, err := p.PrettySort(sort)
	if err != nil {
		return err
	}
	return p.displayFormat(out, format, indent)
}
func (p *SimpleASTPrinterContext) DisplayExpr(out io.Writer, expr ASTHandle, indent uint32) error {
	format, err := p.PrettyExpr(expr)
	if err != nil {
		return err
	}
	return p.displayFormat(out, format, indent)
}
func (p *SimpleASTPrinterContext) DisplayFuncDecl(out io.Writer, decl ASTHandle, indent uint32) error {
	if p == nil || p.manager == nil || out == nil {
		return ErrASTHandle
	}
	name, err := p.manager.GetASTName(decl)
	if err != nil {
		return err
	}
	text, err := p.manager.Symbols.Display(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, strings.Repeat(" ", int(indent))+text)
	return err
}

// The corresponding concrete source overload is NOT_IMPLEMENTED_YET.
func (*SimpleASTPrinterContext) DisplayExprVariables(io.Writer, ASTHandle, uint32, uint32, string, *[]usymbol.Symbol) error {
	return z3exception.MustZ3Error(z3exception.ErrorNotImplementedYet)
}

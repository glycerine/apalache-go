package api

// Source: src/api/api_quant.cpp:24-69,165-194,285-486.

import (
	"bytes"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// QuantifierSpec is the Go ownership-neutral form of the native extended
// quantifier constructor. Sorts and Names describe de Bruijn binders in the
// same order as the source API.
type QuantifierSpec struct {
	Forall               bool
	Sorts                []ast.ASTHandle
	Names                []string
	Body                 ast.ASTHandle
	Weight               int32
	QID, SkolemID        string
	Patterns, NoPatterns []ast.ASTHandle
}

func (c *SMTContext) quantifierManager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.Manager()
}

func internQuantifierSymbol(m *ast.ASTManager, text string) (usymbol.Symbol, error) {
	if text == "" {
		return usymbol.Symbol{}, nil
	}
	return m.Symbols.Intern(text)
}

// Quantifier constructs a quantified expression. As in api_quant.cpp, a
// zero-binder request is the body itself.
func (c *SMTContext) Quantifier(spec QuantifierSpec) (ast.ASTHandle, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(spec.Sorts) != len(spec.Names) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if len(spec.Patterns) != 0 && len(spec.NoPatterns) != 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	bodySort, err := m.GetSort(spec.Body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if bodySort != boolSort {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	names := make([]usymbol.Symbol, len(spec.Names))
	for i, name := range spec.Names {
		names[i], err = m.Symbols.Intern(name)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	qid, err := internQuantifierSymbol(m, spec.QID)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	skid, err := internQuantifierSymbol(m, spec.SkolemID)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = validateQuantifierPatterns(m, qid, uint32(len(spec.Sorts)), spec.Patterns); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(spec.Sorts) == 0 {
		return spec.Body, nil
	}
	return m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: spec.Forall, Sorts: spec.Sorts, Names: names, Body: spec.Body,
		Weight: spec.Weight, QID: qid, SkolemID: skid,
		Patterns: spec.Patterns, NoPatterns: spec.NoPatterns,
	})
}

func (c *SMTContext) Forall(sorts []ast.ASTHandle, names []string, body ast.ASTHandle) (ast.ASTHandle, error) {
	return c.Quantifier(QuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body})
}
func (c *SMTContext) Exists(sorts []ast.ASTHandle, names []string, body ast.ASTHandle) (ast.ASTHandle, error) {
	return c.Quantifier(QuantifierSpec{Sorts: sorts, Names: names, Body: body})
}

func (c *SMTContext) Pattern(terms ...ast.ASTHandle) (ast.ASTHandle, error) {
	if len(terms) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	m, err := c.quantifierManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, term := range terms {
		kind, e := m.GetKind(term)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if kind != ast.ASTApp {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
	}
	return m.MkPattern(terms)
}
func (c *SMTContext) IsQuantifier(value ast.ASTHandle) (bool, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return false, err
	}
	kind, err := m.GetKind(value)
	return kind == ast.ASTQuantifier, err
}
func (c *SMTContext) IsForall(value ast.ASTHandle) (bool, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return false, err
	}
	return m.IsForall(value)
}
func (c *SMTContext) IsExists(value ast.ASTHandle) (bool, error) {
	forall, err := c.IsForall(value)
	return !forall, err
}
func (c *SMTContext) QuantifierWeight(value ast.ASTHandle) (int32, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return 0, err
	}
	return m.GetQuantifierWeight(value)
}
func (c *SMTContext) QuantifierPatternCount(value ast.ASTHandle) (uint32, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return 0, err
	}
	return m.GetNumPatterns(value)
}
func (c *SMTContext) QuantifierPattern(value ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.GetPattern(value, index)
}
func (c *SMTContext) QuantifierNoPatternCount(value ast.ASTHandle) (uint32, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return 0, err
	}
	return m.GetNumNoPatterns(value)
}
func (c *SMTContext) QuantifierNoPattern(value ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.GetNoPattern(value, index)
}
func (c *SMTContext) QuantifierBoundCount(value ast.ASTHandle) (uint32, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return 0, err
	}
	return m.GetNumDeclarations(value)
}
func (c *SMTContext) QuantifierBoundName(value ast.ASTHandle, index uint32) (string, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return "", err
	}
	symbol, err := m.GetDeclarationName(value, index)
	if err != nil {
		return "", err
	}
	return m.Symbols.String(symbol)
}
func (c *SMTContext) QuantifierBoundSort(value ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.GetDeclarationSort(value, index)
}
func (c *SMTContext) QuantifierBody(value ast.ASTHandle) (ast.ASTHandle, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.GetQuantifierBody(value)
}
func (c *SMTContext) IsPattern(value ast.ASTHandle) (bool, error) {
	m, err := c.quantifierManager()
	if err != nil {
		return false, err
	}
	return m.IsPattern(value)
}
func (c *SMTContext) PatternTermCount(value ast.ASTHandle) (uint32, error) {
	yes, err := c.IsPattern(value)
	if err != nil {
		return 0, err
	}
	if !yes {
		return 0, ast.ErrASTDomain
	}
	m, err := c.quantifierManager()
	if err != nil {
		return 0, err
	}
	return m.GetNumArgs(value)
}
func (c *SMTContext) PatternTerm(value ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	yes, err := c.IsPattern(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !yes {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	m, err := c.quantifierManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.GetArg(value, index)
}
func (c *SMTContext) PatternString(value ast.ASTHandle) (string, error) {
	yes, err := c.IsPattern(value)
	if err != nil {
		return "", err
	}
	if !yes {
		return "", ast.ErrASTDomain
	}
	m, err := c.quantifierManager()
	if err != nil {
		return "", err
	}
	printer, err := ast.NewSimpleASTPrinterContext(m, nil, nil)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	err = printer.DisplayExpr(&out, value, 0)
	return out.String(), err
}

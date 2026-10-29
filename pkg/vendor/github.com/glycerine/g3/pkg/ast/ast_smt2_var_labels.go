package ast

// Source: src/ast/ast_smt2_pp.cpp:550-626.

import (
	"math"
	"strconv"

	usmt2 "github.com/glycerine/g3/pkg/util/smt2"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (m *ASTManager) smt2EnsureQuote(s usymbol.Symbol) (string, error) {
	quoted, err := usmt2.IsSMT2QuotedSymbol(m.Symbols, s)
	if err != nil {
		return "", err
	}
	if quoted {
		return usmt2.MkSMT2QuotedSymbol(m.Symbols, s, nil)
	}
	return m.Symbols.String(s)
}

// SMT2PPVar selects a quantifier binder from the innermost end of the source
// name stack. The fallback is source diagnostic syntax for an unbound index.
func (m *ASTManager) SMT2PPVar(index uint32, names []usymbol.Symbol) (ASTHandle, error) {
	if uint64(index) >= uint64(len(names)) {
		return m.FormatString("(:var " + strconv.FormatUint(uint64(index), 10) + ")")
	}
	name, err := m.smt2EnsureQuote(names[len(names)-int(index)-1])
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatString(name)
}

// SMT2PPAttribute preserves the source prefix and nested indentation.
func (m *ASTManager) SMT2PPAttribute(attribute string, body ASTHandle) (ASTHandle, error) {
	if uint64(len(attribute)) > math.MaxUint32 {
		return ASTHandle{}, ErrASTDomain
	}
	prefix, err := m.FormatString(attribute)
	if err != nil {
		return ASTHandle{}, err
	}
	indented, err := m.FormatIndent(uint32(len(attribute)), body)
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatCompose([]ASTHandle{prefix, indented})
}

func (m *ASTManager) SMT2PPSimpleAttributeInt(attribute string, value int32) (ASTHandle, error) {
	prefix, err := m.FormatString(attribute)
	if err != nil {
		return ASTHandle{}, err
	}
	number, err := m.FormatInt(value)
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatCompose([]ASTHandle{prefix, number})
}

func (m *ASTManager) SMT2PPSimpleAttributeSymbol(attribute string, value usymbol.Symbol) (ASTHandle, error) {
	name, err := m.smt2EnsureQuote(value)
	if err != nil {
		return ASTHandle{}, err
	}
	prefix, err := m.FormatString(attribute)
	if err != nil {
		return ASTHandle{}, err
	}
	spelling, err := m.FormatString(name)
	if err != nil {
		return ASTHandle{}, err
	}
	return m.FormatCompose([]ASTHandle{prefix, spelling})
}

// SMT2PPLabels wraps the source expression in one ! form with ordered labels.
func (m *ASTManager) SMT2PPLabels(positive bool, names []usymbol.Symbol, body ASTHandle) (ASTHandle, error) {
	if len(names) == 0 {
		return body, nil
	}
	parts := make([]ASTHandle, 0, 1+len(names))
	parts = append(parts, body)
	attribute := ":lblneg "
	if positive {
		attribute = ":lblpos "
	}
	for _, name := range names {
		label, err := m.SMT2PPSimpleAttributeSymbol(attribute, name)
		if err != nil {
			return ASTHandle{}, err
		}
		parts = append(parts, label)
	}
	return m.FormatSeq1("!", parts)
}

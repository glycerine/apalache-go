package ast

// Source: src/ast/seq_decl_plugin.cpp:692-717,925-931;
// src/ast/seq_decl_plugin.h:319-320.

import (
	"math"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// MkRegexLoop constructs the source unary re.loop declaration with one or two
// integer declaration parameters. Pass no upper bound for the one-parameter
// form, or one upper bound for the bounded form.
func (m *ASTManager) MkRegexLoop(regex ASTHandle, lower int32, upper ...int32) (ASTHandle, error) {
	if len(upper) > 1 {
		return ASTHandle{}, ErrASTDomain
	}
	return m.withSequenceFactory(func() (ASTHandle, error) {
		ref, err := m.requireExpr(regex)
		if err != nil {
			return ASTHandle{}, err
		}
		sort, err := m.sortOf(ref)
		if err != nil {
			return ASTHandle{}, err
		}
		if _, err = m.regexSequenceSort(sort); err != nil {
			return ASTHandle{}, err
		}
		name, err := m.Symbols.Intern("re.loop")
		if err != nil {
			return ASTHandle{}, err
		}
		first := astParameterRecord{Kind: ASTParameterInt, integer: lower}
		if err = m.beginFormatInfo(false, m.sequencePlugin.family, 20, &first); err != nil {
			return ASTHandle{}, err
		}
		if len(upper) == 1 {
			m.formatLexical.secondParameter = astParameterRecord{Kind: ASTParameterInt, integer: upper[0]}
			m.formatLexical.localPair = true
			m.formatLexical.info.count = 2
		}
		decl, err := m.mkFuncDecl(name, []ASTHandle{m.external(sort)}, m.external(sort), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
		m.endFormatInfo()
		if err != nil {
			return ASTHandle{}, err
		}
		app, err := m.mkApp(decl, []ASTHandle{regex})
		return m.external(app), err
	})
}

// MkSequenceSkolem constructs the retained seq_util symbol-parameterized
// skolem application with the concrete argument and result sorts supplied.
func (m *ASTManager) MkSequenceSkolem(name usymbol.Symbol, arguments []ASTHandle, rangeSort ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		rangeRef, err := m.requireKind(rangeSort, ASTSort)
		if err != nil {
			return ASTHandle{}, err
		}
		domain := make([]ASTHandle, len(arguments))
		for i, argument := range arguments {
			ref, e := m.requireExpr(argument)
			if e != nil {
				return ASTHandle{}, e
			}
			sort, e := m.sortOf(ref)
			if e != nil {
				return ASTHandle{}, e
			}
			domain[i] = m.external(sort)
		}
		parameter := astParameterRecord{Kind: ASTParameterSymbol, symbol: name}
		if err = m.beginFormatInfo(false, m.sequencePlugin.family, 42, &parameter); err != nil {
			return ASTHandle{}, err
		}
		decl, err := m.mkFuncDecl(name, domain, m.external(rangeRef), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
		m.endFormatInfo()
		if err != nil {
			return ASTHandle{}, err
		}
		app, err := m.mkApp(decl, arguments)
		return m.external(app), err
	})
}

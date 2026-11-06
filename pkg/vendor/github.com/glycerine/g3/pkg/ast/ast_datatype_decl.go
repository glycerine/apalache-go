package ast

// Source: src/ast/datatype_decl_plugin.cpp:193-381;
// src/ast/datatype_decl_plugin.h:24-38,237-288.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

const ASTDatatypeSort int32 = 0

const (
	ASTDatatypeConstructor int32 = iota
	ASTDatatypeRecognizer
	ASTDatatypeIs
	ASTDatatypeAccessor
	ASTDatatypeUpdateField
)

func datatypeDeclarationError(message string) error {
	return ASTException{z3exception.NewDefaultException(message)}
}

func (m *ASTManager) isDatatypeSort(p *astDatatypePluginOwner, h ASTHandle) (astNodeRef, bool, error) {
	r, err := m.requireKind(h, ASTSort)
	if err != nil {
		return astNodeRef{}, false, err
	}
	info, err := m.sortInfo(m.sortAt(r.slot).info)
	if err != nil {
		return astNodeRef{}, false, err
	}
	return r, info.family == p.family && info.kind == ASTDatatypeSort, nil
}

func (m *ASTManager) datatypeSort(kind int32, parameters []ASTParameter) (result astNodeRef, err error) {
	p := m.datatypePlugin
	if p == nil || !p.published {
		return astNodeRef{}, ErrASTHandle
	}
	if kind != ASTDatatypeSort || len(parameters) < 1 {
		return astNodeRef{}, datatypeDeclarationError("invalid datatype")
	}
	first, err := m.parameter(parameters[0])
	if err != nil {
		return astNodeRef{}, err
	}
	if first.Kind != ASTParameterSymbol {
		return astNodeRef{}, datatypeDeclarationError("invalid datatype")
	}
	for _, parameter := range parameters[1:] {
		value, e := m.parameter(parameter)
		if e != nil {
			return astNodeRef{}, e
		}
		if value.Kind != ASTParameterAST {
			return astNodeRef{}, datatypeDeclarationError("invalid datatype")
		}
		if e = m.validNode(value.ast); e != nil || m.nodeAt(value.ast.slot).kind != ASTSort {
			if e != nil {
				return astNodeRef{}, e
			}
			return astNodeRef{}, datatypeDeclarationError("invalid datatype")
		}
	}
	slot, generation, err := m.newInfo(true, p.family, kind, parameters, InfiniteSortSize(), ASTFuncFlags{}, true)
	if err != nil {
		return astNodeRef{}, err
	}
	defer func() { err = errors.Join(err, m.dropInfoStorage(slot, generation)) }()
	result, err = m.mkSort(first.symbol, ASTSortInfo{m.origin, slot, generation})
	if err == nil {
		if _, found := p.definitionNames[first.symbol]; found {
			err = m.datatypeSetCardinality(result)
		}
	}
	return result, err
}

func (m *ASTManager) datatypeDecl(kind int32, parameters []ASTParameter, domain []ASTHandle, suppliedRange ASTHandle) (astNodeRef, error) {
	p := m.datatypePlugin
	if p == nil || !p.published {
		return astNodeRef{}, ErrASTHandle
	}
	if kind < ASTDatatypeConstructor || kind > ASTDatatypeUpdateField {
		return astNodeRef{}, datatypeDeclarationError("invalid datatype operator kind")
	}
	for _, h := range domain {
		if _, err := m.requireKind(h, ASTSort); err != nil {
			return astNodeRef{}, err
		}
	}
	parameter := func(index int, expected ASTParameterKind) (*astParameterRecord, error) {
		if index < 0 || index >= len(parameters) {
			return nil, ErrASTDomain
		}
		value, err := m.parameter(parameters[index])
		if err != nil {
			return nil, err
		}
		if value.Kind != expected {
			return nil, ErrASTDomain
		}
		return value, nil
	}
	funcDeclParameter := func(index int) (astNodeRef, error) {
		value, err := parameter(index, ASTParameterAST)
		if err != nil {
			return astNodeRef{}, err
		}
		if err = m.validNode(value.ast); err != nil {
			return astNodeRef{}, err
		}
		if m.nodeAt(value.ast.slot).kind != ASTFuncDecl {
			return astNodeRef{}, ErrASTDomain
		}
		return value.ast, nil
	}
	boolRange := func() astNodeRef { return m.boolSort }

	switch kind {
	case ASTDatatypeConstructor:
		if len(parameters) != 1 || suppliedRange.IsNull() {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		name, err := parameter(0, ASTParameterSymbol)
		if err != nil {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		rangeSort, yes, err := m.isDatatypeSort(p, suppliedRange)
		if err != nil {
			return astNodeRef{}, err
		}
		if !yes {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		return m.pluginDecl(name.symbol, p.family, kind, parameters, domain, rangeSort, ASTFuncFlags{}, true)

	case ASTDatatypeRecognizer:
		if len(domain) != 1 || len(parameters) != 2 {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		if _, err := funcDeclParameter(0); err != nil {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		name, err := parameter(1, ASTParameterSymbol)
		if err != nil {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		if _, yes, err := m.isDatatypeSort(p, domain[0]); err != nil || !yes {
			if err != nil {
				return astNodeRef{}, err
			}
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		return m.pluginDecl(name.symbol, p.family, kind, parameters, domain, boolRange(), ASTFuncFlags{}, true)

	case ASTDatatypeIs:
		if len(domain) != 1 || len(parameters) != 1 {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		if _, err := funcDeclParameter(0); err != nil {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		if _, yes, err := m.isDatatypeSort(p, domain[0]); err != nil || !yes {
			if err != nil {
				return astNodeRef{}, err
			}
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		name, err := m.Symbols.Intern("is")
		if err != nil {
			return astNodeRef{}, err
		}
		return m.pluginDecl(name, p.family, kind, parameters, domain, boolRange(), ASTFuncFlags{}, true)

	case ASTDatatypeAccessor:
		if len(domain) != 1 || len(parameters) != 2 || suppliedRange.IsNull() {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		name, err := parameter(0, ASTParameterSymbol)
		if err != nil {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		if _, err = parameter(1, ASTParameterSymbol); err != nil {
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		if _, yes, err := m.isDatatypeSort(p, domain[0]); err != nil || !yes {
			if err != nil {
				return astNodeRef{}, err
			}
			return astNodeRef{}, datatypeDeclarationError("invalid parameter to datatype function")
		}
		rangeSort, err := m.requireKind(suppliedRange, ASTSort)
		if err != nil {
			return astNodeRef{}, err
		}
		decl, err := m.pluginDecl(name.symbol, p.family, kind, parameters, domain, rangeSort, ASTFuncFlags{}, true)
		if err != nil {
			return astNodeRef{}, err
		}
		domainSort, err := m.requireKind(domain[0], ASTSort)
		if err != nil {
			return astNodeRef{}, err
		}
		constructor, err := parameter(1, ASTParameterSymbol)
		if err != nil {
			return astNodeRef{}, err
		}
		key := astDatatypeAccessorKey{sort: domainSort, constructor: constructor.symbol}
		names := p.accessors[key]
		for _, existing := range names {
			if existing == name.symbol {
				return decl, nil
			}
		}
		p.accessors[key] = append(names, name.symbol)
		return decl, nil

	case ASTDatatypeUpdateField:
		if len(parameters) != 1 || len(domain) != 2 {
			return astNodeRef{}, datatypeDeclarationError("invalid parameters for datatype field update")
		}
		accessor, err := funcDeclParameter(0)
		if err != nil {
			return astNodeRef{}, datatypeDeclarationError("datatype field update requires a datatype accessor as the second argument")
		}
		family, accessorKind, err := m.declIdentity(accessor)
		if err != nil || family != p.family || accessorKind != ASTDatatypeAccessor {
			if err != nil {
				return astNodeRef{}, err
			}
			return astNodeRef{}, datatypeDeclarationError("datatype field update requires a datatype accessor as the second argument")
		}
		record := m.declAt(accessor.slot)
		accessorDomain, err := m.readInternal(record.domain, 0)
		if err != nil {
			return astNodeRef{}, err
		}
		first, err := m.requireKind(domain[0], ASTSort)
		if err != nil {
			return astNodeRef{}, err
		}
		second, err := m.requireKind(domain[1], ASTSort)
		if err != nil {
			return astNodeRef{}, err
		}
		if accessorDomain != first {
			return astNodeRef{}, datatypeDeclarationError("first argument to field update should be a data-type")
		}
		if record.rangeSort != second {
			return astNodeRef{}, datatypeDeclarationError("second argument to field update has the wrong sort")
		}
		name, err := m.Symbols.Intern("update-field")
		if err != nil {
			return astNodeRef{}, err
		}
		return m.pluginDecl(name, p.family, kind, parameters, domain, first, ASTFuncFlags{}, false)
	}
	return astNodeRef{}, datatypeDeclarationError("invalid datatype operator kind")
}

package ast

// Source: src/ast/dl_decl_plugin.cpp:266-320,605-625,760-768.

import (
	"errors"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (m *ASTManager) datalogFilterDecl(ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	if len(ps) != 1 || len(domain) != 1 {
		return astNodeRef{}, ErrASTDomain
	}
	relation, e := m.requireKind(domain[0], ASTSort)
	if e != nil {
		return astNodeRef{}, e
	}
	columns, e := m.datalogRelationColumnsRef(relation)
	if e != nil {
		return astNodeRef{}, e
	}
	parameter, e := m.parameter(ps[0])
	if e != nil || parameter.Kind != ASTParameterAST {
		return astNodeRef{}, ErrASTDomain
	}
	predicate := parameter.ast
	if e = m.validNode(predicate); e != nil || m.nodeAt(predicate.slot).kind > ASTQuantifier {
		return astNodeRef{}, ErrASTDomain
	}
	predicateSort, e := m.sortOf(predicate)
	if e != nil || predicateSort != m.basic.boolSort {
		return astNodeRef{}, ErrASTDomain
	}
	todo := []astNodeRef{predicate}
	visited := make(map[astNodeRef]struct{})
	for len(todo) != 0 {
		i := len(todo) - 1
		current := todo[i]
		todo = todo[:i]
		if _, found := visited[current]; found {
			continue
		}
		visited[current] = struct{}{}
		switch m.nodeAt(current.slot).kind {
		case ASTVar:
			variable := m.varAt(current.slot)
			if variable.index >= uint32(len(columns)) || columns[variable.index] != variable.sort {
				return astNodeRef{}, ErrASTDomain
			}
		case ASTApp:
			app := m.appAt(current.slot)
			for j := uint32(0); j < app.count; j++ {
				child, e := m.readInternal(app.args, j)
				if e != nil {
					return astNodeRef{}, e
				}
				todo = append(todo, child)
			}
		case ASTQuantifier:
			return astNodeRef{}, ErrASTDomain
		default:
			return astNodeRef{}, ErrASTDomain
		}
	}
	p := m.auxiliaryPlugin(astDatalogPlugin)
	if p == nil {
		return astNodeRef{}, ErrASTDomain
	}
	return m.pluginDecl(p.symbols[DatalogOpFilter], p.family, DatalogOpFilter, ps, domain, relation, ASTFuncFlags{}, false)
}

func (m *ASTManager) datalogConversionDecl(kind int32, ps []ASTParameter, domain []ASTHandle, rangeSort ASTHandle) (astNodeRef, error) {
	if len(ps) != 0 || len(domain) != 1 || rangeSort.IsNull() {
		return astNodeRef{}, ErrASTDomain
	}
	if _, e := m.requireKind(domain[0], ASTSort); e != nil {
		return astNodeRef{}, e
	}
	rangeRef, e := m.requireKind(rangeSort, ASTSort)
	if e != nil {
		return astNodeRef{}, e
	}
	p := m.auxiliaryPlugin(astDatalogPlugin)
	if p == nil {
		return astNodeRef{}, ErrASTDomain
	}
	name := "rep"
	if kind == DatalogOpAbs {
		name = "abs"
	}
	symbol, e := m.Symbols.Intern(name)
	if e != nil {
		return astNodeRef{}, e
	}
	return m.pluginDecl(symbol, p.family, kind, nil, domain, rangeRef, ASTFuncFlags{}, false)
}

func (m *ASTManager) MkDatalogFilter(relation, predicate ASTHandle) (result ASTHandle, err error) {
	family, e := m.RegisterDatalogPlugin()
	if e != nil {
		return result, e
	}
	parameter, e := m.NewParameterAST(predicate)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.DelParameter(&parameter)) }()
	return m.MkFamilyApp(family, DatalogOpFilter, []ASTParameter{parameter}, []ASTHandle{relation})
}

func (m *ASTManager) MkDatalogRepDecl(domain, rangeSort ASTHandle) (ASTHandle, error) {
	family, e := m.RegisterDatalogPlugin()
	if e != nil {
		return ASTHandle{}, e
	}
	return m.MkFamilyFuncDecl(family, DatalogOpRep, nil, []ASTHandle{domain}, rangeSort)
}

func (m *ASTManager) MkDatalogAbsDecl(domain, rangeSort ASTHandle) (ASTHandle, error) {
	family, e := m.RegisterDatalogPlugin()
	if e != nil {
		return ASTHandle{}, e
	}
	return m.MkFamilyFuncDecl(family, DatalogOpAbs, nil, []ASTHandle{domain}, rangeSort)
}

// MkDatalogRule constructs the source uninterpreted rule declaration and its
// application. Argument handles are borrowed by the factory.
func (m *ASTManager) MkDatalogRule(name usymbol.Symbol, args []ASTHandle) (ASTHandle, error) {
	ruleSort, e := m.MkDatalogRuleSort()
	if e != nil {
		return ASTHandle{}, e
	}
	domain := make([]ASTHandle, len(args))
	for i, arg := range args {
		domain[i], e = m.GetSort(arg)
		if e != nil {
			return ASTHandle{}, e
		}
	}
	declaration, e := m.MkFuncDecl(name, domain, ruleSort, ASTFuncDeclInfo{})
	if e != nil {
		return ASTHandle{}, e
	}
	return m.MkApp(declaration, args)
}

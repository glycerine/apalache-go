package ast

// Source: src/ast/array_decl_plugin.cpp:434-444,498-512,561-564;
// src/ast/array_decl_plugin.h:192-195; src/util/vector.h.

import "errors"

// Array declarations own their temporary parameter descriptors until the
// result is published. The slice itself is ordinary Go storage.
func (m *ASTManager) dropArrayParameters(values []ASTParameter) error {
	var err error
	for _, p := range values {
		err = errors.Join(err, m.dropParameter(p))
	}
	clear(values)
	return err
}
func (m *ASTManager) arrayAsArrayDecl(ps []ASTParameter) (result astNodeRef, err error) {
	if len(ps) != 1 {
		return result, ErrASTDomain
	}
	param, e := m.parameter(ps[0])
	if e != nil {
		return result, e
	}
	if param.Kind != ASTParameterAST {
		return result, ErrASTDomain
	}
	f, e := m.requireKind(m.external(param.ast), ASTFuncDecl)
	if e != nil {
		return result, e
	}
	decl := m.declAt(f.slot)
	if decl.arity == 0 {
		return result, ErrASTDomain
	}
	var parameters []ASTParameter
	defer func() { err = errors.Join(err, m.dropArrayParameters(parameters)) }()
	for i := uint32(0); i < decl.arity; i++ {
		s, e := m.readInternal(decl.domain, i)
		if e != nil {
			return result, e
		}
		p, e := m.newParameterAST(m.external(s))
		if e != nil {
			return result, e
		}
		parameters = append(parameters, p)
	}
	p, e := m.newParameterAST(m.external(decl.rangeSort))
	if e != nil {
		return result, e
	}
	parameters = append(parameters, p)
	sort, e := m.arraySort(0, parameters)
	if e != nil {
		return result, e
	}
	return m.pluginDecl(m.arrayPlugin.names[11], m.arrayPlugin.family, ASTArrayOpAsArray, ps, nil, sort, ASTFuncFlags{}, false)
}

// AsArrayFunction returns the borrowed function declaration held by as-array.
func (m *ASTManager) AsArrayFunction(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	app, e := m.requireKind(h, ASTApp)
	if e != nil {
		return ASTHandle{}, e
	}
	d := m.appAt(app.slot).decl
	fid, op, e := m.declIdentity(d)
	if e != nil {
		return ASTHandle{}, e
	}
	if m.arrayPlugin == nil || fid != m.arrayPlugin.family || op != ASTArrayOpAsArray {
		return ASTHandle{}, ErrASTDomain
	}
	info, e := m.funcInfo(m.declAt(d.slot).info)
	if e != nil {
		return ASTHandle{}, e
	}
	p, e := m.infoParameter(info, 0)
	if e != nil {
		return ASTHandle{}, e
	}
	record, e := m.parameter(p)
	if e != nil {
		return ASTHandle{}, e
	}
	if record.Kind != ASTParameterAST {
		return ASTHandle{}, ErrASTDomain
	}
	f, e := m.requireKind(m.external(record.ast), ASTFuncDecl)
	return m.external(f), e
}
func (m *ASTManager) MkAsArray(f ASTHandle) (result ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	name, e := m.Symbols.Intern("array")
	if e != nil {
		return result, e
	}
	fid, e := m.families.MkID(name)
	if e != nil {
		return result, e
	}
	p, e := m.newParameterAST(f)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(p)) }()
	r, e := m.mkFamilyApp(fid, ASTArrayOpAsArray, []ASTParameter{p}, nil)
	return m.external(r), e
}

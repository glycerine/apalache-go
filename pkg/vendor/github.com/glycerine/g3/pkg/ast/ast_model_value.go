package ast

import "errors"

// MkModelValue creates the source model-value constant with the given index.
func (m *ASTManager) MkModelValue(index uint32, sort ASTHandle) (result ASTHandle, err error) {
	p0, err := m.NewParameterUint(index)
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.DelParameter(&p0)) }()
	p1, err := m.NewParameterAST(sort)
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, m.DelParameter(&p1)) }()
	return m.MkFamilyApp(3, 0, []ASTParameter{p0, p1}, nil)
}

// IsModelValue recognizes constants created by the built-in model-value plugin.
func (m *ASTManager) IsModelValue(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	family, _, e := m.declIdentity(m.appAt(r.slot).decl)
	if e != nil || family < 0 || !m.pluginAt(family).present {
		return false, e
	}
	return m.pluginAt(family).kind == astModelValuePlugin, nil
}

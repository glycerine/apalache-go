package ast

// Source: src/ast/datatype_decl_plugin.cpp:410-477.

func (m *ASTManager) datatypeValue(root astNodeRef) (bool, error) {
	stack := []astNodeRef{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if m.nodeAt(current.slot).kind != ASTApp {
			return false, nil
		}
		app := m.appAt(current.slot)
		family, kind, err := m.declIdentity(app.decl)
		if err != nil {
			return false, err
		}
		if m.datatypePlugin != nil && family == m.datatypePlugin.family {
			if kind != ASTDatatypeConstructor {
				return false, nil
			}
			for i := uint32(0); i < app.count; i++ {
				child, e := m.readInternal(app.args, i)
				if e != nil {
					return false, e
				}
				stack = append(stack, child)
			}
			continue
		}
		value, err := m.isValue(current)
		if err != nil || !value {
			return value, err
		}
	}
	return true, nil
}

func (m *ASTManager) datatypeSomeValue(sort astNodeRef) (ASTHandle, error) {
	constructor, found, err := m.datatypeNonRecursiveConstructor(sort)
	if err != nil {
		return ASTHandle{}, err
	}
	if !found {
		return ASTHandle{}, datatypeDeclarationError("datatype has no non-recursive constructor")
	}
	record := m.declAt(constructor.slot)
	arguments := make([]ASTHandle, record.arity)
	for i := uint32(0); i < record.arity; i++ {
		domain, e := m.readInternal(record.domain, i)
		if e != nil {
			return ASTHandle{}, e
		}
		arguments[i], e = m.getSomeValue(domain)
		if e != nil {
			return ASTHandle{}, e
		}
	}
	value, err := m.mkApp(constructor, arguments)
	return m.external(value), err
}

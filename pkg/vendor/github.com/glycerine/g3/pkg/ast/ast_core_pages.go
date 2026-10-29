package ast

// Concrete typed page growth; each actual old/new directory overlap is charged.

import (
	"reflect"
)

func (m *ASTManager) ensureNodePage(slot uint32) error {
	index := int((slot - 1) / astPageSize)
	if index >= len(m.nodes) {
		if index >= cap(m.nodes) {
			old := cap(m.nodes)
			n, e := m.prepareDirectory(old, index+1)
			if e != nil {
				return e
			}
			next := make([]*astNodePage, len(m.nodes), n)
			copy(next, m.nodes)
			m.nodes = next
			m.publishDirectory(old, n)
		}
		m.nodes = m.nodes[:index+1]
	}
	if m.nodes[index] == nil {
		if e := m.reservePage(uint64(reflect.TypeOf(astNodePage{}).Size())); e != nil {
			return e
		}
		m.nodes[index] = new(astNodePage)
	}
	return nil
}

func (m *ASTManager) ensureSortPage(slot uint32) error {
	index := int((slot - 1) / astPageSize)
	if index >= len(m.sorts) {
		if index >= cap(m.sorts) {
			old := cap(m.sorts)
			n, e := m.prepareDirectory(old, index+1)
			if e != nil {
				return e
			}
			next := make([]*astSortPage, len(m.sorts), n)
			copy(next, m.sorts)
			m.sorts = next
			m.publishDirectory(old, n)
		}
		m.sorts = m.sorts[:index+1]
	}
	if m.sorts[index] == nil {
		if e := m.reservePage(uint64(reflect.TypeOf(astSortPage{}).Size())); e != nil {
			return e
		}
		m.sorts[index] = new(astSortPage)
	}
	return nil
}

func (m *ASTManager) ensureFuncDeclPage(slot uint32) error {
	index := int((slot - 1) / astPageSize)
	if index >= len(m.decls) {
		if index >= cap(m.decls) {
			old := cap(m.decls)
			n, e := m.prepareDirectory(old, index+1)
			if e != nil {
				return e
			}
			next := make([]*astFuncDeclPage, len(m.decls), n)
			copy(next, m.decls)
			m.decls = next
			m.publishDirectory(old, n)
		}
		m.decls = m.decls[:index+1]
	}
	if m.decls[index] == nil {
		if e := m.reservePage(uint64(reflect.TypeOf(astFuncDeclPage{}).Size())); e != nil {
			return e
		}
		m.decls[index] = new(astFuncDeclPage)
	}
	return nil
}

func (m *ASTManager) ensureAppPage(slot uint32) error {
	index := int((slot - 1) / astPageSize)
	if index >= len(m.apps) {
		if index >= cap(m.apps) {
			old := cap(m.apps)
			n, e := m.prepareDirectory(old, index+1)
			if e != nil {
				return e
			}
			next := make([]*astAppPage, len(m.apps), n)
			copy(next, m.apps)
			m.apps = next
			m.publishDirectory(old, n)
		}
		m.apps = m.apps[:index+1]
	}
	if m.apps[index] == nil {
		if e := m.reservePage(uint64(reflect.TypeOf(astAppPage{}).Size())); e != nil {
			return e
		}
		m.apps[index] = new(astAppPage)
	}
	return nil
}

func (m *ASTManager) ensureVarPage(slot uint32) error {
	index := int((slot - 1) / astPageSize)
	if index >= len(m.vars) {
		if index >= cap(m.vars) {
			old := cap(m.vars)
			n, e := m.prepareDirectory(old, index+1)
			if e != nil {
				return e
			}
			next := make([]*astVarPage, len(m.vars), n)
			copy(next, m.vars)
			m.vars = next
			m.publishDirectory(old, n)
		}
		m.vars = m.vars[:index+1]
	}
	if m.vars[index] == nil {
		if e := m.reservePage(uint64(reflect.TypeOf(astVarPage{}).Size())); e != nil {
			return e
		}
		m.vars[index] = new(astVarPage)
	}
	return nil
}

func (m *ASTManager) ensureQuantifierPage(slot uint32) error {
	index := int((slot - 1) / astPageSize)
	if index >= len(m.quantifiers) {
		if index >= cap(m.quantifiers) {
			old := cap(m.quantifiers)
			n, e := m.prepareDirectory(old, index+1)
			if e != nil {
				return e
			}
			next := make([]*astQuantifierPage, len(m.quantifiers), n)
			copy(next, m.quantifiers)
			m.quantifiers = next
			m.publishDirectory(old, n)
		}
		m.quantifiers = m.quantifiers[:index+1]
	}
	if m.quantifiers[index] == nil {
		if e := m.reservePage(uint64(reflect.TypeOf(astQuantifierPage{}).Size())); e != nil {
			return e
		}
		m.quantifiers[index] = new(astQuantifierPage)
	}
	return nil
}

func (m *ASTManager) ensureParameterPage(slot uint32) error {
	index := int((slot - 1) / astPageSize)
	if index >= len(m.Parameters) {
		if index >= cap(m.Parameters) {
			old := cap(m.Parameters)
			n, e := m.prepareDirectory(old, index+1)
			if e != nil {
				return e
			}
			next := make([]*astParameterPage, len(m.Parameters), n)
			copy(next, m.Parameters)
			m.Parameters = next
			m.publishDirectory(old, n)
		}
		m.Parameters = m.Parameters[:index+1]
	}
	if m.Parameters[index] == nil {
		if e := m.reservePage(uint64(reflect.TypeOf(astParameterPage{}).Size())); e != nil {
			return e
		}
		m.Parameters[index] = new(astParameterPage)
	}
	return nil
}

func (m *ASTManager) ensureInfoPage(slot uint32) error {
	index := int((slot - 1) / astPageSize)
	if index >= len(m.infos) {
		if index >= cap(m.infos) {
			old := cap(m.infos)
			n, e := m.prepareDirectory(old, index+1)
			if e != nil {
				return e
			}
			next := make([]*astInfoPage, len(m.infos), n)
			copy(next, m.infos)
			m.infos = next
			m.publishDirectory(old, n)
		}
		m.infos = m.infos[:index+1]
	}
	if m.infos[index] == nil {
		if e := m.reservePage(uint64(reflect.TypeOf(astInfoPage{}).Size())); e != nil {
			return e
		}
		m.infos[index] = new(astInfoPage)
	}
	return nil
}

func (m *ASTManager) ensureRefPage(slot uint32) error {
	index := int((slot - 1) / astPageSize)
	if index >= len(m.references) {
		if index >= cap(m.references) {
			old := cap(m.references)
			n, e := m.prepareDirectory(old, index+1)
			if e != nil {
				return e
			}
			next := make([]*astRefPage, len(m.references), n)
			copy(next, m.references)
			m.references = next
			m.publishDirectory(old, n)
		}
		m.references = m.references[:index+1]
	}
	if m.references[index] == nil {
		if e := m.reservePage(uint64(reflect.TypeOf(astRefPage{}).Size())); e != nil {
			return e
		}
		m.references[index] = new(astRefPage)
	}
	return nil
}

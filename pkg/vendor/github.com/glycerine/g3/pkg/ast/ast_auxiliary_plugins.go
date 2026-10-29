package ast

// Source: src/ast/dl_decl_plugin.cpp:31-48; src/ast/pb_decl_plugin.cpp:23-29;
// src/ast/reg_decl_plugins.cpp:30-55; src/ast/ast.cpp:1635-1638.

import (
	"reflect"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Actual symbol-member owners. Source symbols borrow the symbol table; they
// acquire no AST references. Failed initialization retains the reached prefix.

type astAuxiliaryPluginOwner struct {
	self                                  *astAuxiliaryPluginOwner
	owner                                 *ASTManager
	next                                  *astAuxiliaryPluginOwner
	symbols                               [17]usymbol.Symbol
	kind                                  astPluginKind
	initialized                           uint32
	family                                FamilyID
	header                                uint64
	completed, published, memoryTraceLive bool
	bodyCause                             error
}

func (m *ASTManager) auxiliaryPlugin(k astPluginKind) *astAuxiliaryPluginOwner {
	for p := m.auxiliaryPlugins; p != nil; p = p.next {
		if p.kind == k && p.published {
			return p
		}
	}
	return nil
}
func (m *ASTManager) registerAuxiliaryPlugin(k astPluginKind) (FamilyID, error) {
	family := "pb"
	if k == astDatalogPlugin {
		family = "datalog_relation"
	}
	name, e := m.Symbols.Intern(family)
	if e != nil {
		return NullFamilyID, e
	}
	id, e := m.families.GetID(name)
	if e != nil {
		return NullFamilyID, e
	}
	if id != NullFamilyID && m.pluginAt(id).present {
		return id, nil
	}
	bytes := uint64(reflect.TypeOf(astAuxiliaryPluginOwner{}).Size())
	if e = m.BudgetField.Reserve(bytes); e != nil {
		return NullFamilyID, e
	}
	p := &astAuxiliaryPluginOwner{owner: m, kind: k, header: bytes, next: m.auxiliaryPlugins}
	p.self = p
	line, typeName := 53, "pb_decl_plugin"
	if k == astDatalogPlugin {
		line, typeName = 44, "datalog::dl_decl_plugin"
	}
	m.traceRegisteredPluginAllocated(p.traceIdentity(), p.header, line, typeName, &p.memoryTraceLive)
	m.auxiliaryPlugins = p
	dl := [17]string{"store", "empty", "is_empty", "join", "union", "widen", "project", "filter", "negation_filter", "rename", "complement", "select", "clone", "N", "<", "<=", "R"}
	pb := [5]string{"at-most", "at-least", "pble", "pbge", "pbeq"}
	n := len(pb)
	if k == astDatalogPlugin {
		n = len(dl)
	}
	for i := 0; i < n; i++ {
		var s string
		if k == astDatalogPlugin {
			s = dl[i]
		} else {
			s = pb[i]
		}
		p.symbols[i], e = m.Symbols.Intern(s)
		if e != nil {
			p.bodyCause = e
			return NullFamilyID, e
		}
		p.initialized++
	}
	p.completed = true
	id, e = m.families.MkID(name)
	if e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	if e = m.setPlugin(id, k); e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	p.family = id
	p.published = true // actual registry pointer publication before base set_manager
	return id, nil
}
func (m *ASTManager) RegisterDatalogPlugin() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	return m.registerAuxiliaryPlugin(astDatalogPlugin)
}
func (m *ASTManager) RegisterPseudoBooleanPlugin() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	return m.registerAuxiliaryPlugin(astPseudoBooleanPlugin)
}
func (m *ASTManager) auxiliaryPluginBytes() uint64 {
	var n uint64
	for p := m.auxiliaryPlugins; p != nil; p = p.next {
		n += p.header
	}
	return n
}
func (m *ASTManager) releaseAuxiliaryPluginHistory() {
	for p := m.auxiliaryPlugins; p != nil; p = p.next {
		m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
		m.BudgetField.Release(p.header)
		p.header = 0
	}
	m.auxiliaryPlugins = nil
}

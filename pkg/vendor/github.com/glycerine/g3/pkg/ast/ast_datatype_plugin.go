package ast

// Source: src/ast/datatype_decl_plugin.h:237-246;
// src/ast/datatype_decl_plugin.cpp:150-162.
// The definition maps own the concrete datatype definition graph. Node
// references retained by that graph are recorded in definitionRefs and are
// released before the plugin's native Go containers are discarded.

import (
	"reflect"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type astDatatypeAccessorKey struct {
	sort        astNodeRef
	constructor usymbol.Symbol
}

type astDatatypeDefinition struct {
	name         usymbol.Symbol
	classID      uint32
	sort         astNodeRef
	parameters   []astNodeRef
	constructors []astDatatypeConstructorDefinition
}

type astDatatypeConstructorDefinition struct {
	name, recognizer usymbol.Symbol
	ranges           []astNodeRef
	accessors        []usymbol.Symbol
}

type astDatatypePluginOwner struct {
	self                                  *astDatatypePluginOwner
	owner                                 *ASTManager
	next                                  *astDatatypePluginOwner
	family                                FamilyID
	header                                uint64
	bodyCause                             error
	accessors                             map[astDatatypeAccessorKey][]usymbol.Symbol
	definitions                           map[astNodeRef]*astDatatypeDefinition
	definitionNames                       map[usymbol.Symbol]*astDatatypeDefinition
	definitionRefs                        []astNodeRef
	classID                               uint32
	utility                               *DatatypeUtil
	completed, published, memoryTraceLive bool
	membersEnded                          bool
}

func (m *ASTManager) registerDatatypePlugin() (FamilyID, error) {
	name, e := m.Symbols.Intern("datatype")
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
	bytes := uint64(reflect.TypeOf(astDatatypePluginOwner{}).Size())
	if e = m.BudgetField.Reserve(bytes); e != nil {
		return NullFamilyID, e
	}
	p := &astDatatypePluginOwner{owner: m, header: bytes, next: m.datatypePluginFailed}
	p.accessors = make(map[astDatatypeAccessorKey][]usymbol.Symbol)
	p.definitions = make(map[astNodeRef]*astDatatypeDefinition)
	p.definitionNames = make(map[usymbol.Symbol]*astDatatypeDefinition)
	p.self = p
	m.traceRegisteredPluginAllocated(p.traceIdentity(), p.header, 41, "datatype_decl_plugin", &p.memoryTraceLive)
	m.datatypePluginFailed = p
	p.completed = true
	id, e = m.families.MkID(name)
	if e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	if e = m.setPlugin(id, astDatatypePlugin); e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	m.datatypePluginFailed = p.next
	p.next = nil
	m.datatypePlugin = p
	p.family = id
	p.published = true
	return id, nil
}
func (m *ASTManager) RegisterDatatypePlugin() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	return m.registerDatatypePlugin()
}
func (m *ASTManager) datatypePluginBytes() uint64 {
	var n uint64
	if m.datatypePlugin != nil {
		n = m.datatypePlugin.header
	}
	for p := m.datatypePluginFailed; p != nil; p = p.next {
		n += p.header
	}
	return n
}
func (m *ASTManager) releaseDatatypePluginHistory() {
	release := func(p *astDatatypePluginOwner) {
		m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
		m.BudgetField.Release(p.header)
		p.header = 0
	}
	if m.datatypePlugin != nil {
		release(m.datatypePlugin)
		m.datatypePlugin = nil
	}
	for p := m.datatypePluginFailed; p != nil; p = p.next {
		release(p)
	}
	m.datatypePluginFailed = nil
}

// Source ast.cpp dealloc(plugin) invokes this completed object's BODY and
// automatic members at its actual registry position. History headers survive
// separately until the enclosing Go owner ends.
func (m *ASTManager) finalizeDatatypePlugin() error {
	p := m.datatypePlugin
	if p == nil || p.membersEnded {
		return nil
	}
	for len(p.definitionRefs) != 0 {
		last := len(p.definitionRefs) - 1
		ref := p.definitionRefs[last]
		consumed, err := m.decNode(ref)
		if consumed {
			p.definitionRefs[last] = astNodeRef{}
			p.definitionRefs = p.definitionRefs[:last]
		}
		if err != nil {
			return err
		}
	}
	// Source resets m_defs after deleting definitions and before m_util.
	clear(p.accessors)
	p.accessors = nil
	clear(p.definitions)
	p.definitions = nil
	clear(p.definitionNames)
	p.definitionNames = nil
	if p.utility != nil {
		if err := p.utility.closeLocked(); err != nil {
			return err
		}
		p.utility = nil
	}
	p.membersEnded = true
	return nil
}

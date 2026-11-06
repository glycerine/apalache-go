package ast

// Source: src/ast/format.cpp:25-117; src/ast/ast.cpp:1578-1585,1635-1638.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// The registry owns plugin tags in family order; plugin objects have their own
// lifetimes and are not owned by this native slice.
type astPluginVector struct{ bindings []astPluginBinding }

func (v *astPluginVector) close() { v.bindings = nil }
func (m *ASTManager) pluginAt(id FamilyID) astPluginBinding {
	if id < 0 || int(id) >= len(m.plugins.bindings) {
		return astPluginBinding{}
	}
	return m.plugins.bindings[id]
}
func (m *ASTManager) setPlugin(id FamilyID, kind astPluginKind) error {
	if id < 0 || uint64(id) >= math.MaxUint32 {
		return arena.ErrBounds
	}
	v := &m.plugins
	if need := int(id) + 1; need > len(v.bindings) {
		v.bindings = append(v.bindings, make([]astPluginBinding, need-len(v.bindings))...)
	}
	v.bindings[id] = astPluginBinding{kind: kind, present: true}
	return nil
}

type astFormatPluginOwner struct {
	next                       *astFormatPluginOwner
	self                       *astFormatPluginOwner
	owner                      *ASTManager
	names                      [7]usymbol.Symbol
	family                     FamilyID
	sort                       astNodeRef
	header                     uint64
	completed, published       bool
	memoryTraceLive            bool
	bodyCause, destructorCause error
}

func (m *ASTManager) formatPluginBytes() uint64 {
	var n uint64
	if m.formatPlugin != nil {
		n += m.formatPlugin.header
	}
	for p := m.formatPluginFailed; p != nil; p = p.next {
		n += p.header
	}
	return n
}
func (m *ASTManager) releaseFormatPluginHeaders() {
	if p := m.formatPlugin; p != nil {
		m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
		m.BudgetField.Release(p.header)
		p.header = 0
		m.formatPlugin = nil
	}
	if m.sourceClosing {
		return
	}
	for p := m.formatPluginFailed; p != nil; {
		next := p.next
		m.BudgetField.Release(p.header)
		p.header = 0
		p = next
	}
	m.formatPluginFailed = nil
}
func (m *ASTManager) finalizeFormatPlugin() error {
	p := m.formatPlugin
	if p == nil || p.sort.slot == 0 {
		return nil
	}
	consumed, e := m.decNode(p.sort)
	if consumed {
		p.sort = astNodeRef{}
	}
	if e != nil && p.destructorCause == nil {
		p.destructorCause = e
	}
	return e
}
func (m *ASTManager) formatFamilyID() (FamilyID, error) {
	// Source constructs symbol before reading has_plugin. Its source lookup is
	// non-owning; the cold wrapper is acquired only on the actual new-expression.
	name, e := m.Symbols.Intern("format")
	if e != nil {
		return NullFamilyID, e
	}
	id, e := m.families.GetID(name)
	if e != nil {
		return NullFamilyID, e
	}
	found := id != NullFamilyID
	if found && m.pluginAt(id).present {
		return m.families.MkID(name)
	}
	bytes := uint64(reflect.TypeOf(astFormatPluginOwner{}).Size())
	if e = m.BudgetField.Reserve(bytes); e != nil {
		return NullFamilyID, e
	}
	p := &astFormatPluginOwner{owner: m, header: bytes}
	p.self = p
	m.traceRegisteredPluginAllocatedAt(p.traceIdentity(), p.header, "src/ast/format.cpp", 105, "format_decl_plugin", &p.memoryTraceLive)
	// Unfinished wrapper is named immediately, before actual symbol constructors.
	p.next = m.formatPluginFailed
	m.formatPluginFailed = p
	for i, s := range [...]string{"nil", "string", "indent", "compose", "choice", "cr", "cr++"} {
		p.names[i], e = m.Symbols.Intern(s)
		if e != nil {
			p.bodyCause = e
			return NullFamilyID, e
		}
	}
	p.completed = true
	id, e = m.families.MkID(name)
	if e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	if e = m.setPlugin(id, astFormatPlugin); e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	// register_plugin publishes actual pointer before set_manager creates sort.
	m.formatPluginFailed = p.next
	p.next = nil
	m.formatPlugin = p
	p.published = true
	p.family = id
	if !m.IsFormatManager() {
		p.bodyCause = ErrASTDomain
		return NullFamilyID, ErrASTDomain
	}
	r, e := m.formatSortCreate(name, id)
	p.sort = r
	if e == nil {
		e = m.incNode(r)
	}
	if e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	return m.families.MkID(name)
}

func (m *ASTManager) GetFormatFamilyID() (FamilyID, error) {
	f, e := m.GetFormatManager()
	if e != nil {
		return NullFamilyID, e
	}
	if e = f.lock(); e != nil {
		return NullFamilyID, e
	}
	defer f.mu.Unlock()
	f.formatExecuting = true
	defer func() { f.formatExecuting = false }()
	return f.formatFamilyID()
}

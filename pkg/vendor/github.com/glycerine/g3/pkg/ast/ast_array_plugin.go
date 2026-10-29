package ast

// Source: src/ast/array_decl_plugin.cpp:25-114; src/ast/array_decl_plugin.h:67-130;
// src/ast/ast.cpp:1635-1638.

import (
	"reflect"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type astArrayPluginOwner struct {
	next                       *astArrayPluginOwner
	self                       *astArrayPluginOwner
	owner                      *ASTManager
	names                      [12]usymbol.Symbol
	family                     FamilyID
	header                     uint64
	published, memoryTraceLive bool
	completed                  uint32
	bodyCause                  error
}

func (m *ASTManager) arrayPluginBytes() uint64 {
	var n uint64
	if m.arrayPlugin != nil {
		n += m.arrayPlugin.header
	}
	for p := m.arrayPluginFailed; p != nil; p = p.next {
		n += p.header
	}
	return n
}
func (m *ASTManager) releaseArrayPluginHeaders() {
	if p := m.arrayPlugin; p != nil {
		m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
		m.BudgetField.Release(p.header)
		p.header = 0
		m.arrayPlugin = nil
	}
	if m.sourceClosing {
		return
	}
	for p := m.arrayPluginFailed; p != nil; {
		next := p.next
		m.BudgetField.Release(p.header)
		p.header = 0
		p = next
	}
	m.arrayPluginFailed = nil
}
func (m *ASTManager) registerArrayPlugin() (FamilyID, error) {
	name, e := m.Symbols.Intern("array")
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
	bytes := uint64(reflect.TypeOf(astArrayPluginOwner{}).Size())
	if e = m.BudgetField.Reserve(bytes); e != nil {
		return NullFamilyID, e
	}
	p := &astArrayPluginOwner{owner: m, header: bytes}
	p.self = p
	m.traceRegisteredPluginAllocated(p.traceIdentity(), p.header, 38, "array_decl_plugin", &p.memoryTraceLive)
	p.next = m.arrayPluginFailed
	m.arrayPluginFailed = p
	for j, text := range [...]string{"store", "select", "const", "default", "map", "union", "intersect", "difference", "complement", "subset", "array-ext", "as-array"} {
		p.names[j], e = m.Symbols.Intern(text)
		if e != nil {
			p.bodyCause = e
			return NullFamilyID, e
		}
		p.completed++
	}
	id, e = m.families.MkID(name)
	if e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	if e = m.setPlugin(id, astArrayPlugin); e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	m.arrayPluginFailed = p.next
	p.next = nil
	m.arrayPlugin = p
	p.published = true
	p.family = id
	// The inherited decl_plugin::set_manager stores the already captured manager
	// and family; array has no additional set_manager body or owned AST pins.
	return id, nil
}
func (m *ASTManager) RegisterArrayPlugin() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	return m.registerArrayPlugin()
}

type astArrayParameters struct {
	caller []ASTParameter
	locals *[2]astParameterRecord
}

func (ps astArrayParameters) count() int {
	if ps.locals != nil {
		return 2
	}
	return len(ps.caller)
}
func (ps astArrayParameters) at(m *ASTManager, i int) (*astParameterRecord, error) {
	if i < 0 || i >= ps.count() {
		return nil, ErrASTDomain
	}
	if ps.locals != nil {
		return &ps.locals[i], nil
	}
	return m.parameter(ps.caller[i])
}
func (m *ASTManager) arraySort(k int32, ps []ASTParameter) (_result astNodeRef, _body error) {
	if k == 1 {
		if len(ps) != 1 {
			return astNodeRef{}, ErrASTDomain
		}
		original, e := m.parameter(ps[0])
		if e != nil {
			return astNodeRef{}, e
		}
		locals := [2]astParameterRecord{*original, {}}
		var copied *astArrayCardinalityFrame
		if original.Kind == ASTParameterRational {
			copied, e = m.newArrayCardinalityFrame()
			if e != nil {
				return astNodeRef{}, e
			}
			defer func() { _body = m.finishArrayCardinality(copied, _body) }()
			if e = copied.construct(2, 0, false); e != nil {
				return astNodeRef{}, e
			}
			copied.completed[2] = false
			if e = m.numerals.Copy(&copied.Values[2], &original.Rational); e != nil {
				return astNodeRef{}, e
			}
			copied.completed[2] = true
			copied.tokens[2] = copied.Values[2]
			locals[0].Rational = copied.Values[2]
		}
		locals[1] = astParameterRecord{Kind: ASTParameterAST, ast: m.boolSort}
		return m.arraySortCore(0, astArrayParameters{locals: &locals})
	}
	return m.arraySortCore(k, astArrayParameters{caller: ps})
}
func (m *ASTManager) arraySortCore(k int32, ps astArrayParameters) (_result astNodeRef, _body error) {
	var frame *astArrayCardinalityFrame
	defer func() {
		if frame != nil {
			_body = m.finishArrayCardinality(frame, _body)
		}
	}()
	if ASTSourceDebug && k != 0 {
		return astNodeRef{}, ErrASTDomain
	}
	if ps.count() < 2 {
		return astNodeRef{}, ErrASTDomain
	}
	for i := 0; i < ps.count(); i++ {
		r, e := ps.at(m, i)
		if e != nil {
			return astNodeRef{}, e
		}
		if r.Kind != ASTParameterAST {
			return astNodeRef{}, ErrASTDomain
		}
		if _, e = m.requireKind(m.external(r.ast), ASTSort); e != nil {
			return astNodeRef{}, e
		}
	}
	last, e := ps.at(m, ps.count()-1)
	if e != nil {
		return astNodeRef{}, e
	}
	rangeInfo, e := m.sortInfo(m.sortAt(last.ast.slot).info)
	if e != nil {
		return astNodeRef{}, e
	}
	size := VeryBigSortSize()
	singleton := false
	if rangeInfo.size.IsFinite() {
		n, e := rangeInfo.size.Size()
		if e != nil {
			return astNodeRef{}, e
		}
		singleton = n == 1
	}
	if singleton {
		size = FiniteSortSize(1)
	} else {
		infinite, verybig := false, false
		for i := 0; i < ps.count(); i++ {
			r, e := ps.at(m, i)
			if e != nil {
				return astNodeRef{}, e
			}
			info, e := m.sortInfo(m.sortAt(r.ast.slot).info)
			if e != nil {
				return astNodeRef{}, e
			}
			if info.size.IsInfinite() {
				infinite = true
			}
			if info.size.IsVeryBig() {
				verybig = true
			}
		}
		if infinite {
			size = InfiniteSortSize()
		} else if !verybig {
			size, frame, e = m.finiteArraySize(ps)
			if e != nil {
				return astNodeRef{}, e
			}
		}
	}
	name, e := m.Symbols.Intern("Array")
	if e != nil {
		return astNodeRef{}, e
	}
	if e = m.beginFormatInfo(true, m.arrayPlugin.family, 0, nil); e != nil {
		return astNodeRef{}, e
	}
	defer m.endFormatInfo()
	if ps.locals != nil {
		m.formatLexical.parameter = ps.locals[0]
		m.formatLexical.secondParameter = ps.locals[1]
		m.formatLexical.localPair = true
	} else {
		m.formatLexical.borrowed = ps.caller
	}
	m.formatLexical.info.count = uint32(ps.count())
	m.formatLexical.info.size = size
	return m.mkSort(name, ASTSortInfo{m.origin, ^uint32(0) - 1, m.formatLexical.epoch})
}

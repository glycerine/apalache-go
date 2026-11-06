package ast

// Source: src/ast/bv_decl_plugin.cpp:26-178; src/ast/bv_decl_plugin.h:143-208.

import (
	"math"
	"reflect"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type astBitVectorPluginOwner struct {
	literalPrimary                          astBVLiteralFrame
	literalFrames                           *astBVLiteralFrame
	literalSerial                           uint64
	numeralPrimary                          astBVNumeralFrame
	numeralFrames                           *astBVNumeralFrame
	numeralSerial                           uint64
	next                                    *astBitVectorPluginOwner
	self                                    *astBitVectorPluginOwner
	owner                                   *ASTManager
	names                                   [10]usymbol.Symbol
	fields                                  [5]astNodeRef
	fieldAcquired                           [5]bool
	sortAcquired                            []byte // actual successful INC flags, alongside native sort slots
	fixed                                   [61][]astNodeRef
	fixedPending                            astNodeRef
	fixedPendingKind                        int32
	fixedPendingWidth                       uint32
	fixedPendingIndex                       uint32
	bit2bool                                [][]astNodeRef
	sorts                                   []astNodeRef
	family                                  FamilyID
	header                                  uint64
	power                                   unumeral.OwnedRational
	powerActive, published, memoryTraceLive bool
	bodyCause, destructorCause, powerCause  error
}

func (m *ASTManager) bitVectorPluginBytes() uint64 {
	var n uint64
	if p := m.bitVectorPlugin; p != nil {
		n += p.header + p.numeralBytes() + p.literalBytes()
	}
	for p := m.bitVectorPluginFailed; p != nil; p = p.next {
		n += p.header + p.numeralBytes() + p.literalBytes()
	}
	return n
}
func (m *ASTManager) releaseBitVectorPluginHeaders() {
	if p := m.bitVectorPlugin; p != nil {
		if !p.powerActive {
			p.releaseLiteralFrames()
			p.releaseNumeralFrames()
			p.releaseBooleanBits()
			p.releaseFixed()
			p.sorts = nil
			p.sortAcquired = nil
			m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
			m.BudgetField.Release(p.header)
			p.header = 0
			m.bitVectorPlugin = nil
		}
	}
	if m.sourceClosing {
		return
	}
	for p := m.bitVectorPluginFailed; p != nil; {
		next := p.next
		p.releaseLiteralFrames()
		p.releaseNumeralFrames()
		p.releaseBooleanBits()
		p.releaseFixed()
		p.sorts = nil
		p.sortAcquired = nil
		m.BudgetField.Release(p.header)
		p.header = 0
		p = next
	}
	m.bitVectorPluginFailed = nil
}
func (m *ASTManager) cleanupBitVectorPower() error {
	p := m.bitVectorPlugin
	if p == nil || !p.powerActive {
		return nil
	}
	e := m.numerals.Del(&p.power)
	if e != nil {
		if p.powerCause == nil {
			p.powerCause = e
		}
		return e
	}
	p.powerActive = false
	return nil
}
func (m *ASTManager) finalizeBitVectorPlugin() error {
	p := m.bitVectorPlugin
	if p == nil {
		return nil
	}
	if e := p.finalizeFixedPending(!m.sourceClosing); e != nil {
		return e
	}
	if e := p.destroyLiteralFrames(!m.sourceClosing); e != nil {
		return e
	}
	if e := p.destroyNumeralFrames(!m.sourceClosing); e != nil {
		return e
	}
	for j := range p.fields {
		if p.fields[j].slot == 0 {
			continue
		}
		if !m.sourceClosing && !p.fieldAcquired[j] {
			// No owned reference was acquired at this source INC leaf. The manager's
			// actual table pass retains the node; own abort must not fabricate a DEC.
			p.fields[j] = astNodeRef{}
			continue
		}
		consumed, e := m.decNode(p.fields[j])
		if consumed {
			p.fields[j] = astNodeRef{}
			p.fieldAcquired[j] = false
		}
		if e != nil {
			if p.destructorCause == nil {
				p.destructorCause = e
			}
			return e
		}
	}
	for j, r := range p.sorts {
		if r.slot == 0 {
			continue
		}
		if !m.sourceClosing && p.sortAcquired[j] == 0 {
			continue
		}
		consumed, e := m.decNode(r)
		if consumed {
			p.sorts[j] = astNodeRef{}
			p.sortAcquired[j] = 0
		}
		if e != nil {
			if p.destructorCause == nil {
				p.destructorCause = e
			}
			return e
		}
	}
	return p.finalizeFixed()
}
func (p *astBitVectorPluginOwner) mkSort(bits uint32) (astNodeRef, error) {
	m := p.owner
	if bits == math.MaxUint32 {
		return astNodeRef{}, ErrASTDomain
	}
	if uint64(len(p.sorts)) < uint64(bits)+1 {
		gap := int(bits) + 1 - len(p.sorts)
		p.sorts = append(p.sorts, make([]astNodeRef, gap)...)
		p.sortAcquired = append(p.sortAcquired, make([]byte, gap)...)
	}
	r := p.sorts[bits]
	if r.slot != 0 {
		return r, nil
	}
	if p.powerActive {
		return astNodeRef{}, ErrASTPending
	}
	size := VeryBigSortSize()
	if !SortSizeIsVeryBigBase2(bits) {
		power, e := m.numerals.PowerOfTwo(bits)
		if e != nil {
			return astNodeRef{}, e
		}
		// Actual successful returned numerical acquisition, not cache/count inference.
		p.power = power
		p.powerActive = true
		size, e = SortSizeFromRational(m.numerals, &p.power)
		if e != nil {
			return astNodeRef{}, e
		}
		if e = m.numerals.Del(&p.power); e != nil {
			if p.powerCause == nil {
				p.powerCause = e
			}
			return astNodeRef{}, e
		}
		p.powerActive = false
	}
	name, e := m.Symbols.Intern("bv")
	if e != nil {
		return astNodeRef{}, e
	}
	param := astParameterRecord{Kind: ASTParameterInt, integer: int32(bits)}
	if e = m.beginFormatInfo(true, p.family, 0, &param); e != nil {
		return astNodeRef{}, e
	}
	m.formatLexical.info.size = size
	r, e = m.mkSort(name, ASTSortInfo{m.origin, ^uint32(0) - 1, m.formatLexical.epoch})
	m.endFormatInfo()
	if e != nil {
		return astNodeRef{}, e
	}
	p.sorts[bits] = r
	if e = m.incNode(r); e != nil {
		return r, e
	}
	p.sortAcquired[bits] = 1
	return r, nil
}
func (p *astBitVectorPluginOwner) setManager() error {
	m := p.owner
	for bits := uint32(1); bits <= 64; bits++ {
		if _, e := p.mkSort(bits); e != nil {
			return e
		}
	}
	one := p.sorts[1]
	var e error
	p.fields[0], e = m.makePluginDeclaration(p.family, "bit0", 2, nil, one, ASTFuncFlags{})
	if e != nil {
		return e
	}
	p.fields[1], e = m.makePluginDeclaration(p.family, "bit1", 1, nil, one, ASTFuncFlags{})
	if e != nil {
		return e
	}
	if e = m.incNode(p.fields[0]); e != nil {
		return e
	}
	p.fieldAcquired[0] = true
	if e = m.incNode(p.fields[1]); e != nil {
		return e
	}
	p.fieldAcquired[1] = true
	b := m.basic.boolSort
	domain := [3]astNodeRef{b, b, b}
	p.fields[2], e = m.makePluginDeclaration(p.family, "carry", 59, domain[:], b, ASTFuncFlags{})
	if e != nil {
		return e
	}
	if e = m.incNode(p.fields[2]); e != nil {
		return e
	}
	p.fieldAcquired[2] = true
	p.fields[3], e = m.makePluginDeclaration(p.family, "xor3", 60, domain[:], b, ASTFuncFlags{})
	if e != nil {
		return e
	}
	if e = m.incNode(p.fields[3]); e != nil {
		return e
	}
	p.fieldAcquired[3] = true
	name, e := m.Symbols.Intern("arith")
	if e != nil {
		return e
	}
	arith, e := m.families.MkID(name)
	if e != nil {
		return e
	}
	if m.pluginAt(arith).kind == astArithmeticPlugin && m.pluginAt(arith).present {
		p.fields[4] = m.arithmeticPlugin.fields[1]
	}
	// The invalid missing-arith profile remains the existing explicit Go source
	// assertion/domain guard; supported generated registration installs arith first.
	if p.fields[4].slot == 0 {
		return ErrASTDomain
	}
	if e = m.incNode(p.fields[4]); e != nil {
		return e
	}
	p.fieldAcquired[4] = true
	return nil
}
func (m *ASTManager) RegisterBitVectorPlugin() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	name, e := m.Symbols.Intern("bv")
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
	bytes := uint64(reflect.TypeOf(astBitVectorPluginOwner{}).Size())
	if e = m.BudgetField.Reserve(bytes); e != nil {
		return NullFamilyID, e
	}
	p := &astBitVectorPluginOwner{owner: m, header: bytes}
	p.self = p
	m.traceRegisteredPluginAllocated(p.traceIdentity(), p.header, 35, "bv_decl_plugin", &p.memoryTraceLive)
	p.next = m.bitVectorPluginFailed
	m.bitVectorPluginFailed = p
	for j, s := range [...]string{"bv", "concat", "sign_extend", "zero_extend", "extract", "rotate_left", "rotate_right", "repeat", "bit2bool", "mkbv"} {
		p.names[j], e = m.Symbols.Intern(s)
		if e != nil {
			p.bodyCause = e
			return NullFamilyID, e
		}
	}
	id, e = m.families.MkID(name)
	if e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	if e = m.setPlugin(id, astBitVectorPlugin); e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	m.bitVectorPluginFailed = p.next
	p.next = nil
	m.bitVectorPlugin = p
	p.family = id
	p.published = true
	if e = p.setManager(); e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	return id, nil
}
func (m *ASTManager) BitVectorSort(bits uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	if bits == 0 || m.bitVectorPlugin == nil {
		return ASTHandle{}, ErrASTDomain
	}
	r, e := m.bitVectorPlugin.mkSort(bits)
	return m.external(r), e
}

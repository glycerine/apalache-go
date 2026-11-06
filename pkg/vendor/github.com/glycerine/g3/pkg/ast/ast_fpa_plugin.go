package ast

// Source: src/ast/fpa_decl_plugin.cpp:23-49,182-231;
// src/ast/fpa_decl_plugin.h:109-122; src/ast/ast.cpp:1406-1414,1635-1638.

import (
	"errors"
	"math"
	"reflect"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// Source m_fm owns its distinct unsynchronized mpq; the AST and original
// rational service remain borrowed. Empty id_gen and scoped_mpf_vector have
// null inline storage and introduce no allocation or default numeral action.
// Populated MPF values use native hash buckets with actual MPF hash/equality.
type astFloatingPointPluginOwner struct {
	literalPrimary                                                               astFloatLiteralFrame
	literalFrames                                                                *astFloatLiteralFrame
	literalSerial                                                                uint64
	numeralProbePrimary                                                          astFloatNumeralProbe
	numeralProbeFrames                                                           *astFloatNumeralProbe
	assertions                                                                   *uparams.GlobalParamsService
	next                                                                         *astFloatingPointPluginOwner
	self                                                                         *astFloatingPointPluginOwner
	owner                                                                        *ASTManager
	numeric                                                                      *unumeral.ArbitraryFloatManager
	failedNumeric                                                                error
	nextID                                                                       uint32
	freeIDs                                                                      []uint32
	values                                                                       astScopedFloatVector
	table                                                                        map[uint32][]uint32
	real, integer                                                                astNodeRef
	bitvector                                                                    *astBitVectorPluginOwner
	family, arithFamily, bvFamily                                                FamilyID
	acquired                                                                     [2]bool
	header                                                                       uint64
	numericReady, tableReady, completed, published, sourceEnded, memoryTraceLive bool
	bodyCause, sourceCause, ownCause                                             error
}

func (m *ASTManager) floatingPointPluginBytes() uint64 {
	bytes := func(p *astFloatingPointPluginOwner) uint64 {
		n := p.header + p.literalBytes() + p.numeralProbeBytes()
		if p.numeric != nil {
			n += p.numeric.OwnedStorageBytes()
		} else {
			var failed *unumeral.ArbitraryFloatManager
			if errors.As(p.failedNumeric, &failed) {
				n += failed.OwnedStorageBytes()
			}
		}
		return n
	}
	var n uint64
	if m.floatingPointPlugin != nil {
		n += bytes(m.floatingPointPlugin)
	}
	for p := m.floatingPointPluginFailed; p != nil; p = p.next {
		n += bytes(p)
	}
	return n
}
func (m *ASTManager) registerFloatingPointPlugin() (FamilyID, error) {
	name, e := m.Symbols.Intern("fpa")
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
	bytes := uint64(reflect.TypeOf(astFloatingPointPluginOwner{}).Size())
	if e = m.BudgetField.Reserve(bytes); e != nil {
		return NullFamilyID, e
	}
	p := &astFloatingPointPluginOwner{owner: m, header: bytes, next: m.floatingPointPluginFailed}
	p.self = p
	m.traceRegisteredPluginAllocated(p.traceIdentity(), p.header, 50, "fpa_decl_plugin", &p.memoryTraceLive)
	m.floatingPointPluginFailed = p
	p.numeric, e = m.numerals.NewArbitraryFloatManager()
	if e != nil {
		p.failedNumeric = e
		p.bodyCause = e
		return NullFamilyID, e
	}
	p.numericReady = true
	p.values.bind(p)
	p.table = make(map[uint32][]uint32)
	p.tableReady = true
	// These assignments are the literal constructor BODY, after all members.
	p.real, p.integer = astNodeRef{}, astNodeRef{}
	p.bitvector = nil
	p.completed = true
	id, e = m.families.MkID(name)
	if e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	if e = m.setPlugin(id, astFloatingPointPlugin); e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	m.floatingPointPluginFailed = p.next
	p.next = nil
	m.floatingPointPlugin = p
	p.published = true
	p.family = id
	name, e = m.Symbols.Intern("arith")
	if e != nil {
		p.bodyCause = e
		return id, e
	}
	p.arithFamily, e = m.families.MkID(name)
	if e != nil {
		p.bodyCause = e
		return id, e
	}
	if m.pluginAt(p.arithFamily).present && m.pluginAt(p.arithFamily).kind == astArithmeticPlugin {
		p.real = m.arithmeticPlugin.fields[0]
	}
	// Actual registration installs arith and BV first. Missing prerequisite
	// continuation remains an explicit checked pointer-domain guard here.
	if p.real.slot == 0 {
		p.bodyCause = ErrASTDomain
		return id, p.bodyCause
	}
	if e = m.incNode(p.real); e != nil {
		p.bodyCause = e
		return id, e
	}
	p.acquired[0] = true
	if m.pluginAt(p.arithFamily).present && m.pluginAt(p.arithFamily).kind == astArithmeticPlugin {
		p.integer = m.arithmeticPlugin.fields[1]
	}
	if p.integer.slot == 0 {
		p.bodyCause = ErrASTDomain
		return id, p.bodyCause
	}
	if e = m.incNode(p.integer); e != nil {
		p.bodyCause = e
		return id, e
	}
	p.acquired[1] = true
	name, e = m.Symbols.Intern("bv")
	if e != nil {
		p.bodyCause = e
		return id, e
	}
	idBV, e := m.families.GetID(name)
	if e != nil {
		p.bodyCause = e
		return id, e
	}
	if idBV == NullFamilyID || !m.pluginAt(idBV).present || m.pluginAt(idBV).kind != astBitVectorPlugin {
		p.bodyCause = ErrASTDomain
		return id, p.bodyCause
	}
	p.bvFamily, e = m.families.MkID(name)
	if e != nil {
		p.bodyCause = e
		return id, e
	}
	p.bitvector = m.bitVectorPlugin
	return id, nil
}
func (m *ASTManager) RegisterFloatingPointPlugin() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	return m.registerFloatingPointPlugin()
}

// ExistingFloatingPointFamilyID reads the registered family without creating
// a plugin. Model evaluation uses it to dispatch only already reached FPA
// declarations and sorts.
func (m *ASTManager) ExistingFloatingPointFamilyID() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return NullFamilyID, nil
	}
	return p.family, nil
}
func (m *ASTManager) floatingPointSort(k int32, ps []ASTParameter) (astNodeRef, error) {
	p := m.floatingPointPlugin
	var ebits, sbits uint32
	switch k {
	case 0:
		if len(ps) != 2 {
			return astNodeRef{}, ErrASTDomain
		}
		a, e := m.parameter(ps[0])
		if e != nil {
			return astNodeRef{}, e
		}
		if a.Kind != ASTParameterInt {
			return astNodeRef{}, ErrASTDomain
		}
		b, e := m.parameter(ps[1])
		if e != nil {
			return astNodeRef{}, e
		}
		if b.Kind != ASTParameterInt {
			return astNodeRef{}, ErrASTDomain
		}
		ebits, sbits = uint32(a.integer), uint32(b.integer)
	case 1:
		name, e := m.Symbols.Intern("RoundingMode")
		if e != nil {
			return astNodeRef{}, e
		}
		if e = m.beginFormatInfo(true, p.family, 1, nil); e != nil {
			return astNodeRef{}, e
		}
		defer m.endFormatInfo()
		return m.mkSort(name, ASTSortInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
	case 2:
		ebits, sbits = 5, 11
	case 3:
		ebits, sbits = 8, 24
	case 4:
		ebits, sbits = 11, 53
	case 5:
		ebits, sbits = 15, 113
	default:
		return astNodeRef{}, ErrASTDomain
	}
	return m.mkFloatSort(ebits, sbits)
}

// Source: src/ast/fpa_decl_plugin.cpp:192-203. Same real lexical sort engine.
func (m *ASTManager) mkFloatSort(ebits, sbits uint32) (astNodeRef, error) {
	p := m.floatingPointPlugin
	// The unsigned conversion occurs at the call boundary before these checks.
	if sbits < 2 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("minimum number of significand bits is 1")}
	}
	if ebits < 2 {
		return astNodeRef{}, ASTException{z3exception.NewDefaultException("minimum number of exponent bits is 2")}
	}
	a, b := astParameterRecord{Kind: ASTParameterInt, integer: int32(ebits)}, astParameterRecord{Kind: ASTParameterInt, integer: int32(sbits)}
	name, e := m.Symbols.Intern("FloatingPoint")
	if e != nil {
		return astNodeRef{}, e
	}
	if e = m.beginFormatInfo(true, p.family, 0, &a); e != nil {
		return astNodeRef{}, e
	}
	defer m.endFormatInfo()
	f := &m.formatLexical
	f.secondParameter = b
	f.localPair = true
	f.info.count = 2
	f.info.size = VeryBigSortSize()
	return m.mkSort(name, ASTSortInfo{m.origin, math.MaxUint32 - 1, f.epoch})
}
func (m *ASTManager) finalizeFloatingPointPlugin() error {
	p := m.floatingPointPlugin
	if p == nil {
		return nil
	}
	if m.sourceClosing && p.sourceCause != nil {
		return p.sourceCause
	}
	for i, r := range [2]*astNodeRef{&p.real, &p.integer} {
		if r.slot == 0 || !m.sourceClosing && !p.acquired[i] {
			continue
		}
		consumed, e := m.decNode(*r)
		if consumed {
			*r = astNodeRef{}
			p.acquired[i] = false
		}
		if e != nil {
			if m.sourceClosing && p.sourceCause == nil {
				p.sourceCause = e
			}
			return e
		}
	}
	return nil
}
func (m *ASTManager) destroyFloatingPointMembers(own bool) error {
	destroy := func(p *astFloatingPointPluginOwner) error {
		stop := func(e error) error {
			if e != nil {
				if own {
					if p.ownCause == nil {
						p.ownCause = e
					}
				} else if p.sourceCause == nil {
					p.sourceCause = e
				}
			}
			return e
		}
		if !own && p.sourceCause != nil {
			return p.sourceCause
		}
		if e := p.destroyLiteralFrames(own); e != nil {
			return stop(e)
		}
		if e := p.destroyNumeralProbes(own); e != nil {
			return stop(e)
		}
		if p.tableReady {
			p.table = nil
			p.tableReady = false
		}
		if p.values.owner != nil {
			if e := p.values.destroy(own); e != nil {
				return stop(e)
			}
		}
		p.nextID, p.freeIDs = 0, nil
		if p.numeric != nil {
			var e error
			if own {
				e = p.numeric.Close()
			} else if !p.sourceEnded {
				e = p.numeric.SourceFinalize()
			}
			if e != nil {
				return stop(e)
			}
		}
		if own && p.failedNumeric != nil {
			var failed umemory.ConstructionFailure
			if errors.As(p.failedNumeric, &failed) {
				if e := failed.Cleanup(); e != nil {
					return stop(e)
				}
			}
		}
		p.sourceEnded = true
		return nil
	}
	if p := m.floatingPointPlugin; p != nil {
		if e := destroy(p); e != nil {
			return e
		}
	}
	if own {
		for p := m.floatingPointPluginFailed; p != nil; p = p.next {
			if e := destroy(p); e != nil {
				return e
			}
		}
	}
	return nil
}
func (m *ASTManager) releaseFloatingPointHistory() {
	if p := m.floatingPointPlugin; p != nil {
		p.releaseLiteralFrames()
		p.releaseNumeralProbes()
		m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
		m.BudgetField.Release(p.header)
		p.header = 0
	}
	m.floatingPointPlugin = nil
	for p := m.floatingPointPluginFailed; p != nil; p = p.next {
		p.releaseLiteralFrames()
		p.releaseNumeralProbes()
		m.BudgetField.Release(p.header)
		p.header = 0
	}
	m.floatingPointPluginFailed = nil
}

// This is the actual cold resource-service borrow used by id_gen::recycle.
func (p *astFloatingPointPluginOwner) outOfMemory() bool {
	return p.numeric.RuntimeOwner().State().OutOfMemory
}

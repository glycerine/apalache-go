package ast

// Source: src/ast/seq_decl_plugin.cpp:343-357,500-610;
// src/ast/seq_decl_plugin.h:126-175.

import (
	"errors"
	"math"
	"reflect"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Concrete signature owners retain actual sort references. Native slices hold
// published signatures and each owning domain in source declaration order.
type astSequenceSignature struct {
	next                                                           *astSequenceSignature
	owner                                                          *astSequencePluginOwner
	name                                                           usymbol.Symbol
	parameters, id, cursor                                         uint32
	domain                                                         []astNodeRef
	rangeSort                                                      astNodeRef
	header                                                         uint64
	rangeAcquired, completed, ended, contextEnded, memoryTraceLive bool
	bodyCause, sourceCause, ownCause                               error
}
type astSequencePluginOwner struct {
	next                                    *astSequencePluginOwner
	self                                    *astSequencePluginOwner
	owner                                   *ASTManager
	names                                   [2]usymbol.Symbol
	family                                  FamilyID
	signatures                              []*astSequenceSignature
	owned                                   *astSequenceSignature
	char, stringSort, regex                 astNodeRef
	acquired                                [3]bool // exact successful String, Char, RE INC leaves
	serial                                  uint32
	header                                  uint64
	published, initialized, memoryTraceLive bool
	bodyCause, sourceCause                  error
}

func (p *astSequencePluginOwner) newSignature(name string, n uint32, dom []astNodeRef, rng astNodeRef, line int) (_s *astSequenceSignature, _body error) {
	m := p.owner
	if p.serial == math.MaxUint32 {
		return nil, ErrASTPending
	}
	bytes := uint64(reflect.TypeOf(astSequenceSignature{}).Size())
	if e := m.BudgetField.Reserve(bytes); e != nil {
		return nil, e
	}
	p.serial++
	s := &astSequenceSignature{owner: p, id: p.serial, header: bytes, next: p.owned, memoryTraceLive: true}
	p.owned = s
	p.traceSignatureAllocated(s, line)
	defer func() {
		if _body != nil {
			s.bodyCause = _body
			_body = errors.Join(_body, p.destroySignature(s, false))
		}
	}()
	var e error
	s.name, e = m.Symbols.Intern(name)
	if e != nil {
		return s, e
	}
	s.parameters = n
	s.rangeSort = rng
	if e = m.incNode(rng); e != nil {
		return s, e
	}
	s.rangeAcquired = true
	for _, r := range dom {
		if e = m.incNode(r); e != nil {
			return s, e
		}
		s.domain = append(s.domain, r)
	}
	s.completed = true
	return s, nil
}
func (p *astSequencePluginOwner) destroySignature(s *astSequenceSignature, own bool) error {
	if s.owner != p || s.id == 0 || s.id > p.serial {
		return ErrASTHandle
	}
	if s.contextEnded {
		return nil
	}
	if s.ended {
		return nil
	}
	if !own && s.sourceCause != nil {
		return s.sourceCause
	}
	stop := func(e error) error {
		if e != nil {
			if own {
				if s.ownCause == nil {
					s.ownCause = e
				}
			} else if s.sourceCause == nil {
				s.sourceCause = e
			}
		}
		return e
	}
	m := p.owner
	if s.rangeAcquired {
		consumed, e := m.decNode(s.rangeSort)
		if consumed {
			s.rangeAcquired = false
		}
		if e != nil {
			return stop(e)
		}
	}
	for s.cursor < uint32(len(s.domain)) {
		consumed, e := m.decNode(s.domain[s.cursor])
		if consumed {
			s.domain[s.cursor] = astNodeRef{}
			s.cursor++
		}
		if e != nil {
			return stop(e)
		}
		if !consumed {
			return stop(ErrASTPending)
		}
	}
	s.domain = nil
	s.ended = true
	p.traceSignatureDeallocated(s)
	return nil
}
func (m *ASTManager) sequencePluginBytes() uint64 {
	var n uint64
	add := func(p *astSequencePluginOwner) {
		if p == nil {
			return
		}
		n += p.header
		for s := p.owned; s != nil; s = s.next {
			n += s.header
		}
	}
	add(m.sequencePlugin)
	for p := m.sequencePluginFailed; p != nil; p = p.next {
		add(p)
	}
	return n
}
func (m *ASTManager) endSequenceContexts() {
	// Called only after this exact manager's source allocator member has ended.
	// It grants no source replay/DEC against ended nodes or vector records.
	for _, p := range []*astSequencePluginOwner{m.sequencePlugin, m.sequencePluginFailed} {
		for ; p != nil; p = p.next {
			for s := p.owned; s != nil; s = s.next {
				s.contextEnded = true
			}
		}
	}
}
func (m *ASTManager) releaseSequencePluginHeaders() {
	// Source plugin destruction ends only completed signatures. Retain the real
	// Go owner/cause/tuple storage for unjoined constructor acquisitions until
	// actual enclosing owner completion, separately from the source registry.
	if m.sourceClosing {
		return
	}
	for p := m.sequencePlugin; p != nil; {
		next := p.next
		m.traceRegisteredPluginDeallocated(p.traceIdentity(), p.published, &p.memoryTraceLive)
		p.signatures = nil
		for s := p.owned; s != nil; s = s.next {
			m.BudgetField.Release(s.header)
			s.header = 0
		}
		m.BudgetField.Release(p.header)
		p.header = 0
		p = next
	}
	m.sequencePlugin = nil
	if m.sourceClosing {
		return
	}
	for p := m.sequencePluginFailed; p != nil; {
		next := p.next
		p.signatures = nil
		for s := p.owned; s != nil; s = s.next {
			m.BudgetField.Release(s.header)
			s.header = 0
		}
		m.BudgetField.Release(p.header)
		p.header = 0
		p = next
	}
	m.sequencePluginFailed = nil
}
func (m *ASTManager) finalizeSequencePlugin() error {
	p := m.sequencePlugin
	if p == nil {
		return nil
	}
	if m.sourceClosing && p.sourceCause != nil {
		return p.sourceCause
	}
	for _, s := range p.signatures {
		if s == nil {
			continue
		}
		if e := p.destroySignature(s, !m.sourceClosing); e != nil {
			if p.sourceCause == nil && m.sourceClosing {
				p.sourceCause = e
			}
			return e
		}
	}
	fields := [3]*astNodeRef{&p.stringSort, &p.char, &p.regex}
	for i, r := range fields {
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
	if !m.sourceClosing {
		for s := p.owned; s != nil; s = s.next {
			if e := p.destroySignature(s, true); e != nil {
				return e
			}
		}
	}
	return nil
}
func (m *ASTManager) registerSequencePlugin() (FamilyID, error) {
	name, e := m.Symbols.Intern("seq")
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
	bytes := uint64(reflect.TypeOf(astSequencePluginOwner{}).Size())
	if e = m.BudgetField.Reserve(bytes); e != nil {
		return NullFamilyID, e
	}
	p := &astSequencePluginOwner{owner: m, header: bytes, next: m.sequencePluginFailed}
	p.self = p
	m.traceRegisteredPluginAllocated(p.traceIdentity(), p.header, 47, "seq_decl_plugin", &p.memoryTraceLive)
	m.sequencePluginFailed = p
	for i, text := range [...]string{"String", "Char"} {
		p.names[i], e = m.Symbols.Intern(text)
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
	if e = m.setPlugin(id, astSequencePlugin); e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	m.sequencePluginFailed = p.next
	p.next = nil
	m.sequencePlugin = p
	p.family = id
	p.published = true
	if m.bitVectorPlugin == nil {
		return id, ErrASTDomain
	}
	p.char, e = m.bitVectorPlugin.mkSort(8)
	if e != nil {
		p.bodyCause = e
		return id, e
	}
	if e = m.incNode(p.char); e != nil {
		p.bodyCause = e
		return id, e
	}
	p.acquired[1] = true
	param := astParameterRecord{Kind: ASTParameterAST, ast: p.char}
	p.stringSort, e = m.sequenceSortCreate(p, "String", 0, &param)
	if e != nil {
		p.bodyCause = e
		return id, e
	}
	if e = m.incNode(p.stringSort); e != nil {
		p.bodyCause = e
		return id, e
	}
	p.acquired[0] = true
	param.ast = p.stringSort
	p.regex, e = m.sequenceSort(p, 1, &param)
	if e != nil {
		p.bodyCause = e
		return id, e
	}
	if e = m.incNode(p.regex); e != nil {
		p.bodyCause = e
		return id, e
	}
	p.acquired[2] = true
	return id, nil
}
func (m *ASTManager) RegisterSequencePlugin() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	return m.registerSequencePlugin()
}

func (m *ASTManager) SequenceFamilyID() (FamilyID, error) {
	if err := m.prepareSequencePlugin(); err != nil {
		return NullFamilyID, err
	}
	if err := m.lock(); err != nil {
		return NullFamilyID, err
	}
	defer m.mu.Unlock()
	if m.sequencePlugin == nil || !m.sequencePlugin.published {
		return NullFamilyID, ErrASTHandle
	}
	return m.sequencePlugin.family, nil
}
func (m *ASTManager) sequenceSortCreate(p *astSequencePluginOwner, text string, k int32, param *astParameterRecord) (astNodeRef, error) {
	name, e := m.Symbols.Intern(text)
	if e != nil {
		return astNodeRef{}, e
	}
	if e = m.beginFormatInfo(true, p.family, k, param); e != nil {
		return astNodeRef{}, e
	}
	defer m.endFormatInfo()
	return m.mkSort(name, ASTSortInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
}
func (m *ASTManager) sequenceSort(p *astSequencePluginOwner, k int32, param *astParameterRecord) (astNodeRef, error) {
	if e := m.initializeSequence(p); e != nil {
		return astNodeRef{}, e
	}
	if k == 2 {
		return p.stringSort, nil
	}
	if param == nil || param.Kind != ASTParameterAST || m.validNode(param.ast) != nil || m.nodeAt(param.ast.slot).kind != ASTSort {
		return astNodeRef{}, ErrASTDomain
	}
	switch k {
	case 0:
		if param.ast == p.char {
			return p.stringSort, nil
		}
		return m.sequenceSortCreate(p, "Seq", k, param)
	case 1:
		return m.sequenceSortCreate(p, "RegEx", k, param)
	}
	return astNodeRef{}, ErrASTDomain
}

func (m *ASTManager) initializeSequence(p *astSequencePluginOwner) error {
	if p.initialized {
		return nil
	}
	p.initialized = true // source publication precedes every recursive declaration
	k, e := m.registerUserName(usymbol.Numerical(0))
	if e != nil {
		return e
	}
	A, e := m.userSort(k, nil)
	if e != nil {
		return e
	}
	strT := p.stringSort
	param := astParameterRecord{Kind: ASTParameterAST, ast: A}
	seqA, e := m.sequenceSort(p, 0, &param)
	if e != nil {
		return e
	}
	param.ast = seqA
	reA, e := m.sequenceSort(p, 1, &param)
	if e != nil {
		return e
	}
	param.ast = strT
	reT, e := m.sequenceSort(p, 1, &param)
	if e != nil {
		return e
	}
	boolT := m.boolSort
	if m.arithmeticPlugin == nil {
		return ErrASTDomain
	}
	intT := m.arithmeticPlugin.fields[1]
	locals := [2]astParameterRecord{{Kind: ASTParameterAST, ast: A}, {Kind: ASTParameterAST, ast: boolT}}
	predA, e := m.arraySortCore(0, astArrayParameters{locals: &locals})
	if e != nil {
		return e
	}
	p.signatures = make([]*astSequenceSignature, 44)
	type row struct {
		index      uint32
		line       int
		name       string
		parameters uint32
		size       int
		domain     [3]astNodeRef
		rangeSort  astNodeRef
	}
	rows := [...]row{
		{0, 530, "seq.unit", 1, 1, [3]astNodeRef{A}, seqA},
		{1, 531, "seq.empty", 1, 0, [3]astNodeRef{}, seqA},
		{2, 532, "seq.++", 1, 2, [3]astNodeRef{seqA, seqA}, seqA},
		{3, 533, "seq.prefixof", 1, 2, [3]astNodeRef{seqA, seqA}, boolT},
		{4, 534, "seq.suffixof", 1, 2, [3]astNodeRef{seqA, seqA}, boolT},
		{5, 535, "seq.contains", 1, 2, [3]astNodeRef{seqA, seqA}, boolT},
		{6, 536, "seq.extract", 1, 3, [3]astNodeRef{seqA, intT, intT}, seqA},
		{7, 537, "seq.replace", 1, 3, [3]astNodeRef{seqA, seqA, seqA}, seqA},
		{10, 538, "seq.indexof", 1, 3, [3]astNodeRef{seqA, seqA, intT}, intT},
		{8, 539, "seq.at", 1, 2, [3]astNodeRef{seqA, intT}, seqA},
		{9, 540, "seq.len", 1, 1, [3]astNodeRef{seqA}, intT},
		{13, 541, "re.+", 1, 1, [3]astNodeRef{reA}, reA},
		{14, 542, "re.*", 1, 1, [3]astNodeRef{reA}, reA},
		{15, 543, "re.opt", 1, 1, [3]astNodeRef{reA}, reA},
		{16, 544, "re.range", 1, 2, [3]astNodeRef{seqA, seqA}, reA},
		{17, 545, "re.++", 1, 2, [3]astNodeRef{reA, reA}, reA},
		{18, 546, "re.union", 1, 2, [3]astNodeRef{reA, reA}, reA},
		{19, 547, "re.inter", 1, 2, [3]astNodeRef{reA, reA}, reA},
		{20, 548, "re.loop", 1, 1, [3]astNodeRef{reA}, reA},
		{21, 549, "re.complement", 1, 1, [3]astNodeRef{reA}, reA},
		{22, 550, "re.empty", 1, 0, [3]astNodeRef{}, reA},
		{23, 551, "re.all", 1, 0, [3]astNodeRef{}, reA},
		{24, 570, "re.allchar", 1, 0, [3]astNodeRef{}, reA},
		{25, 553, "re.of.pred", 1, 1, [3]astNodeRef{predA}, reA},
		{11, 554, "seq.to.re", 1, 1, [3]astNodeRef{seqA}, reA},
		{12, 555, "seq.in.re", 1, 2, [3]astNodeRef{seqA, reA}, boolT},
		{39, 557, "str.indexof", 0, 3, [3]astNodeRef{strT, strT, intT}, intT},
		{29, 558, "str.replace", 0, 3, [3]astNodeRef{strT, strT, strT}, strT},
		{27, 559, "int.to.str", 0, 1, [3]astNodeRef{intT}, strT},
		{28, 560, "str.to.int", 0, 1, [3]astNodeRef{strT}, intT},
		{30, 561, "str.++", 1, 2, [3]astNodeRef{strT, strT}, strT},
		{31, 562, "str.len", 0, 1, [3]astNodeRef{strT}, intT},
		{32, 563, "str.contains", 0, 2, [3]astNodeRef{strT, strT}, boolT},
		{37, 564, "str.at", 0, 2, [3]astNodeRef{strT, intT, intT}, strT},
		{33, 565, "str.prefixof", 0, 2, [3]astNodeRef{strT, strT}, boolT},
		{34, 566, "str.suffixof", 0, 2, [3]astNodeRef{strT, strT}, boolT},
		{35, 567, "str.in.re", 0, 2, [3]astNodeRef{strT, reT}, boolT},
		{36, 568, "str.to.re", 0, 1, [3]astNodeRef{strT}, reT},
		{40, 569, "re.nostr", 0, 0, [3]astNodeRef{}, reT},
		{41, 570, "re.allchar", 0, 0, [3]astNodeRef{}, reT},
		{38, 571, "str.substr", 0, 3, [3]astNodeRef{strT, intT, intT}, strT},
		{43, 572, "_re.unroll", 0, 2, [3]astNodeRef{reT, intT}, strT},
	}
	for _, r := range rows {
		sig, e := p.newSignature(r.name, r.parameters, r.domain[:r.size], r.rangeSort, r.line)
		if e != nil {
			return e
		}
		p.signatures[r.index] = sig // only completed signature construction
	}
	return nil
}

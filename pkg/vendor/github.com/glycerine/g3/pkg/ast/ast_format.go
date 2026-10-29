package ast

// Source: src/ast/format.cpp:66-101,147-212; src/ast/format.h:29-66.

import (
	"math"
	"strconv"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

const (
	ASTFormatNil int32 = iota
	ASTFormatString
	ASTFormatIndent
	ASTFormatCompose
	ASTFormatChoice
	ASTFormatLineBreak
	ASTFormatLineBreakExt
)

// These are the actual source stack parameter/info values, embedded in the
// charged cold manager. Tokens are readable only in this exact active epoch.
// registerNode/copyInfo creates the real owned copies at source publication.
type astFormatLexical struct {
	secondParameter astParameterRecord
	localPair       bool
	info            astInfoRecord
	parameter       astParameterRecord
	borrowed        []ASTParameter
	epoch           uint32
	active          bool
}

func (m *ASTManager) beginFormatInfo(sort bool, id FamilyID, kind int32, parameter *astParameterRecord) error {
	f := &m.formatLexical
	if f.active || f.epoch == math.MaxUint32/2 {
		return ErrASTPending
	}
	f.epoch++
	f.active = true
	f.info = astInfoRecord{isSort: sort, live: true, lexical: true, family: id, kind: kind}
	if parameter != nil {
		f.parameter = *parameter
		f.parameter.Live = true
		f.info.count = 1
	}
	return nil
}
func (m *ASTManager) endFormatInfo() {
	m.formatLexical.localPair = false
	m.formatLexical.secondParameter = astParameterRecord{}
	m.formatLexical.borrowed = nil
	m.formatLexical.active = false
	m.formatLexical.info = astInfoRecord{}
	m.formatLexical.parameter = astParameterRecord{}
}
func (m *ASTManager) formatSortCreate(name usymbol.Symbol, id FamilyID) (astNodeRef, error) {
	if e := m.beginFormatInfo(true, id, 0, nil); e != nil {
		return astNodeRef{}, e
	}
	defer m.endFormatInfo()
	return m.mkSort(name, ASTSortInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
}
func (m *ASTManager) formatDecl(k int32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	p := m.formatPlugin
	if p == nil || !p.published {
		return astNodeRef{}, ErrASTHandle
	}
	if k < ASTFormatNil || k > ASTFormatLineBreakExt {
		return astNodeRef{}, nil
	}
	// The source OP_STRING/INDENT/EXT copy all actually passed parameters. Normal
	// primitives pass the one concrete lexical parameter below; this engine also
	// accepts ordinary parameter arrays through existing family overloads.
	if k == ASTFormatString || k == ASTFormatIndent || k == ASTFormatLineBreakExt {
		if len(ps) == 1 && ps[0].slot == math.MaxUint32-1 && ps[0].origin == m.origin {
			if !m.formatLexical.active || ps[0].generation != m.formatLexical.epoch {
				return astNodeRef{}, ErrASTHandle
			}
			return m.mkFuncDecl(p.names[k], domain, m.external(p.sort), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
		}
		return m.pluginDecl(p.names[k], p.family, k, ps, domain, p.sort, ASTFuncFlags{}, false)
	}
	if e := m.beginFormatInfo(false, p.family, k, nil); e != nil {
		return astNodeRef{}, e
	}
	defer m.endFormatInfo()
	return m.mkFuncDecl(p.names[k], domain, m.external(p.sort), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
}
func (m *ASTManager) formatApp(k int32, parameter *astParameterRecord, args []ASTHandle) (astNodeRef, error) {
	id, e := m.formatFamilyID()
	if e != nil {
		return astNodeRef{}, e
	}
	if parameter == nil {
		return m.mkFamilyApp(id, k, nil, args)
	}
	if e = m.beginFormatInfo(false, id, k, parameter); e != nil {
		return astNodeRef{}, e
	}
	defer m.endFormatInfo()
	p := [1]ASTParameter{{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch}}
	return m.mkFamilyApp(id, k, p[:], args)
}
func (m *ASTManager) lockFormat() (*ASTManager, error) {
	f, e := m.GetFormatManager()
	if e != nil {
		return nil, e
	}
	if e = f.lock(); e != nil {
		return nil, e
	}
	f.formatExecuting = true
	return f, nil
}
func (m *ASTManager) unlockFormat() { m.formatExecuting = false; m.mu.Unlock() }
func (m *ASTManager) FormatSort() (ASTHandle, error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	_, e = f.formatFamilyID()
	if e != nil {
		return ASTHandle{}, e
	}
	return f.external(f.formatPlugin.sort), nil
}
func (m *ASTManager) FormatString(s string) (ASTHandle, error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	name, e := f.Symbols.Intern(s)
	if e != nil {
		return ASTHandle{}, e
	}
	p := astParameterRecord{Kind: ASTParameterSymbol, symbol: name}
	r, e := f.formatApp(ASTFormatString, &p, nil)
	return f.external(r), e
}
func (m *ASTManager) FormatInt(i int32) (ASTHandle, error) {
	// Source sprintf has a local 128-byte buffer and disregards ostream flags.
	var buffer [128]byte
	b := strconv.AppendInt(buffer[:0], int64(i), 10)
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	name, e := f.Symbols.InternBytes(b)
	if e != nil {
		return ASTHandle{}, e
	}
	p := astParameterRecord{Kind: ASTParameterSymbol, symbol: name}
	r, e := f.formatApp(ASTFormatString, &p, nil)
	return f.external(r), e
}
func (m *ASTManager) FormatUnsigned(i uint32) (ASTHandle, error) {
	var buffer [128]byte
	b := strconv.AppendUint(buffer[:0], uint64(i), 10)
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	name, e := f.Symbols.InternBytes(b)
	if e != nil {
		return ASTHandle{}, e
	}
	p := astParameterRecord{Kind: ASTParameterSymbol, symbol: name}
	r, e := f.formatApp(ASTFormatString, &p, nil)
	return f.external(r), e
}
func (m *ASTManager) FormatIndent(i uint32, h ASTHandle) (ASTHandle, error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	p := astParameterRecord{Kind: ASTParameterInt, integer: int32(i)}
	args := [1]ASTHandle{h}
	r, e := f.formatApp(ASTFormatIndent, &p, args[:])
	return f.external(r), e
}
func (m *ASTManager) FormatLineBreak() (ASTHandle, error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	r, e := f.formatApp(ASTFormatLineBreak, nil, nil)
	return f.external(r), e
}
func (m *ASTManager) FormatNil() (ASTHandle, error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	r, e := f.formatApp(ASTFormatNil, nil, nil)
	return f.external(r), e
}
func (m *ASTManager) FormatChoice(a, b ASTHandle) (ASTHandle, error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	args := [2]ASTHandle{a, b}
	r, e := f.formatApp(ASTFormatChoice, nil, args[:])
	return f.external(r), e
}
func (m *ASTManager) FormatCompose(args []ASTHandle) (ASTHandle, error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	r, e := f.formatApp(ASTFormatCompose, nil, args)
	return f.external(r), e
}

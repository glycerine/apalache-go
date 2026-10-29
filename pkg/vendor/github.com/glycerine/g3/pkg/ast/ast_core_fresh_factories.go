package ast

// Source: src/ast/ast.cpp:1354,2213-2238; src/ast/ast.h:1811-1825.

import (
	"bytes"
	"math"
	"strconv"
	"strings"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// The special token denotes only this manager's active lexical info object.
// registerNode/copyInfo creates the real owned info before node publication.
func (m *ASTManager) mkFreshFuncDecl(prefix, suffix usymbol.Symbol, domain []ASTHandle, sort ASTHandle) (astNodeRef, error) {
	if m.freshInfoActive || m.freshInfoEpoch == math.MaxUint32 {
		return astNodeRef{}, ErrASTPending
	}
	m.freshInfoEpoch++
	m.freshInfo = astInfoRecord{live: true, family: NullFamilyID, kind: -1, flags: ASTFuncFlags{Skolem: true}}
	m.freshInfoActive = true
	defer func() { m.freshInfoActive = false; m.freshInfo = astInfoRecord{} }()
	info := ASTFuncDeclInfo{m.origin, math.MaxUint32, m.freshInfoEpoch}
	var d astNodeRef
	var e error
	if prefix == (usymbol.Symbol{}) && suffix == (usymbol.Symbol{}) {
		d, e = m.mkFuncDeclInfo(usymbol.Numerical(m.freshID), domain, sort, info, false)
	} else {
		d, e = m.freshNamedDecl(prefix, suffix, domain, sort, info)
	}
	if e != nil {
		return astNodeRef{}, e
	}
	m.freshID++
	if ASTSourceDebug {
		p := m.declAt(d.slot).info
		if p == (ASTFuncDeclInfo{}) {
			return astNodeRef{}, ErrASTDomain
		}
		v, e := m.funcInfo(p)
		if e != nil {
			return astNodeRef{}, e
		}
		if !v.flags.Skolem {
			return astNodeRef{}, ErrASTDomain
		}
	}
	return d, nil
}

func (m *ASTManager) MkFreshFuncDecl(prefix, suffix usymbol.Symbol, domain []ASTHandle, sort ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.mkFreshFuncDecl(prefix, suffix, domain, sort)
	return m.external(r), e
}

// MkFreshConst translates the char-prefix overload's symbol construction before
// the fresh declaration and its later zero-argument application construction.
func (m *ASTManager) MkFreshConst(prefix string, sort ASTHandle) (ASTHandle, error) {
	return m.MkFreshConstNullable(&prefix, sort)
}

// MkFreshConstNullable retains the source symbol(nullptr) constructor shortcut.
func (m *ASTManager) MkFreshConstNullable(prefix *string, sort ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	p, e := m.Symbols.InternNullable(prefix)
	if e != nil {
		return ASTHandle{}, e
	}
	d, e := m.mkFreshFuncDecl(p, usymbol.Symbol{}, nil, sort)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e := m.mkApp(d, nil)
	return m.external(r), e
}

// MkFreshVarName is the source mk_fresh_var_name scalar used when pulling
// binders with repeated names. The numeric suffix consumes the same fresh_id
// sequence as fresh declarations only after the symbol is interned.
func (m *ASTManager) MkFreshVarName(prefix *string) (usymbol.Symbol, error) {
	if e := m.lock(); e != nil {
		return usymbol.Symbol{}, e
	}
	defer m.mu.Unlock()
	stem := "var"
	if prefix != nil {
		stem = *prefix
		if i := strings.IndexByte(stem, 0); i >= 0 {
			stem = stem[:i]
		}
	}
	name, e := m.Symbols.Intern(stem + "!" + strconv.FormatUint(uint64(m.freshID), 10))
	if e != nil {
		return usymbol.Symbol{}, e
	}
	m.freshID++
	return name, nil
}

// The native output is interned before fresh_id publication.
func (m *ASTManager) freshNamedDecl(prefix, suffix usymbol.Symbol, domain []ASTHandle, sort ASTHandle, info ASTFuncDeclInfo) (astNodeRef, error) {
	text, e := m.Symbols.Display(prefix)
	if e != nil {
		return astNodeRef{}, e
	}
	var buffer bytes.Buffer
	buffer.WriteString(text)
	if prefix == (usymbol.Symbol{}) {
		buffer.WriteString("sk")
	}
	buffer.WriteByte('!')
	if suffix != (usymbol.Symbol{}) {
		text, e = m.Symbols.Display(suffix)
		if e != nil {
			return astNodeRef{}, e
		}
		buffer.WriteString(text)
		buffer.WriteByte('!')
	}
	buffer.WriteString(strconv.FormatInt(int64(int32(m.freshID)), 10))
	name, e := m.Symbols.InternBytes(buffer.Bytes())
	if e != nil {
		return astNodeRef{}, e
	}
	return m.mkFuncDeclInfo(name, domain, sort, info, false)
}

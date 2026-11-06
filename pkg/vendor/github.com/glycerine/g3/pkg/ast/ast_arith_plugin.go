package ast

// Source: src/ast/arith_decl_plugin.cpp:115-332; src/ast/arith_decl_plugin.h:81-145; src/ast/ast.cpp:1635-1638.

import (
	"reflect"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Actual cold new-expression owner. The pointer-free field array follows
// finalize's source member order, not the different set_manager action order.
// The source numeral cache vectors and lazy m_aw start empty.

type astArithmeticPluginOwner struct {
	numeral                    astArithNumeralFrame
	algebraic                  *astAlgebraicNumbersWrapper
	numeralSerial              uint64
	small                      [2][]astNodeRef // owning Int, Real numeral caches
	convertIntNumerals         bool
	acquired                   [46]bool // exact successful INC leaves, distinct from published raw fields
	next                       *astArithmeticPluginOwner
	self                       *astArithmeticPluginOwner
	owner                      *ASTManager
	names                      [3]usymbol.Symbol
	fields                     [46]astNodeRef
	family                     FamilyID
	header                     uint64
	completed                  uint32
	published                  bool
	memoryTraceLive            bool
	bodyCause, destructorCause error
}

func (m *ASTManager) arithmeticPluginBytes() uint64 {
	var n uint64
	if m.arithmeticPlugin != nil {
		n += m.arithmeticPlugin.header
	}
	for p := m.arithmeticPluginFailed; p != nil; p = p.next {
		n += p.header
	}
	return n
}
func (m *ASTManager) releaseArithmeticPluginHeaders() {
	if p := m.arithmeticPlugin; p != nil {
		if p.numeral.active || p.algebraic != nil {
			return
		}
		p.small[1] = nil
		p.small[0] = nil
		p.traceArithmeticPluginDeallocated()
		m.BudgetField.Release(p.header)
		p.header = 0
		m.arithmeticPlugin = nil
	}
	if m.sourceClosing {
		return
	}
	for p := m.arithmeticPluginFailed; p != nil; {
		next := p.next
		m.BudgetField.Release(p.header)
		p.header = 0
		p = next
	}
	m.arithmeticPluginFailed = nil
}
func (m *ASTManager) finalizeArithmeticPlugin() error {
	p := m.arithmeticPlugin
	if p == nil {
		return nil
	}
	if e := p.numeral.destroy(m, !m.sourceClosing); e != nil {
		return e
	}
	for j := range p.fields {
		if p.fields[j].slot == 0 {
			continue
		}
		if !m.sourceClosing && !p.acquired[j] {
			p.fields[j] = astNodeRef{}
			continue
		}
		consumed, e := m.decNode(p.fields[j])
		if consumed {
			p.fields[j] = astNodeRef{}
			p.acquired[j] = false
		}
		if e != nil {
			if p.destructorCause == nil {
				p.destructorCause = e
			}
			return e
		}
	}
	for j := range p.small {
		v := &p.small[j]
		for i, r := range *v {
			if r.slot == 0 {
				continue
			}
			consumed, e := m.decNode(r)
			if consumed {
				(*v)[i] = astNodeRef{}
			}
			if e != nil {
				return e
			}
		}
	}
	if p.algebraic != nil {
		if e := p.algebraic.close(); e != nil {
			return e
		}
		p.algebraic = nil
	}
	return nil
}
func (p *astArithmeticPluginOwner) sortField(index int, literal string, kind int32) error {
	m := p.owner
	name, e := m.Symbols.Intern(literal)
	if e != nil {
		return e
	}
	if e := m.beginFormatInfo(true, p.family, kind, nil); e != nil {
		return e
	}
	r, e := m.mkSort(name, ASTSortInfo{m.origin, ^uint32(0) - 1, m.formatLexical.epoch})
	m.endFormatInfo()
	p.fields[index] = r
	if e != nil {
		return e
	}
	if e = m.incNode(r); e != nil {
		return e
	}
	p.acquired[index] = true
	p.completed++
	return nil
}
func (p *astArithmeticPluginOwner) makeDeclaration(name string, kind int32, domain []astNodeRef, result astNodeRef, flags ASTFuncFlags) (astNodeRef, error) {
	return p.owner.makePluginDeclaration(p.family, name, kind, domain, result, flags)
}
func (p *astArithmeticPluginOwner) declaration(index int, name string, kind int32, domain []astNodeRef, result astNodeRef, flags ASTFuncFlags) error {
	r, e := p.makeDeclaration(name, kind, domain, result, flags)
	p.fields[index] = r
	if e != nil {
		return e
	}
	if e = p.owner.incNode(r); e != nil {
		return e
	}
	p.acquired[index] = true
	p.completed++
	return nil
}
func (p *astArithmeticPluginOwner) constant(index int, name string, kind int32, result astNodeRef) error {
	d, e := p.makeDeclaration(name, kind, nil, result, ASTFuncFlags{})
	if e != nil {
		return e
	}
	r, e := p.owner.mkApp(d, nil)
	p.fields[index] = r
	if e != nil {
		return e
	}
	if e = p.owner.incNode(r); e != nil {
		return e
	}
	p.acquired[index] = true
	p.completed++
	return nil
}
func (p *astArithmeticPluginOwner) setManager() error {
	m := p.owner
	var e error
	if e = p.sortField(0, "Real", 0); e != nil {
		return e
	}
	if e = p.sortField(1, "Int", 1); e != nil {
		return e
	}
	r, i, b := p.fields[0], p.fields[1], m.basic.boolSort
	if e = p.declaration(2, "<=", 2, []astNodeRef{r, r}, b, ASTFuncFlags{Chainable: true}); e != nil {
		return e
	}
	if e = p.declaration(3, ">=", 3, []astNodeRef{r, r}, b, ASTFuncFlags{Chainable: true}); e != nil {
		return e
	}
	if e = p.declaration(4, "<", 4, []astNodeRef{r, r}, b, ASTFuncFlags{Chainable: true}); e != nil {
		return e
	}
	if e = p.declaration(5, ">", 5, []astNodeRef{r, r}, b, ASTFuncFlags{Chainable: true}); e != nil {
		return e
	}
	if e = p.declaration(11, "<=", 2, []astNodeRef{i, i}, b, ASTFuncFlags{Chainable: true}); e != nil {
		return e
	}
	if e = p.declaration(12, ">=", 3, []astNodeRef{i, i}, b, ASTFuncFlags{Chainable: true}); e != nil {
		return e
	}
	if e = p.declaration(13, "<", 4, []astNodeRef{i, i}, b, ASTFuncFlags{Chainable: true}); e != nil {
		return e
	}
	if e = p.declaration(14, ">", 5, []astNodeRef{i, i}, b, ASTFuncFlags{Chainable: true}); e != nil {
		return e
	}
	if e = p.declaration(6, "+", 6, []astNodeRef{r, r}, r, ASTFuncFlags{LeftAssociative: true, RightAssociative: true, FlatAssociative: true, Commutative: true}); e != nil {
		return e
	}
	if e = p.declaration(7, "-", 7, []astNodeRef{r, r}, r, ASTFuncFlags{LeftAssociative: true}); e != nil {
		return e
	}
	if e = p.declaration(9, "*", 9, []astNodeRef{r, r}, r, ASTFuncFlags{LeftAssociative: true, RightAssociative: true, FlatAssociative: true, Commutative: true}); e != nil {
		return e
	}
	if e = p.declaration(10, "/", 10, []astNodeRef{r, r}, r, ASTFuncFlags{LeftAssociative: true}); e != nil {
		return e
	}
	if e = p.declaration(8, "-", 8, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(15, "+", 6, []astNodeRef{i, i}, i, ASTFuncFlags{LeftAssociative: true, RightAssociative: true, FlatAssociative: true, Commutative: true}); e != nil {
		return e
	}
	if e = p.declaration(16, "-", 7, []astNodeRef{i, i}, i, ASTFuncFlags{LeftAssociative: true}); e != nil {
		return e
	}
	if e = p.declaration(18, "*", 9, []astNodeRef{i, i}, i, ASTFuncFlags{LeftAssociative: true, RightAssociative: true, FlatAssociative: true, Commutative: true}); e != nil {
		return e
	}
	if e = p.declaration(19, "div", 11, []astNodeRef{i, i}, i, ASTFuncFlags{LeftAssociative: true}); e != nil {
		return e
	}
	if e = p.declaration(21, "rem", 12, []astNodeRef{i, i}, i, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(20, "mod", 13, []astNodeRef{i, i}, i, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(17, "-", 8, []astNodeRef{i}, i, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(22, "to_real", 14, []astNodeRef{i}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(23, "to_int", 15, []astNodeRef{r}, i, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(24, "is_int", 16, []astNodeRef{r}, b, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(26, "^", 18, []astNodeRef{r, r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(25, "^", 18, []astNodeRef{i, i}, i, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(27, "abs", 17, []astNodeRef{i}, i, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(28, "abs", 17, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(29, "sin", 19, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(30, "cos", 20, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(31, "tan", 21, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(32, "asin", 22, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(33, "acos", 23, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(34, "atan", 24, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(35, "sinh", 25, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(36, "cosh", 26, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(37, "tanh", 27, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(38, "asinh", 28, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(39, "acosh", 29, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(40, "atanh", 30, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.constant(41, "pi", 31, r); e != nil {
		return e
	}
	if e = p.constant(42, "euler", 32, r); e != nil {
		return e
	}
	if e = p.declaration(43, "neg-root", 33, []astNodeRef{r, r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(44, "asin-u", 34, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	if e = p.declaration(45, "acos-u", 35, []astNodeRef{r}, r, ASTFuncFlags{}); e != nil {
		return e
	}
	return nil
}
func (m *ASTManager) registerArithmeticPlugin() (FamilyID, error) {
	name, e := m.Symbols.Intern("arith")
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
	bytes := uint64(reflect.TypeOf(astArithmeticPluginOwner{}).Size())
	if e = m.BudgetField.Reserve(bytes); e != nil {
		return NullFamilyID, e
	}
	p := &astArithmeticPluginOwner{owner: m, header: bytes}
	p.self = p
	p.traceArithmeticPluginAllocated()
	p.next = m.arithmeticPluginFailed
	m.arithmeticPluginFailed = p
	for j, s := range [...]string{"Int", "Real", "RootObject"} {
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
	if e = m.setPlugin(id, astArithmeticPlugin); e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	m.arithmeticPluginFailed = p.next
	p.next = nil
	m.arithmeticPlugin = p
	p.published = true
	p.family = id
	if e = p.setManager(); e != nil {
		p.bodyCause = e
		return NullFamilyID, e
	}
	return id, nil
}
func (m *ASTManager) RegisterArithmeticPlugin() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	m.formatExecuting = true
	defer func() { m.formatExecuting = false }()
	return m.registerArithmeticPlugin()
}

func (m *ASTManager) makePluginDeclaration(family FamilyID, name string, kind int32, domain []astNodeRef, result astNodeRef, flags ASTFuncFlags) (astNodeRef, error) {
	s, e := m.Symbols.Intern(name)
	if e != nil {
		return astNodeRef{}, e
	}
	if e = m.beginFormatInfo(false, family, kind, nil); e != nil {
		return astNodeRef{}, e
	}
	m.formatLexical.info.flags = flags
	var args [3]ASTHandle
	for j, r := range domain {
		args[j] = m.external(r)
	}
	d, e := m.mkFuncDecl(s, args[:len(domain)], m.external(result), ASTFuncDeclInfo{m.origin, ^uint32(0) - 1, m.formatLexical.epoch})
	m.endFormatInfo()
	return d, e
}

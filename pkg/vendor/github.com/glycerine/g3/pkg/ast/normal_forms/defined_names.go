package normal_forms

// Source: src/ast/normal_forms/defined_names.cpp:37-320;
// src/ast/normal_forms/defined_names.h:26-86.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type definedNameEntry struct {
	key               ast.ASTHandle
	expr, name, proof ast.ASTRef
}

type definedNameTable struct {
	owner    *DefinedNames
	positive bool
	byExpr   map[ast.ASTHandle]int
	entries  []definedNameEntry
	limits   []int
}

// DefinedNames owns the ordinary and positive-occurrence names introduced for
// expressions. Its caches support the same paired push/pop discipline as the
// native defined_names object.
type DefinedNames struct {
	self     *DefinedNames
	manager  *ast.ASTManager
	prefix   usymbol.Symbol
	closed   bool
	regular  definedNameTable
	positive definedNameTable
}

// NewDefinedNames uses the source default fresh-name prefix, "z3name".
func NewDefinedNames(m *ast.ASTManager) (*DefinedNames, error) {
	prefix := "z3name"
	return NewDefinedNamesWithPrefix(m, &prefix)
}

// NewDefinedNamesWithPrefix preserves the source nullable prefix. A nil prefix
// selects the AST manager's unnamed fresh-declaration spelling.
func NewDefinedNamesWithPrefix(m *ast.ASTManager, prefix *string) (*DefinedNames, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	p, e := m.Symbols.InternNullable(prefix)
	if e != nil {
		return nil, e
	}
	d := &DefinedNames{manager: m, prefix: p}
	d.self = d
	d.regular = definedNameTable{owner: d, byExpr: make(map[ast.ASTHandle]int)}
	d.positive = definedNameTable{owner: d, positive: true, byExpr: make(map[ast.ASTHandle]int)}
	return d, nil
}

func (d *DefinedNames) valid() error {
	if d == nil || d.self != d || d.manager == nil || d.closed {
		return ast.ErrASTHandle
	}
	return nil
}

// MkName publishes the cached or newly-created ordinary name and its source
// definition/proofs into caller-owned AST references.
func (d *DefinedNames) MkName(e ast.ASTHandle, newDef, newDefProof, name, proof ast.ASTRef) (bool, error) {
	if x := d.valid(); x != nil {
		return false, x
	}
	return d.regular.mkName(e, newDef, newDefProof, name, proof)
}

// MkPositiveName is the positive-occurrence counterpart of MkName.
func (d *DefinedNames) MkPositiveName(e ast.ASTHandle, newDef, newDefProof, name, proof ast.ASTRef) (bool, error) {
	if x := d.valid(); x != nil {
		return false, x
	}
	return d.positive.mkName(e, newDef, newDefProof, name, proof)
}

func (t *definedNameTable) mkName(e ast.ASTHandle, newDef, newDefProof, name, proof ast.ASTRef) (_ bool, result error) {
	m := t.owner.manager
	if i, found := t.byExpr[e]; found {
		n, x := m.RefGet(t.entries[i].name)
		if x != nil {
			return false, x
		}
		if x = m.SetRef(name, n); x != nil {
			return false, x
		}
		enabled, x := m.ProofsEnabled()
		if x != nil {
			return false, x
		}
		if enabled {
			p, x := m.RefGet(t.entries[i].proof)
			if x != nil {
				return false, x
			}
			if x = m.SetRef(proof, p); x != nil {
				return false, x
			}
		}
		return false, nil
	}

	var held []ast.ASTRef
	hold := func(h ast.ASTHandle) (ast.ASTHandle, error) {
		r, x := m.NewRef(h)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		held = append(held, r)
		return h, nil
	}
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&held[i]))
		}
	}()

	n, sorts, names, x := t.genName(e, hold)
	if x != nil {
		return false, x
	}
	for i, j := 0, len(sorts)-1; i < j; i, j = i+1, j-1 {
		sorts[i], sorts[j] = sorts[j], sorts[i]
		names[i], names[j] = names[j], names[i]
	}
	definition, x := t.mkDefinition(e, n, sorts, names, hold)
	if x != nil {
		return false, x
	}
	enabled, x := m.ProofsEnabled()
	if x != nil {
		return false, x
	}
	var definitionProof, applyProof ast.ASTHandle
	if enabled {
		definitionProof, x = m.MkProofDefIntro(definition)
		if x != nil {
			return false, x
		}
		if _, x = hold(definitionProof); x != nil {
			return false, x
		}
		applyProof, x = m.MkProofApplyDef(e, n, definitionProof)
		if x != nil {
			return false, x
		}
		if _, x = hold(applyProof); x != nil {
			return false, x
		}
	}

	entry := definedNameEntry{key: e}
	if entry.expr, x = m.NewRef(e); x != nil {
		return false, x
	}
	if entry.name, x = m.NewRef(n); x != nil {
		result = errors.Join(result, m.DelRef(&entry.expr))
		return false, errors.Join(x, result)
	}
	if enabled {
		if entry.proof, x = m.NewRef(applyProof); x != nil {
			result = errors.Join(result, m.DelRef(&entry.name), m.DelRef(&entry.expr))
			return false, errors.Join(x, result)
		}
	}
	index := len(t.entries)
	t.entries = append(t.entries, entry)
	t.byExpr[e] = index
	if x = m.SetRef(name, n); x != nil {
		return true, x
	}
	if x = m.SetRef(newDef, definition); x != nil {
		return true, x
	}
	if enabled {
		if x = m.SetRef(newDefProof, definitionProof); x != nil {
			return true, x
		}
		if x = m.SetRef(proof, applyProof); x != nil {
			return true, x
		}
	}
	return true, nil
}

func (t *definedNameTable) genName(e ast.ASTHandle, hold func(ast.ASTHandle) (ast.ASTHandle, error)) (ast.ASTHandle, []ast.ASTHandle, []usymbol.Symbol, error) {
	m := t.owner.manager
	used, x := ast.NewASTUsedVars(m)
	if x != nil {
		return ast.ASTHandle{}, nil, nil, x
	}
	if x = used.Collect(e); x != nil {
		return ast.ASTHandle{}, nil, nil, x
	}
	boolSort, x := m.MkBoolSort()
	if x != nil {
		return ast.ASTHandle{}, nil, nil, x
	}
	count := used.MaxFoundVarIndexPlusOne()
	sorts := make([]ast.ASTHandle, 0, count)
	names := make([]usymbol.Symbol, 0, count)
	domain := make([]ast.ASTHandle, 0, used.NumVars())
	args := make([]ast.ASTHandle, 0, used.NumVars())
	for i := uint32(0); i < count; i++ {
		s := used.Contains(i)
		if s.IsNull() {
			sorts = append(sorts, boolSort)
		} else {
			domain = append(domain, s)
			v, x := m.MkVar(i, s)
			if x != nil {
				return ast.ASTHandle{}, nil, nil, x
			}
			if _, x = hold(v); x != nil {
				return ast.ASTHandle{}, nil, nil, x
			}
			args = append(args, v)
			sorts = append(sorts, s)
		}
		names = append(names, usymbol.Numerical(i))
	}
	rangeSort, x := m.GetSort(e)
	if x != nil {
		return ast.ASTHandle{}, nil, nil, x
	}
	decl, x := m.MkFreshFuncDecl(t.owner.prefix, usymbol.Symbol{}, domain, rangeSort)
	if x != nil {
		return ast.ASTHandle{}, nil, nil, x
	}
	if _, x = hold(decl); x != nil {
		return ast.ASTHandle{}, nil, nil, x
	}
	name, x := m.MkApp(decl, args)
	if x != nil {
		return ast.ASTHandle{}, nil, nil, x
	}
	if _, x = hold(name); x != nil {
		return ast.ASTHandle{}, nil, nil, x
	}
	return name, sorts, names, nil
}

func (t *definedNameTable) boundVars(conjunct, name ast.ASTHandle, sorts []ast.ASTHandle, names []usymbol.Symbol, hold func(ast.ASTHandle) (ast.ASTHandle, error)) (ast.ASTHandle, error) {
	if len(sorts) == 0 {
		return conjunct, nil
	}
	m := t.owner.manager
	pattern, x := m.MkPattern([]ast.ASTHandle{name})
	if x != nil {
		return ast.ASTHandle{}, x
	}
	if _, x = hold(pattern); x != nil {
		return ast.ASTHandle{}, x
	}
	q, x := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: conjunct, Weight: 1, Patterns: []ast.ASTHandle{pattern}})
	if x != nil {
		return ast.ASTHandle{}, x
	}
	if _, x = hold(q); x != nil {
		return ast.ASTHandle{}, x
	}
	eliminated, x := rewriter.ElimUnusedVarsUFBV(m, q)
	if x != nil {
		return ast.ASTHandle{}, x
	}
	result, x := m.RefGet(eliminated)
	if x == nil {
		_, x = hold(result)
	}
	return result, errors.Join(x, m.DelRef(&eliminated))
}

func (t *definedNameTable) mkDefinition(e, name ast.ASTHandle, sorts []ast.ASTHandle, names []usymbol.Symbol, hold func(ast.ASTHandle) (ast.ASTHandle, error)) (ast.ASTHandle, error) {
	m := t.owner.manager
	if t.positive {
		notName, x := m.MkNot(name)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if _, x = hold(notName); x != nil {
			return ast.ASTHandle{}, x
		}
		first, x := m.MkOr(notName, e)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if _, x = hold(first); x != nil {
			return ast.ASTHandle{}, x
		}
		return t.boundVars(first, name, sorts, names, hold)
	}
	sort, x := m.GetSort(e)
	if x != nil {
		return ast.ASTHandle{}, x
	}
	boolSort, x := m.MkBoolSort()
	if x != nil {
		return ast.ASTHandle{}, x
	}
	var conjuncts [2]ast.ASTHandle
	if sort == boolSort {
		notName, x := m.MkNot(name)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if _, x = hold(notName); x != nil {
			return ast.ASTHandle{}, x
		}
		first, x := m.MkOr(notName, e)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if _, x = hold(first); x != nil {
			return ast.ASTHandle{}, x
		}
		conjuncts[0] = first
		notE, x := m.MkNot(e)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if _, x = hold(notE); x != nil {
			return ast.ASTHandle{}, x
		}
		conjuncts[1], x = m.MkOr(name, notE)
		if x != nil {
			return ast.ASTHandle{}, x
		}
	} else {
		kind, x := m.GetKind(e)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		isITE := false
		if kind == ast.ASTApp {
			family, y := m.GetFamilyID(e)
			if y != nil {
				return ast.ASTHandle{}, y
			}
			op, y := m.GetDeclKind(e)
			if y != nil {
				return ast.ASTHandle{}, y
			}
			isITE = family == 0 && op == ast.ASTOpIte
		}
		if !isITE {
			eq, y := m.MkEq(e, name)
			if y != nil {
				return ast.ASTHandle{}, y
			}
			if _, y = hold(eq); y != nil {
				return ast.ASTHandle{}, y
			}
			return t.boundVars(eq, name, sorts, names, hold)
		}
		condition, y := m.GetArg(e, 0)
		if y != nil {
			return ast.ASTHandle{}, y
		}
		thenPart, y := m.GetArg(e, 1)
		if y != nil {
			return ast.ASTHandle{}, y
		}
		elsePart, y := m.GetArg(e, 2)
		if y != nil {
			return ast.ASTHandle{}, y
		}
		notCondition, y := m.MkNot(condition)
		if y != nil {
			return ast.ASTHandle{}, y
		}
		if _, y = hold(notCondition); y != nil {
			return ast.ASTHandle{}, y
		}
		eqThen, y := m.MkEq(name, thenPart)
		if y != nil {
			return ast.ASTHandle{}, y
		}
		if _, y = hold(eqThen); y != nil {
			return ast.ASTHandle{}, y
		}
		conjuncts[0], y = m.MkOr(notCondition, eqThen)
		if y != nil {
			return ast.ASTHandle{}, y
		}
		eqElse, y := m.MkEq(name, elsePart)
		if y != nil {
			return ast.ASTHandle{}, y
		}
		if _, y = hold(eqElse); y != nil {
			return ast.ASTHandle{}, y
		}
		conjuncts[1], y = m.MkOr(condition, eqElse)
		if y != nil {
			return ast.ASTHandle{}, y
		}
	}
	for i := range conjuncts {
		if _, x = hold(conjuncts[i]); x != nil {
			return ast.ASTHandle{}, x
		}
		conjuncts[i], x = t.boundVars(conjuncts[i], name, sorts, names, hold)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if _, x = hold(conjuncts[i]); x != nil {
			return ast.ASTHandle{}, x
		}
	}
	definition, x := m.MkAnd(conjuncts[0], conjuncts[1])
	if x != nil {
		return ast.ASTHandle{}, x
	}
	if _, x = hold(definition); x != nil {
		return ast.ASTHandle{}, x
	}
	return definition, nil
}

func (d *DefinedNames) Push() error {
	if e := d.valid(); e != nil {
		return e
	}
	d.regular.limits = append(d.regular.limits, len(d.regular.entries))
	d.positive.limits = append(d.positive.limits, len(d.positive.entries))
	return nil
}

func (t *definedNameTable) releaseTo(size int) error {
	for len(t.entries) > size {
		i := len(t.entries) - 1
		entry := &t.entries[i]
		if entry.expr != (ast.ASTRef{}) {
			if e := t.owner.manager.DelRef(&entry.expr); e != nil {
				return e
			}
		}
		if entry.name != (ast.ASTRef{}) {
			if e := t.owner.manager.DelRef(&entry.name); e != nil {
				return e
			}
		}
		if entry.proof != (ast.ASTRef{}) {
			if e := t.owner.manager.DelRef(&entry.proof); e != nil {
				return e
			}
		}
		delete(t.byExpr, entry.key)
		t.entries = t.entries[:i]
	}
	return nil
}

func (d *DefinedNames) Pop(scopes uint32) error {
	if e := d.valid(); e != nil {
		return e
	}
	if uint64(scopes) > uint64(len(d.regular.limits)) || uint64(scopes) > uint64(len(d.positive.limits)) {
		return ast.ErrASTDomain
	}
	if scopes == 0 {
		return nil
	}
	regularLevel := len(d.regular.limits) - int(scopes)
	positiveLevel := len(d.positive.limits) - int(scopes)
	if e := d.regular.releaseTo(d.regular.limits[regularLevel]); e != nil {
		return e
	}
	if e := d.positive.releaseTo(d.positive.limits[positiveLevel]); e != nil {
		return e
	}
	d.regular.limits = d.regular.limits[:regularLevel]
	d.positive.limits = d.positive.limits[:positiveLevel]
	return nil
}

func (d *DefinedNames) Reset() error {
	if e := d.valid(); e != nil {
		return e
	}
	if e := d.regular.releaseTo(0); e != nil {
		return e
	}
	if e := d.positive.releaseTo(0); e != nil {
		return e
	}
	d.regular.limits = nil
	d.positive.limits = nil
	return nil
}

func (d *DefinedNames) NumNames() (uint32, error) {
	if e := d.valid(); e != nil {
		return 0, e
	}
	return uint32(len(d.regular.entries) + len(d.positive.entries)), nil
}

func (d *DefinedNames) NameDecl(index uint32) (ast.ASTHandle, error) {
	if e := d.valid(); e != nil {
		return ast.ASTHandle{}, e
	}
	entries := d.regular.entries
	if uint64(index) >= uint64(len(entries)) {
		index -= uint32(len(entries))
		entries = d.positive.entries
	}
	if uint64(index) >= uint64(len(entries)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	name, e := d.manager.RefGet(entries[index].name)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return d.manager.GetDecl(name)
}

func (d *DefinedNames) Close() error {
	if d == nil || d.self != d || d.manager == nil {
		return ast.ErrASTHandle
	}
	if d.closed {
		return nil
	}
	if e := d.Reset(); e != nil {
		return e
	}
	d.closed = true
	return nil
}

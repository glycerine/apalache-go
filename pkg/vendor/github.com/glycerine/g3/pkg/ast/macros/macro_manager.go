package macros

// Source: src/ast/macros/macro_manager.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type managerEntry struct {
	declaration, quantifier, proof ast.ASTHandle
	dependency                     ast.ASTDependency
}
type managerScope struct{ declarations, forbidden int }

// Manager owns installed macro formulas, optional proofs and dependencies.
type Manager struct {
	self          *Manager
	manager       *ast.ASTManager
	util          *Util
	dependencies  *ast.FuncDeclDependencies
	entries       []managerEntry
	byDeclaration map[ast.ASTHandle]int
	forbidden     []ast.ASTHandle
	forbiddenSet  map[ast.ASTHandle]struct{}
	scopes        []managerScope
	header        uint64
	closed        bool
}

func NewManager(m *ast.ASTManager) (owner *Manager, result error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(Manager{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	owner = &Manager{manager: m, header: n, byDeclaration: make(map[ast.ASTHandle]int), forbiddenSet: make(map[ast.ASTHandle]struct{})}
	owner.self = owner
	fail := func(err error) (*Manager, error) { return owner, errors.Join(err, owner.Close()) }
	var err error
	owner.util, err = NewUtil(m)
	if err != nil {
		return fail(err)
	}
	owner.dependencies, err = ast.NewFuncDeclDependencies(m)
	if err != nil {
		return fail(err)
	}
	return owner, nil
}

func (m *Manager) open() error {
	if m == nil || m.self != m || m.manager == nil || m.closed {
		return ast.ErrASTHandle
	}
	_, err := m.manager.ProofMode()
	return err
}
func (m *Manager) Util() *Util {
	if m == nil || m.closed || m.self != m {
		return nil
	}
	return m.util
}
func (m *Manager) HasMacros() bool { return m != nil && !m.closed && len(m.entries) != 0 }
func (m *Manager) NumMacros() int {
	if m == nil || m.closed {
		return 0
	}
	return len(m.entries)
}
func (m *Manager) FirstMacroLastLevel() int {
	if m == nil || m.closed || len(m.scopes) == 0 {
		return 0
	}
	return m.scopes[len(m.scopes)-1].declarations
}

func (m *Manager) PushScope() error {
	if err := m.open(); err != nil {
		return err
	}
	m.scopes = append(m.scopes, managerScope{len(m.entries), len(m.forbidden)})
	return nil
}

func (m *Manager) releaseEntry(e *managerEntry) (result error) {
	if !e.dependency.IsNull() {
		result = errors.Join(result, m.manager.DependencyDec(e.dependency))
		e.dependency = ast.ASTDependency{}
	}
	for _, h := range [...]ast.ASTHandle{e.proof, e.quantifier, e.declaration} {
		if !h.IsNull() {
			result = errors.Join(result, m.manager.DecRef(h))
		}
	}
	*e = managerEntry{}
	return result
}

func (m *Manager) restoreDeclarations(limit int) (result error) {
	for len(m.entries) > limit {
		i := len(m.entries) - 1
		e := &m.entries[i]
		delete(m.byDeclaration, e.declaration)
		result = errors.Join(result, m.dependencies.Erase(e.declaration), m.releaseEntry(e))
		m.entries = m.entries[:i]
	}
	return result
}
func (m *Manager) restoreForbidden(limit int) (result error) {
	for len(m.forbidden) > limit {
		i := len(m.forbidden) - 1
		d := m.forbidden[i]
		delete(m.forbiddenSet, d)
		result = errors.Join(result, m.manager.DecRef(d))
		m.forbidden = m.forbidden[:i]
	}
	m.publishForbidden()
	return result
}
func (m *Manager) publishForbidden() {
	values := make([]ast.ASTHandle, 0, len(m.forbiddenSet))
	for d := range m.forbiddenSet {
		values = append(values, d)
	}
	m.util.SetForbidden(values)
}

func (m *Manager) PopScope(count uint32) error {
	if err := m.open(); err != nil {
		return err
	}
	if uint64(count) > uint64(len(m.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(m.scopes) - int(count)
	if count == 0 {
		return nil
	}
	s := m.scopes[level]
	err := errors.Join(m.restoreDeclarations(s.declarations), m.restoreForbidden(s.forbidden))
	m.scopes = m.scopes[:level]
	return err
}

func (m *Manager) GetHeadDefinition(q, declaration ast.ASTHandle) (head, definition ast.ASTHandle, result error) {
	kind, err := m.manager.GetKind(q)
	if err != nil || kind != ast.ASTQuantifier {
		return head, definition, ast.ErrASTDomain
	}
	body, err := m.manager.GetQuantifierBody(q)
	if err != nil {
		return head, definition, err
	}
	fid, err := m.manager.GetFamilyID(body)
	if err != nil {
		return head, definition, err
	}
	op, err := m.manager.GetDeclKind(body)
	if err != nil {
		return head, definition, err
	}
	if fid != m.manager.BasicFamilyID() || op != ast.ASTOpEq && op != ast.ASTOpIff {
		return head, definition, ast.ErrASTDomain
	}
	lhs, err := m.manager.GetArg(body, 0)
	if err != nil {
		return head, definition, err
	}
	rhs, err := m.manager.GetArg(body, 1)
	if err != nil {
		return head, definition, err
	}
	ld, le := m.manager.GetDecl(lhs)
	rd, re := m.manager.GetDecl(rhs)
	left, right := le == nil && ld == declaration, re == nil && rd == declaration
	if left == right {
		return head, definition, ast.ErrASTDomain
	}
	if left {
		return lhs, rhs, nil
	}
	return rhs, lhs, nil
}

func (m *Manager) Insert(declaration, quantifier, proof ast.ASTHandle, dependency ast.ASTDependency) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	if _, found := m.byDeclaration[declaration]; found {
		return false, nil
	}
	_, definition, err := m.GetHeadDefinition(quantifier, declaration)
	if err != nil {
		return false, err
	}
	set := make(ast.FuncDeclSet)
	if err = m.dependencies.Collect(definition, set); err != nil {
		return false, err
	}
	yes, err := m.dependencies.Insert(declaration, set)
	if err != nil || !yes {
		return yes, err
	}
	proofs, err := m.manager.ProofsEnabled()
	if err != nil {
		_ = m.dependencies.Erase(declaration)
		return false, err
	}
	if !proofs {
		proof = ast.ASTHandle{}
	}
	values := [...]ast.ASTHandle{declaration, quantifier, proof}
	owned := 0
	for _, h := range values {
		if err = m.manager.IncRef(h); err != nil {
			for i := owned - 1; i >= 0; i-- {
				_ = m.manager.DecRef(values[i])
			}
			_ = m.dependencies.Erase(declaration)
			return false, err
		}
		owned++
	}
	if !dependency.IsNull() {
		if err = m.manager.DependencyInc(dependency); err != nil {
			for i := owned - 1; i >= 0; i-- {
				_ = m.manager.DecRef(values[i])
			}
			_ = m.dependencies.Erase(declaration)
			return false, err
		}
	}
	m.byDeclaration[declaration] = len(m.entries)
	m.entries = append(m.entries, managerEntry{declaration, quantifier, proof, dependency})
	return true, nil
}

func (m *Manager) MacroDeclaration(i int) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if i < 0 || i >= len(m.entries) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return m.entries[i].declaration, nil
}
func (m *Manager) MacroQuantifier(declaration ast.ASTHandle) (ast.ASTHandle, bool) {
	if m == nil || m.closed {
		return ast.ASTHandle{}, false
	}
	i, yes := m.byDeclaration[declaration]
	if !yes {
		return ast.ASTHandle{}, false
	}
	return m.entries[i].quantifier, true
}
func (m *Manager) MacroInterpretation(i int, out ast.ASTRef) (ast.ASTHandle, error) {
	d, err := m.MacroDeclaration(i)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	e := m.entries[i]
	head, def, err := m.GetHeadDefinition(e.quantifier, d)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	n, err := m.manager.GetNumDeclarations(e.quantifier)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = m.util.NormalizeExpr(head, n, def, out); err != nil {
		return ast.ASTHandle{}, err
	}
	return d, nil
}

func (m *Manager) MarkForbidden(expressions []ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	seen := make(map[ast.ASTHandle]struct{})
	stack := append([]ast.ASTHandle(nil), expressions...)
	for len(stack) != 0 {
		i := len(stack) - 1
		e := stack[i]
		stack = stack[:i]
		if _, yes := seen[e]; yes {
			continue
		}
		seen[e] = struct{}{}
		kind, err := m.manager.GetKind(e)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			n, e2 := m.manager.GetNumArgs(e)
			if e2 != nil {
				return e2
			}
			if n > 0 {
				d, e2 := m.manager.GetDecl(e)
				if e2 != nil {
					return e2
				}
				fid, e2 := m.manager.GetFamilyID(d)
				if e2 != nil {
					return e2
				}
				if fid == ast.NullFamilyID {
					if _, yes := m.forbiddenSet[d]; !yes {
						if e2 = m.manager.IncRef(d); e2 != nil {
							return e2
						}
						m.forbiddenSet[d] = struct{}{}
						m.forbidden = append(m.forbidden, d)
					}
				}
			}
			for j := n; j > 0; j-- {
				a, e2 := m.manager.GetArg(e, j-1)
				if e2 != nil {
					return e2
				}
				stack = append(stack, a)
			}
		case ast.ASTQuantifier:
			body, e2 := m.manager.GetQuantifierBody(e)
			if e2 != nil {
				return e2
			}
			stack = append(stack, body)
			patterns, e2 := m.manager.GetNumPatterns(e)
			if e2 != nil {
				return e2
			}
			for j := patterns; j > 0; j-- {
				pattern, e3 := m.manager.GetPattern(e, j-1)
				if e3 != nil {
					return e3
				}
				stack = append(stack, pattern)
			}
			noPatterns, e2 := m.manager.GetNumNoPatterns(e)
			if e2 != nil {
				return e2
			}
			for j := noPatterns; j > 0; j-- {
				pattern, e3 := m.manager.GetNoPattern(e, j-1)
				if e3 != nil {
					return e3
				}
				stack = append(stack, pattern)
			}
		}
	}
	m.publishForbidden()
	return nil
}
func (m *Manager) IsForbidden(d ast.ASTHandle) bool { _, yes := m.forbiddenSet[d]; return yes }

func (m *Manager) Reset() error {
	if err := m.open(); err != nil {
		return err
	}
	err := errors.Join(m.restoreDeclarations(0), m.restoreForbidden(0), m.dependencies.Reset())
	m.scopes = nil
	return err
}
func (m *Manager) Close() error {
	if m == nil || m.self != m {
		return ast.ErrASTHandle
	}
	if m.closed {
		return nil
	}
	var result error
	if m.dependencies != nil {
		result = errors.Join(result, m.restoreDeclarations(0), m.restoreForbidden(0), m.dependencies.Close())
		m.dependencies = nil
	}
	if result != nil {
		return result
	}
	m.manager.Budget().Release(m.header)
	m.header = 0
	m.closed = true
	return nil
}

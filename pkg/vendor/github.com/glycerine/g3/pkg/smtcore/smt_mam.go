package smtcore

// Source: src/smt/mam.h and src/smt/mam.cpp retained E-matching semantics.
// The Go port uses a direct congruence-aware matcher over context enodes; the
// source instruction trees and approximate-label indexes are optimizations.

import "github.com/glycerine/g3/pkg/ast"

type smtMAMPattern struct {
	quantifier ast.ASTHandle
	pattern    ast.ASTHandle
}

type SMTMAM struct {
	state    *SMTInternalizerState
	registry *SMTQuantifierRegistry
	patterns []smtMAMPattern
	known    map[smtMAMPattern]struct{}
	shared   map[*SMTEnode]struct{}
	scopes   []int
	dirty    bool
}

func NewSMTMAM(state *SMTInternalizerState, registry *SMTQuantifierRegistry) (*SMTMAM, error) {
	if state == nil || state.manager == nil || registry == nil || registry.manager != state.manager {
		return nil, ast.ErrASTHandle
	}
	return &SMTMAM{state: state, registry: registry, known: make(map[smtMAMPattern]struct{}), shared: make(map[*SMTEnode]struct{})}, nil
}

func (m *SMTMAM) collectGround(term ast.ASTHandle) error {
	kind, err := m.state.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return err
	}
	ground, err := m.state.manager.IsGround(term)
	if err != nil {
		return err
	}
	if ground {
		if err = m.state.Internalize(term, false); err != nil {
			return err
		}
		node := m.state.context.enodes[term]
		if node == nil {
			return ast.ErrASTDomain
		}
		m.shared[node] = struct{}{}
		return nil
	}
	count, err := m.state.manager.GetNumArgs(term)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		child, childErr := m.state.manager.GetArg(term, i)
		if childErr != nil {
			return childErr
		}
		childKind, childErr := m.state.manager.GetKind(child)
		if childErr != nil {
			return childErr
		}
		if childKind == ast.ASTApp {
			if childErr = m.collectGround(child); childErr != nil {
				return childErr
			}
		}
	}
	return nil
}

func (m *SMTMAM) AddPattern(quantifier, pattern ast.ASTHandle) error {
	if m == nil || m.state == nil {
		return ast.ErrASTHandle
	}
	isPattern, err := m.state.manager.IsPattern(pattern)
	if err != nil || !isPattern {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	count, err := m.state.manager.GetNumArgs(pattern)
	if err != nil || count == 0 {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	for i := uint32(0); i < count; i++ {
		term, termErr := m.state.manager.GetArg(pattern, i)
		if termErr != nil {
			return termErr
		}
		ground, termErr := m.state.manager.IsGround(term)
		if termErr != nil {
			return termErr
		}
		if ground {
			return nil
		}
		if termErr = m.collectGround(term); termErr != nil {
			return termErr
		}
	}
	key := smtMAMPattern{quantifier: quantifier, pattern: pattern}
	if _, found := m.known[key]; found {
		return nil
	}
	m.known[key] = struct{}{}
	m.patterns = append(m.patterns, key)
	m.dirty = true
	return nil
}

func (m *SMTMAM) PushScope() error {
	if m == nil || m.state == nil {
		return ast.ErrASTHandle
	}
	m.scopes = append(m.scopes, len(m.patterns))
	return nil
}

func (m *SMTMAM) PopScopes(count uint32) error {
	if m == nil || count == 0 || count > uint32(len(m.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(m.scopes) - int(count)
	limit := m.scopes[level]
	for _, entry := range m.patterns[limit:] {
		delete(m.known, entry)
	}
	m.patterns = m.patterns[:limit]
	m.scopes = m.scopes[:level]
	m.shared = make(map[*SMTEnode]struct{})
	for _, entry := range m.patterns {
		count, err := m.state.manager.GetNumArgs(entry.pattern)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			term, err := m.state.manager.GetArg(entry.pattern, i)
			if err != nil {
				return err
			}
			if err = m.collectGround(term); err != nil {
				return err
			}
		}
	}
	m.dirty = true
	return nil
}

func (m *SMTMAM) Relevant(*SMTEnode) {
	if m != nil {
		m.dirty = true
	}
}
func (m *SMTMAM) AddEquality(*SMTEnode, *SMTEnode) {
	if m != nil {
		m.dirty = true
	}
}
func (m *SMTMAM) HasWork() bool { return m != nil && m.dirty }

func (m *SMTMAM) matchTerm(pattern ast.ASTHandle, candidate *SMTEnode, bindings []*SMTEnode, used *[]*SMTEnode) (bool, error) {
	kind, err := m.state.manager.GetKind(pattern)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		index, err := m.state.manager.GetVarIndex(pattern)
		if err != nil || index >= uint32(len(bindings)) {
			if err != nil {
				return false, err
			}
			return false, ast.ErrASTDomain
		}
		position := len(bindings) - int(index) - 1
		root := candidate.root
		if bindings[position] != nil {
			return bindings[position].root == root, nil
		}
		bindings[position] = root.EqualityClassMinGeneration()
		*used = append(*used, candidate)
		return true, nil
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	declaration, err := m.state.manager.GetDecl(pattern)
	if err != nil {
		return false, err
	}
	count, err := m.state.manager.GetNumArgs(pattern)
	if err != nil {
		return false, err
	}
	for current := candidate.root; ; current = current.next {
		if current.declaration == declaration && uint32(len(current.arguments)) == count {
			savedBindings := append([]*SMTEnode(nil), bindings...)
			savedUsed := len(*used)
			matched := true
			for i := uint32(0); i < count; i++ {
				child, childErr := m.state.manager.GetArg(pattern, i)
				if childErr != nil {
					return false, childErr
				}
				matched, childErr = m.matchTerm(child, current.arguments[i], bindings, used)
				if childErr != nil {
					return false, childErr
				}
				if !matched {
					break
				}
			}
			if matched {
				*used = append(*used, current)
				return true, nil
			}
			copy(bindings, savedBindings)
			*used = (*used)[:savedUsed]
		}
		if current.next == candidate.root {
			break
		}
	}
	return false, nil
}

func (m *SMTMAM) candidates(term ast.ASTHandle) ([]*SMTEnode, error) {
	declaration, err := m.state.manager.GetDecl(term)
	if err != nil {
		return nil, err
	}
	var result []*SMTEnode
	for _, node := range m.state.context.byDeclaration[declaration] {
		if m.state.isRelevant(node.owner) {
			result = append(result, node)
		}
	}
	return result, nil
}

func (m *SMTMAM) matchPattern(entry smtMAMPattern) error {
	decls, err := m.state.manager.GetNumDeclarations(entry.quantifier)
	if err != nil {
		return err
	}
	count, err := m.state.manager.GetNumArgs(entry.pattern)
	if err != nil {
		return err
	}
	terms := make([]ast.ASTHandle, count)
	candidates := make([][]*SMTEnode, count)
	for i := uint32(0); i < count; i++ {
		terms[i], err = m.state.manager.GetArg(entry.pattern, i)
		if err != nil {
			return err
		}
		candidates[i], err = m.candidates(terms[i])
		if err != nil || len(candidates[i]) == 0 {
			return err
		}
	}
	bindings := make([]*SMTEnode, decls)
	var visit func(int, []*SMTEnode, []*SMTEnode, []*SMTEnode) error
	visit = func(index int, currentBindings, used, top []*SMTEnode) error {
		if index == len(terms) {
			for _, binding := range currentBindings {
				if binding == nil {
					return nil
				}
			}
			maxGeneration := SMTEnodesMaxGeneration(used)
			minTop, maxTop := uint32(0), uint32(0)
			for i, node := range top {
				generation := node.generation
				if i == 0 || generation < minTop {
					minTop = generation
				}
				if generation > maxTop {
					maxTop = generation
				}
			}
			_, err := m.registry.AddInstance(entry.quantifier, entry.pattern, currentBindings, maxGeneration, minTop, maxTop)
			return err
		}
		for _, candidate := range candidates[index] {
			nextBindings := append([]*SMTEnode(nil), currentBindings...)
			nextUsed := append([]*SMTEnode(nil), used...)
			matched, err := m.matchTerm(terms[index], candidate, nextBindings, &nextUsed)
			if err != nil {
				return err
			}
			if matched {
				nextUsed = append(nextUsed, candidate)
				nextTop := append(append([]*SMTEnode(nil), top...), candidate)
				if err = visit(index+1, nextBindings, nextUsed, nextTop); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(0, bindings, nil, nil)
}

func (m *SMTMAM) Match() error {
	if m == nil || m.state == nil {
		return ast.ErrASTHandle
	}
	for _, entry := range m.patterns {
		if err := m.matchPattern(entry); err != nil {
			return err
		}
	}
	m.dirty = false
	return nil
}

func (m *SMTMAM) Rematch(_ bool) error {
	if m == nil {
		return ast.ErrASTHandle
	}
	m.dirty = true
	return m.Match()
}

func (m *SMTMAM) Reset() {
	if m == nil {
		return
	}
	m.patterns = nil
	m.known = make(map[smtMAMPattern]struct{})
	m.shared = make(map[*SMTEnode]struct{})
	m.scopes = nil
	m.dirty = false
}

func (m *SMTMAM) IsShared(node *SMTEnode) bool {
	if m == nil || node == nil {
		return false
	}
	for shared := range m.shared {
		if shared.root == node.root {
			return true
		}
	}
	return false
}

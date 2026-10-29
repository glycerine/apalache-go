package smtcore

// Source: src/smt/theory_seq.cpp solution_map and exclusion_table;
// src/smt/theory_seq.h corresponding declarations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtSequenceAssumption struct {
	left, right *SMTEnode
	literal     SMTLiteral
}

type smtSequenceDependency struct {
	left, right *smtSequenceDependency
	assumption  *smtSequenceAssumption
}

type smtSequenceDependencyManager struct{}

func (*smtSequenceDependencyManager) Leaf(assumption smtSequenceAssumption) *smtSequenceDependency {
	return &smtSequenceDependency{assumption: &assumption}
}

func (*smtSequenceDependencyManager) Join(left, right *smtSequenceDependency) *smtSequenceDependency {
	if left == nil {
		return right
	}
	if right == nil || left == right {
		return left
	}
	return &smtSequenceDependency{left: left, right: right}
}

func (*smtSequenceDependencyManager) Linearize(root *smtSequenceDependency) []smtSequenceAssumption {
	if root == nil {
		return nil
	}
	result := make([]smtSequenceAssumption, 0)
	seenNodes := make(map[*smtSequenceDependency]struct{})
	seenAssumptions := make(map[*smtSequenceAssumption]struct{})
	stack := []*smtSequenceDependency{root}
	for len(stack) != 0 {
		index := len(stack) - 1
		current := stack[index]
		stack = stack[:index]
		if current == nil {
			continue
		}
		if _, found := seenNodes[current]; found {
			continue
		}
		seenNodes[current] = struct{}{}
		if current.assumption != nil {
			if _, found := seenAssumptions[current.assumption]; !found {
				seenAssumptions[current.assumption] = struct{}{}
				result = append(result, *current.assumption)
			}
			continue
		}
		stack = append(stack, current.right, current.left)
	}
	return result
}

type smtSequenceSolution struct {
	value      ast.ASTHandle
	dependency *smtSequenceDependency
}

type smtSequenceSolutionUpdate struct {
	key, value ast.ASTRef
	previous   smtSequenceSolution
	had        bool
}

type smtSequenceCachedSolution struct {
	key, value ast.ASTRef
	dependency *smtSequenceDependency
}

type smtSequenceSolutionMap struct {
	manager      *ast.ASTManager
	dependencies *smtSequenceDependencyManager
	entries      map[ast.ASTHandle]smtSequenceSolution
	updates      []smtSequenceSolutionUpdate
	limits       []int
	cache        map[ast.ASTHandle]smtSequenceSolution
	cacheRefs    []smtSequenceCachedSolution
	closed       bool
}

func newSMTSequenceSolutionMap(manager *ast.ASTManager, dependencies *smtSequenceDependencyManager) (*smtSequenceSolutionMap, error) {
	if manager == nil || dependencies == nil {
		return nil, ast.ErrASTHandle
	}
	return &smtSequenceSolutionMap{
		manager: manager, dependencies: dependencies,
		entries: make(map[ast.ASTHandle]smtSequenceSolution), cache: make(map[ast.ASTHandle]smtSequenceSolution),
	}, nil
}

func (m *smtSequenceSolutionMap) open() error {
	if m == nil || m.manager == nil || m.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (m *smtSequenceSolutionMap) resetCache() (result error) {
	for i := range m.cacheRefs {
		result = errors.Join(result, m.manager.DelRef(&m.cacheRefs[i].key), m.manager.DelRef(&m.cacheRefs[i].value))
	}
	m.cacheRefs = nil
	clear(m.cache)
	return result
}

func (m *smtSequenceSolutionMap) Update(key, value ast.ASTHandle, dependency *smtSequenceDependency) (result error) {
	if err := m.open(); err != nil {
		return err
	}
	if key.IsNull() || value.IsNull() {
		return ast.ErrASTDomain
	}
	if key == value {
		return nil
	}
	if err := m.resetCache(); err != nil {
		return err
	}
	keyRef, err := m.manager.NewRef(key)
	if err != nil {
		return err
	}
	valueRef, err := m.manager.NewRef(value)
	if err != nil {
		_ = m.manager.DelRef(&keyRef)
		return err
	}
	previous, had := m.entries[key]
	m.entries[key] = smtSequenceSolution{value: value, dependency: dependency}
	m.updates = append(m.updates, smtSequenceSolutionUpdate{key: keyRef, value: valueRef, previous: previous, had: had})
	return nil
}

func (m *smtSequenceSolutionMap) IsRoot(expression ast.ASTHandle) bool {
	if m == nil || m.closed {
		return false
	}
	_, found := m.entries[expression]
	return !found
}

func (m *smtSequenceSolutionMap) FindOne(expression ast.ASTHandle, dependency *smtSequenceDependency) (ast.ASTHandle, *smtSequenceDependency, bool, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	value, found := m.entries[expression]
	if !found {
		return expression, dependency, false, nil
	}
	return value.value, m.dependencies.Join(dependency, value.dependency), true, nil
}

func (m *smtSequenceSolutionMap) Find(expression ast.ASTHandle) (ast.ASTHandle, *smtSequenceDependency, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, nil, err
	}
	if value, found := m.cache[expression]; found {
		return value.value, value.dependency, nil
	}
	original := expression
	var dependency *smtSequenceDependency
	seen := make(map[ast.ASTHandle]struct{})
	for {
		if _, found := seen[expression]; found {
			return ast.ASTHandle{}, nil, ast.ErrASTDomain
		}
		seen[expression] = struct{}{}
		value, found := m.entries[expression]
		if !found {
			break
		}
		dependency = m.dependencies.Join(dependency, value.dependency)
		expression = value.value
	}
	if original != expression {
		keyRef, err := m.manager.NewRef(original)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		valueRef, err := m.manager.NewRef(expression)
		if err != nil {
			_ = m.manager.DelRef(&keyRef)
			return ast.ASTHandle{}, nil, err
		}
		m.cache[original] = smtSequenceSolution{value: expression, dependency: dependency}
		m.cacheRefs = append(m.cacheRefs, smtSequenceCachedSolution{key: keyRef, value: valueRef, dependency: dependency})
	}
	return expression, dependency, nil
}

func (m *smtSequenceSolutionMap) PushScope() error {
	if err := m.open(); err != nil {
		return err
	}
	m.limits = append(m.limits, len(m.updates))
	return nil
}

func (m *smtSequenceSolutionMap) PopScopes(count uint32) (result error) {
	if err := m.open(); err != nil {
		return err
	}
	if count == 0 || count > uint32(len(m.limits)) {
		return ast.ErrASTDomain
	}
	if err := m.resetCache(); err != nil {
		return err
	}
	level := len(m.limits) - int(count)
	limit := m.limits[level]
	for i := len(m.updates) - 1; i >= limit; i-- {
		update := &m.updates[i]
		key, err := m.manager.RefGet(update.key)
		if err != nil {
			result = errors.Join(result, err)
		} else if update.had {
			m.entries[key] = update.previous
		} else {
			delete(m.entries, key)
		}
		result = errors.Join(result, m.manager.DelRef(&update.key), m.manager.DelRef(&update.value))
	}
	m.updates = m.updates[:limit]
	m.limits = m.limits[:level]
	return result
}

func (m *smtSequenceSolutionMap) Close() (result error) {
	if err := m.open(); err != nil {
		return err
	}
	result = m.resetCache()
	for i := range m.updates {
		result = errors.Join(result, m.manager.DelRef(&m.updates[i].key), m.manager.DelRef(&m.updates[i].value))
	}
	m.entries = nil
	m.updates = nil
	m.limits = nil
	m.manager = nil
	m.dependencies = nil
	m.closed = true
	return result
}

type smtSequencePair struct{ left, right ast.ASTHandle }

func smtSequenceOrderedPair(left, right ast.ASTHandle) smtSequencePair {
	if left.Slot() > right.Slot() {
		left, right = right, left
	}
	return smtSequencePair{left: left, right: right}
}

type smtSequenceExclusion struct{ left, right ast.ASTRef }

type smtSequenceExclusionTable struct {
	manager *ast.ASTManager
	pairs   map[smtSequencePair]struct{}
	trail   []smtSequenceExclusion
	limits  []int
	closed  bool
}

func newSMTSequenceExclusionTable(manager *ast.ASTManager) (*smtSequenceExclusionTable, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &smtSequenceExclusionTable{manager: manager, pairs: make(map[smtSequencePair]struct{})}, nil
}

func (t *smtSequenceExclusionTable) Contains(left, right ast.ASTHandle) bool {
	if t == nil || t.closed || left == right {
		return false
	}
	_, found := t.pairs[smtSequenceOrderedPair(left, right)]
	return found
}

func (t *smtSequenceExclusionTable) Update(left, right ast.ASTHandle) error {
	if t == nil || t.manager == nil || t.closed {
		return ast.ErrASTHandle
	}
	if left.IsNull() || right.IsNull() {
		return ast.ErrASTDomain
	}
	pair := smtSequenceOrderedPair(left, right)
	if left == right {
		return nil
	}
	if _, found := t.pairs[pair]; found {
		return nil
	}
	leftRef, err := t.manager.NewRef(pair.left)
	if err != nil {
		return err
	}
	rightRef, err := t.manager.NewRef(pair.right)
	if err != nil {
		_ = t.manager.DelRef(&leftRef)
		return err
	}
	t.pairs[pair] = struct{}{}
	t.trail = append(t.trail, smtSequenceExclusion{left: leftRef, right: rightRef})
	return nil
}

func (t *smtSequenceExclusionTable) PushScope() error {
	if t == nil || t.manager == nil || t.closed {
		return ast.ErrASTHandle
	}
	t.limits = append(t.limits, len(t.trail))
	return nil
}

func (t *smtSequenceExclusionTable) PopScopes(count uint32) (result error) {
	if t == nil || t.manager == nil || t.closed {
		return ast.ErrASTHandle
	}
	if count == 0 || count > uint32(len(t.limits)) {
		return ast.ErrASTDomain
	}
	level := len(t.limits) - int(count)
	limit := t.limits[level]
	for i := len(t.trail) - 1; i >= limit; i-- {
		entry := &t.trail[i]
		left, leftErr := t.manager.RefGet(entry.left)
		right, rightErr := t.manager.RefGet(entry.right)
		result = errors.Join(result, leftErr, rightErr)
		if leftErr == nil && rightErr == nil {
			delete(t.pairs, smtSequenceOrderedPair(left, right))
		}
		result = errors.Join(result, t.manager.DelRef(&entry.left), t.manager.DelRef(&entry.right))
	}
	t.trail = t.trail[:limit]
	t.limits = t.limits[:level]
	return result
}

func (t *smtSequenceExclusionTable) Close() (result error) {
	if t == nil || t.manager == nil || t.closed {
		return ast.ErrASTHandle
	}
	for i := range t.trail {
		result = errors.Join(result, t.manager.DelRef(&t.trail[i].left), t.manager.DelRef(&t.trail[i].right))
	}
	t.pairs = nil
	t.trail = nil
	t.limits = nil
	t.manager = nil
	t.closed = true
	return result
}

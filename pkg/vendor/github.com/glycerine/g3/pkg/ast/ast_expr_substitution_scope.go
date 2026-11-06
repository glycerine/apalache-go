package ast

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

const astScopedExprSubstBytes uint32 = 8
const astScopedExprSubstMagic uint32 = 0x53455355

const (
	astScopedSubstIdle uint8 = iota
	astScopedSubstErase
	astScopedSubstRelease
	astScopedSubstDestroyTail
	astScopedSubstDestroyPrefix
)

type astScopedExprSubstState struct {
	subst                ASTExprSubstitution
	trail                []astNodeRef
	limits               []int
	operation            uint8
	index, target, level int
	header               uint64
}

func (m *ASTManager) scopedSubstState(s ASTScopedExprSubstitution) (*astScopedExprSubstState, error) {
	a := m.scopedExprSubstitutions[s.record]
	if s.origin != m.origin || a == nil {
		return nil, ErrASTHandle
	}
	if size, e := m.allocator.Size(s.record); e != nil || size != astScopedExprSubstBytes {
		return nil, ErrASTHandle
	}
	if tag, e := m.readWord(s.record, 0); e != nil || tag != astScopedExprSubstMagic {
		return nil, ErrASTHandle
	}
	return a, nil
}
func (m *ASTManager) NewScopedExprSubstitution(subst ASTExprSubstitution) (ASTScopedExprSubstitution, error) {
	if e := m.lock(); e != nil {
		return ASTScopedExprSubstitution{}, e
	}
	defer m.mu.Unlock()
	if _, e := m.substState(subst); e != nil {
		return ASTScopedExprSubstitution{}, e
	}
	header := uint64(reflect.TypeOf(astScopedExprSubstState{}).Size())
	if e := m.BudgetField.Reserve(header); e != nil {
		return ASTScopedExprSubstitution{}, e
	}
	r, e := m.allocator.Allocate(astScopedExprSubstBytes)
	if e != nil {
		m.BudgetField.Release(header)
		return ASTScopedExprSubstitution{}, e
	}
	if e = m.writeWord(r, 0, astScopedExprSubstMagic); e != nil {
		m.allocator.Deallocate(astScopedExprSubstBytes, r)
		m.BudgetField.Release(header)
		return ASTScopedExprSubstitution{}, e
	}
	if m.scopedExprSubstitutions == nil {
		m.scopedExprSubstitutions = make(map[arena.SmallBytesRef]*astScopedExprSubstState)
	}
	m.scopedExprSubstitutions[r] = &astScopedExprSubstState{subst: subst, header: header}
	return ASTScopedExprSubstitution{m.origin, r}, nil
}
func (m *ASTManager) ScopedExprSubstitutionInsert(s ASTScopedExprSubstitution, key, def, proof ASTHandle, dep ASTDependency) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	a, e := m.scopedSubstState(s)
	if e != nil {
		return e
	}
	if a.operation != 0 {
		return ErrASTPending
	}
	k, e := m.substRawExpr(key)
	if e != nil {
		return e
	}
	present, e := m.substContains(a.subst, k)
	if e != nil || present {
		return e
	}
	v, e := m.substRawExpr(def)
	if e != nil {
		return e
	}
	flags, e := m.substState(a.subst)
	if e != nil {
		return e
	}
	var p, d astNodeRef
	if flags&1 != 0 {
		p, e = m.substRawExpr(proof)
		if e != nil {
			return e
		}
	}
	if flags&2 != 0 {
		d, e = m.substRawDependency(dep)
		if e != nil {
			return e
		}
	}
	if e = m.substInsert(a.subst, k, v, p, d); e != nil {
		return e
	}
	if e = m.incNode(k); e != nil {
		return e
	}
	a.trail = append(a.trail, k)
	return nil
}
func (m *ASTManager) ScopedExprSubstitutionPush(s ASTScopedExprSubstitution) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	a, e := m.scopedSubstState(s)
	if e != nil {
		return e
	}
	if a.operation != astScopedSubstIdle {
		return ErrASTPending
	}
	a.limits = append(a.limits, len(a.trail))
	return nil
}
func (m *ASTManager) scopedSubstPop(a *astScopedExprSubstState, n uint32) error {
	if a.operation == astScopedSubstIdle {
		if n == 0 {
			return nil
		}
		if uint64(n) > uint64(len(a.limits)) {
			return ErrASTDomain
		}
		a.level = len(a.limits) - int(n)
		a.target = a.limits[a.level]
		a.index = a.target
		a.operation = astScopedSubstErase
	} else if a.operation != astScopedSubstErase && a.operation != astScopedSubstRelease {
		return ErrASTPending
	}
	if a.operation == astScopedSubstErase {
		for a.index < len(a.trail) {
			r := a.trail[a.index]
			if e := m.substErase(a.subst, r); e != nil {
				table, x := m.substTable(a.subst, 0)
				if x == nil {
					_, present, x := m.substFind(table, r)
					if x == nil && !present {
						a.index++
					}
				}
				return e
			}
			a.index++
		}
		a.operation = astScopedSubstRelease
		a.index = a.target
	}
	failure := m.scopedSubstDecTrail(a)
	if a.index != len(a.trail) {
		return failure
	}
	a.trail = a.trail[:a.target]
	a.limits = a.limits[:a.level]
	a.operation = astScopedSubstIdle
	a.index, a.target, a.level = 0, 0, 0
	return failure
}
func (m *ASTManager) ScopedExprSubstitutionPop(s ASTScopedExprSubstitution, n uint32) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	a, e := m.scopedSubstState(s)
	if e != nil {
		return e
	}
	return m.scopedSubstPop(a, n)
}
func (m *ASTManager) ScopedExprSubstitutionScopeLevel(s ASTScopedExprSubstitution) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	a, e := m.scopedSubstState(s)
	if e != nil {
		return 0, e
	}
	return uint32(len(a.limits)), nil
}
func (m *ASTManager) ScopedExprSubstitutionEmpty(s ASTScopedExprSubstitution) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	a, e := m.scopedSubstState(s)
	if e != nil {
		return false, e
	}
	t, e := m.substTable(a.subst, 0)
	if e != nil {
		return false, e
	}
	return t.entries.Len() == 0, nil
}
func (m *ASTManager) ScopedExprSubstitutionContains(s ASTScopedExprSubstitution, key ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	a, e := m.scopedSubstState(s)
	if e != nil {
		return false, e
	}
	k, e := m.substRawExpr(key)
	if e != nil {
		return false, e
	}
	return m.substContains(a.subst, k)
}
func (m *ASTManager) ScopedExprSubstitutionFindProof(s ASTScopedExprSubstitution, key ASTHandle, def, proof *ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	a, e := m.scopedSubstState(s)
	if e != nil {
		return false, e
	}
	k, e := m.substRawExpr(key)
	if e != nil {
		return false, e
	}
	return m.substFindOutputs(a.subst, k, def, proof, nil)
}
func (m *ASTManager) ScopedExprSubstitutionFindDependency(s ASTScopedExprSubstitution, key ASTHandle, def, proof *ASTHandle, dep *ASTDependency) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if dep == nil {
		return false, ErrASTDomain
	}
	a, e := m.scopedSubstState(s)
	if e != nil {
		return false, e
	}
	k, e := m.substRawExpr(key)
	if e != nil {
		return false, e
	}
	return m.substFindOutputs(a.subst, k, def, proof, dep)
}
func (m *ASTManager) ScopedExprSubstitutionFindValue(s ASTScopedExprSubstitution, key ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	a, e := m.scopedSubstState(s)
	if e != nil {
		return ASTHandle{}, e
	}
	k, e := m.substRawExpr(key)
	if e != nil {
		return ASTHandle{}, e
	}
	var def, pr ASTHandle
	found, e := m.substFindOutputs(a.subst, k, &def, &pr, nil)
	if e != nil {
		return ASTHandle{}, e
	}
	if found {
		return def, nil
	}
	return key, nil
}
func (m *ASTManager) ScopedExprSubstitutionCleanup(s ASTScopedExprSubstitution) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	a, e := m.scopedSubstState(s)
	if e != nil {
		return e
	}
	return m.substCleanup(a.subst)
}

// The cursor advances only after an actual reference callback consumes its root.
// Clearing consumed entries also prevents dead locators escaping through backing.
func (m *ASTManager) scopedSubstDecTrail(a *astScopedExprSubstState) error {
	var failure error
	for a.index < len(a.trail) {
		consumed, e := m.decNode(a.trail[a.index])
		if !consumed {
			if failure != nil {
				return failure
			}
			return e
		}
		a.trail[a.index] = astNodeRef{}
		a.index++
		if failure == nil {
			failure = e
		}
	}
	return failure
}
func (m *ASTManager) scopedSubstReleaseTrail(a *astScopedExprSubstState) error {
	var failure error
	if a.operation == astScopedSubstDestroyTail {
		failure = m.scopedSubstDecTrail(a)
		if a.index != len(a.trail) {
			return failure
		}
		a.trail = a.trail[:a.target]
		a.operation = astScopedSubstDestroyPrefix
		a.index = 0
	}
	if a.operation != astScopedSubstDestroyPrefix {
		a.operation = astScopedSubstDestroyPrefix
		a.index = 0
	}
	e := m.scopedSubstDecTrail(a)
	if failure == nil {
		failure = e
	}
	if a.index != len(a.trail) {
		return failure
	}
	a.trail = nil
	a.operation = astScopedSubstIdle
	a.index, a.target, a.level = 0, 0, 0
	return failure
}
func (m *ASTManager) ScopedExprSubstitutionReset(s ASTScopedExprSubstitution) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	a, e := m.scopedSubstState(s)
	if e != nil {
		return e
	}
	if a.operation != astScopedSubstIdle && a.operation != astScopedSubstDestroyPrefix {
		return ErrASTPending
	}
	if a.operation == astScopedSubstIdle {
		if e = m.substReset(a.subst); e != nil {
			return e
		}
	}
	failure := m.scopedSubstReleaseTrail(a)
	if a.operation == astScopedSubstIdle {
		a.limits = nil
	}
	return failure
}
func (m *ASTManager) DelScopedExprSubstitution(s *ASTScopedExprSubstitution) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if s == nil {
		return ErrASTHandle
	}
	a, e := m.scopedSubstState(*s)
	if e != nil {
		return e
	}
	if a.operation == astScopedSubstRelease {
		a.operation = astScopedSubstDestroyTail
	}
	// The limits die before the trail. Destruction leaves the borrowed map intact.
	a.limits = nil
	failure := m.scopedSubstReleaseTrail(a)
	if a.operation != astScopedSubstIdle {
		return failure
	}
	if e = m.allocator.Deallocate(astScopedExprSubstBytes, s.record); e != nil {
		return e
	}
	m.BudgetField.Release(a.header)
	delete(m.scopedExprSubstitutions, s.record)
	*s = ASTScopedExprSubstitution{}
	return failure
}

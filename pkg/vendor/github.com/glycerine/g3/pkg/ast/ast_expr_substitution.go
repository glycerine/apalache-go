package ast

// Concrete expr_substitution: checked scalar owners and native dictionaries.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/insdict"
)

type ASTExprSubstitution struct {
	origin uint64
	record arena.SmallBytesRef
}
type ASTScopedExprSubstitution struct {
	origin uint64
	record arena.SmallBytesRef
}
type astExprSubstTable struct {
	entries insdict.Dict[uint64, astExprSubstEntry]
	header  uint64
}
type astExprSubstEntry struct {
	key, value astNodeRef
	owned      uint32
}
type astExprSubstPending struct {
	node  astNodeRef
	which uint32
}
type astExprSubstState struct {
	base    astExprSubstTable
	proof   *astExprSubstTable
	dep     *astExprSubstTable
	pending []astExprSubstPending
	header  uint64
}

const astExprSubstBytes uint32 = 16
const astExprSubstMagic uint32 = 0x45535542

func (m *ASTManager) substState(s ASTExprSubstitution) (uint32, error) {
	if s.origin != m.origin || m.exprSubstitutions[s.record] == nil {
		return 0, ErrASTHandle
	}
	if size, e := m.allocator.Size(s.record); e != nil || size != astExprSubstBytes {
		return 0, ErrASTHandle
	}
	tag, e := m.readWord(s.record, 4)
	if e != nil {
		return 0, e
	}
	if tag != astExprSubstMagic {
		return 0, ErrASTHandle
	}
	return m.readWord(s.record, 0)
}
func (m *ASTManager) substTable(s ASTExprSubstitution, which uint32) (*astExprSubstTable, error) {
	if _, e := m.substState(s); e != nil {
		return nil, e
	}
	if which >= 3 {
		return nil, ErrASTDomain
	}
	state := m.exprSubstitutions[s.record]
	switch which {
	case 0:
		return &state.base, nil
	case 1:
		if state.proof != nil {
			return state.proof, nil
		}
	case 2:
		if state.dep != nil {
			return state.dep, nil
		}
	}
	return nil, ErrASTDomain
}
func (m *ASTManager) newExprSubstitution(cores, proofs, explicit bool) (ASTExprSubstitution, error) {
	header := uint64(reflect.TypeOf(astExprSubstState{}).Size())
	if e := m.BudgetField.Reserve(header); e != nil {
		return ASTExprSubstitution{}, e
	}
	r, e := m.allocator.Allocate(astExprSubstBytes)
	if e != nil {
		m.BudgetField.Release(header)
		return ASTExprSubstitution{}, e
	}
	state := &astExprSubstState{header: header}
	fail := func(e error) (ASTExprSubstitution, error) {
		m.releaseExprSubstOptionalTables(state)
		m.allocator.Deallocate(astExprSubstBytes, r)
		m.BudgetField.Release(header)
		return ASTExprSubstitution{}, e
	}
	if explicit && ASTSourceDebug && proofs && m.proofMode == ASTProofDisabled {
		return fail(ErrASTDomain)
	}
	flags := uint32(0)
	if proofs {
		flags |= 1
	}
	if cores {
		flags |= 2
	}
	for i, x := range [4]uint32{flags, astExprSubstMagic, 0, 0} {
		if e = m.writeWord(r, uint32(i)*4, x); e != nil {
			return fail(e)
		}
	}
	if proofs {
		state.proof, e = m.newExprSubstOptionalTable(29, "expr2proof")
		if e != nil {
			return fail(e)
		}
	}
	if cores {
		state.dep, e = m.newExprSubstOptionalTable(31, "expr2expr_dependency")
		if e != nil {
			return fail(e)
		}
	}
	if m.exprSubstitutions == nil {
		m.exprSubstitutions = make(map[arena.SmallBytesRef]*astExprSubstState)
	}
	m.exprSubstitutions[r] = state
	return ASTExprSubstitution{m.origin, r}, nil
}
func (m *ASTManager) releaseExprSubstitutionsAfterNodeTeardown() {
	for _, s := range m.exprSubstitutions {
		m.releaseExprSubstOptionalTables(s)
		m.BudgetField.Release(s.header)
	}
	m.exprSubstitutions = nil
	for _, s := range m.scopedExprSubstitutions {
		m.BudgetField.Release(s.header)
	}
	m.scopedExprSubstitutions = nil
}
func (m *ASTManager) NewExprSubstitution() (ASTExprSubstitution, error) {
	if e := m.lock(); e != nil {
		return ASTExprSubstitution{}, e
	}
	defer m.mu.Unlock()
	return m.newExprSubstitution(false, m.proofMode != ASTProofDisabled, false)
}
func (m *ASTManager) NewExprSubstitutionCores(cores bool) (ASTExprSubstitution, error) {
	if e := m.lock(); e != nil {
		return ASTExprSubstitution{}, e
	}
	defer m.mu.Unlock()
	return m.newExprSubstitution(cores, m.proofMode != ASTProofDisabled, false)
}
func (m *ASTManager) NewExprSubstitutionPolicy(cores, proofs bool) (ASTExprSubstitution, error) {
	if e := m.lock(); e != nil {
		return ASTExprSubstitution{}, e
	}
	defer m.mu.Unlock()
	return m.newExprSubstitution(cores, proofs, true)
}
func (m *ASTManager) ExprSubstitutionManager(s ASTExprSubstitution) (*ASTManager, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	if _, e := m.substState(s); e != nil {
		return nil, e
	}
	return m, nil
}
func (m *ASTManager) ExprSubstitutionProofsEnabled(s ASTExprSubstitution) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	f, e := m.substState(s)
	return f&1 != 0, e
}
func (m *ASTManager) ExprSubstitutionUnsatCoreEnabled(s ASTExprSubstitution) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	f, e := m.substState(s)
	return f&2 != 0, e
}
func (m *ASTManager) ExprSubstitutionEmpty(s ASTExprSubstitution) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	t, e := m.substTable(s, 0)
	if e != nil {
		return false, e
	}
	return t.entries.Len() == 0, nil
}

package ast

// Source: src/ast/ast.cpp:1297-1334; src/ast/ast.h:1454-1487.

import (
	"errors"
)

const (
	astCtorAllocator uint32 = 1 << iota
	astCtorFamilies
	_ // retired unused reference-vector allocator bit
	astCtorDependencies
	astCtorTable
	astCtorIDGenerators
	astCtorDebugFree
)

type ASTConstructorState struct {
	Complete                         bool
	CompletedMembers                 uint32
	UnwindStage                      uint8
	BodyCause, UnwindCause, OwnCause error
}

// ConstructorState is explicit diagnostic storage inspection of a returned
// unfinished Go context, not permission to call the incomplete C++ object.
func (m *ASTManager) ConstructorState() ASTConstructorState {
	if m == nil || m.Self != m {
		return ASTConstructorState{BodyCause: ErrASTHandle}
	}
	if m.traceExecuting || m.formatExecuting {
		panic(ErrASTPending)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return ASTConstructorState{m.constructorComplete, m.constructorMembers, m.constructorMemberStage, m.constructorBodyCause, m.constructorUnwindCause, m.constructorOwnCause}
}

func (m *ASTManager) unwindConstructorMembers() error {
	if m.constructorUnwindCause != nil {
		return m.constructorUnwindCause
	}
	stop := func(e error) error {
		if e != nil && m.constructorUnwindCause == nil {
			m.constructorUnwindCause = e
		}
		return e
	}
	// Body-owned trace and format pointers are deliberately not source members.
	// Plugins' pointer records also have POD destruction, not plugin finalize.
	if m.constructorMemberStage == 0 {
		m.plugins.close()
		if m.debugFreeComplete {
			m.DebugFreeIndices = nil
			m.debugFreeComplete = false
		}
		m.constructorMemberStage = 1
	}
	if m.constructorMemberStage == 1 {
		if m.constructorMembers&astCtorIDGenerators != 0 {
			m.declIDs.Cleanup(1 << 31)
			m.ExprIDs.Cleanup(0)
		}
		m.constructorMemberStage = 2
	}
	if m.constructorMemberStage == 2 {
		if m.constructorMembers&astCtorTable != 0 {
			m.table = astCanonicalTable{}
			m.traceConstructorTableEnded()
		}
		m.constructorMemberStage = 3
	}
	if m.constructorMemberStage == 3 {
		if m.constructorMembers&astCtorDependencies != 0 {
			m.dependencies.closeAfterNodeTeardown()
		}
		m.constructorMemberStage = 4
	}
	if m.constructorMemberStage == 4 {
		if m.families != nil {
			if e := m.families.Close(); e != nil {
				return stop(e)
			}
			m.families = nil
		}
		m.constructorMemberStage = 5
	}
	if m.constructorMemberStage == 5 {
		// Concrete Go vector payload provider is a separate owned adapter child.
		// This Go provider represents separately allocated global vector/parameter
		// backing; it is not a source member of ast_manager and remains owned until
		// explicit abort, including body-acquired plugin info arrays.

		if m.allocator != nil {
			m.allocator.Close()
			m.endSequenceContexts()
			m.traceConstructorNodesEnded()
			m.allocator = nil
		}
		m.Limit.Close() // first automatic member, last reverse-member destructor
		m.constructorMemberStage = 6
	}
	return nil
}

func (m *ASTManager) AbortConstruction() error {
	if m == nil || m.Self != m {
		return ErrASTHandle
	}
	if m.traceExecuting || m.formatExecuting {
		return ErrASTPending
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Closed {
		return nil
	}
	return m.abortConstructionCore()
}

func (m *ASTManager) abortConstructionCore() error {
	if m.constructorComplete || m.constructorBodyCause == nil {
		return ErrASTDomain
	}
	stop := func(e error) error {
		if e != nil && m.constructorOwnCause == nil {
			m.constructorOwnCause = e
		}
		return e
	}
	if e := m.llFrameDestroy(true); e != nil {
		return stop(e)
	}
	if e := m.retireConstructorTraceAcquisitions(); e != nil {
		return stop(e)
	}
	// No successful constructor body is replayed. The exact returned child owner
	// distinguishes incomplete placement work from a completed format pointer.
	if m.constructorPendingFormat != nil {
		if e := m.constructorPendingFormat.AbortConstruction(); e != nil {
			return stop(e)
		}
		m.constructorPendingFormat = nil
	}
	if m.Format != nil {
		child := m.Format
		if e := child.Close(); e != nil {
			return stop(e)
		}
		child.traceFormatDeallocated()
		m.Format = nil
	}
	if m.TraceFile != nil {
		if e := m.TraceFile.Close(); e != nil {
			return stop(e)
		}
		m.TraceFile = nil
		m.TraceStream = nil
		m.traceOwner = false
	}
	if m.constructorPendingFile != nil {
		if e := m.constructorPendingFile.Close(); e != nil {
			return stop(e)
		}
		m.constructorPendingFile = nil
	}
	if m.constructorUnwindCause != nil {
		return stop(errors.Join(m.constructorBodyCause, m.constructorUnwindCause))
	}
	// Automatic members have already physically consumed their payloads. Body
	// plugin cache storage is still an exact acquired Go owner; no node DEC or
	// ast_manager destructor is authorized for this incomplete outer object.
	for slot := uint32(1); slot < m.nextInfo; slot++ {
		r := m.infoAt(slot)
		if r.live {
			if e := m.dropInfoStorage(slot, r.generation); e != nil {
				return stop(e)
			}
		}
	}
	for slot := uint32(1); slot < m.nextParameter; slot++ {
		r := m.parameterAt(slot)
		if r.Live {
			if e := m.dropParameter(ASTParameter{m.origin, slot, r.generation}); e != nil {
				return stop(e)
			}
		}
	}
	if e := m.destroyFloatingPointMembers(true); e != nil {
		return stop(e)
	}
	m.releaseFloatingPointHistory()
	m.releaseAuxiliaryPluginHistory()
	m.releaseDatatypePluginHistory()
	m.destroyPluginBacking()
	m.releaseTraceAcquisitionHistory()
	m.releasePages()
	m.BudgetField.Release(m.ownerBytes)
	m.ownerBytes = 0
	m.Closed = true
	return nil
}

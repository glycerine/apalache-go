package ast

// Source: src/ast/ast.cpp:1397-1479; src/ast/ast.h:1454-1490.

func (m *ASTManager) SourceFinalize() error {
	if m == nil || m.Self != m {
		return ErrASTHandle
	}
	if m.traceExecuting || m.formatExecuting {
		return ErrASTPending
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.exprVisit.active && (m.exprVisit.pendingMark.slot == 0 || m.exprVisit.mode == 0 && !m.exprVisit.localFast.owner.closed) {
		return ErrASTPending
	}
	if !m.constructorComplete {
		return ErrASTDomain
	}
	if m.sourceDTCause != nil {
		return m.sourceDTCause
	}
	if m.sourceDTCompleted {
		return nil
	}
	if m.Closed {
		return ErrASTHandle
	}
	m.sourceDTStarted = true
	e := m.closeCore(true)
	if e != nil && m.sourceDTCause == nil {
		m.sourceDTCause = e
	}
	return e
}

// Actual source DT consumed member storage. This explicit owning continuation
// drains exact remaining global parameter/info allocations and the Go header;
// it never replays the source destructor or its EOF action.
func (m *ASTManager) closeSourceOwnedStorageCore() error {
	if e := m.cleanupAppCoercion(true); e != nil {
		return e
	}
	if e := m.llFrameDestroy(true); e != nil {
		return e
	}
	if e := m.cleanupTraceAcquisitions(); e != nil {
		return e
	}
	stop := func(e error) error {
		if e != nil && m.errorOwnClose == nil {
			m.errorOwnClose = e
		}
		return e
	}
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
	m.releaseTraceAcquisitionHistory()
	m.releaseExprVisitHistory()
	m.releaseFormatPluginHeaders()
	if e := m.cleanupBitVectorPower(); e != nil {
		return e
	}
	m.releaseSequencePluginHeaders()
	m.releaseArrayPluginHeaders()
	m.releaseArithmeticPluginHeaders()
	m.releaseBitVectorPluginHeaders()
	m.releaseFormatFrames()
	if e := m.releasePPFrames(); e != nil {
		return e
	}
	if e := m.cleanupArrayCardinality(); e != nil {
		return e
	}
	m.releaseArrayCardinalityHistory()
	if e := m.destroyFloatingPointMembers(true); e != nil {
		return stop(e)
	}
	m.releaseFloatingPointHistory()
	m.releaseAuxiliaryPluginHistory()
	m.releaseDatatypePluginHistory()
	m.releasePages()
	m.BudgetField.Release(m.ownerBytes)
	m.ownerBytes = 0
	m.Closed = true
	return nil
}

package ast

// Source: src/ast/ast.cpp:1781-1844.
// Source delete_node ascending pushes/LIFO pops, using a native Go worklist.

// The only suspended phase here is info storage destruction, after del_eh,
// table removal and ID recycling. None of those consumed actions may replay.
// Copy the worklist only on refusal so ordinary inline deletion stays local.
type astDeletionContinuation struct {
	current  astNodeRef
	worklist []astNodeRef
}

func (m *ASTManager) suspendNodeDeletion(current astNodeRef, worklist []astNodeRef) {
	m.pendingDeletions = append(m.pendingDeletions, astDeletionContinuation{
		current: current, worklist: append([]astNodeRef(nil), worklist...),
	})
}

func (m *ASTManager) cleanupNodeDeletions() error {
	for len(m.pendingDeletions) != 0 {
		pending := m.pendingDeletions[0]
		rest := m.pendingDeletions[1:]
		m.pendingDeletions = nil
		// Nested deletions reached while resuming belong before previously
		// suspended independent scopes, in the original source call order.
		err := m.deleteNodeWorklist(pending.worklist, pending.current)
		m.pendingDeletions = append(m.pendingDeletions, rest...)
		if err != nil {
			return err
		}
	}
	m.pendingDeletions = nil
	m.pendingDeletionCurrent = astNodeRef{}
	m.pendingDeletionHead = astNodeRef{}
	return nil
}

// consumed reports whether the input reference was consumed, even when
// a destruction callback returns an error.
func (m *ASTManager) decNode(r astNodeRef) (consumed bool, err error) {
	if r.slot == 0 {
		return true, nil
	}
	if e := m.validNode(r); e != nil {
		return false, e
	}
	n := m.nodeAt(r.slot)
	if n.rc == 0 {
		return false, ErrASTDomain
	}
	n.rc--
	if n.rc != 0 {
		return true, nil
	}
	return true, m.deleteNode(r)
}
func (m *ASTManager) deleteNode(first astNodeRef) error {
	var inline [16]astNodeRef
	m.traceDeleteNodeBug(first)
	inline[0] = first
	return m.deleteNodeWorklist(inline[:1], astNodeRef{})
}

func (m *ASTManager) deleteNodeWorklist(stack []astNodeRef, resumed astNodeRef) error {
	var firstErr error
	parent, previousHead := m.deletionCurrent, m.deletionHead
	complete := false
	defer func() {
		if !complete {
			m.pendingDeletionCurrent = m.deletionCurrent
			m.pendingDeletionHead = m.deletionHead
		}
		m.deletionCurrent = parent
		m.deletionHead = previousHead
	}()
	note := func(e error) {
		if e != nil && firstErr == nil {
			firstErr = e
		}
	}
	push := func(r astNodeRef) {
		stack = append(stack, r)
		m.deletionHead = r
	}
	for resumed.slot != 0 || len(stack) != 0 {
		current := resumed
		resumeInfo := current.slot != 0
		resumed = astNodeRef{}
		if !resumeInfo {
			current = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		}
		head := astNodeRef{}
		if len(stack) != 0 {
			head = stack[len(stack)-1]
		}
		m.deletionHead = head
		n := m.nodeAt(current.slot)
		m.deletionCurrent = current
		if !resumeInfo {
			m.traceDeletingNode(current, n)
			if e := m.deletionBefore(current); e != nil {
				return e
			}
			if e := m.tableEraseIdentity(current); e != nil {
				return e
			}
			if e := m.deletionAfter(current); e != nil {
				return e
			}
			_, alreadyFreed := m.DebugFreeIndices[n.id]
			if ASTSourceDebug && m.DebugRefCount && alreadyFreed {
				return ErrASTDomain
			}
			n = m.nodeAt(current.slot)
			generator := &m.ExprIDs
			if n.kind == ASTSort || n.kind == ASTFuncDecl {
				generator = &m.declIDs
			}
			// Publication preflight normally makes this infallible. The source OOM
			// callback still controls its real recycle skip at this exact action site.
			if !m.DebugRefCount {
				note(generator.Recycle(n.id))
			}
			slot, generation := m.infoForNode(current)
			if slot != 0 && m.DebugRefCount {
				info, e := m.info(slot, generation)
				if e != nil {
					return e
				}
				// Actual source skips info->del_eh/dealloc at this branch. The Go
				// containing owner retains this exact allocation for explicit Close.
				info.retainedByDebug = true
			}
			if slot != 0 && !m.DebugRefCount {
				info, e := m.info(slot, generation)
				if e != nil {
					return e
				}
				for index := uint32(0); index < info.count; index++ {
					p, e := m.infoParameter(info, index)
					if e != nil {
						return e
					}
					v := m.parameterAt(p.slot)
					if v.Kind == ASTParameterAST {
						_, e = m.decNode(v.ast)
						note(e)
					}
					if v.Kind == ASTParameterExternal && info.family >= 0 && m.pluginAt(info.family).present {
						switch m.pluginAt(info.family).kind {
						case astFloatingPointPlugin:
							if m.floatingPointPlugin == nil || m.floatingPointPlugin.family != info.family {
								return ErrASTHandle
							}
							if e = m.floatingPointPlugin.recycledValueID(v.external); e != nil {
								return e
							}
						case astArithmeticPlugin:
							if m.arithmeticPlugin == nil || m.arithmeticPlugin.family != info.family || m.arithmeticPlugin.algebraic == nil {
								return ErrASTHandle
							}
							if e = m.arithmeticPlugin.algebraic.recycle(v.external); e != nil {
								return e
							}
						}
					}
					// The original eager plugins inherit default external del(),
					// which has no action. A cleared registry also has no callback.
				}
				info.attached = false
			}
		}
		// This stage is separate from del_eh because a native rational
		// destructor can refuse after earlier parameters have been consumed.
		slot, generation := m.infoForNode(current)
		if slot != 0 && !m.DebugRefCount {
			if e := m.dropInfoStorage(slot, generation); e != nil {
				m.suspendNodeDeletion(current, stack)
				return e
			}
		}
		n = m.nodeAt(current.slot)
		count := m.nodeChildCount(current)
		for index := uint32(0); index < count; index++ {
			child, e := m.nodeChild(current, index)
			if e != nil {
				return e
			}
			if child.slot != 0 {
				if e = m.validNode(child); e != nil {
					return e
				}
				cn := m.nodeAt(child.slot)
				if cn.rc == 0 {
					return ErrASTDomain
				}
				cn.rc--
				if cn.rc == 0 {
					push(child)
				}
			}
		}
		if m.DebugRefCount {
			m.DebugFreeIndices[n.id] = struct{}{}
		}
		if e := m.freeNodeStorage(current); e != nil {
			return e
		}
		m.deletionCurrent = parent
		m.deletionHead = astNodeRef{}
		if len(stack) != 0 {
			m.deletionHead = stack[len(stack)-1]
		}
	}
	complete = true
	return firstErr
}

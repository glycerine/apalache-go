package ast

// Source: src/ast/ast.cpp:1781-1844.
// Source delete_node ascending pushes/LIFO pops, using a native Go worklist.

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
	stack := inline[:0]
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
	m.traceDeleteNodeBug(first)
	push(first)
	for len(stack) != 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		head := astNodeRef{}
		if len(stack) != 0 {
			head = stack[len(stack)-1]
		}
		m.deletionHead = head
		n := m.nodeAt(current.slot)
		m.deletionCurrent = current
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
			if e = m.dropInfoStorage(slot, generation); e != nil {
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

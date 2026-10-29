package ast

// Source: src/ast/for_each_expr.h:27-105; src/ast/for_each_expr.cpp:23-66.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// The source lexical sbuffer has sixteen inline pair<expr*, unsigned> entries.
// This concrete representation has 12-byte pointer-free entries. Its actual
// lexical header/inline storage lives in the reflected manager header; only
// dynamic backing is separately charged. It does not allocate a vector owner.
type astExprPendingMark struct {
	next                             *astExprPendingMark
	node                             astNodeRef
	owner, invocation, bytes         uint64
	acknowledged                     bool
	bodyCause, sourceCause, ownCause error
}

type astExprVisitEntry struct {
	node  astNodeRef
	index uint32
}
type astExprVisitFrame struct {
	native                           []astExprVisitEntry
	visited                          map[ASTHandle]bool
	inline                           [16]astExprVisitEntry
	heap                             []astExprVisitEntry
	size                             uint32
	bytes                            uint64
	localFast                        astFastMarkState
	localBits                        astMarkBits
	fast                             *astFastMarkState
	expr                             *ASTExprMark
	pendingMark                      astNodeRef
	unlistedNode                     astNodeRef
	unlistedOwner                    uint64
	unlistedArchived                 bool
	pendingOwner                     uint64
	invocation                       uint64
	mode                             uint8
	active                           bool
	bodyCause, sourceCause, ownCause error
}

func (f *astExprVisitFrame) at(i uint32) *astExprVisitEntry {
	if f.mode == 4 {
		return &f.native[i]
	}
	if f.heap == nil {
		return &f.inline[i]
	}
	return &f.heap[i]
}
func (f *astExprVisitFrame) push(m *ASTManager, n astNodeRef) error {
	if f.mode == 4 {
		f.native = append(f.native[:f.size], astExprVisitEntry{node: n})
		f.size++
		return nil
	}
	cap := uint64(16)
	if f.heap != nil {
		cap = uint64(len(f.heap))
	}
	if uint64(f.size) == cap {
		nextCap := cap * 2
		bytes := nextCap * uint64(reflect.TypeOf(astExprVisitEntry{}).Size())
		if nextCap > uint64(^uint32(0)) || bytes > uint64(^uint(0)>>1) {
			return arena.ErrBounds
		}
		if e := m.BudgetField.Reserve(bytes); e != nil {
			return e
		}
		next := make([]astExprVisitEntry, int(nextCap))
		for i := uint32(0); i < f.size; i++ {
			next[i] = *f.at(i)
		}
		m.BudgetField.Release(f.bytes)
		f.heap = next
		f.bytes = bytes
	}
	*f.at(f.size) = astExprVisitEntry{node: n}
	f.size++
	return nil
}
func nextASTExprMarkIdentity() (uint64, error) {
	for {
		old := astMarkIdentity.Load()
		if old == ^uint64(0) {
			return 0, arena.ErrBounds
		}
		if astMarkIdentity.CompareAndSwap(old, old+1) {
			return old + 1, nil
		}
	}
}

// mark precedes stack/mark-trail append in the source. An append refusal keeps
// the exact unlisted mark token for a separate owning acknowledgment; it is not
// discovered by scanning mark bits or reference counts.
func (f *astExprVisitFrame) mark(m *ASTManager, n astNodeRef) (bool, error) {
	if e := m.validNode(n); e != nil {
		return false, e
	}
	if f.mode != 2 && m.nodeAt(n.slot).rc <= 1 {
		return false, nil
	}
	if f.mode == 4 {
		h := m.external(n)
		if f.visited[h] {
			return true, nil
		}
		f.visited[h] = true
		return false, nil
	}
	if f.mode == 2 || f.mode == 3 {
		bits := &f.localBits
		if f.expr != nil {
			bits = &f.expr.marks
		}
		id := m.nodeAt(n.slot).id
		yes, e := bits.marked(id)
		if e != nil || yes {
			return yes, e
		}
		return false, bits.mark(id, true)
	}
	s := f.fast
	yes, e := m.nodeMark(n, 1, s.owner.identity, 0, false)
	if e != nil || yes {
		return yes, e
	}
	if _, e = m.nodeMark(n, 1, s.owner.identity, 1, true); e != nil {
		return false, e
	}
	f.pendingMark = n
	f.pendingOwner = s.owner.identity
	if e = s.append(m, n); e != nil {
		f.unlistedNode = n
		f.unlistedOwner = s.owner.identity
		return false, e
	}
	f.pendingMark = astNodeRef{}
	f.pendingOwner = 0
	return false, nil
}
func (f *astExprVisitFrame) sourceFinish(m *ASTManager) error {
	// sbuffer is the younger lexical value; its element type is POD.
	m.BudgetField.Release(f.bytes)
	f.heap = nil
	f.native = nil
	f.visited = nil
	f.bytes = 0
	f.size = 0
	if f.mode == 0 {
		if e := f.localFast.reset(m); e != nil {
			if f.sourceCause == nil {
				f.sourceCause = e
			}
			return e
		}
		m.BudgetField.Release(f.localFast.heapBytes)
		f.localFast.heap = nil
		f.localFast.heapBytes = 0
		f.localFast.owner.finish()
	} else if f.mode == 3 {
		f.localBits.close()
	}
	if f.pendingMark.slot == 0 {
		f.active = false
		f.fast = nil
		f.expr = nil
	}
	return nil
}

// Own only the current manager-local lexical frame and its exact captured
// unlisted mark. A dead token is ended, never permission to touch a reused slot.
func (m *ASTManager) archiveExprVisitFrame() error {
	f := &m.exprVisit
	if !f.active && (f.unlistedNode.slot == 0 || f.unlistedArchived) {
		return nil
	}
	// Source-ended diagnostics acknowledged by an interrupted public Close
	// still belong to the original context until true context end. Preserve
	// them before any later source-valid call reuses this lexical storage.
	nodeToken, owner := f.pendingMark, f.pendingOwner
	acknowledged := !f.active
	if acknowledged {
		nodeToken = f.unlistedNode
		owner = f.unlistedOwner
	}
	if nodeToken.slot == 0 || f.mode == 0 && !f.localFast.owner.closed {
		return ErrASTPending
	}
	bytes := uint64(reflect.TypeOf(astExprPendingMark{}).Size())
	if e := m.BudgetField.Reserve(bytes); e != nil {
		return e
	}
	node := &astExprPendingMark{node: nodeToken, owner: owner, invocation: f.invocation, bytes: bytes, bodyCause: f.bodyCause, sourceCause: f.sourceCause, ownCause: f.ownCause, acknowledged: acknowledged, next: m.exprVisitHistory}
	m.exprVisitHistory = node
	f.unlistedArchived = true
	f.pendingMark = astNodeRef{}
	f.pendingOwner = 0
	f.active = false
	f.fast = nil
	f.expr = nil
	return nil
}
func (m *ASTManager) acknowledgeExprMark(node astNodeRef, owner uint64) error {
	if m.validNode(node) != nil {
		return nil
	} // ended exact generation, no replacement read
	n := m.nodeAt(node.slot)
	if !n.mark1 && n.markOwner1 == 0 {
		return nil
	} // actual earlier reset consumed action
	if n.markOwner1 != owner {
		return ErrASTDomain
	}
	_, e := m.nodeMark(node, 1, owner, 2, false)
	return e
}
func (m *ASTManager) cleanupExprVisitFrame() error {
	f := &m.exprVisit
	if f.active {
		if f.mode == 0 {
			s := &f.localFast
			for i := uint32(0); i < s.size; i++ {
				if e := m.acknowledgeExprMark(s.at(i), s.owner.identity); e != nil {
					if f.ownCause == nil {
						f.ownCause = e
					}
					return e
				}
			}
			s.size = 0
			m.BudgetField.Release(s.heapBytes)
			s.heap = nil
			s.heapBytes = 0
			if !s.owner.closed {
				s.owner.finish()
			}
		} else if f.mode == 3 {
			f.localBits.close()
		}
		if f.pendingMark.slot != 0 {
			if e := m.acknowledgeExprMark(f.pendingMark, f.pendingOwner); e != nil {
				if f.ownCause == nil {
					f.ownCause = e
				}
				return e
			}
			f.pendingMark = astNodeRef{}
			f.pendingOwner = 0
		}
		m.BudgetField.Release(f.bytes)
		f.heap = nil
		f.native = nil
		f.visited = nil
		f.bytes = 0
		f.size = 0
		f.fast = nil
		f.expr = nil
		f.active = false
	}
	for h := m.exprVisitHistory; h != nil; h = h.next {
		if h.acknowledged {
			continue
		}
		if e := m.acknowledgeExprMark(h.node, h.owner); e != nil {
			if h.ownCause == nil {
				h.ownCause = e
			}
			return e
		}
		h.acknowledged = true
	}
	return nil
}

// The younger mark action ACK never ends its enclosing AST owner. Original
// tuples/causes remain charged even if a later member destructor stops. Only
// the actual owning context storage/header teardown retires these receipts.
func (m *ASTManager) releaseExprVisitHistory() {
	for h := m.exprVisitHistory; h != nil; h = h.next {
		m.BudgetField.Release(h.bytes)
	}
	m.exprVisitHistory = nil
}
func (m *ASTManager) visitExprProc(n astNodeRef, skolem bool, total *uint32) (bool, error) {
	if e := m.validNode(n); e != nil {
		return false, e
	}
	switch m.nodeAt(n.slot).kind {
	case ASTVar, ASTQuantifier:
		if !skolem {
			*total++
		}
		return false, nil
	case ASTApp:
		a := m.appAt(n.slot)
		if !skolem {
			*total++
		}
		flags, e := m.funcFlags(a.decl)
		if e != nil {
			return false, e
		}
		if skolem {
			return flags.Skolem && a.count > 0, nil
		}
		if flags.LeftAssociative && flags.RightAssociative {
			*total += a.count - 2
		}
		return false, nil
	default:
		return false, ErrASTDomain
	}
}
func (m *ASTManager) visitExprCore(root astNodeRef, skolem bool) (uint32, bool, error) {
	f := &m.exprVisit
	var total uint32
	yes, e := f.mark(m, root)
	if e != nil || yes {
		return 0, false, e
	}
	if e = f.push(m, root); e != nil {
		return 0, false, e
	}
	for f.size > 0 {
		fr := f.at(f.size - 1)
		curr := fr.node
		if e = m.validNode(curr); e != nil {
			return total, false, e
		}
		switch m.nodeAt(curr.slot).kind {
		case ASTVar:
			found, e := m.visitExprProc(curr, skolem, &total)
			if e != nil || found {
				return total, found, e
			}
			f.size--
		case ASTApp:
			a := m.appAt(curr.slot)
			descended := false
			for fr.index < a.count {
				arg, e := m.readInternal(a.args, fr.index)
				if e != nil {
					return total, false, e
				}
				fr.index++
				yes, e := f.mark(m, arg)
				if e != nil {
					return total, false, e
				}
				if yes {
					continue
				}
				switch m.nodeAt(arg.slot).kind {
				case ASTVar:
					found, e := m.visitExprProc(arg, skolem, &total)
					if e != nil || found {
						return total, found, e
					}
				case ASTApp:
					if m.appAt(arg.slot).count == 0 {
						found, e := m.visitExprProc(arg, skolem, &total)
						if e != nil || found {
							return total, found, e
						}
					} else {
						if e = f.push(m, arg); e != nil {
							return total, false, e
						}
						descended = true
					}
				case ASTQuantifier:
					if e = f.push(m, arg); e != nil {
						return total, false, e
					}
					descended = true
				default:
					return total, false, ErrASTDomain
				}
				if descended {
					break
				}
			}
			if descended {
				continue
			}
			f.size--
			found, e := m.visitExprProc(curr, skolem, &total)
			if e != nil || found {
				return total, found, e
			}
		case ASTQuantifier:
			q := m.quantifierAt(curr.slot)
			count := 1 + q.patternCount + q.noPatternCount
			descended := false
			for fr.index < count {
				i := fr.index
				var child astNodeRef
				if i == 0 {
					child = q.body
				} else if i <= q.patternCount {
					child, e = m.readInternal(q.patterns, i-1)
				} else {
					child, e = m.readInternal(q.noPatterns, i-q.patternCount-1)
				}
				if e != nil {
					return total, false, e
				}
				fr.index++
				yes, e := f.mark(m, child)
				if e != nil {
					return total, false, e
				}
				if yes {
					continue
				}
				if e = f.push(m, child); e != nil {
					return total, false, e
				}
				descended = true
				break
			}
			if descended {
				continue
			}
			f.size--
			found, e := m.visitExprProc(curr, skolem, &total)
			if e != nil || found {
				return total, found, e
			}
		default:
			return total, false, ErrASTDomain
		}
	}
	return total, false, nil
}
func (m *ASTManager) runExprVisit(h ASTHandle, expr *ASTExprMark, fast *ASTFastMark1, visited map[ASTHandle]bool, skolem bool) (uint32, bool, error) {
	if e := m.lock(); e != nil {
		return 0, false, e
	}
	defer m.mu.Unlock()
	f := &m.exprVisit
	root, e := m.requireExpr(h)
	if e != nil {
		return 0, false, e
	}
	mode := uint8(0)
	if expr != nil {
		if e = expr.owner.check(); e != nil {
			return 0, false, e
		}
		if expr.owner.manager != m {
			return 0, false, ErrASTHandle
		}
		mode = 2
	}
	if fast != nil {
		if e = fast.owner.check(); e != nil {
			return 0, false, e
		}
		if fast.owner.manager != m {
			return 0, false, ErrASTHandle
		}
		mode = 1
	}
	if skolem {
		mode = 3
	}
	if visited != nil {
		mode = 4
	}
	// Source root-mark shortcut precedes lexical stack construction, even
	// with an older source-ended acquisition retained by this owner.
	if mode == 2 {
		yes, e := expr.marks.marked(m.nodeAt(root.slot).id)
		if e != nil || yes {
			return 0, false, e
		}
	} else if mode == 1 && m.nodeAt(root.slot).rc > 1 {
		yes, e := m.nodeMark(root, 1, fast.owner.identity, 0, false)
		if e != nil || yes {
			return 0, false, e
		}
	}
	if mode == 4 && m.nodeAt(root.slot).rc > 1 && visited[h] {
		return 0, false, nil
	}
	if f.invocation == ^uint64(0) {
		return 0, false, arena.ErrBounds
	}
	if e := m.archiveExprVisitFrame(); e != nil {
		return 0, false, e
	}
	serial := f.invocation + 1
	*f = astExprVisitFrame{mode: mode, invocation: serial, active: true, expr: expr, visited: visited}
	if mode == 0 {
		id, e := nextASTExprMarkIdentity()
		if e != nil {
			f.active = false
			return 0, false, e
		}
		f.localFast.plane = 1
		f.localFast.owner.bind(m, 0, id)
		f.fast = &f.localFast
	}
	if mode == 1 {
		f.fast = &fast.astFastMarkState
	}
	total, found, body := m.visitExprCore(root, skolem)
	f.bodyCause = body
	source := f.sourceFinish(m)
	return total, found, errors.Join(body, source)
}
func (m *ASTManager) GetNumExprs(h ASTHandle) (uint32, error) {
	n, _, e := m.runExprVisit(h, nil, nil, nil, false)
	return n, e
}
func (m *ASTManager) GetNumExprsWithExprMark(h ASTHandle, v *ASTExprMark) (uint32, error) {
	if v == nil {
		return 0, ErrASTHandle
	}
	n, _, e := m.runExprVisit(h, v, nil, nil, false)
	return n, e
}
func (m *ASTManager) GetNumExprsWithFastMark1(h ASTHandle, v *ASTFastMark1) (uint32, error) {
	if v == nil {
		return 0, ErrASTHandle
	}
	n, _, e := m.runExprVisit(h, nil, v, nil, false)
	return n, e
}
func (m *ASTManager) HasSkolemFunctions(h ASTHandle) (bool, error) {
	_, yes, e := m.runExprVisit(h, nil, nil, nil, true)
	return yes, e
}

// GetNumExprsWithVisited shares native membership across goal formulas. Only
// shared nodes (reference count > 1) are deduplicated, as in the source visitor.
func (m *ASTManager) GetNumExprsWithVisited(h ASTHandle, visited map[ASTHandle]bool) (uint32, error) {
	if visited == nil {
		return 0, ErrASTHandle
	}
	n, _, e := m.runExprVisit(h, nil, nil, visited, false)
	return n, e
}

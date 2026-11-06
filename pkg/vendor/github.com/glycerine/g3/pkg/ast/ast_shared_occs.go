package ast

import (
	"reflect"

	"github.com/glycerine/insdict"
)

type ASTSharedOccurrencesConfig struct{ TrackAtomic, VisitQuantifiers, VisitPatterns bool }

func DefaultASTSharedOccurrencesConfig() ASTSharedOccurrencesConfig {
	return ASTSharedOccurrencesConfig{VisitQuantifiers: true}
}

type astSharedFrame struct {
	node astNodeRef
	next uint32
}

type astSharedEntry struct {
	node  astNodeRef
	owned bool
}

// Entries own one reference per distinct expression. Native dictionary order
// makes iteration and destruction follow first discovery deterministically.
type ASTSharedOccurrences struct {
	owner        astMarkOwner
	entries      insdict.Dict[uint64, astSharedEntry]
	config       ASTSharedOccurrencesConfig
	stack        []astSharedFrame
	view         *astSharedView
	resetPending bool
}

func NewASTSharedOccurrences(m *ASTManager) (*ASTSharedOccurrences, error) {
	return NewASTSharedOccurrencesWithConfig(m, DefaultASTSharedOccurrencesConfig())
}
func NewASTSharedOccurrencesWithConfig(m *ASTManager, c ASTSharedOccurrencesConfig) (*ASTSharedOccurrences, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	bytes := uint64(reflect.TypeOf(ASTSharedOccurrences{}).Size())
	id, e := prepareASTMarkOwner(m, bytes)
	if e != nil {
		return nil, e
	}
	s := &ASTSharedOccurrences{config: c}
	s.owner.bind(m, bytes, id)
	return s, nil
}
func (s *ASTSharedOccurrences) push(r astNodeRef) {
	s.stack = append(s.stack, astSharedFrame{node: r})
}
func astSharedKey(r astNodeRef) uint64 { return uint64(r.slot)<<32 | uint64(r.generation) }
func (s *ASTSharedOccurrences) insert(m *ASTManager, r astNodeRef) error {
	key := astSharedKey(r)
	if _, found := s.entries.Get2(key); found {
		return nil
	}
	// Source insert_if_not_there_core publishes the set entry before INC.
	s.entries.Put(key, astSharedEntry{node: r})
	s.view = nil
	if e := m.incNode(r); e != nil {
		return e
	}
	s.entries.Set(key, astSharedEntry{node: r, owned: true})
	return nil
}
func (s *ASTSharedOccurrences) process(m *ASTManager, r astNodeRef, v *astSharedMarkState) (bool, error) {
	if e := m.validNode(r); e != nil {
		return false, e
	}
	n := m.nodeAt(r.slot)
	switch n.kind {
	case ASTApp:
		count := m.appAt(r.slot).count
		if n.rc > 1 && (s.config.TrackAtomic || count > 0) {
			if n.shared {
				return true, s.insert(m, r)
			}
			if e := v.mark(m, r, true); e != nil {
				return false, e
			}
		}
		if count == 0 {
			return true, nil
		}
		s.push(r)
		return false, nil
	case ASTVar:
		if s.config.TrackAtomic && n.rc > 1 {
			if n.shared {
				return true, s.insert(m, r)
			}
			return true, v.mark(m, r, true)
		}
		return true, nil
	case ASTQuantifier:
		if n.rc > 1 {
			if n.shared {
				return true, s.insert(m, r)
			}
			if e := v.mark(m, r, true); e != nil {
				return false, e
			}
		}
		if !s.config.VisitQuantifiers {
			return true, nil
		}
		s.push(r)
		return false, nil
	default:
		return false, ErrASTDomain
	}
}
func (s *ASTSharedOccurrences) collectLocked(m *ASTManager, r astNodeRef, v *astSharedMarkState) error {
	if ASTSourceDebug && len(s.stack) != 0 {
		return ErrASTDomain
	}
	done, e := s.process(m, r, v)
	if e != nil || done {
		return e
	}
	for len(s.stack) > 0 {
		idx := len(s.stack) - 1
		fr := s.stack[idx]
		if e = m.validNode(fr.node); e != nil {
			return e
		}
		n := m.nodeAt(fr.node.slot)
		var count uint32
		var child astNodeRef
		switch n.kind {
		case ASTApp:
			a := m.appAt(fr.node.slot)
			count = a.count
			if fr.next < count {
				child, e = m.readInternal(a.args, fr.next)
			}
		case ASTQuantifier:
			if ASTSourceDebug && !s.config.VisitQuantifiers {
				return ErrASTDomain
			}
			q := m.quantifierAt(fr.node.slot)
			count = 1
			if s.config.VisitPatterns {
				count += q.patternCount + q.noPatternCount
			}
			if fr.next < count {
				if fr.next == 0 {
					child = q.body
				} else if fr.next <= q.patternCount {
					child, e = m.readInternal(q.patterns, fr.next-1)
				} else {
					child, e = m.readInternal(q.noPatterns, fr.next-q.patternCount-1)
				}
			}
		default:
			return ErrASTDomain
		}
		if e != nil {
			return e
		}
		if fr.next == count {
			s.stack = s.stack[:idx]
			continue
		}
		s.stack[idx].next++
		if _, e = s.process(m, child, v); e != nil {
			return e
		}
	}
	return nil
}
func (s *ASTSharedOccurrences) resetLocked(m *ASTManager) error {
	s.resetPending = true
	s.view = nil
	for key, entry := range s.entries.All() {
		if !entry.owned {
			// The source set may retain this raw entry after INC refuses;
			// owning cleanup has no reference to decrement for it.
			s.entries.Del(key)
			continue
		}
		consumed, e := m.decNode(entry.node)
		if consumed {
			s.entries.Del(key)
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ErrASTDomain
		}
	}
	s.entries.Clear()
	s.resetPending = false
	return nil
}
func (s *ASTSharedOccurrences) Collect(h ASTHandle) (result error) {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	if ASTSourceDebug && len(s.stack) != 0 {
		return ErrASTDomain
	}
	var v astSharedMarkState
	defer func() {
		if e := v.close(m); result == nil {
			result = e
		}
	}()
	if e = s.resetLocked(m); e != nil {
		return e
	}
	r, e := m.internal(h)
	if e != nil {
		return e
	}
	return s.collectLocked(m, r, &v)
}
func (s *ASTSharedOccurrences) CollectWithMark(h ASTHandle, v *ASTSharedOccurrencesMark) error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e = v.owner.check(); e != nil {
		return e
	}
	if v.owner.manager != m {
		return ErrASTHandle
	}
	if s.resetPending {
		return ErrASTPending
	}
	r, e := m.internal(h)
	if e != nil {
		return e
	}
	return s.collectLocked(m, r, &v.state)
}
func (s *ASTSharedOccurrences) IsShared(h ASTHandle) (bool, error) {
	m, e := s.owner.lock()
	if e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	_, yes := s.entries.Get2(astSharedKey(r))
	return yes, nil
}
func (s *ASTSharedOccurrences) NumShared() uint32 { return uint32(s.entries.Len()) }
func (s *ASTSharedOccurrences) Reset() error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	return s.resetLocked(m)
}
func (s *ASTSharedOccurrences) Cleanup() error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e = s.resetLocked(m); e != nil {
		return e
	}
	s.stack = nil
	return nil
}
func (s *ASTSharedOccurrences) Close() error {
	if e := s.owner.check(); e != nil {
		return e
	}
	m := s.owner.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.Closed {
		if e := s.resetLocked(m); e != nil {
			return e
		}
	}
	s.stack = nil
	s.entries.Clear()
	s.view = nil
	s.owner.finish()
	return nil
}
